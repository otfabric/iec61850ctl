// SPDX-License-Identifier: MIT

package formatter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

func TestFormatTypeSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec     *mms.TypeSpec
		expected string
	}{
		{"nil spec", nil, "UNKNOWN"},
		{"Boolean", &mms.TypeSpec{Type: mms.ValueTypeBoolean}, "BOOL"},
		{"Int32", &mms.TypeSpec{Type: mms.ValueTypeInteger, Size: 32}, "INT32"},
		{"Float", &mms.TypeSpec{Type: mms.ValueTypeFloat}, "FLOAT"},
		{"VisibleString", &mms.TypeSpec{Type: mms.ValueTypeVisibleString}, "STRING"},
		{"UTCTime", &mms.TypeSpec{Type: mms.ValueTypeUTCTime}, "UTC_TIME"},
		{"Structure", &mms.TypeSpec{Type: mms.ValueTypeStructure}, "STRUCT"},
		{"Structure with elements", &mms.TypeSpec{Type: mms.ValueTypeStructure, Elements: []mms.TypeSpecElement{{}, {}, {}}}, "STRUCT(3)"},
		{"Array with size", &mms.TypeSpec{Type: mms.ValueTypeArray, Count: 10}, "ARRAY[10]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTypeSpec(tt.spec)
			if result != tt.expected {
				t.Errorf("FormatTypeSpec() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatTypeSpec_UnknownType(t *testing.T) {
	spec := &mms.TypeSpec{Type: mms.ValueType(999)}
	result := FormatTypeSpec(spec)
	if result != "TYPE_999" {
		t.Errorf("FormatTypeSpec(unknown) = %q, want TYPE_999", result)
	}
}

func TestFormatUtcTimeValue(t *testing.T) {
	s := FormatUtcTimeValue(1737646499123, 0x00)
	if s == "" {
		t.Errorf("FormatUtcTimeValue() = %q", s)
	}
	if !strings.Contains(s, "UTC") {
		t.Errorf("FormatUtcTimeValue() = %q, want containing UTC", s)
	}
	if !strings.Contains(s, "leap-unknown") {
		t.Errorf("FormatUtcTimeValue() = %q, want containing leap-unknown", s)
	}
}

func TestFormatMmsValue(t *testing.T) {
	structVal := mms.NewStructure([]*mms.Value{
		mms.NewInteger(42),
		mms.NewVisibleString("hello"),
	})
	got := FormatMmsValue(structVal)
	if !strings.Contains(got, "42") || !strings.Contains(got, "hello") {
		t.Errorf("FormatMmsValue(struct) = %q, want nested values", got)
	}

	utc := mms.NewUTCTimeWithQuality(time.UnixMilli(1737646499123), 0)
	got = FormatMmsValue(utc)
	if !strings.Contains(got, "UTC") {
		t.Errorf("FormatMmsValue(utc) = %q, want containing UTC", got)
	}
}

func TestParseTimeToUnixMs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMs  uint64
		wantErr bool
	}{
		{"raw ms", "1730289600000", 1730289600000, false},
		{"RFC3339", "2024-10-30T12:00:00Z", 1730289600000, false},
		{"space UTC", "2024-10-30 12:00:00", 1730289600000, false},
		{"empty", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ParseTimeToUnixMs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimeToUnixMs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantMs {
				t.Errorf("ParseTimeToUnixMs() = %d, want %d", got, tt.wantMs)
			}
		})
	}
}

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		s    string
		want OutputFormat
		ok   bool
	}{
		{"json", OutputFormatJSON, true},
		{"JSON", OutputFormatJSON, true},
		{"csv", OutputFormatCSV, true},
		{"table", OutputFormatTable, true},
		{"yaml", OutputFormatYAML, true},
		{"YAML", OutputFormatYAML, true},
		{"text", OutputFormatText, true},
		{"", OutputFormatText, false},
		{"invalid", OutputFormatText, false},
	}
	for _, tt := range tests {
		got, ok := ParseOutputFormat(tt.s)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseOutputFormat(%q) = %v, %v; want %v, %v", tt.s, got, ok, tt.want, tt.ok)
		}
	}
}

func TestFormatLogicalDevices_JSON_YAML_CSV_Table(t *testing.T) {
	devices := []view.LogicalDevice{
		{Name: "LD1", LNCount: 2, DSCount: 1, URCBCount: 0, BRCBCount: 1},
		{Name: "LD2", LNCount: 1, DSCount: 0, URCBCount: 1, BRCBCount: 0},
	}

	t.Run("JSON", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter().WithOutputFormat(OutputFormatJSON)
		if err := f.RenderLogicalDevices(devices, &buf); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, `"name": "LD1"`) {
			t.Errorf("JSON output missing name: %s", out)
		}
		if !strings.Contains(out, `"ln_count": 2`) {
			t.Errorf("JSON output missing ln_count: %s", out)
		}
	})

	t.Run("YAML", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter().WithOutputFormat(OutputFormatYAML)
		if err := f.RenderLogicalDevices(devices, &buf); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "name: LD1") {
			t.Errorf("YAML output missing name: %s", out)
		}
		if !strings.Contains(out, "2") || !strings.Contains(out, "LD1") {
			t.Errorf("YAML output missing LD1 or count: %s", out)
		}
	})

	t.Run("CSV", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter().WithOutputFormat(OutputFormatCSV)
		if err := f.RenderLogicalDevices(devices, &buf); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "Name,LNCount,DSCount") {
			t.Errorf("CSV missing header: %s", out)
		}
		if !strings.Contains(out, "LD1,2,1,0,1") {
			t.Errorf("CSV missing LD1 row: %s", out)
		}
	})

	t.Run("Table", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter().WithOutputFormat(OutputFormatTable)
		if err := f.RenderLogicalDevices(devices, &buf); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "LD1") || !strings.Contains(out, "LD2") {
			t.Errorf("Table missing device names: %s", out)
		}
	})
}
