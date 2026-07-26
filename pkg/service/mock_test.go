// SPDX-License-Identifier: MIT

// Package services provides business logic for IEC 61850 device exploration and data reading.
// This file contains Phase 4 tests: mocking, generics, batch API, formatter service.

package service

import (
	"context"
	"io"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

// mockConnection is a mock implementation of IEC61850Connection for testing.
type mockConnection struct {
	logicalDevices []string
	logicalNodes   []string
	dataObjects    []string
	dataSets       []string
	reports        []string
	journals       []string

	readFunc            func(ref iec61850.Ref) (*iec61850.Value, error)
	getVariableTypeFunc func(ref iec61850.Ref) (*mms.TypeSpec, error)
	listChildrenFunc    func(ref iec61850.Ref) ([]iec61850.BrowseNode, error)

	dataSet               *iec61850.DataSet
	dataSetValues         []iec61850.DataSetValue
	getDataSetErr         error
	readDataSetErr        error
	listDataSetsErr       error
	listReportsErr        error
	rcb                   *iec61850.ReportControlBlock
	getRCBErr             error
	getRCBByItemID        map[string]*iec61850.ReportControlBlock
	getRCBErrByItemID     map[string]error
	listJournalsErr       error
	journalResult         *iec61850.JournalReadResult
	journalAfterResult    *iec61850.JournalReadResult
	readJournalErr        error
	readJournalAfterErr   error
	readJournalCalls      int
	readJournalAfterCalls int
}

func (m *mockConnection) ListLogicalDevices(_ context.Context) ([]iec61850.LogicalDevice, error) {
	if m.logicalDevices == nil {
		return []iec61850.LogicalDevice{{Name: "LD0"}, {Name: "LD1"}}, nil
	}
	out := make([]iec61850.LogicalDevice, len(m.logicalDevices))
	for i, name := range m.logicalDevices {
		out[i] = iec61850.LogicalDevice{Name: name}
	}
	return out, nil
}

func (m *mockConnection) ListLogicalNodes(_ context.Context, _ string) ([]iec61850.LogicalNode, error) {
	if m.logicalNodes != nil {
		out := make([]iec61850.LogicalNode, len(m.logicalNodes))
		for i, name := range m.logicalNodes {
			out[i] = iec61850.LogicalNode{Name: name}
		}
		return out, nil
	}
	return []iec61850.LogicalNode{{Name: "LLN0"}, {Name: "MMXU1"}}, nil
}

func (m *mockConnection) ListDataObjects(_ context.Context, _, _ string) ([]iec61850.DataObject, error) {
	if m.dataObjects != nil {
		out := make([]iec61850.DataObject, len(m.dataObjects))
		for i, name := range m.dataObjects {
			out[i] = iec61850.DataObject{Name: name}
		}
		return out, nil
	}
	return []iec61850.DataObject{{Name: "DO1"}, {Name: "DO2"}}, nil
}

func (m *mockConnection) ListChildren(_ context.Context, ref iec61850.Ref) ([]iec61850.BrowseNode, error) {
	if m.listChildrenFunc != nil {
		return m.listChildrenFunc(ref)
	}
	magRef, _ := ref.Child("mag")
	qRef, _ := ref.Child("q")
	return []iec61850.BrowseNode{
		{Name: "mag", Reference: magRef},
		{Name: "q", Reference: qRef},
	}, nil
}

func (m *mockConnection) TreeWithOptions(_ context.Context, _ iec61850.TreeOptions) (*iec61850.ModelNode, error) {
	return &iec61850.ModelNode{Name: "root"}, nil
}

func (m *mockConnection) FindPaths(_ context.Context, _ iec61850.FindQuery) ([]iec61850.Ref, error) {
	return nil, nil
}

func (m *mockConnection) Read(_ context.Context, ref iec61850.Ref) (*iec61850.Value, error) {
	if m.readFunc != nil {
		return m.readFunc(ref)
	}
	return iec61850.NewValue(mms.NewFloat(50.01)), nil
}

func (m *mockConnection) ReadMultiple(_ context.Context, _ []iec61850.Ref) ([]iec61850.ReadResult, error) {
	return nil, nil
}

func (m *mockConnection) GetVariableType(_ context.Context, ref iec61850.Ref) (*mms.TypeSpec, error) {
	if m.getVariableTypeFunc != nil {
		return m.getVariableTypeFunc(ref)
	}
	// DO-level refs (single path component) behave as structures so
	// Explorer/BulkFind can walk into mag/q children by default.
	if len(ref.Path) <= 1 {
		return &mms.TypeSpec{Type: mms.ValueTypeStructure}, nil
	}
	return &mms.TypeSpec{Type: mms.ValueTypeFloat}, nil
}

func (m *mockConnection) ListDataSets(_ context.Context, _ string) ([]string, error) {
	if m.listDataSetsErr != nil {
		return nil, m.listDataSetsErr
	}
	if m.dataSets != nil {
		return append([]string(nil), m.dataSets...), nil
	}
	return []string{"LLN0$ds1", "LLN0$ds2", "MMXU1$meas"}, nil
}

func (m *mockConnection) GetDataSet(_ context.Context, _, dsName string) (*iec61850.DataSet, error) {
	if m.getDataSetErr != nil {
		return nil, m.getDataSetErr
	}
	if m.dataSet != nil {
		return m.dataSet, nil
	}
	ref, _ := iec61850.ParseRef("LD0/LLN0.DO1.mag.f")
	return &iec61850.DataSet{
		Reference: "LD0/LLN0." + dsName,
		Deletable: false,
		Members: []iec61850.DataSetMember{
			{Ref: ref, DomainID: "LD0", ItemID: "LLN0$MX$DO1$mag$f"},
		},
	}, nil
}

func (m *mockConnection) ReadDataSet(_ context.Context, _, _ string) ([]iec61850.DataSetValue, error) {
	if m.readDataSetErr != nil {
		return nil, m.readDataSetErr
	}
	if m.dataSetValues != nil {
		return m.dataSetValues, nil
	}
	return []iec61850.DataSetValue{
		{Value: iec61850.NewValue(mms.NewFloat(12.5))},
	}, nil
}

func (m *mockConnection) ListReports(_ context.Context, _ string) ([]string, error) {
	if m.listReportsErr != nil {
		return nil, m.listReportsErr
	}
	if m.reports != nil {
		return append([]string(nil), m.reports...), nil
	}
	return []string{"LLN0$BR$rcb1", "LLN0$RP$urcb1", "MMXU1$BR$brcb01"}, nil
}

func (m *mockConnection) GetReportControlBlock(_ context.Context, _, itemID string) (*iec61850.ReportControlBlock, error) {
	if m.getRCBErrByItemID != nil {
		if err, ok := m.getRCBErrByItemID[itemID]; ok {
			return nil, err
		}
	}
	if m.getRCBByItemID != nil {
		if rcb, ok := m.getRCBByItemID[itemID]; ok {
			return rcb, nil
		}
	}
	if m.getRCBErr != nil {
		return nil, m.getRCBErr
	}
	if m.rcb != nil {
		return m.rcb, nil
	}
	return &iec61850.ReportControlBlock{
		RptID:   "test-rpt",
		DatSet:  "LD0/LLN0$ds1",
		RptEna:  true,
		ConfRev: 1,
		BufTm:   100,
		SqNum:   7,
		IntgPd:  1000,
		Resv:    true,
		TrgOps:  iec61850.TrgOpDataChanged | iec61850.TrgOpGI,
		OptFlds: iec61850.OptFldSeqNum | iec61850.OptFldTimeStamp,
	}, nil
}

func (m *mockConnection) SetReportControlBlock(_ context.Context, _, _ string, _ iec61850.RCBUpdate) error {
	return nil
}

func (m *mockConnection) TriggerGI(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockConnection) SubscribeReport(_ context.Context, _ string, _ iec61850.SubscribeReportOptions) (*iec61850.ReportSubscription, error) {
	return nil, nil
}

func (m *mockConnection) ListFiles(_ context.Context, _ string) ([]iec61850.FileEntry, error) {
	return nil, nil
}

func (m *mockConnection) DownloadFile(_ context.Context, _ string, _ io.Writer) (*iec61850.FileEntry, error) {
	return nil, nil
}

func (m *mockConnection) GetFileAttributes(_ context.Context, _ string) (*iec61850.FileEntry, error) {
	return nil, nil
}

func (m *mockConnection) ListJournals(_ context.Context, _ string) ([]string, error) {
	if m.listJournalsErr != nil {
		return nil, m.listJournalsErr
	}
	if m.journals != nil {
		return append([]string(nil), m.journals...), nil
	}
	return []string{"LLN0$EventLog", "LLN0.GeneralLog"}, nil
}

func (m *mockConnection) ReadJournal(_ context.Context, _, _ string, _, _ time.Time) (*iec61850.JournalReadResult, error) {
	m.readJournalCalls++
	if m.readJournalErr != nil {
		return nil, m.readJournalErr
	}
	if m.journalResult != nil {
		return m.journalResult, nil
	}
	return &iec61850.JournalReadResult{
		Entries: []iec61850.JournalEntry{
			{
				EntryID:        []byte{0x01},
				OccurrenceTime: time.UnixMilli(1_700_000_000_000).UTC(),
				Variables: []iec61850.JournalVariable{
					{Tag: "tag1", Value: iec61850.NewValue(mms.NewBoolean(true))},
				},
			},
		},
		MoreFollows: false,
	}, nil
}

func (m *mockConnection) ReadJournalAfter(_ context.Context, _, _ string, _ time.Time, _ []byte) (*iec61850.JournalReadResult, error) {
	m.readJournalAfterCalls++
	if m.readJournalAfterErr != nil {
		return nil, m.readJournalAfterErr
	}
	if m.journalAfterResult != nil {
		return m.journalAfterResult, nil
	}
	return &iec61850.JournalReadResult{
		Entries: []iec61850.JournalEntry{
			{
				EntryID:        []byte{0x02},
				OccurrenceTime: time.UnixMilli(1_700_000_000_500).UTC(),
				Variables: []iec61850.JournalVariable{
					{Tag: "tag2", Value: iec61850.NewValue(mms.NewInteger(42))},
				},
			},
		},
		MoreFollows: false,
	}, nil
}

func (m *mockConnection) Close(_ context.Context) error { return nil }
func (m *mockConnection) Abort(_ context.Context) error { return nil }

func TestIEC61850ConnectionInterface(t *testing.T) {
	t.Run("MockImplementsInterface", func(t *testing.T) {
		var _ IEC61850Connection = &mockConnection{}
	})

	t.Run("ExplorerWithMock", func(t *testing.T) {
		mock := &mockConnection{
			logicalDevices: []string{"TestLD1", "TestLD2"},
		}

		explorer := NewExplorer(mock)
		devices, err := explorer.ListLogicalDevices()

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(devices) != 2 {
			t.Fatalf("Expected 2 devices, got %d", len(devices))
		}
		if devices[0] != "TestLD1" {
			t.Errorf("Expected first device 'TestLD1', got %q", devices[0])
		}
	})
}

func TestReaderWithMock(t *testing.T) {
	t.Run("ReadObjectWithMock", func(t *testing.T) {
		mock := &mockConnection{
			readFunc: func(ref iec61850.Ref) (*iec61850.Value, error) {
				return iec61850.NewValue(mms.NewFloat(60.0)), nil
			},
		}
		reader := NewReader(mock)

		obj, err := reader.ReadObject("LD0/LN0.Hz.mag.f", domain.FC_MX)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if obj.Value == nil || obj.Value.Raw != float64(60.0) {
			t.Errorf("Expected 60.0, got %v", obj.Value)
		}
	})
}

func TestFormatterService(t *testing.T) {
	t.Run("FileSizeFormattingSI", func(t *testing.T) {
		f := formatter.NewFormatter().WithByteFormat(formatter.ByteFormatSI)

		tests := []struct {
			bytes    uint64
			expected string
		}{
			{500, "500 B"},
			{1500, "1.5 kB"},
			{1500000, "1.5 MB"},
			{1500000000, "1.5 GB"},
		}

		for _, tt := range tests {
			result := f.FileSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FileSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		}
	})

	t.Run("TimestampISO", func(t *testing.T) {
		f := formatter.NewFormatter().WithTimeFormat(formatter.TimeFormatISO)
		result := f.Timestamp(1577836800000)
		if result != "2020-01-01T00:00:00Z" {
			t.Errorf("Expected '2020-01-01T00:00:00Z', got %q", result)
		}
	})
}

func TestNewClientAdapter(t *testing.T) {
	conn := NewClientAdapter(nil)
	if conn == nil {
		t.Fatal("NewClientAdapter(nil) returned nil")
	}
	if _, ok := conn.(*ClientAdapter); !ok {
		t.Fatalf("expected *ClientAdapter, got %T", conn)
	}
}
