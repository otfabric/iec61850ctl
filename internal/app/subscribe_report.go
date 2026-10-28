// SPDX-License-Identifier: MIT

package app

import (
	"context"
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
	Context       context.Context // cancels the subscription (nil = background)
	Writer        io.Writer
	ErrWriter     io.Writer
	Format        string
	KeepAlive     time.Duration // liveness probe interval (0 = off)

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
		Context:       input.Context,
		Writer:        input.Writer,
		ErrWriter:     input.ErrWriter,
		Format:        input.Format,
		KeepAlive:     input.KeepAlive,
		PurgeBuf:      input.PurgeBuf,
		EntryID:       input.EntryID,
		ResvTms:       input.ResvTms,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	reporter := service.NewReporter(a.conn).WithConfig(cfg)
	if err := reporter.Run(); err != nil {
		if reporter.Started() {
			return reporter, err // stream ran, then failed: stats are still meaningful
		}
		return nil, err
	}
	return reporter, nil
}
