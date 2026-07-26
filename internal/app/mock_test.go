// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"fmt"
	"io"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

type mockConn struct {
	lds          []string
	lns          []string
	dos          []string
	datasets     []string
	reports      []string
	journals     []string
	files        []iec61850.FileEntry
	rcb          *iec61850.ReportControlBlock
	ds           *iec61850.DataSet
	dsValues     []iec61850.DataSetValue
	readVal      *iec61850.Value
	subscribeErr error
	err          error

	// Optional control/write support for Phase 4 app tests.
	ctlModel    iec61850.CtlModel
	ctlModelSet bool
	allowWrite  bool
}

func (m *mockConn) ListLogicalDevices(_ context.Context) ([]iec61850.LogicalDevice, error) {
	if m.err != nil {
		return nil, m.err
	}
	names := m.lds
	if names == nil {
		names = []string{"LD0"}
	}
	out := make([]iec61850.LogicalDevice, len(names))
	for i, n := range names {
		out[i] = iec61850.LogicalDevice{Name: n}
	}
	return out, nil
}

func (m *mockConn) ListLogicalNodes(_ context.Context, _ string) ([]iec61850.LogicalNode, error) {
	if m.err != nil {
		return nil, m.err
	}
	names := m.lns
	if names == nil {
		names = []string{"LLN0", "MMXU1"}
	}
	out := make([]iec61850.LogicalNode, len(names))
	for i, n := range names {
		out[i] = iec61850.LogicalNode{Name: n}
	}
	return out, nil
}

func (m *mockConn) ListDataObjects(_ context.Context, _, _ string) ([]iec61850.DataObject, error) {
	names := m.dos
	if names == nil {
		names = []string{"Hz", "Mod"}
	}
	out := make([]iec61850.DataObject, len(names))
	for i, n := range names {
		out[i] = iec61850.DataObject{Name: n}
	}
	return out, nil
}

func (m *mockConn) ListChildren(_ context.Context, ref iec61850.Ref) ([]iec61850.BrowseNode, error) {
	c, _ := ref.Child("mag")
	return []iec61850.BrowseNode{{Name: "mag", Reference: c}}, nil
}

func (m *mockConn) TreeWithOptions(_ context.Context, _ iec61850.TreeOptions) (*iec61850.ModelNode, error) {
	return &iec61850.ModelNode{Name: "root"}, nil
}

func (m *mockConn) FindPaths(_ context.Context, _ iec61850.FindQuery) ([]iec61850.Ref, error) {
	ref, _ := iec61850.ParseRef("LD0/MMXU1.Hz")
	return []iec61850.Ref{ref}, nil
}

func (m *mockConn) Read(_ context.Context, _ iec61850.Ref) (*iec61850.Value, error) {
	if m.readVal != nil {
		return m.readVal, nil
	}
	return iec61850.NewValue(mms.NewFloat(50.0)), nil
}

func (m *mockConn) ReadMultiple(_ context.Context, _ []iec61850.Ref) ([]iec61850.ReadResult, error) {
	return nil, nil
}

func (m *mockConn) GetVariableType(_ context.Context, _ iec61850.Ref) (*mms.TypeSpec, error) {
	return &mms.TypeSpec{Type: mms.ValueTypeFloat}, nil
}

func (m *mockConn) ListDataSets(_ context.Context, _ string) ([]string, error) {
	if m.datasets != nil {
		return m.datasets, nil
	}
	return []string{"LLN0$Meas"}, nil
}

func (m *mockConn) GetDataSet(_ context.Context, _, _ string) (*iec61850.DataSet, error) {
	if m.ds != nil {
		return m.ds, nil
	}
	ref, _ := iec61850.ParseRef("LD0/MMXU1.Hz.mag.f")
	return &iec61850.DataSet{
		Reference: "LD0/LLN0.Meas",
		Members:   []iec61850.DataSetMember{{Ref: ref, DomainID: "LD0", ItemID: "MMXU1$MX$Hz$mag$f"}},
	}, nil
}

func (m *mockConn) ReadDataSet(_ context.Context, _, _ string) ([]iec61850.DataSetValue, error) {
	if m.dsValues != nil {
		return m.dsValues, nil
	}
	return []iec61850.DataSetValue{{
		Value: iec61850.NewValue(mms.NewFloat(50.0)),
	}}, nil
}

func (m *mockConn) ListReports(_ context.Context, _ string) ([]string, error) {
	if m.reports != nil {
		return m.reports, nil
	}
	return []string{"LLN0$BR$rcb1"}, nil
}

func (m *mockConn) GetReportControlBlock(_ context.Context, _, _ string) (*iec61850.ReportControlBlock, error) {
	if m.rcb != nil {
		return m.rcb, nil
	}
	return &iec61850.ReportControlBlock{RptID: "rpt1", DatSet: "LD0/LLN0$Meas"}, nil
}

func (m *mockConn) SetReportControlBlock(_ context.Context, _, _ string, _ iec61850.RCBUpdate) error {
	return nil
}
func (m *mockConn) TriggerGI(_ context.Context, _, _ string) error { return nil }
func (m *mockConn) SubscribeReport(_ context.Context, _ string, _ iec61850.SubscribeReportOptions) (*iec61850.ReportSubscription, error) {
	if m.subscribeErr != nil {
		return nil, m.subscribeErr
	}
	// Default: fail cleanly — a nil subscription would panic in Reporter.Run.
	return nil, fmt.Errorf("subscribe not available in tests")
}

func (m *mockConn) ListFiles(_ context.Context, _ string) ([]iec61850.FileEntry, error) {
	if m.files != nil {
		return m.files, nil
	}
	return []iec61850.FileEntry{{Name: "conf.xml", Size: 10}}, nil
}

func (m *mockConn) DownloadFile(_ context.Context, name string, w io.Writer) (*iec61850.FileEntry, error) {
	_, _ = w.Write([]byte("data"))
	return &iec61850.FileEntry{Name: name, Size: 4}, nil
}

func (m *mockConn) GetFileAttributes(_ context.Context, name string) (*iec61850.FileEntry, error) {
	return &iec61850.FileEntry{Name: name, Size: 4}, nil
}

func (m *mockConn) ListJournals(_ context.Context, _ string) ([]string, error) {
	if m.journals != nil {
		return m.journals, nil
	}
	return []string{"LLN0$EventLog"}, nil
}

func (m *mockConn) ReadJournal(_ context.Context, _, _ string, _, _ time.Time) (*iec61850.JournalReadResult, error) {
	return &iec61850.JournalReadResult{}, nil
}

func (m *mockConn) ReadJournalAfter(_ context.Context, _, _ string, _ time.Time, _ []byte) (*iec61850.JournalReadResult, error) {
	return &iec61850.JournalReadResult{}, nil
}

func (m *mockConn) Write(_ context.Context, _ iec61850.Ref, _ *mms.Value) error {
	if m.allowWrite {
		return nil
	}
	return fmt.Errorf("unexpected Write")
}
func (m *mockConn) ReadCtlModel(_ context.Context, _ iec61850.Ref) (iec61850.CtlModel, error) {
	if m.ctlModelSet {
		return m.ctlModel, nil
	}
	return 0, fmt.Errorf("unexpected ReadCtlModel")
}
func (m *mockConn) Select(_ context.Context, _ iec61850.Ref) (string, error) {
	if m.ctlModelSet {
		return "selected", nil
	}
	return "", fmt.Errorf("unexpected Select")
}
func (m *mockConn) SelectWithValue(_ context.Context, _ iec61850.Ref, _ iec61850.OperateParams) error {
	if m.ctlModelSet {
		return nil
	}
	return fmt.Errorf("unexpected SelectWithValue")
}
func (m *mockConn) Operate(_ context.Context, _ iec61850.Ref, _ iec61850.OperateParams) error {
	if m.ctlModelSet {
		return nil
	}
	return fmt.Errorf("unexpected Operate")
}
func (m *mockConn) Cancel(_ context.Context, _ iec61850.Ref, _ iec61850.CancelParams) error {
	if m.ctlModelSet {
		return nil
	}
	return fmt.Errorf("unexpected Cancel")
}
func (m *mockConn) ReadLastApplError(_ context.Context, _ iec61850.Ref) (*iec61850.LastApplError, error) {
	if m.ctlModelSet {
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected ReadLastApplError")
}

func (m *mockConn) Close(_ context.Context) error { return nil }
func (m *mockConn) Abort(_ context.Context) error { return nil }

var _ service.IEC61850Connection = (*mockConn)(nil)
