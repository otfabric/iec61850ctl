// SPDX-License-Identifier: MIT

// Package http provides HTTP/REST transport for IEC 61850 operations.
// It exposes the internal/app use cases as RESTful endpoints.
package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

// Server holds the HTTP server and app dependencies.
type Server struct {
	app        *app.App
	router     *http.ServeMux
	httpServer *http.Server
}

// Config contains server configuration.
type Config struct {
	ListenAddr string // HTTP listen address (e.g., ":8080")
	IECHost    string // IEC 61850 device host
	IECPort    int    // IEC 61850 device port
}

// NewServer creates a new HTTP server with the given configuration.
func NewServer(cfg Config) (*Server, error) {
	conn, err := client.NewConnection(client.ConnectionInput{
		Host:           cfg.IECHost,
		Port:           cfg.IECPort,
		ConnectTimeout: 10,
		RequestTimeout: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IEC 61850 device: %w", err)
	}

	a := app.New(conn)
	s := &Server{
		app:    a,
		router: http.NewServeMux(),
	}
	s.registerRoutes()

	s.httpServer = &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s, nil
}

// Start starts the HTTP server (blocking).
func (s *Server) Start() error {
	log.Printf("Starting HTTP server on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// registerRoutes sets up all HTTP endpoints.
func (s *Server) registerRoutes() {
	s.router.HandleFunc("/api/logical-devices", s.handleListLogicalDevices)
	s.router.HandleFunc("/api/logical-devices/names", s.handleListLogicalDeviceNames)
	s.router.HandleFunc("/api/logical-nodes", s.handleListLogicalNodes)
	s.router.HandleFunc("/api/logical-nodes/names", s.handleListLogicalNodeNames)
	s.router.HandleFunc("/api/data-objects", s.handleListDataObjects)
	s.router.HandleFunc("/api/data-objects/names", s.handleListDataObjectNames)
	s.router.HandleFunc("/api/data-attributes", s.handleListDataAttributes)
	s.router.HandleFunc("/api/datasets", s.handleListDataSets)
	s.router.HandleFunc("/api/datasets/names", s.handleListDataSetNames)
	s.router.HandleFunc("/api/datasets/details", s.handleGetDataSet)
	s.router.HandleFunc("/api/datasets/with-values", s.handleGetDataSetWithValues)
	s.router.HandleFunc("/api/reports", s.handleListReports)
	s.router.HandleFunc("/api/reports/all", s.handleListAllReports)
	s.router.HandleFunc("/api/reports/names", s.handleListReportNames)
	s.router.HandleFunc("/api/reports/details", s.handleGetReport)
	s.router.HandleFunc("/api/journals", s.handleListJournals)
	s.router.HandleFunc("/api/journals/entries", s.handleGetJournalEntries)
	s.router.HandleFunc("/api/find/path", s.handleFindPath)
	s.router.HandleFunc("/api/find/bulk", s.handleBulkFind)
	s.router.HandleFunc("/health", s.handleHealth)
}

// Helper methods for JSON responses.
func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func (s *Server) errorResponse(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, map[string]string{"error": message})
}

// Health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	s.jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}
