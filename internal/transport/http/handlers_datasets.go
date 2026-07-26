// SPDX-License-Identifier: MIT

package http

import (
	"net/http"
	"strconv"

	"github.com/otfabric/iec61850ctl/internal/app"
)

// handleListDataSets handles GET /api/datasets?ld=<value>&ln=<value>.
func (s *Server) handleListDataSets(w http.ResponseWriter, r *http.Request) {
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

	datasets, err := s.app.ListDataSets(app.ListDataSetsInput{LD: ld, LN: ln})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"datasets": datasets,
	})
}

// handleListDataSetNames handles GET /api/datasets/names?ld=<value>&ln=<value>.
func (s *Server) handleListDataSetNames(w http.ResponseWriter, r *http.Request) {
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

	names, err := s.app.ListDataSetNames(app.ListDataSetsInput{LD: ld, LN: ln})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"names": names,
	})
}

// handleGetDataSet handles GET /api/datasets/details?ld=<value>&ln=<value>&name=<value>.
func (s *Server) handleGetDataSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	ln := r.URL.Query().Get("ln")
	name := r.URL.Query().Get("name")
	if ld == "" || ln == "" || name == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameters: ld, ln, name")
		return
	}

	dataset, err := s.app.GetDataSet(app.GetDataSetInput{LD: ld, LN: ln, Name: name})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, dataset)
}

// handleGetDataSetWithValues handles GET /api/datasets/with-values?ld=<value>&ln=<value>&name=<value>&read_values=<bool>.
func (s *Server) handleGetDataSetWithValues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	ln := r.URL.Query().Get("ln")
	name := r.URL.Query().Get("name")
	if ld == "" || ln == "" || name == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameters: ld, ln, name")
		return
	}

	readValues := r.URL.Query().Get("read_values") != "false" // default true

	dataset, err := s.app.GetDataSetWithValues(app.GetDataSetWithValuesInput{
		LD:         ld,
		LN:         ln,
		Name:       name,
		ReadValues: readValues,
	})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, dataset)
}

// handleListReports handles GET /api/reports?ld=<value>&ln=<value>.
func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
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

	unbuffered, buffered, err := s.app.ListReportNames(app.ListReportsInput{LD: ld, LN: ln})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"unbuffered": unbuffered,
		"buffered":   buffered,
	})
}

// handleListAllReports handles GET /api/reports/all.
func (s *Server) handleListAllReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	refs, err := s.app.ListAllReports()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"reports": refs,
	})
}

// handleListReportNames handles GET /api/reports/names?ld=<value>&ln=<value>.
func (s *Server) handleListReportNames(w http.ResponseWriter, r *http.Request) {
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

	unbuffered, buffered, err := s.app.ListReportNames(app.ListReportsInput{LD: ld, LN: ln})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"unbuffered": unbuffered,
		"buffered":   buffered,
	})
}

// handleGetReport handles GET /api/reports/details?ld=<value>&ln=<value>&name=<value>.
func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	ln := r.URL.Query().Get("ln")
	name := r.URL.Query().Get("name")
	if ld == "" || ln == "" || name == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameters: ld, ln, name")
		return
	}

	result, err := s.app.GetReport(app.GetReportInput{LD: ld, LN: ln, Name: name})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

// handleListJournals handles GET /api/journals?ld=<value>.
func (s *Server) handleListJournals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ld := r.URL.Query().Get("ld")
	if ld == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameter: ld")
		return
	}

	journals, err := s.app.ListJournals(app.ListJournalsInput{LD: ld})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"journals": journals,
	})
}

// handleGetJournalEntries handles GET /api/journals/entries?domain=<value>&journal=<value>&from=<ms>&to=<ms>.
func (s *Server) handleGetJournalEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	domain := r.URL.Query().Get("domain")
	journal := r.URL.Query().Get("journal")
	fromStr := r.URL.Query().Get("from")
	if domain == "" || journal == "" || fromStr == "" {
		s.errorResponse(w, http.StatusBadRequest, "Missing required parameters: domain, journal, from")
		return
	}

	fromMs, err := strconv.ParseUint(fromStr, 10, 64)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "Invalid from parameter (must be milliseconds since epoch)")
		return
	}

	var toMs *uint64
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		parsed, err := strconv.ParseUint(toStr, 10, 64)
		if err != nil {
			s.errorResponse(w, http.StatusBadRequest, "Invalid to parameter (must be milliseconds since epoch)")
			return
		}
		toMs = &parsed
	}

	result, err := s.app.GetJournalEntries(app.GetJournalEntriesInput{
		DomainID:    domain,
		JournalName: journal,
		FromMs:      fromMs,
		ToMs:        toMs,
	})
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}
