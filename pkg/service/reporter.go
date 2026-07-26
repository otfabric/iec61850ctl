// SPDX-License-Identifier: MIT

// Package services provides business logic for IEC 61850 device exploration and data reading.
// reporter.go manages report (RCB) subscription lifecycle: signal handling, cleanup, and statistics.
package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

const reportChanBuffer = 64

// ReporterConfig configures a report subscription run.
type ReporterConfig struct {
	ReportRef     string        // LD/LN.FC.reportName e.g. ZS1REF620A1LD0/LLN0.BR.rcbStatUrgB05
	Duration      time.Duration // 0 = run until signal
	MaxReports    int           // 0 = no limit
	ShowValues    bool          // print dataset values in each report
	Interrogation bool          // if true, trigger one GI after report is enabled
	Sync          bool          // if true, read report dataset once for baseline snapshot (DatasetRef must be set)
	DatasetRef    string        // dataset reference for --sync (e.g. from RCB DatSet attribute); empty skips sync
	Writer        io.Writer     // result stream (default os.Stdout)
	ErrWriter     io.Writer     // diagnostics when Format is jsonl (default os.Stderr)
	Format        string        // "text" (default) or "jsonl"
}

// Validate checks if the configuration is valid.
func (c ReporterConfig) Validate() error {
	if c.ReportRef == "" {
		return fmt.Errorf("%w: ReportRef is required", ErrInvalidConfig)
	}
	if c.Sync && c.DatasetRef == "" {
		return fmt.Errorf("%w: --sync requires DatasetRef to be set", ErrInvalidConfig)
	}
	if c.Duration < 0 {
		return fmt.Errorf("%w: Duration cannot be negative", ErrInvalidConfig)
	}
	if c.MaxReports < 0 {
		return fmt.Errorf("%w: MaxReports cannot be negative", ErrInvalidConfig)
	}
	switch strings.ToLower(strings.TrimSpace(c.Format)) {
	case "", "text", "jsonl":
	default:
		return fmt.Errorf("%w: Format must be text or jsonl", ErrInvalidConfig)
	}
	return nil
}

// Reporter handles subscribing to an RCB, receiving reports, and graceful shutdown with cleanup.
type Reporter struct {
	conn         IEC61850Connection
	config       ReporterConfig
	doneChan     chan struct{} // closed when we should stop (signal, duration, or max-reports)
	stopOnce     sync.Once
	count        atomic.Int64
	startTime    time.Time
	onReportHook func(*domain.Report) // custom callback with parsed domain.Report
}

// NewReporter creates a Reporter for the given connection.
// Configure with WithConfig or the With* fluent setters before Run.
func NewReporter(conn IEC61850Connection) *Reporter {
	return &Reporter{
		conn:     conn,
		doneChan: make(chan struct{}),
	}
}

// WithConfig sets the configuration for the reporter (fluent API).
func (r *Reporter) WithConfig(config ReporterConfig) *Reporter {
	r.config = config
	if r.config.Writer == nil {
		r.config.Writer = os.Stdout
	}
	if r.config.ErrWriter == nil {
		r.config.ErrWriter = os.Stderr
	}
	if r.config.Format == "" {
		r.config.Format = "text"
	}
	return r
}

func (r *Reporter) jsonl() bool {
	return strings.EqualFold(r.config.Format, "jsonl")
}

func (r *Reporter) diag() io.Writer {
	if r.jsonl() {
		return r.config.ErrWriter
	}
	return r.config.Writer
}

// WithReportRef sets the report reference (fluent API).
func (r *Reporter) WithReportRef(reportRef string) *Reporter {
	r.config.ReportRef = reportRef
	return r
}

// WithDuration sets the maximum duration (fluent API).
func (r *Reporter) WithDuration(d time.Duration) *Reporter {
	r.config.Duration = d
	return r
}

// WithMaxReports sets the maximum number of reports (fluent API).
func (r *Reporter) WithMaxReports(max int) *Reporter {
	r.config.MaxReports = max
	return r
}

// WithShowValues enables value display (fluent API).
func (r *Reporter) WithShowValues(show bool) *Reporter {
	r.config.ShowValues = show
	return r
}

// WithWriter sets the output writer (fluent API).
func (r *Reporter) WithWriter(w io.Writer) *Reporter {
	r.config.Writer = w
	return r
}

// OnReport sets a custom callback to be invoked for each received report (fluent API).
func (r *Reporter) OnReport(callback func(*domain.Report)) *Reporter {
	r.onReportHook = callback
	return r
}

// resolveReportRef parses LD/LN.FC.reportName into LD, RCB item ID, and buffered flag.
func resolveReportRef(reportRef string) (ld, rcbItemID string, buffered bool, err error) {
	ld, ln, fc, rcbName, ok := domain.ParseReportRef(reportRef)
	if !ok {
		return "", "", false, fmt.Errorf("invalid report reference %q", reportRef)
	}
	fcStr := string(fc)
	switch fcStr {
	case "BR":
		buffered = true
	case "RP":
		buffered = false
	default:
		return "", "", false, fmt.Errorf("report FC must be BR or RP, got %q", fcStr)
	}
	rcbItemID = fmt.Sprintf("%s$%s$%s", ln, fcStr, rcbName)
	return ld, rcbItemID, buffered, nil
}

// Run subscribes to the RCB, runs until duration/max-reports/signal, then closes the subscription.
func (r *Reporter) Run() error {
	ctx := context.Background()
	w := r.config.Writer
	r.startTime = time.Now()

	ld, rcbItemID, buffered, err := resolveReportRef(r.config.ReportRef)
	if err != nil {
		return err
	}

	rcb, err := r.conn.GetReportControlBlock(ctx, ld, rcbItemID)
	if err != nil {
		return fmt.Errorf("get RCB values: %w", err)
	}
	if rcb.RptID == "" {
		return fmt.Errorf("get RCB values: empty RptID")
	}

	opts := iec61850.SubscribeReportOptions{
		QueueSize:  reportChanBuffer,
		AutoEnable: true,
		LD:         ld,
		RCBItemID:  rcbItemID,
	}
	if !buffered {
		opts.ReserveURCB = true
	}
	if r.config.Interrogation {
		opts.GIOnSubscribe = true
	}

	sub, err := r.conn.SubscribeReport(ctx, rcb.RptID, opts)
	if err != nil {
		return fmt.Errorf("subscribe report: %w", err)
	}

	diag := r.diag()
	_, _ = fmt.Fprintf(diag, "Subscribing to: %s (RptID=%s)\n", r.config.ReportRef, rcb.RptID)
	_, _ = fmt.Fprintln(diag, "Report enabled. Waiting for reports... (Press Ctrl+C to stop)")
	if r.config.Interrogation {
		_, _ = fmt.Fprintln(diag, "GI triggered")
	}

	if r.config.Sync && r.config.DatasetRef != "" {
		r.runBaselineSync(ctx, w)
	}
	if !r.jsonl() {
		_, _ = fmt.Fprintln(w)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	if r.config.Duration > 0 {
		time.AfterFunc(r.config.Duration, func() {
			r.stopOnce.Do(func() { close(r.doneChan) })
		})
	}

	go r.runReportReader(sub.Reports())

	select {
	case <-sigChan:
		_, _ = fmt.Fprintln(diag, "\nShutting down gracefully...")
	case <-r.doneChan:
	}

	r.stopOnce.Do(func() { close(r.doneChan) })

	closeErr := sub.Close()
	clean := closeErr == nil
	if closeErr != nil {
		_, _ = fmt.Fprintf(diag, "warning: subscription close: %v\n", closeErr)
	}
	if r.jsonl() {
		_ = writeJSONLSummary(w, r.count.Load(), clean, time.Since(r.startTime))
	}
	return nil
}

func (r *Reporter) runBaselineSync(ctx context.Context, w io.Writer) {
	ref := r.config.DatasetRef
	diag := r.diag()
	_, _ = fmt.Fprintf(diag, "Reading dataset for baseline sync: %s\n", ref)

	ld, _, dsName, ok := domain.ParseDataSetRef(ref)
	if !ok {
		_, _ = fmt.Fprintf(diag, "Warning: baseline sync failed (cannot parse dataset ref %q); continuing subscription.\n", ref)
		return
	}

	values, err := r.conn.ReadDataSet(ctx, ld, dsName)
	if err != nil {
		_, _ = fmt.Fprintf(diag, "Warning: baseline sync failed (%v); continuing subscription.\n", err)
		return
	}

	if r.jsonl() {
		_ = writeJSONL(w, jsonlEvent{
			Event:   "baseline",
			DataSet: ref,
			Values:  baselineValuesJSON(values, r.config.ShowValues),
		})
		return
	}

	_, _ = fmt.Fprintf(w, "Baseline snapshot: %d values\n", len(values))
	for i, dv := range values {
		if dv.Err != nil {
			_, _ = fmt.Fprintf(w, "  [%d] (error: %v)\n", i+1, dv.Err)
			continue
		}
		if dv.Value == nil {
			_, _ = fmt.Fprintf(w, "  [%d] (nil)\n", i+1)
			continue
		}
		modelVal := domain.ValueFromMMS(dv.Value.MMS())
		valStr := "<nil>"
		if modelVal != nil {
			valStr = modelVal.String()
		}
		_, _ = fmt.Fprintf(w, "  [%d] %s\n", i+1, valStr)
	}
	_, _ = fmt.Fprintln(w, "Baseline sync complete")
}

func (r *Reporter) runReportReader(reports <-chan *iec61850.ReportIndication) {
	w := r.config.Writer
	maxReports := r.config.MaxReports
	showValues := r.config.ShowValues

	for {
		select {
		case <-r.doneChan:
			return
		case report, ok := <-reports:
			if !ok {
				return
			}
			r.printReport(w, report, showValues)
			n := r.count.Add(1)
			if n == 1 && r.config.Interrogation && !r.jsonl() {
				_, _ = fmt.Fprintln(r.diag(), "Initial state received (GI)")
			}
			if maxReports > 0 && int(n) >= maxReports {
				r.stopOnce.Do(func() { close(r.doneChan) })
				return
			}
		}
	}
}

func (r *Reporter) printReport(w io.Writer, report *iec61850.ReportIndication, showValues bool) {
	modelReport := domain.FromReportIndication(report, showValues)
	if modelReport == nil {
		return
	}

	if r.onReportHook != nil {
		r.onReportHook(modelReport)
	}

	seqNum := int64(0)
	if modelReport.SeqNum != nil {
		seqNum = int64(*modelReport.SeqNum)
	}

	if r.jsonl() {
		_ = writeJSONL(w, jsonlEvent{
			Event:          "report",
			RptID:          modelReport.RptID,
			SequenceNumber: &seqNum,
			DataSet:        modelReport.DatSet,
			Values:         reportValuesJSON(modelReport, showValues),
			Reasons:        reasonTokens(modelReport),
		})
		return
	}

	tsStr := "—"
	if modelReport.Timestamp != nil {
		tsStr = modelReport.Timestamp.String()
	}

	triggerStr := "—"
	if len(modelReport.Elements) > 0 && modelReport.Elements[0].Reason != "" {
		triggerStr = string(modelReport.Elements[0].Reason)
	}

	_, _ = fmt.Fprintf(w, "[Report #%d] %s\n", r.count.Load()+1, tsStr)
	_, _ = fmt.Fprintf(w, "  Seq: %d | Trigger: %s | DataSet: %s\n", seqNum, triggerStr, modelReport.DatSet)

	if showValues && len(modelReport.Elements) > 0 {
		_, _ = fmt.Fprintln(w, "  Values:")
		for _, elem := range modelReport.Elements {
			ref := elem.Ref
			if ref == "" {
				ref = fmt.Sprintf("[%d]", elem.Index+1)
			}
			valStr := "<nil>"
			if elem.Value != nil {
				valStr = elem.Value.String()
			}
			_, _ = fmt.Fprintf(w, "    [%d] %s: %s\n", elem.Index+1, ref, valStr)
		}
	}
	_, _ = fmt.Fprintln(w, "  --------------------------------------------------")
}

// WriteStats writes subscription statistics to w. Call after Run() returns.
// For jsonl format, the summary was already emitted at the end of Run.
func (r *Reporter) WriteStats(w io.Writer) {
	if r.jsonl() {
		return
	}
	if w == nil {
		w = os.Stdout
	}
	total := r.count.Load()
	dur := time.Since(r.startTime)
	rate := 0.0
	if dur.Seconds() > 0 {
		rate = float64(total) / dur.Seconds()
	}
	_, _ = fmt.Fprintln(w, "\nStatistics:")
	_, _ = fmt.Fprintf(w, "  Total reports: %d\n", total)
	_, _ = fmt.Fprintf(w, "  Duration: %s\n", dur.Round(time.Millisecond))
	_, _ = fmt.Fprintf(w, "  Average rate: %.2f reports/sec\n", rate)
}
