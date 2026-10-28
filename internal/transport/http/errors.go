// SPDX-License-Identifier: MIT

package http

import (
	"net/http"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// Stable machine-readable error codes returned in the "code" field of
// JSON error bodies. They are part of the REST contract.
const (
	codeInvalidInput     = "invalid_input"
	codeNotFound         = "not_found"
	codeRejected         = "rejected"
	codeUnsupported      = "unsupported"
	codeTimeout          = "timeout"
	codeConnection       = "connection_failed"
	codeInterrupted      = "interrupted"
	codeMethodNotAllowed = "method_not_allowed"
	codeInternal         = "internal"
)

// errorBody is the JSON body of every error response.
type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// statusForError maps an error chain to an HTTP status and a stable error
// code using errs.ExitCode (errors.Is/As classification, never strings).
func statusForError(err error) (int, string) {
	switch errs.ExitCode(err) {
	case errs.ExitUsage:
		return http.StatusBadRequest, codeInvalidInput
	case errs.ExitNotFound:
		return http.StatusNotFound, codeNotFound
	case errs.ExitRejected:
		return http.StatusForbidden, codeRejected
	case errs.ExitUnsupported:
		return http.StatusNotImplemented, codeUnsupported
	case errs.ExitTimeout:
		return http.StatusGatewayTimeout, codeTimeout
	case errs.ExitConnection:
		return http.StatusBadGateway, codeConnection
	case errs.ExitInterrupted:
		return http.StatusServiceUnavailable, codeInterrupted
	default:
		return http.StatusInternalServerError, codeInternal
	}
}

// codeForStatus returns the default error code for a transport-level status
// (request validation, method checks).
func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return codeInvalidInput
	case http.StatusMethodNotAllowed:
		return codeMethodNotAllowed
	case http.StatusNotFound:
		return codeNotFound
	default:
		return codeInternal
	}
}

// writeError writes an app/service error with the HTTP status derived from
// its classification.
func (s *Server) writeError(w http.ResponseWriter, err error) {
	status, code := statusForError(err)
	s.jsonResponse(w, status, errorBody{Error: err.Error(), Code: code})
}
