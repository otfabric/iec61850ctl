// SPDX-License-Identifier: MIT

// Package service provides business logic for IEC 61850 device exploration and data reading.
// This file re-exports the shared sentinel errors from pkg/errs so existing
// callers can keep using service.ErrX with errors.Is.
package service

import "github.com/otfabric/iec61850ctl/pkg/errs"

// Common error types that can be checked with errors.Is(). They are the same
// values as the pkg/errs sentinels.
var (
	// ErrNotFound indicates the requested resource (LD, LN, DO, DA, dataset, report, etc.) does not exist.
	ErrNotFound = errs.ErrNotFound

	// ErrConnectionFailed indicates the connection to the IEC 61850 server failed or was lost.
	ErrConnectionFailed = errs.ErrConnectionFailed

	// ErrInvalidInput indicates the provided input parameters are invalid or malformed.
	ErrInvalidInput = errs.ErrInvalidInput

	// ErrTimeout indicates an operation timed out waiting for a response.
	ErrTimeout = errs.ErrTimeout

	// ErrPermissionDenied indicates the server rejected the operation (e.g., setting RptEna when access is denied).
	ErrPermissionDenied = errs.ErrPermissionDenied

	// ErrInvalidReference indicates an object reference string is malformed or unparseable.
	ErrInvalidReference = errs.ErrInvalidReference

	// ErrInvalidConfig indicates a configuration struct failed validation.
	ErrInvalidConfig = errs.ErrInvalidConfig
)
