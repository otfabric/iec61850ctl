// SPDX-License-Identifier: MIT

package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/scl"
	"github.com/otfabric/go-mms/transport/iso"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// startInteropServer starts an in-process IEC 61850 server backed by
// e2e/testdata/interop.icd (with the report engine enabled) and returns a
// connected client. Both are torn down via t.Cleanup.
func startInteropServer(t *testing.T) *iec61850.Client {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	icd := filepath.Join(filepath.Dir(thisFile), "..", "..", "e2e", "testdata", "interop.icd")
	sclData, err := scl.ParseFile(icd)
	if err != nil {
		t.Fatalf("parse ICD: %v", err)
	}
	model, err := iec61850.NewServerModelFromSCL(sclData, "InteropIED", "")
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	srv, err := iec61850.NewServer(model, iec61850.ServerOptions{
		Identity: &iec61850.ServerIdentity{Vendor: "test", Model: "test", Revision: "1"},
	})
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	srv.EnableReports()
	ln, err := iso.Listen("127.0.0.1:0")
	if err != nil {
		srv.Close()
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.ListenAndServe(ctx, ln)
	}()
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		srv.Close()
		<-done
	})

	dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dcancel()
	client, err := iec61850.Dial(dctx, ln.Addr().String(), iec61850.DialOptions{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	return client
}

// syncWriter serialises writes so tests can read output written by the
// reporter's reader goroutine without data races.
type syncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func jsonlEvents(t *testing.T, s string) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("decode jsonl line %q: %v", line, err)
		}
		out = append(out, ev)
	}
	return out
}

func lastSummary(t *testing.T, events []map[string]any) map[string]any {
	t.Helper()
	if len(events) == 0 || events[len(events)-1]["event"] != "summary" {
		t.Fatalf("expected trailing summary event, got %v", events)
	}
	return events[len(events)-1]
}

func TestReporterRun_Integration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        ReporterConfig
		cancelCtx  time.Duration // >0: cancel the context after this delay
		wantReport bool
		check      func(t *testing.T, out, diag string, r *Reporter)
	}{
		{
			name: "urcb gi max-reports text",
			cfg: ReporterConfig{
				ReportRef:     "InteropLD/LLN0.RP.urcb01",
				Interrogation: true,
				MaxReports:    1,
				ShowValues:    true,
				Duration:      10 * time.Second,
			},
			wantReport: true,
			check: func(t *testing.T, out, _ string, r *Reporter) {
				if !strings.Contains(out, "[Report #1]") || !strings.Contains(out, "Values:") {
					t.Fatalf("missing report output:\n%s", out)
				}
				if !strings.Contains(out, "GI triggered") || !strings.Contains(out, "Initial state received (GI)") {
					t.Fatalf("missing GI diagnostics:\n%s", out)
				}
				var stats bytes.Buffer
				r.WriteStats(&stats)
				if !strings.Contains(stats.String(), "Total reports: 1") {
					t.Fatalf("stats: %s", stats.String())
				}
			},
		},
		{
			name: "brcb gi max-reports jsonl with sync",
			cfg: ReporterConfig{
				ReportRef:     "InteropLD/LLN0.BR.brcb01",
				Interrogation: true,
				MaxReports:    1,
				ShowValues:    true,
				Sync:          true,
				DatasetRef:    "InteropLD/LLN0.dsInterop",
				PurgeBuf:      true,
				Format:        "jsonl",
				Duration:      10 * time.Second,
			},
			wantReport: true,
			check: func(t *testing.T, out, diag string, r *Reporter) {
				events := jsonlEvents(t, out)
				if events[0]["event"] != "baseline" {
					t.Fatalf("first event = %v, want baseline", events[0])
				}
				var report map[string]any
				for _, ev := range events {
					if ev["event"] == "report" {
						report = ev
					}
				}
				if report == nil || report["rpt_id"] != "interop_brcb01" {
					t.Fatalf("missing brcb report: %v", events)
				}
				sum := lastSummary(t, events)
				if sum["reports_received"] != float64(1) || sum["clean_disable"] != true {
					t.Fatalf("summary = %v", sum)
				}
				if !strings.Contains(diag, "BRCB PurgeBuf written") || !strings.Contains(diag, "Subscribing to") {
					t.Fatalf("diag:\n%s", diag)
				}
				var stats bytes.Buffer
				r.WriteStats(&stats)
				if stats.Len() != 0 {
					t.Fatalf("WriteStats must be a no-op for jsonl, got %q", stats.String())
				}
			},
		},
		{
			name: "duration elapses without reports",
			cfg: ReporterConfig{
				ReportRef: "InteropLD/LLN0.RP.urcb01",
				Duration:  150 * time.Millisecond,
				Format:    "jsonl",
			},
			check: func(t *testing.T, out, _ string, _ *Reporter) {
				sum := lastSummary(t, jsonlEvents(t, out))
				if _, ok := sum["reports_received"]; ok {
					t.Fatalf("expected zero reports, summary = %v", sum)
				}
				if sum["clean_disable"] != true {
					t.Fatalf("summary = %v", sum)
				}
			},
		},
		{
			name: "context cancellation stops run",
			cfg: ReporterConfig{
				ReportRef: "InteropLD/LLN0.RP.urcb01",
			},
			cancelCtx: 150 * time.Millisecond,
			check: func(t *testing.T, out, _ string, _ *Reporter) {
				if !strings.Contains(out, "Shutting down gracefully") {
					t.Fatalf("missing shutdown message:\n%s", out)
				}
			},
		},
		{
			name: "sync with unparsable dataset ref continues",
			cfg: ReporterConfig{
				ReportRef:  "InteropLD/LLN0.RP.urcb01",
				Sync:       true,
				DatasetRef: "not-a-dataset",
				Duration:   50 * time.Millisecond,
			},
			check: func(t *testing.T, out, _ string, _ *Reporter) {
				if !strings.Contains(out, "cannot parse dataset ref") {
					t.Fatalf("missing sync warning:\n%s", out)
				}
			},
		},
		{
			name: "text sync baseline",
			cfg: ReporterConfig{
				ReportRef:  "InteropLD/LLN0.RP.urcb01",
				Sync:       true,
				DatasetRef: "InteropLD/LLN0.dsInterop",
				Duration:   50 * time.Millisecond,
			},
			check: func(t *testing.T, out, _ string, _ *Reporter) {
				if !strings.Contains(out, "Baseline snapshot: 2 values") || !strings.Contains(out, "Baseline sync complete") {
					t.Fatalf("missing baseline:\n%s", out)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := startInteropServer(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if tt.cancelCtx > 0 {
				time.AfterFunc(tt.cancelCtx, cancel)
			}
			var out, diag syncWriter
			cfg := tt.cfg
			cfg.Context = ctx
			cfg.Writer = &out
			cfg.ErrWriter = &diag
			if err := cfg.Validate(); err != nil {
				t.Fatalf("validate: %v", err)
			}
			var hookCount int
			var hookMu sync.Mutex
			r := NewReporter(NewClientAdapter(client)).WithConfig(cfg).OnReport(func(*domain.Report) {
				hookMu.Lock()
				hookCount++
				hookMu.Unlock()
			})
			if err := r.Run(); err != nil {
				t.Fatalf("Run: %v\nout:\n%s\ndiag:\n%s", err, out.String(), diag.String())
			}
			if !r.Started() {
				t.Fatal("Started() = false after successful Run")
			}
			hookMu.Lock()
			got := hookCount
			hookMu.Unlock()
			if tt.wantReport && got == 0 {
				t.Fatalf("OnReport hook not called; out:\n%s", out.String())
			}
			tt.check(t, out.String(), diag.String(), r)
		})
	}
}

func TestReporterRun_StreamClosedByPeer(t *testing.T) {
	t.Parallel()
	client := startInteropServer(t)
	var out, diag syncWriter
	var once sync.Once
	r := NewReporter(NewClientAdapter(client)).WithConfig(ReporterConfig{
		ReportRef:     "InteropLD/LLN0.RP.urcb01",
		Interrogation: true,
		Format:        "jsonl",
		Duration:      15 * time.Second,
		Writer:        &out,
		ErrWriter:     &diag,
	}).OnReport(func(*domain.Report) {
		// Simulate connection loss after the first (GI) report.
		once.Do(func() { go func() { _ = client.Close(context.Background()) }() })
	})

	err := r.Run()
	if err == nil {
		t.Fatalf("expected error when report stream closes; out:\n%s", out.String())
	}
	if !errors.Is(err, errs.ErrConnectionFailed) || !errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("error %v does not wrap ErrConnectionFailed", err)
	}
	if !r.Started() {
		t.Fatal("Started() = false; subscription was established")
	}
	sum := lastSummary(t, jsonlEvents(t, out.String()))
	if sum["clean_disable"] != false {
		t.Fatalf("summary clean_disable = %v, want false", sum["clean_disable"])
	}
}

// subscribeErrConn wraps mockConnection to fail SubscribeReport.
type subscribeErrConn struct {
	mockConnection
	err error
}

func (m *subscribeErrConn) SubscribeReport(context.Context, string, iec61850.SubscribeReportOptions) (*iec61850.ReportSubscription, error) {
	return nil, m.err
}

func TestReporterRun_Errors(t *testing.T) {
	t.Parallel()
	resv := int32(5)
	tests := []struct {
		name    string
		conn    IEC61850Connection
		cfg     ReporterConfig
		wantErr error
		wantMsg string
	}{
		{
			name:    "invalid ref",
			conn:    &mockConnection{},
			cfg:     ReporterConfig{ReportRef: "bad"},
			wantErr: errs.ErrInvalidInput,
		},
		{
			name:    "invalid fc",
			conn:    &mockConnection{},
			cfg:     ReporterConfig{ReportRef: "LD0/LLN0.ST.rcb1"},
			wantErr: errs.ErrInvalidInput,
		},
		{
			name:    "get rcb error",
			conn:    &mockConnection{getRCBErr: errs.ErrNotFound},
			cfg:     ReporterConfig{ReportRef: "LD0/LLN0.RP.rcb1"},
			wantErr: errs.ErrNotFound,
			wantMsg: "get RCB values",
		},
		{
			name:    "empty rptid",
			conn:    &mockConnection{rcb: &iec61850.ReportControlBlock{}},
			cfg:     ReporterConfig{ReportRef: "LD0/LLN0.RP.rcb1"},
			wantMsg: "empty RptID",
		},
		{
			name:    "brcb options on urcb",
			conn:    &mockConnection{},
			cfg:     ReporterConfig{ReportRef: "LD0/LLN0.RP.rcb1", ResvTms: &resv},
			wantErr: ErrInvalidConfig,
		},
		{
			name:    "brcb pre-enable failure",
			conn:    &brcbSetMock{err: errs.ErrRejected},
			cfg:     ReporterConfig{ReportRef: "LD0/LLN0.BR.rcb1", PurgeBuf: true},
			wantErr: errs.ErrRejected,
			wantMsg: "BRCB pre-enable",
		},
		{
			name:    "subscribe failure",
			conn:    &subscribeErrConn{err: errs.ErrRejected},
			cfg:     ReporterConfig{ReportRef: "LD0/LLN0.BR.rcb1", EntryID: []byte{1}},
			wantErr: errs.ErrRejected,
			wantMsg: "subscribe report",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			cfg := tt.cfg
			cfg.Writer = &out
			cfg.ErrWriter = &out
			r := NewReporter(tt.conn).WithConfig(cfg)
			err := r.Run()
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want errors.Is %v", err, tt.wantErr)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("err = %v, want substring %q", err, tt.wantMsg)
			}
			if r.Started() {
				t.Fatal("Started() = true on pre-subscribe failure")
			}
		})
	}
}

func TestReporter_FluentSettersAndDefaults(t *testing.T) {
	t.Parallel()
	r := NewReporter(&mockConnection{}).WithConfig(ReporterConfig{}).
		WithDuration(time.Second).WithMaxReports(3).WithShowValues(true)
	if r.config.Writer == nil || r.config.ErrWriter == nil || r.config.Format != "text" {
		t.Fatalf("defaults not applied: %+v", r.config)
	}
	if r.config.Duration != time.Second || r.config.MaxReports != 3 || !r.config.ShowValues {
		t.Fatalf("setters not applied: %+v", r.config)
	}
	if r.diag() != r.config.Writer {
		t.Fatal("text diag must go to Writer")
	}
	var e bytes.Buffer
	r.config.ErrWriter = &e
	r.config.Format = "JSONL"
	if r.diag() != &e {
		t.Fatal("jsonl diag must go to ErrWriter")
	}
	if err := (ReporterConfig{ReportRef: "x", Format: "xml"}).Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("format validate: %v", err)
	}
}

// probeFailConn fails GetReportControlBlock after the first call, the way a
// dead association fails the keepalive probe.
type probeFailConn struct {
	IEC61850Connection
	calls atomic.Int32
}

func (c *probeFailConn) GetReportControlBlock(ctx context.Context, ld, rcb string) (*iec61850.ReportControlBlock, error) {
	if c.calls.Add(1) > 1 {
		return nil, errors.New("probe: broken pipe")
	}
	return c.IEC61850Connection.GetReportControlBlock(ctx, ld, rcb)
}

func TestReporterRun_KeepAliveDetectsLoss(t *testing.T) {
	t.Parallel()
	conn := &probeFailConn{IEC61850Connection: NewClientAdapter(startInteropServer(t))}
	var out syncWriter
	r := NewReporter(conn).WithConfig(ReporterConfig{
		ReportRef: "InteropLD/LLN0.RP.urcb01",
		Duration:  15 * time.Second,
		KeepAlive: 20 * time.Millisecond,
		Writer:    &out,
		ErrWriter: &out,
	})
	start := time.Now()
	err := r.Run()
	if !errors.Is(err, errs.ErrConnectionFailed) || !strings.Contains(err.Error(), "broken pipe") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("keepalive did not stop the run")
	}
	if err := (ReporterConfig{ReportRef: "x", KeepAlive: -1}).Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("negative keepalive: %v", err)
	}
}

func TestReporterRun_KeepAliveCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	var out syncWriter
	r := NewReporter(NewClientAdapter(startInteropServer(t))).WithConfig(ReporterConfig{
		ReportRef: "InteropLD/LLN0.RP.urcb01",
		KeepAlive: 10 * time.Millisecond,
		Context:   ctx,
		Writer:    &out,
		ErrWriter: &out,
	})
	time.AfterFunc(100*time.Millisecond, cancel)
	if err := r.Run(); err != nil {
		t.Fatalf("cancelled run: %v", err)
	}
}
