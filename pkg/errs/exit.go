// SPDX-License-Identifier: MIT

package errs

import (
	"context"
	"errors"
	"io/fs"
	"net"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-mms"
)

// Process exit codes. They are part of the CLI contract (see README "Exit codes").
const (
	ExitOK          = 0   // success
	ExitFailure     = 1   // unclassified failure
	ExitUsage       = 2   // invalid flags, arguments or input values
	ExitConnection  = 3   // dial, association or connection loss
	ExitTimeout     = 4   // connect or request deadline exceeded
	ExitNotFound    = 5   // object, file, dataset, report or IED does not exist
	ExitRejected    = 6   // server rejected or denied the operation
	ExitMismatch    = 7   // write verification read-back mismatch
	ExitUnsupported = 8   // service not supported by the peer or platform
	ExitInterrupted = 130 // cancelled by SIGINT/SIGTERM (128 + SIGINT)
)

// ExitError forces a specific exit code for an error chain.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return "exit status"
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// WithExitCode wraps err so that ExitCode returns code. A nil err stays nil.
func WithExitCode(err error, code int) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: code, Err: err}
}

// ExitCode maps an error chain to a process exit code. Classification is by
// errors.Is/As against this package's sentinels and the go-iec61850, go-mms,
// context and net error types; error strings are never inspected.
// Order matters: the most specific classification wins.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	switch {
	case errors.Is(err, context.Canceled):
		return ExitInterrupted
	case IsTimeout(err):
		return ExitTimeout
	case errors.Is(err, ErrVerifyMismatch):
		return ExitMismatch
	case errors.Is(err, ErrInvalidInput),
		errors.Is(err, ErrInvalidReference),
		errors.Is(err, ErrInvalidConfig),
		errors.Is(err, iec61850.ErrInvalidReference),
		errors.Is(err, iec61850.ErrInvalidFunctionalConstraint),
		errors.Is(err, iec61850.ErrInvalidArgument),
		errors.Is(err, iec61850.ErrSCLParse),
		errors.Is(err, goose.ErrInvalidConfig),
		errors.Is(err, goose.ErrInvalidValue),
		errors.Is(err, goose.ErrSCL):
		return ExitUsage
	case errors.Is(err, ErrNotFound),
		errors.Is(err, iec61850.ErrNotFound),
		errors.Is(err, fs.ErrNotExist),
		errors.Is(err, goose.ErrNotFound),
		errors.Is(err, goose.ErrInterfaceNotFound):
		return ExitNotFound
	case errors.Is(err, ErrUnsupported),
		errors.Is(err, iec61850.ErrUnsupportedService),
		errors.Is(err, mms.ErrUnsupported),
		errors.Is(err, goose.ErrUnsupportedPlatform):
		return ExitUnsupported
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, ErrRejected),
		errors.Is(err, iec61850.ErrDataAccess),
		errors.Is(err, iec61850.ErrControlFailed),
		errors.Is(err, iec61850.ErrSelectFailed),
		errors.Is(err, iec61850.ErrOperateFailed),
		errors.Is(err, iec61850.ErrCancelFailed),
		errors.Is(err, iec61850.ErrNotControllable),
		errors.Is(err, mms.ErrDataAccess),
		errors.Is(err, mms.ErrServiceRejected),
		errors.Is(err, mms.ErrAuthenticationFailed),
		errors.Is(err, goose.ErrPermission):
		return ExitRejected
	case errors.Is(err, ErrConnectionFailed),
		errors.Is(err, iec61850.ErrClosed),
		errors.Is(err, mms.ErrClosed),
		errors.Is(err, mms.ErrConnectionRejected),
		errors.Is(err, mms.ErrAssociationFailed),
		errors.Is(err, mms.ErrNegotiationFailed),
		errors.Is(err, mms.ErrServerConnClosed),
		isNetError(err):
		return ExitConnection
	}
	return ExitFailure
}

// IsTimeout reports whether err is a deadline/timeout in any layer.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, mms.ErrInvokeTimeout) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func isNetError(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	var oe *net.OpError
	return errors.As(err, &oe)
}
