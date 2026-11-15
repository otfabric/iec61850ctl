// SPDX-License-Identifier: MIT

package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/otfabric/iec61850ctl/internal/app"
)

// Bounds of the find endpoints, checked before any device request. A body
// over maxFindBodyBytes is answered with 413, a longer ln_pattern or more
// mappings with 400 (code invalid_input). SECURITY.md and docs/HTTP.md list
// them.
const (
	maxFindBodyBytes = 1 << 20 // request body of /api/find/path and /api/find/bulk
	maxLNPatternLen  = 256     // bytes of ln_pattern, the regular expression of /api/find/path
	maxBulkMappings  = 10000   // mappings of one /api/find/bulk request
)

// decodeFindBody decodes the JSON body of a find request, reading at most
// maxFindBodyBytes of it. It writes the error response and reports false
// when the body is too large or not valid JSON.
func (s *Server) decodeFindBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFindBodyBytes)
	err := json.NewDecoder(r.Body).Decode(v)
	if err == nil {
		return true
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		s.errorResponse(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body larger than %d bytes", maxFindBodyBytes))
		return false
	}
	s.errorResponse(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
	return false
}

// handleFindPath handles POST /api/find/path with JSON body.
func (s *Server) handleFindPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input app.FindPathInput
	if !s.decodeFindBody(w, r, &input) {
		return
	}
	if len(input.LNPattern) > maxLNPatternLen {
		s.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("ln_pattern is %d bytes, at most %d are accepted", len(input.LNPattern), maxLNPatternLen))
		return
	}

	result, err := s.app.FindPath(input)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

// handleBulkFind handles POST /api/find/bulk with JSON body.
func (s *Server) handleBulkFind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input app.BulkFindInput
	if !s.decodeFindBody(w, r, &input) {
		return
	}
	if len(input.Mappings) > maxBulkMappings {
		s.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("%d mappings, at most %d are accepted", len(input.Mappings), maxBulkMappings))
		return
	}

	result, err := s.app.BulkFind(input)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}
