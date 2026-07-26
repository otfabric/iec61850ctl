// SPDX-License-Identifier: MIT

package http

import (
	"encoding/json"
	"net/http"

	"github.com/otfabric/iec61850ctl/internal/app"
)

// handleFindPath handles POST /api/find/path with JSON body.
func (s *Server) handleFindPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input app.FindPathInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	result, err := s.app.FindPath(input)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
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
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	result, err := s.app.BulkFind(input)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}
