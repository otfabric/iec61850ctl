// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

const (
	sessionCloseTimeout = 2 * time.Second
	sessionAbortTimeout = 2 * time.Second
)

// clientSessionOptions configures dial timeouts and connection status output.
type clientSessionOptions struct {
	ConnectTimeout time.Duration
	RequestTimeout time.Duration
	Quiet          bool // when true, skip "Connecting to" status
}

// clientSession owns a live connection and performs bounded Close→Abort teardown.
type clientSession struct {
	conn   service.IEC61850Connection
	stderr io.Writer
}

// Conn returns the underlying service connection.
func (s *clientSession) Conn() service.IEC61850Connection {
	return s.conn
}

// Close performs a bounded Close; on failure or timeout it calls Abort.
// Errors are diagnostic-only (written to stderr when available).
func (s *clientSession) Close() {
	if s == nil || s.conn == nil {
		return
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), sessionCloseTimeout)
	err := s.conn.Close(closeCtx)
	cancel()
	if err == nil {
		return
	}
	if s.stderr != nil {
		_, _ = fmt.Fprintf(s.stderr, "warning: connection close: %v\n", err)
	}
	abortCtx, abortCancel := context.WithTimeout(context.Background(), sessionAbortTimeout)
	abortErr := s.conn.Abort(abortCtx)
	abortCancel()
	if abortErr != nil && s.stderr != nil {
		_, _ = fmt.Fprintf(s.stderr, "warning: connection abort: %v\n", abortErr)
	}
}

// CloseStrict is like Close but returns an error when cleanup fails.
// Used by commands that require confirmed teardown (e.g. report subscription).
func (s *clientSession) CloseStrict() error {
	if s == nil || s.conn == nil {
		return nil
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), sessionCloseTimeout)
	err := s.conn.Close(closeCtx)
	cancel()
	if err == nil {
		return nil
	}
	abortCtx, abortCancel := context.WithTimeout(context.Background(), sessionAbortTimeout)
	abortErr := s.conn.Abort(abortCtx)
	abortCancel()
	if abortErr != nil {
		return fmt.Errorf("close: %v; abort: %w", err, abortErr)
	}
	return fmt.Errorf("close: %w (aborted)", err)
}

// connectFunc dials a server; tests replace it to inject fake connections.
var connectFunc = client.NewConnection

// openClientSession resolves host/port/IED name from flags and env, dials, and
// returns a session. Callers must defer session.Close().
func openClientSession(cmd *cobra.Command, opts clientSessionOptions) (*clientSession, error) {
	h, p, err := getHostPort()
	if err != nil {
		return nil, err
	}
	return openClientSessionTo(cmd.Context(), cmd, h, p, opts)
}

// openClientSessionTo dials a specific host:port using the shared IED-name
// resolution and the global timeout/retry/logging flags. Non-zero opts
// timeouts override the global flags. Cancelling ctx aborts the dial and
// every in-flight request on the session.
func openClientSessionTo(ctx context.Context, cmd *cobra.Command, h string, p int, opts clientSessionOptions) (*clientSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	stderr := io.Writer(io.Discard)
	if cmd != nil {
		stderr = cmd.ErrOrStderr()
	}

	if !opts.Quiet {
		_, _ = fmt.Fprintf(stderr, "Connecting to %s\n", net.JoinHostPort(h, strconv.Itoa(p)))
	}

	connectTimeout := opts.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = connectTimeoutFlag
	}
	reqTimeout := opts.RequestTimeout
	if reqTimeout <= 0 {
		reqTimeout = requestTimeout()
	}

	conn, err := connectFunc(ctx, client.ConnectionInput{
		Host:           h,
		Port:           p,
		ConnectTimeout: connectTimeout,
		RequestTimeout: reqTimeout,
		IEDName:        getIEDName(),
		Retry:          client.RetryPolicy{Retries: retryFlag, MaxDelay: retryMaxDelayFlag},
		Logger:         cmdLogger(),
	})
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	return &clientSession{conn: conn, stderr: stderr}, nil
}
