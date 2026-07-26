// SPDX-License-Identifier: MIT

// Package services provides business logic for IEC 61850 device exploration and data reading.
// This file defines common error types for better error handling in commands.
package service

import "errors"

// Common error types that can be checked with errors.Is().
var (
	// ErrNotFound indicates the requested resource (LD, LN, DO, DA, dataset, report, etc.) does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrConnectionFailed indicates the connection to the IEC 61850 server failed or was lost.
	ErrConnectionFailed = errors.New("connection failed")

	// ErrInvalidInput indicates the provided input parameters are invalid or malformed.
	ErrInvalidInput = errors.New("invalid input")

	// ErrTimeout indicates an operation timed out waiting for a response.
	ErrTimeout = errors.New("operation timed out")

	// ErrPermissionDenied indicates the server rejected the operation (e.g., setting RptEna when access is denied).
	ErrPermissionDenied = errors.New("permission denied")

	// ErrInvalidReference indicates an object reference string is malformed or unparseable.
	ErrInvalidReference = errors.New("invalid reference format")

	// ErrInvalidConfig indicates a configuration struct failed validation.
	ErrInvalidConfig = errors.New("invalid configuration")
)
