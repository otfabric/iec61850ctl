// SPDX-License-Identifier: MIT

package errs

import (
	"context"
	"errors"
	"io/fs"
	"net"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-iec61850/sv"
	"github.com/otfabric/go-mms"
)

// Process exit codes. They are part of the CLI contract (see docs/exitcodes.md).
const (
	ExitOK           = 0   // success
	ExitUsage        = 2   // invalid flags, arguments or input values, missing confirmation
	ExitInvalidInput = 3   // invalid reference, configuration or SCL
	ExitConnection   = 4   // dial, association or connection loss
	ExitTimeout      = 5   // connect or request deadline exceeded
	ExitRejected     = 6   // server rejected the operation
	ExitUnsupported  = 8   // service not supported by the peer or platform
	ExitPermission   = 9   // access denied, authentication failed, no raw-socket permission
	ExitFailure      = 10  // unclassified failure
	ExitNotFound     = 11  // object, file, dataset, report or IED does not exist
	ExitMismatch     = 12  // write verification read-back mismatch
	ExitInterrupted  = 130 // cancelled by SIGINT/SIGTERM (128 + SIGINT)
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
	case errors.Is(err, ErrInvalidInput):
		return ExitUsage
	case errors.Is(err, ErrInvalidReference),
		errors.Is(err, ErrInvalidConfig),
		errors.Is(err, iec61850.ErrInvalidReference),
		errors.Is(err, iec61850.ErrInvalidFunctionalConstraint),
		errors.Is(err, iec61850.ErrInvalidArgument),
		errors.Is(err, iec61850.ErrSCLParse),
		errors.Is(err, goose.ErrInvalidConfig),
		errors.Is(err, goose.ErrInvalidValue),
		errors.Is(err, goose.ErrSCL),
		errors.Is(err, sv.ErrInvalidConfig),
		errors.Is(err, sv.ErrInvalidValue),
		errors.Is(err, sv.ErrSCL),
		errors.Is(err, sv.ErrValidation):
		return ExitInvalidInput
	case errors.Is(err, ErrNotFound),
		IsObjectNonExistent(err),
		errors.Is(err, iec61850.ErrNotFound),
		errors.Is(err, fs.ErrNotExist),
		errors.Is(err, goose.ErrNotFound),
		errors.Is(err, goose.ErrInterfaceNotFound),
		errors.Is(err, sv.ErrNotFound):
		return ExitNotFound
	case errors.Is(err, ErrUnsupported),
		errors.Is(err, iec61850.ErrUnsupportedService),
		errors.Is(err, mms.ErrUnsupported),
		errors.Is(err, goose.ErrUnsupportedPlatform):
		return ExitUnsupported
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, mms.ErrAuthenticationFailed),
		errors.Is(err, goose.ErrPermission):
		return ExitPermission
	case errors.Is(err, ErrRejected),
		errors.Is(err, iec61850.ErrDataAccess),
		errors.Is(err, iec61850.ErrControlFailed),
		errors.Is(err, iec61850.ErrSelectFailed),
		errors.Is(err, iec61850.ErrOperateFailed),
		errors.Is(err, iec61850.ErrCancelFailed),
		errors.Is(err, iec61850.ErrNotControllable),
		errors.Is(err, mms.ErrDataAccess),
		errors.Is(err, mms.ErrServiceRejected):
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

// IsObjectNonExistent reports whether err carries the MMS
// object-non-existent result: the per-variable data-access error
// (go-mms or go-iec61850 bulk form) or the access-class service error.
// ExitCode maps it to ExitNotFound; every other data-access or service
// error stays ExitRejected.
func IsObjectNonExistent(err error) bool {
	var mdae *mms.DataAccessError
	if errors.As(err, &mdae) && mdae.Code == mms.DataAccessErrorObjectNonExistent {
		return true
	}
	var idae *iec61850.DataAccessError
	if errors.As(err, &idae) && idae.ErrorCode == int(mms.DataAccessErrorObjectNonExistent) {
		return true
	}
	var se *mms.ServiceError
	return errors.As(err, &se) && se.Class == mms.ErrorClassAccess && se.Code == accessErrObjectNonExistent
}

// accessErrObjectNonExistent is the ISO 9506-2 access-class service error
// code for object-non-existent.
const accessErrObjectNonExistent = 2

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
