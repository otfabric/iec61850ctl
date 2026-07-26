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
	readFunc       func(ref iec61850.Ref) (*iec61850.Value, error)
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
	return []iec61850.LogicalNode{{Name: "LLN0"}, {Name: "MMXU1"}}, nil
}

func (m *mockConnection) ListDataObjects(_ context.Context, _, _ string) ([]iec61850.DataObject, error) {
	return []iec61850.DataObject{{Name: "DO1"}, {Name: "DO2"}}, nil
}

func (m *mockConnection) ListChildren(_ context.Context, ref iec61850.Ref) ([]iec61850.BrowseNode, error) {
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

func (m *mockConnection) GetVariableType(_ context.Context, _ iec61850.Ref) (*mms.TypeSpec, error) {
	return &mms.TypeSpec{Type: mms.ValueTypeFloat}, nil
}

func (m *mockConnection) ListDataSets(_ context.Context, _ string) ([]string, error) {
	return []string{"LLN0$ds1"}, nil
}

func (m *mockConnection) GetDataSet(_ context.Context, _, _ string) (*iec61850.DataSet, error) {
	return &iec61850.DataSet{}, nil
}

func (m *mockConnection) ReadDataSet(_ context.Context, _, _ string) ([]iec61850.DataSetValue, error) {
	return []iec61850.DataSetValue{}, nil
}

func (m *mockConnection) ListReports(_ context.Context, _ string) ([]string, error) {
	return []string{"LLN0$BR$rcb1"}, nil
}

func (m *mockConnection) GetReportControlBlock(_ context.Context, _, _ string) (*iec61850.ReportControlBlock, error) {
	return &iec61850.ReportControlBlock{RptID: "test-rpt"}, nil
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
	return nil, nil
}

func (m *mockConnection) ReadJournal(_ context.Context, _, _ string, _, _ time.Time) (*iec61850.JournalReadResult, error) {
	return nil, nil
}

func (m *mockConnection) ReadJournalAfter(_ context.Context, _, _ string, _ time.Time, _ []byte) (*iec61850.JournalReadResult, error) {
	return nil, nil
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
