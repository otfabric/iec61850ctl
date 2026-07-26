// SPDX-License-Identifier: MIT

package app

import (
	"io"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/service"
)

// SubscribeReportInput configures an RCB subscription run.
type SubscribeReportInput struct {
	ReportRef     string
	Duration      time.Duration
	MaxReports    int
	ShowValues    bool
	Interrogation bool
	Sync          bool
	DatasetRef    string
	Writer        io.Writer
	ErrWriter     io.Writer
	Format        string

	PurgeBuf bool
	EntryID  []byte
	ResvTms  *int32
}

// SubscribeReport runs a report subscription until duration/max/signal.
func (a *App) SubscribeReport(input SubscribeReportInput) (*service.Reporter, error) {
	cfg := service.ReporterConfig{
		ReportRef:     input.ReportRef,
		Duration:      input.Duration,
		MaxReports:    input.MaxReports,
		ShowValues:    input.ShowValues,
		Interrogation: input.Interrogation,
		Sync:          input.Sync,
		DatasetRef:    input.DatasetRef,
		Writer:        input.Writer,
		ErrWriter:     input.ErrWriter,
		Format:        input.Format,
		PurgeBuf:      input.PurgeBuf,
		EntryID:       input.EntryID,
		ResvTms:       input.ResvTms,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	reporter := service.NewReporter(a.conn).WithConfig(cfg)
	if err := reporter.Run(); err != nil {
		return nil, err
	}
	return reporter, nil
}
