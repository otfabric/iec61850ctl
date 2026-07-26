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

	t.Run("Text", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter().WithOutputFormat(OutputFormatText)
		if err := f.RenderLogicalDevices(devices, &buf); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "LD1") || !strings.Contains(out, "LNs:") {
			t.Errorf("Text missing device info: %s", out)
		}
	})
}

func outputFormatName(f OutputFormat) string {
	switch f {
	case OutputFormatJSON:
		return "json"
	case OutputFormatCSV:
		return "csv"
	case OutputFormatTable:
		return "table"
	case OutputFormatYAML:
		return "yaml"
	case OutputFormatText:
		return "text"
	default:
		return "unknown"
	}
}

func TestRenderLogicalNodes_DataObjects_DataAttributes(t *testing.T) {
	nodes := []view.LogicalNode{{Name: "LLN0", DOCount: 3, DSCount: 1, RCBCount: 2}}
	objects := []view.DataObject{{Name: "Mod", DACount: 4}}
	attrs := []view.DataAttribute{
		{Name: "stVal", Type: "INT32", FC: "ST", Value: "1", Time: "t", Quality: "good"},
	}

	for _, format := range []OutputFormat{OutputFormatJSON, OutputFormatText, OutputFormatCSV, OutputFormatTable, OutputFormatYAML} {
		name := outputFormatName(format)
		t.Run(name+"/nodes", func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewFormatter().WithOutputFormat(format).RenderLogicalNodes(nodes, &buf); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), "LLN0") {
				t.Errorf("missing LLN0 in %s", buf.String())
			}
		})
		t.Run(name+"/objects", func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewFormatter().WithOutputFormat(format).RenderDataObjects(objects, &buf); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), "Mod") {
				t.Errorf("missing Mod in %s", buf.String())
			}
		})
		t.Run(name+"/attrs", func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewFormatter().WithOutputFormat(format).RenderDataAttributes(attrs, &buf); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), "stVal") {
				t.Errorf("missing stVal in %s", buf.String())
			}
		})
	}
}

func TestRenderDataSetAndReportControlBlock(t *testing.T) {
	ds := &view.DataSet{
		Name:        "ds1",
		IsDeletable: true,
		MemberCount: 1,
		Members:     []view.DataSetMember{{Index: 1, Ref: "LD0/LLN0.Mod.stVal", FC: "ST", Value: "on"}},
	}
	enabled := true
	intg := uint32(1000)
	reserved := false
	rcb := &view.ReportControlBlock{
		Name: "urcbA", LD: "LD0", LN: "LLN0", Buffered: false, Ref: "LD0/LLN0.urcbA",
		RptID: "rpt1", DatSet: "ds1", Enabled: &enabled, IntgPd: &intg, Reserved: &reserved,
		TriggerOptions: view.TriggerOptions{DataChange: true, GI: true, Transient: true},
		OptionalFields: view.OptionalFields{SequenceNumber: true, TimeStamp: true},
	}

	t.Run("nil dataset", func(t *testing.T) {
		err := NewFormatter().RenderDataSet(nil, &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected error for nil dataset")
		}
	})
	t.Run("nil rcb", func(t *testing.T) {
		err := NewFormatter().RenderReportControlBlock(nil, &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected error for nil rcb")
		}
	})

	for _, format := range []OutputFormat{OutputFormatJSON, OutputFormatText, OutputFormatCSV, OutputFormatYAML, OutputFormatTable} {
		name := outputFormatName(format)
		t.Run(name+"/dataset", func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewFormatter().WithOutputFormat(format).RenderDataSet(ds, &buf); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), "ds1") {
				t.Errorf("missing ds1: %s", buf.String())
			}
		})
		t.Run(name+"/rcb", func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewFormatter().WithOutputFormat(format).RenderReportControlBlock(rcb, &buf); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), "urcbA") {
				t.Errorf("missing urcbA: %s", buf.String())
			}
		})
	}
}

func TestFormatWrappers(t *testing.T) {
	if got := FormatFileSize(2048); !strings.Contains(got, "KB") {
		t.Errorf("FormatFileSize = %q", got)
	}
	if got := FormatDomainValue(&domain.Value{Raw: true, Type: domain.TypeBoolean}); got != "true" {
		t.Errorf("FormatDomainValue = %q", got)
	}
	if got := FormatBinaryTime(uint64(0)); got != "Unknown" {
		t.Errorf("FormatBinaryTime = %q", got)
	}
	if got := FormatUnixTimestamp(int64(1730289600)); !strings.Contains(got, "2024-10-30") {
		t.Errorf("FormatUnixTimestamp = %q", got)
	}
	if got := FormatQualityValue(uint16(0)); !strings.Contains(got, "good") {
		t.Errorf("FormatQualityValue = %q", got)
	}
	if got := FormatLeafValue(true, domain.TypeBoolean, ""); got != "true" {
		t.Errorf("FormatLeafValue = %q", got)
	}
	if got := FormatDataAttributeValue(nil); got != "(nil)" {
		t.Errorf("FormatDataAttributeValue = %q", got)
	}
	ts := domain.Timestamp{UnixMs: 1737646499123, TimeQuality: 0x80}
	if got := FormatUtcTimestamp(ts); !strings.Contains(got, "UTC") {
		t.Errorf("FormatUtcTimestamp = %q", got)
	}
}

func TestFormatterMethods(t *testing.T) {
	t.Run("FileSize formats", func(t *testing.T) {
		if got := NewFormatter().WithByteFormat(ByteFormatBytes).FileSize(500); got != "500 bytes" {
			t.Errorf("bytes = %q", got)
		}
		if got := NewFormatter().WithByteFormat(ByteFormatSI).FileSize(1500); !strings.Contains(got, "1.5") {
			t.Errorf("SI = %q", got)
		}
		if got := NewFormatter().WithByteFormat(ByteFormatIEC).FileSize(2048); !strings.Contains(got, "2.0") {
			t.Errorf("IEC = %q", got)
		}
		if got := NewFormatter().WithByteFormat(ByteFormatSI).FileSize(500); got != "500 B" {
			t.Errorf("SI small = %q", got)
		}
		if got := NewFormatter().WithByteFormat(ByteFormatIEC).FileSize(500); got != "500 B" {
			t.Errorf("IEC small = %q", got)
		}
		// Exercise multi-unit SI/IEC paths (MB/MiB and beyond).
		if got := NewFormatter().WithByteFormat(ByteFormatSI).FileSize(2_000_000); !strings.Contains(got, "M") {
			t.Errorf("SI MB = %q", got)
		}
		if got := NewFormatter().WithByteFormat(ByteFormatIEC).FileSize(2 * 1024 * 1024); !strings.Contains(got, "M") {
			t.Errorf("IEC MiB = %q", got)
		}
	})

	t.Run("Timestamp formats", func(t *testing.T) {
		ms := uint64(1737646499123)
		if got := NewFormatter().WithTimeFormat(TimeFormatISO).Timestamp(ms); !strings.Contains(got, "2025-01-23") {
			t.Errorf("ISO = %q", got)
		}
		if got := NewFormatter().WithTimeFormat(TimeFormatUnix).Timestamp(ms); got != "1737646499123" {
			t.Errorf("Unix = %q", got)
		}
		relPast := NewFormatter().WithTimeFormat(TimeFormatRelative).Timestamp(uint64(time.Now().Add(-2 * time.Hour).UnixMilli()))
		if !strings.Contains(relPast, "ago") {
			t.Errorf("relative past = %q", relPast)
		}
		relFuture := NewFormatter().WithTimeFormat(TimeFormatRelative).Timestamp(uint64(time.Now().Add(90 * time.Second).UnixMilli()))
		if !strings.Contains(relFuture, "in ") {
			t.Errorf("relative future = %q", relFuture)
		}
		relMin := NewFormatter().WithTimeFormat(TimeFormatRelative).Timestamp(uint64(time.Now().Add(-90 * time.Second).UnixMilli()))
		if !strings.Contains(relMin, "m ago") {
			t.Errorf("relative minutes = %q", relMin)
		}
		relDay := NewFormatter().WithTimeFormat(TimeFormatRelative).Timestamp(uint64(time.Now().Add(-48 * time.Hour).UnixMilli()))
		if !strings.Contains(relDay, "d ago") {
			t.Errorf("relative days = %q", relDay)
		}
		relSec := NewFormatter().WithTimeFormat(TimeFormatRelative).Timestamp(uint64(time.Now().Add(-10 * time.Second).UnixMilli()))
		if !strings.Contains(relSec, "s ago") {
			t.Errorf("relative seconds = %q", relSec)
		}
	})

	t.Run("BinaryTime", func(t *testing.T) {
		f := NewFormatter().WithTimeFormat(TimeFormatUnix)
		six := []byte{0, 0, 0, 0, 0, 1}
		if got := f.BinaryTime(six); got == "" {
			t.Errorf("6-byte BinaryTime empty")
		}
		four := []byte{0, 0, 0, 1}
		if got := f.BinaryTime(four); got != "1000" {
			t.Errorf("4-byte BinaryTime = %q", got)
		}
		if got := f.BinaryTime(uint64(123)); got != "123" {
			t.Errorf("uint64 BinaryTime = %q", got)
		}
		if got := f.BinaryTime(int64(456)); got != "456" {
			t.Errorf("int64 BinaryTime = %q", got)
		}
		if got := f.BinaryTime("x"); got != "x" {
			t.Errorf("fallback BinaryTime = %q", got)
		}
	})

	t.Run("UtcTimeValue and helpers", func(t *testing.T) {
		ts := domain.Timestamp{UnixMs: 1737646499123, TimeQuality: 0x00}
		got := NewFormatter().WithTimeFormat(TimeFormatISO).UtcTimeValue(ts)
		if !strings.Contains(got, "leap-unknown") {
			t.Errorf("UtcTimeValue = %q", got)
		}
		tsFlags := domain.Timestamp{UnixMs: 1737646499123, TimeQuality: 0x40 | 0x20 | 0x0A}
		got = NewFormatter().UtcTimeValue(tsFlags)
		if !strings.Contains(got, "clock-failure") || !strings.Contains(got, "clock-not-synced") || !strings.Contains(got, "acc<=") {
			t.Errorf("UtcTimeValue flags = %q", got)
		}
		tsClean := domain.Timestamp{UnixMs: 1737646499123, TimeQuality: 0x80}
		got = NewFormatter().UtcTimeValue(tsClean)
		if strings.Contains(got, "[") {
			t.Errorf("UtcTimeValue clean unexpected flags: %q", got)
		}
		if NewFormatter().DataAttribute(nil) != "" {
			t.Errorf("DataAttribute(nil) should be empty")
		}
		da := &domain.DataAttribute{Value: &domain.Value{Display: "shown"}}
		if got := NewFormatter().DataAttribute(da); got != "shown" {
			t.Errorf("DataAttribute = %q", got)
		}
		if got := NewFormatter().TypeSpec(nil); got != "UNKNOWN" {
			t.Errorf("TypeSpec = %q", got)
		}
		if got := NewFormatter().LeafValue(nil, domain.TypeBoolean, ""); got != "(null)" {
			t.Errorf("LeafValue nil = %q", got)
		}
		tsVal := domain.Timestamp{UnixMs: 1737646499123, TimeQuality: 0x80}
		if got := NewFormatter().LeafValue(tsVal, domain.TypeUtcTime, ""); !strings.Contains(got, "UTC") {
			t.Errorf("LeafValue timestamp = %q", got)
		}
		if got := NewFormatter().LeafValue(&tsVal, domain.TypeUtcTime, ""); !strings.Contains(got, "UTC") {
			t.Errorf("LeafValue *timestamp = %q", got)
		}
		if got := NewFormatter().LeafValue(true, domain.TypeBoolean, ""); got != "true" {
			t.Errorf("LeafValue bool = %q", got)
		}
	})
}

func TestNewRenderer(t *testing.T) {
	for _, format := range []OutputFormat{
		OutputFormatJSON, OutputFormatCSV, OutputFormatTable, OutputFormatYAML, OutputFormatText, OutputFormat(99),
	} {
		r := NewRenderer(format)
		if r == nil {
			t.Fatalf("NewRenderer(%v) nil", format)
		}
		var buf bytes.Buffer
		if err := r.RenderLogicalDevices([]view.LogicalDevice{{Name: "X"}}, &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "X") {
			t.Errorf("renderer %v missing X: %s", format, buf.String())
		}
	}
}
