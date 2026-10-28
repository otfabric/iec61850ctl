// SPDX-License-Identifier: MIT

package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/scl"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/go-mms/transport/iso"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

var errDeviceDown = fmt.Errorf("device: %w", errs.ErrTimeout)

// failConn fails every browse/read call with errDeviceDown.
type failConn struct{ mockConn }

func (m *failConn) ListLogicalDevices(context.Context) ([]iec61850.LogicalDevice, error) {
	return nil, errDeviceDown
}
func (m *failConn) ListLogicalNodes(context.Context, string) ([]iec61850.LogicalNode, error) {
	return nil, errDeviceDown
}
func (m *failConn) ListDataObjects(context.Context, string, string) ([]iec61850.DataObject, error) {
	return nil, errDeviceDown
}
func (m *failConn) ListChildren(context.Context, iec61850.Ref) ([]iec61850.BrowseNode, error) {
	return nil, errDeviceDown
}
func (m *failConn) GetVariableType(context.Context, iec61850.Ref) (*mms.TypeSpec, error) {
	return nil, errDeviceDown
}
func (m *failConn) Read(context.Context, iec61850.Ref) (*iec61850.Value, error) {
	return nil, errDeviceDown
}
func (m *failConn) ListDataSets(context.Context, string) ([]string, error) { return nil, errDeviceDown }
func (m *failConn) GetDataSet(context.Context, string, string) (*iec61850.DataSet, error) {
	return nil, errDeviceDown
}
func (m *failConn) ReadDataSet(context.Context, string, string) ([]iec61850.DataSetValue, error) {
	return nil, errDeviceDown
}
func (m *failConn) ListReports(context.Context, string) ([]string, error) { return nil, errDeviceDown }
func (m *failConn) GetReportControlBlock(context.Context, string, string) (*iec61850.ReportControlBlock, error) {
	return nil, errDeviceDown
}
func (m *failConn) ListJournals(context.Context, string) ([]string, error) { return nil, errDeviceDown }
func (m *failConn) ReadJournal(context.Context, string, string, time.Time, time.Time) (*iec61850.JournalReadResult, error) {
	return nil, errDeviceDown
}

func TestApp_ErrorsPropagate(t *testing.T) {
	t.Parallel()
	br := true
	tests := []struct {
		name string
		call func(a *App) error
	}{
		{"BulkFind", func(a *App) error {
			_, err := a.BulkFind(BulkFindInput{Mappings: []service.BulkMappingEntry{{ControlledPropertyId: "x", BaseLn: "MMXU", DoDaPath: "TotW.mag.f"}}})
			return err
		}},
		{"FindPath", func(a *App) error { _, err := a.FindPath(FindPathInput{LNPattern: ".*", DOName: "TotW"}); return err }},
		{"GetDataSetWithValues", func(a *App) error {
			_, err := a.GetDataSetWithValues(GetDataSetWithValuesInput{LD: "LD0", LN: "LLN0", Name: "ds"})
			return err
		}},
		{"GetReport probe", func(a *App) error {
			_, err := a.GetReport(GetReportInput{LD: "LD0", LN: "LLN0", Name: "rcb"})
			return err
		}},
		{"GetReport buffered", func(a *App) error {
			_, err := a.GetReport(GetReportInput{LD: "LD0", LN: "LLN0", Name: "rcb", Buffered: &br})
			return err
		}},
		{"ListDataAttributes", func(a *App) error {
			_, err := a.ListDataAttributes(ListDataAttributesInput{LD: "LD0", LN: "MMXU1", DO: "TotW"})
			return err
		}},
		{"GetDataAttributes", func(a *App) error {
			_, err := a.GetDataAttributes(ListDataAttributesInput{LD: "LD0", LN: "MMXU1", DO: "TotW"})
			return err
		}},
		{"ListDataObjects", func(a *App) error {
			_, err := a.ListDataObjects(ListDataObjectsInput{LD: "LD0", LN: "MMXU1"})
			return err
		}},
		{"GetDataSet", func(a *App) error {
			_, err := a.GetDataSet(GetDataSetInput{LD: "LD0", LN: "LLN0", Name: "ds"})
			return err
		}},
		{"ListDataSets", func(a *App) error { _, err := a.ListDataSets(ListDataSetsInput{LD: "LD0", LN: "LLN0"}); return err }},
		{"ListJournals", func(a *App) error { _, err := a.ListJournals(ListJournalsInput{LD: "LD0"}); return err }},
		{"GetJournalEntries", func(a *App) error {
			_, err := a.GetJournalEntries(GetJournalEntriesInput{DomainID: "LD0", JournalName: "LLN0$EventLog", FromMs: 1})
			return err
		}},
		{"ListLogicalNodes", func(a *App) error { _, err := a.ListLogicalNodes(ListLogicalNodesInput{LD: "LD0"}); return err }},
		{"ListAllReports", func(a *App) error { _, err := a.ListAllReports(); return err }},
		{"ListReportNames", func(a *App) error {
			_, _, err := a.ListReportNames(ListReportsInput{LD: "LD0", LN: "LLN0"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.call(New(&failConn{})); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestApp_ErrorClassificationKept(t *testing.T) {
	t.Parallel()
	_, err := New(&failConn{}).GetDataAttributes(ListDataAttributesInput{LD: "LD0", LN: "MMXU1", DO: "TotW"})
	if !errors.Is(err, errs.ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout (device-down must not become empty result)", err)
	}
}

// partialConn lists datasets but fails per-dataset lookups and value reads
// (LN-qualified first, then bare-name fallback).
type partialConn struct {
	mockConn
	mu        sync.Mutex
	readCalls []string
}

func (m *partialConn) GetDataSet(ctx context.Context, ld, name string) (*iec61850.DataSet, error) {
	if strings.HasPrefix(name, "broken") || strings.Contains(name, "$broken") {
		return nil, errDeviceDown
	}
	return m.mockConn.GetDataSet(ctx, ld, name)
}

func (m *partialConn) ReadDataSet(ctx context.Context, ld, name string) ([]iec61850.DataSetValue, error) {
	m.mu.Lock()
	m.readCalls = append(m.readCalls, name)
	m.mu.Unlock()
	if strings.Contains(name, "$") {
		return nil, errDeviceDown
	}
	return m.mockConn.ReadDataSet(ctx, ld, name)
}

func TestApp_PartialFailuresTolerated(t *testing.T) {
	t.Parallel()
	conn := &partialConn{mockConn: mockConn{datasets: []string{"LLN0$Meas", "LLN0$broken"}}}
	a := New(conn)

	dss, err := a.ListDataSets(ListDataSetsInput{LD: "LD0", LN: "LLN0"})
	if err != nil || len(dss) != 1 {
		t.Fatalf("ListDataSets = %+v, %v (broken dataset must be skipped)", dss, err)
	}

	ds, err := a.GetDataSetWithValues(GetDataSetWithValuesInput{LD: "LD0", LN: "LLN0", Name: "Meas", ReadValues: true})
	if err != nil || ds == nil {
		t.Fatalf("GetDataSetWithValues: %v", err)
	}
	if len(conn.readCalls) < 2 || conn.readCalls[0] != "LLN0$Meas" || conn.readCalls[1] != "Meas" {
		t.Fatalf("read calls = %v (want LN-qualified then bare fallback)", conn.readCalls)
	}
	if len(ds.Members) == 0 || ds.Members[0].Value == "" {
		t.Fatalf("members = %+v", ds.Members)
	}
}

// startInteropServer starts an in-process IEC 61850 server for
// e2e/testdata/interop.icd and returns a connected client.
func startInteropServer(t *testing.T) *iec61850.Client {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
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

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *lockedBuf) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *lockedBuf) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

func TestApp_SubscribeReport_Integration(t *testing.T) {
	t.Parallel()

	t.Run("max reports", func(t *testing.T) {
		t.Parallel()
		client := startInteropServer(t)
		var out lockedBuf
		r, err := New(service.NewClientAdapter(client)).SubscribeReport(SubscribeReportInput{
			ReportRef:     "InteropLD/LLN0.RP.urcb01",
			Interrogation: true,
			MaxReports:    1,
			Duration:      10 * time.Second,
			Format:        "jsonl",
			Writer:        &out,
			ErrWriter:     &out,
		})
		if err != nil || r == nil || !r.Started() {
			t.Fatalf("SubscribeReport: r=%v err=%v\n%s", r, err, out.String())
		}
		if !strings.Contains(out.String(), `"reports_received":1`) {
			t.Fatalf("out:\n%s", out.String())
		}
	})

	t.Run("stream lost after start returns reporter and error", func(t *testing.T) {
		t.Parallel()
		client := startInteropServer(t)
		var out lockedBuf
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		time.AfterFunc(200*time.Millisecond, func() { _ = client.Close(context.Background()) })
		r, err := New(service.NewClientAdapter(client)).SubscribeReport(SubscribeReportInput{
			ReportRef: "InteropLD/LLN0.RP.urcb01",
			Context:   ctx,
			Writer:    &out,
		})
		if !errors.Is(err, errs.ErrConnectionFailed) {
			t.Fatalf("err = %v, want ErrConnectionFailed", err)
		}
		if r == nil || !r.Started() {
			t.Fatal("expected reporter for a started subscription")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		t.Parallel()
		_, err := New(&mockConn{}).SubscribeReport(SubscribeReportInput{ReportRef: "x", Format: "xml"})
		if !errors.Is(err, errs.ErrInvalidConfig) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestApp_ListLogicalNodes_DOCountTolerant(t *testing.T) {
	t.Parallel()
	conn := &lnOnlyConn{}
	lns, err := New(conn).ListLogicalNodes(ListLogicalNodesInput{LD: "LD0"})
	if err != nil || len(lns) == 0 {
		t.Fatalf("lns=%+v err=%v", lns, err)
	}
}

// lnOnlyConn lists LNs but fails DO listing.
type lnOnlyConn struct{ mockConn }

func (m *lnOnlyConn) ListDataObjects(context.Context, string, string) ([]iec61850.DataObject, error) {
	return nil, errDeviceDown
}

func (m *failConn) ReadJournalAfter(context.Context, string, string, time.Time, []byte) (*iec61850.JournalReadResult, error) {
	return nil, errDeviceDown
}
