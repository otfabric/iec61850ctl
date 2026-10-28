// SPDX-License-Identifier: MIT

package scl

import (
	"testing"
)

func TestFormatEntry(t *testing.T) {
	e := FlattenEntry{
		Path:  "IED1LD0/LN1.DO1.d",
		FC:    "DC",
		Value: "description",
		BType: "STRING",
	}
	// detailed=true: include [FC], [type], [enum]
	got := FormatEntry(e, true)
	want := "IED1LD0/LN1.DO1.d[DC]: description [type:STRING]"
	if got != want {
		t.Errorf("FormatEntry(detailed): got %q want %q", got, want)
	}
	e.Value = ""
	got = FormatEntry(e, true)
	want = "IED1LD0/LN1.DO1.d[DC]: ? [type:STRING]"
	if got != want {
		t.Errorf("FormatEntry empty value (detailed): got %q want %q", got, want)
	}
	// detailed=false: path and value only
	e.Value = "description"
	got = FormatEntry(e, false)
	want = "IED1LD0/LN1.DO1.d: description"
	if got != want {
		t.Errorf("FormatEntry(simple): got %q want %q", got, want)
	}
	e.Value = ""
	got = FormatEntry(e, false)
	want = "IED1LD0/LN1.DO1.d: ?"
	if got != want {
		t.Errorf("FormatEntry empty value (simple): got %q want %q", got, want)
	}
}

func TestNormalizeBType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"BOOLEAN", "BOOL"},
		{"Timestamp", "UTC_TIME"},
		{"INT8", "INT8"},
		{"INT8U", "UINT8"},
		{"INT32U", "UINT32"},
		{"Check", "BIT_STRING"},
		{"Octet64", "BIT_STRING"},
		{"Quality", "BIT_STRING"},
		{"VisString255", "STRING"},
		{"Unicode255", "STRING"},
		{"", "STRING"},
		{"INT16", "INT16"},
		{"INT32", "INT32"},
		{"INT64", "INT64"},
		{"INT128", "INT128"},
		{"INT16U", "UINT16"},
		{"UINT8", "UINT8"},
		{"UINT32", "UINT32"},
		{"UINT64", "UINT64"},
		{"Enum", "INT8"},
		{"EnumFoo", "INT8"},
		{"MyEnumKind", "INT8"},
		{"Float", "Float"},
		{"Double", "Double"},
		{"FLOAT32", "FLOAT32"},
		{"Dbpos", "Dbpos"},
		{"Struct", "Struct"},
	}
	for _, tt := range tests {
		got := normalizeBType(tt.in)
		if got != tt.want {
			t.Errorf("normalizeBType(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}
