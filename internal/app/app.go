// SPDX-License-Identifier: MIT

// Package app provides application-level use cases for IEC 61850 operations.
// Each use case encapsulates service orchestration and domain-to-view projection,
// making the same logic reusable across transports (CLI, REST, gRPC).
//
// Use cases return view types and never expose domain internals or MMS library types.
package app

import (
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// App holds shared dependencies for all use cases.
type App struct {
	conn service.IEC61850Connection
}

// New creates a new App with the given IEC 61850 connection.
func New(conn service.IEC61850Connection) *App {
	return &App{conn: conn}
}

// Explorer returns a configured Explorer service.
func (a *App) Explorer() *service.Explorer {
	return service.NewExplorer(a.conn)
}

// DataSetService returns a configured DataSetService.
func (a *App) DataSetService() *service.DataSetService {
	return service.NewDataSetService(a.conn)
}

// ReportService returns a configured ReportService.
func (a *App) ReportService() *service.ReportService {
	return service.NewReportService(a.conn)
}

// JournalService returns a configured Journal service.
func (a *App) JournalService() *service.Journal {
	return service.NewJournal(a.conn)
}

// Reader returns a configured Reader service.
func (a *App) Reader() *service.Reader {
	return service.NewReader(a.conn)
}

// Controller returns a configured Controller service.
func (a *App) Controller() *service.Controller {
	return service.NewController(a.conn)
}

// Writer returns a configured Writer service.
func (a *App) Writer() *service.Writer {
	return service.NewWriter(a.conn)
}

// Connection returns the underlying IEC 61850 connection for operations
// that need direct protocol access (e.g., file transfer, raw reads).
func (a *App) Connection() service.IEC61850Connection {
	return a.conn
}
