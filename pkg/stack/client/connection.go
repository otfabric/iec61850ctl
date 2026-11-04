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

	// Addressing overrides the ISO addressing sent to the IED. The zero
	// value sends the go-mms defaults (T-selector 0001/0001, TPDU size
	// 1024, S-selector 0001, P-selector 00000001, AP-titles 1.1.1.999 /
	// 1.1.1.999.1, AE-qualifier 12), which IEDs expect.
	Addressing Addressing
}

// Addressing is the ISO addressing of a connection. A nil selector or
// AP-title keeps the go-mms default; a non-nil empty one omits the
// parameter.
type Addressing struct {
	LocalTSelector    []byte      // calling T-selector (COTP 0xC1)
	RemoteTSelector   []byte      // called T-selector (COTP 0xC2)
	RemoteSSelector   []byte      // called session selector
	RemotePSelector   []byte      // called presentation selector
	RemoteAPTitle     mms.APTitle // called AP-title
	RemoteAEQualifier *int        // called AE-qualifier; nil keeps the default
	MaxTPDULength     int         // COTP TPDU size; 0 keeps the default (1024)
}

// apply sets the addressing on opts.
func (a Addressing) apply(opts *iec61850.DialOptions) {
	opts.MMS.Transport.LocalTSelector = a.LocalTSelector
	opts.MMS.Transport.RemoteTSelector = a.RemoteTSelector
	opts.MMS.ISO.RemoteSSelector = a.RemoteSSelector
	opts.MMS.ISO.RemotePSelector = a.RemotePSelector
	opts.MMS.ISO.RemoteAPTitle = a.RemoteAPTitle
	if a.RemoteAEQualifier != nil {
		opts.MMS.ISO.RemoteAEQualifier = *a.RemoteAEQualifier
	}
	opts.MaxTPDULength = a.MaxTPDULength
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
	in.Addressing.apply(&opts)
	var c *iec61850.Client
	attempt := 0
	err := in.Retry.Do(ctx, logger, func(ctx context.Context) error {
		attempt++
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

// RetryPolicy configures connect retries. It maps onto go-mms's
// mms.RetryPolicy (exponential backoff, multiplier 2, 50% downward jitter)
// and mms.Retry, with this package's retry classification.
type RetryPolicy struct {
	// Retries is the number of additional attempts after the first (0 = no retry).
	Retries int
	// BaseDelay is the delay before the first retry; it doubles per attempt.
	// Zero uses DefaultRetryBaseDelay.
	BaseDelay time.Duration
	// MaxDelay caps the delay between attempts. Zero uses DefaultRetryMaxDelay.
	MaxDelay time.Duration
}

// MMS returns the equivalent go-mms policy. onRetry may be nil.
func (p RetryPolicy) MMS(onRetry func(attempt int, err error, delay time.Duration)) mms.RetryPolicy {
	base, maxDelay := p.BaseDelay, p.MaxDelay
	if base <= 0 {
		base = DefaultRetryBaseDelay
	}
	if maxDelay <= 0 {
		maxDelay = DefaultRetryMaxDelay
	}
	return mms.RetryPolicy{
		MaxAttempts:    max(p.Retries, 0) + 1,
		InitialBackoff: base,
		MaxBackoff:     maxDelay,
		Multiplier:     2,
		Jitter:         0.5,
		Retryable:      retryable,
		OnRetry:        onRetry,
	}
}

// Do runs fn under mms.Retry until it succeeds, returns a permanent error,
// ctx is done, or the retry budget is exhausted. When retries were allowed
// the returned error notes how many attempts were made.
func (p RetryPolicy) Do(ctx context.Context, logger *slog.Logger, fn func(ctx context.Context) error) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	pol := p.MMS(func(attempt int, err error, delay time.Duration) {
		logger.Warn("connect failed, retrying", "attempt", attempt, "of", max(p.Retries, 0)+1, "delay", delay, "err", err)
	})
	attempts := 0
	err := mms.Retry(ctx, pol, func(ctx context.Context) error {
		attempts++
		return fn(ctx)
	})
	if err == nil || pol.MaxAttempts == 1 {
		return err
	}
	if attempts == pol.MaxAttempts && ctx.Err() == nil {
		return fmt.Errorf("%w (gave up after %d attempt(s))", err, attempts)
	}
	return fmt.Errorf("%w (after %d attempt(s))", err, attempts)
}

// retryable reports whether a failed dial is worth another attempt: the
// go-mms classification plus this module's invalid-input sentinels.
func retryable(err error) bool {
	if errors.Is(err, iec61850.ErrInvalidArgument) || errors.Is(err, errs.ErrInvalidInput) {
		return false
	}
	return mms.IsRetryable(err)
}
