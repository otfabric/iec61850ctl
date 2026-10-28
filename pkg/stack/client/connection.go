// SPDX-License-Identifier: MIT

// Package client provides connection management for IEC 61850 MMS clients
// built on github.com/otfabric/go-iec61850: dialing with bounded retry and
// exponential backoff, per-request timeouts, cancellation and structured logging.
package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// Defaults applied when the corresponding ConnectionInput field is zero.
const (
	DefaultConnectTimeout = 10 * time.Second
	DefaultRequestTimeout = 10 * time.Second
	DefaultRetryBaseDelay = 250 * time.Millisecond
	DefaultRetryMaxDelay  = 5 * time.Second
)

// ConnectionInput defines parameters for creating an IEC 61850 client connection.
type ConnectionInput struct {
	Host string // Server hostname or IP address
	Port int    // Server port (typically 102)

	// ConnectTimeout bounds each dial attempt (TCP, COTP, ACSE, MMS initiate).
	// Zero uses DefaultConnectTimeout.
	ConnectTimeout time.Duration

	// RequestTimeout bounds every MMS request whose context has no deadline.
	// Zero uses DefaultRequestTimeout; a negative value disables the bound.
	// Report subscriptions and file downloads are never bounded by it.
	RequestTimeout time.Duration

	// IEDName is the optional IED name used for MMS domain normalisation.
	IEDName string

	// Retry controls reconnect attempts when dialing fails.
	Retry RetryPolicy

	// Logger receives structured go-iec61850/go-mms and retry diagnostics.
	// Nil disables logging.
	Logger *slog.Logger
}

// Addr returns the host:port dial address (IPv6-safe).
func (in ConnectionInput) Addr() string {
	return net.JoinHostPort(in.Host, strconv.Itoa(in.Port))
}

// dialFunc is the dial seam; tests replace it through newConnection.
type dialFunc func(ctx context.Context, addr string, opts iec61850.DialOptions) (*iec61850.Client, error)

// NewConnection dials an IEC 61850 MMS server and returns a service connection.
//
// Failed attempts are retried per in.Retry. Cancelling ctx aborts dialing and,
// for the lifetime of the returned connection, every in-flight request.
// Errors are classified with errs.ErrConnectionFailed (and are timeouts per
// errs.IsTimeout when the last attempt hit its deadline).
func NewConnection(ctx context.Context, in ConnectionInput) (service.IEC61850Connection, error) {
	return newConnection(ctx, in, iec61850.Dial)
}

func newConnection(ctx context.Context, in ConnectionInput, dial dialFunc) (service.IEC61850Connection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if in.Host == "" {
		return nil, errs.Invalidf("host is required")
	}
	if in.Port <= 0 || in.Port > 65535 {
		return nil, errs.Invalidf("port %d out of range 1..65535", in.Port)
	}
	connectTimeout := in.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = DefaultConnectTimeout
	}
	requestTimeout := in.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = DefaultRequestTimeout
	}
	logger := in.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	addr := in.Addr()
	opts := iec61850.DialOptions{IEDName: in.IEDName, Logger: in.Logger}
	var c *iec61850.Client
	err := in.Retry.Do(ctx, logger, func(attempt int) error {
		actx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		var derr error
		c, derr = dial(actx, addr, opts)
		if derr == nil && c == nil {
			derr = errors.New("dial returned nil client")
		}
		if derr != nil {
			logger.Debug("dial attempt failed", "addr", addr, "attempt", attempt, "err", derr)
		}
		return derr
	})
	if err != nil {
		return nil, errs.Mark(fmt.Errorf("dial %s: %w", addr, err), errs.ErrConnectionFailed)
	}
	logger.Info("connected", "addr", addr, "ied", in.IEDName)
	return &timeoutConn{IEC61850Connection: service.NewClientAdapter(c), base: ctx, timeout: requestTimeout}, nil
}

// RetryPolicy configures bounded exponential backoff with jitter.
type RetryPolicy struct {
	// Retries is the number of additional attempts after the first (0 = no retry).
	Retries int
	// BaseDelay is the delay before the first retry; it doubles per attempt.
	// Zero uses DefaultRetryBaseDelay.
	BaseDelay time.Duration
	// MaxDelay caps the delay between attempts. Zero uses DefaultRetryMaxDelay.
	MaxDelay time.Duration
	// sleep waits for d or until ctx is done; nil uses a timer. Test seam.
	sleep func(ctx context.Context, d time.Duration) error
	// jitter maps a backoff ceiling to the actual delay; nil uses equal jitter. Test seam.
	jitter func(d time.Duration) time.Duration
}

// Backoff returns the delay before retry number n (1-based), before jitter.
func (p RetryPolicy) Backoff(n int) time.Duration {
	base, maxDelay := p.BaseDelay, p.MaxDelay
	if base <= 0 {
		base = DefaultRetryBaseDelay
	}
	if maxDelay <= 0 {
		maxDelay = DefaultRetryMaxDelay
	}
	d := base
	for i := 1; i < n; i++ {
		if d >= maxDelay/2 {
			return maxDelay
		}
		d *= 2
	}
	return min(d, maxDelay)
}

// Do runs fn until it succeeds, returns a permanent error, ctx is done, or
// the retry budget is exhausted. The returned error is the last attempt's
// error annotated with the attempt count.
func (p RetryPolicy) Do(ctx context.Context, logger *slog.Logger, fn func(attempt int) error) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	sleep, jitter := p.sleep, p.jitter
	if sleep == nil {
		sleep = sleepCtx
	}
	if jitter == nil {
		jitter = equalJitter
	}
	total := max(p.Retries, 0) + 1
	var last error
	for attempt := 1; attempt <= total; attempt++ {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return fmt.Errorf("%w (after %d attempt(s), last error: %v)", err, attempt-1, last)
			}
			return err
		}
		last = fn(attempt)
		if last == nil {
			return nil
		}
		if attempt == total || !retryable(ctx, last) {
			break
		}
		d := jitter(p.Backoff(attempt))
		logger.Warn("connect failed, retrying", "attempt", attempt, "of", total, "delay", d, "err", last)
		if err := sleep(ctx, d); err != nil {
			return fmt.Errorf("%w (after %d attempt(s), last error: %v)", err, attempt, last)
		}
	}
	if total > 1 {
		return fmt.Errorf("%w (gave up after %d attempt(s))", last, total)
	}
	return last
}

// retryable reports whether a failed dial is worth another attempt.
// Caller cancellation, bad arguments and rejected credentials are permanent.
func retryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, iec61850.ErrInvalidArgument),
		errors.Is(err, errs.ErrInvalidInput),
		errors.Is(err, mms.ErrAuthenticationFailed):
		return false
	}
	return true
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
