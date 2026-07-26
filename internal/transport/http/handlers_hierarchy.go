// SPDX-License-Identifier: MIT

package http

import (
	"net/http"

	"github.com/otfabric/iec61850ctl/internal/app"
)

// handleListLogicalDevices handles GET /api/logical-devices.
func (s *Server) handleListLogicalDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	devices, err := s.app.ListLogicalDevices()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"logical_devices": devices,
	})
}

// handleListLogicalDeviceNames handles GET /api/logical-devices/names.
func (s *Server) handleListLogicalDeviceNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	names, err := s.app.ListLogicalDeviceNames()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"names": names,
	})
}

// handleListLogicalNodes handles GET /api/logical-nodes?ld=<value>.
func (s *Server) handleListLogicalNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	if ld == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameter: ld")
		return
	}

	nodes, err := s.app.ListLogicalNodes(app.ListLogicalNodesInput{LD: ld})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"logical_nodes": nodes,
	})
}

// handleListLogicalNodeNames handles GET /api/logical-nodes/names?ld=<value>.
func (s *Server) handleListLogicalNodeNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	if ld == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameter: ld")
		return
	}

	names, err := s.app.ListLogicalNodeNames(app.ListLogicalNodesInput{LD: ld})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"names": names,
	})
}

// handleListDataObjects handles GET /api/data-objects?ld=<value>&ln=<value>.
func (s *Server) handleListDataObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	ln := r.URL.Query().Get("ln")
	if ld == "" || ln == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameters: ld, ln")
		return
	}

	objects, err := s.app.ListDataObjects(app.ListDataObjectsInput{LD: ld, LN: ln})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"data_objects": objects,
	})
}

// handleListDataObjectNames handles GET /api/data-objects/names?ld=<value>&ln=<value>.
func (s *Server) handleListDataObjectNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	ln := r.URL.Query().Get("ln")
	if ld == "" || ln == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameters: ld, ln")
		return
	}

	names, err := s.app.ListDataObjectNames(app.ListDataObjectsInput{LD: ld, LN: ln})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"names": names,
	})
}

// handleListDataAttributes handles GET /api/data-attributes?ld=<value>&ln=<value>&do=<value>&detailed=<bool>.
func (s *Server) handleListDataAttributes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	ln := r.URL.Query().Get("ln")
	do := r.URL.Query().Get("do")
	if ld == "" || ln == "" || do == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameters: ld, ln, do")
		return
	}

	detailed := r.URL.Query().Get("detailed") == "true"

	attrs, err := s.app.ListDataAttributes(app.ListDataAttributesInput{
		LD:       ld,
		LN:       ln,
		DO:       do,
		Detailed: detailed,
	})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"data_attributes": attrs,
	})
}
