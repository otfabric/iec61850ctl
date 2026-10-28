// SPDX-License-Identifier: MIT

// Package http provides HTTP/REST transport for IEC 61850 operations.
// It exposes the internal/app use cases as RESTful endpoints.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/errs"
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
	ListenAddr string                 // HTTP listen address (e.g., ":8080")
	Connection client.ConnectionInput // IEC 61850 device connection (host, port, timeouts, retry, logger)
}

// NewServer connects to the IEC 61850 device and creates an HTTP server.
// Cancelling ctx aborts the connect and any in-flight device request.
func NewServer(ctx context.Context, cfg Config) (*Server, error) {
	conn, err := client.NewConnection(ctx, cfg.Connection)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IEC 61850 device: %w", err)
	}

	return NewServerWithApp(cfg.ListenAddr, app.New(conn)), nil
}

// NewServerWithApp creates an HTTP server backed by an existing App (for tests and custom wiring).
func NewServerWithApp(listenAddr string, a *app.App) *Server {
	s := &Server{
		app:    a,
		router: http.NewServeMux(),
	}
	s.registerRoutes()
	s.httpServer = &http.Server{
		Addr:         listenAddr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s
}

// Start starts the HTTP server (blocking).
func (s *Server) Start() error {
	return s.Serve(context.Background(), nil)
}

// Serve serves HTTP until ctx is done, then shuts down gracefully and closes
// the device connection. A nil ln listens on the configured address.
// It returns nil after a ctx-initiated shutdown.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", s.httpServer.Addr)
		if err != nil {
			return errs.Mark(fmt.Errorf("listen on %s: %w", s.httpServer.Addr, err), errs.ErrConnectionFailed)
		}
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.httpServer.Serve(ln) }()
	select {
	case err := <-errCh:
		s.closeConn()
		return err
	case <-ctx.Done():
	}
	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.httpServer.Shutdown(shCtx)
	if serveErr := <-errCh; err == nil && serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		err = serveErr
	}
	s.closeConn()
	return err
}

// Close closes the device connection without serving (used when startup fails).
func (s *Server) Close() { s.closeConn() }

func (s *Server) closeConn() {
	if s.app == nil || s.app.Connection() == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.app.Connection().Close(ctx); err != nil {
		_ = s.app.Connection().Abort(ctx)
	}
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
	s.jsonResponse(w, status, errorBody{Error: message, Code: codeForStatus(status)})
}

// Health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	s.jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}
