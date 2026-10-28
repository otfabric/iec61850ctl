// SPDX-License-Identifier: MIT

// Package errs defines the sentinel errors shared by every iec61850ctl layer
// and the mapping from an error chain to a stable process exit code.
//
// Lower layers classify failures by wrapping one of the sentinels (directly
// with %w or through [Mark]/[Invalidf]); the CLI maps the chain to an exit
// code with [ExitCode]. No layer inspects error strings.
package errs

import (
	"errors"
	"fmt"
)

// Sentinel errors. Test with [errors.Is].
var (
	// ErrInvalidInput indicates invalid command-line input or a malformed argument.
	ErrInvalidInput = errors.New("invalid input")

	// ErrInvalidReference indicates an object reference string is malformed or unparseable.
	ErrInvalidReference = errors.New("invalid reference format")

	// ErrInvalidConfig indicates a configuration struct failed validation.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrNotFound indicates the requested resource (LD, LN, DO, DA, dataset, report, IED, file) does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrConnectionFailed indicates the connection to the IEC 61850 server failed or was lost.
	ErrConnectionFailed = errors.New("connection failed")

	// ErrTimeout indicates an operation timed out waiting for a response.
	ErrTimeout = errors.New("operation timed out")

	// ErrPermissionDenied indicates the server denied access to an object or service.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrRejected indicates the server rejected an operation (write, control, RCB update).
	ErrRejected = errors.New("operation rejected")

	// ErrVerifyMismatch indicates a read-back after write did not match the requested value.
	ErrVerifyMismatch = errors.New("verification mismatch")

	// ErrUnsupported indicates the server, platform or this tool does not support the request.
	ErrUnsupported = errors.New("unsupported")
)

// marked carries a sentinel classification while keeping the original message.
type marked struct {
	sentinel error
	err      error
}

func (m *marked) Error() string   { return m.err.Error() }
func (m *marked) Unwrap() []error { return []error{m.sentinel, m.err} }

// Mark returns err classified as sentinel: errors.Is(Mark(err, s), s) is true,
// errors.Is/As still see err, and the message is err's message unchanged.
// Mark(nil, s) returns nil.
func Mark(err, sentinel error) error {
	if err == nil {
		return nil
	}
	if sentinel == nil || errors.Is(err, sentinel) {
		return err
	}
	return &marked{sentinel: sentinel, err: err}
}

// Newf formats a message and classifies it as sentinel. The message does not
// repeat the sentinel text.
func Newf(sentinel error, format string, args ...any) error {
	return Mark(fmt.Errorf(format, args...), sentinel)
}

// Invalidf is shorthand for Newf(ErrInvalidInput, ...).
func Invalidf(format string, args ...any) error {
	return Newf(ErrInvalidInput, format, args...)
}
