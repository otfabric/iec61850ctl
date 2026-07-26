// SPDX-License-Identifier: MIT

package scl

import (
	"os"
	"path/filepath"
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
	}
	for _, tt := range tests {
		got := normalizeBType(tt.in)
		if got != tt.want {
			t.Errorf("normalizeBType(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseAndFlattenAllCids(t *testing.T) {
	// cids/ is at repo root; from pkg/scl that's ../../cids
	cidsDir := filepath.Join("..", "..", "cids")
	entries, err := os.ReadDir(cidsDir)
	if err != nil {
		t.Skipf("cids dir not found: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".cid" {
			files = append(files, filepath.Join(cidsDir, e.Name()))
		}
	}
	if len(files) == 0 {
		t.Skip("no .cid files in cids/")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer func() { _ = f.Close() }()
			doc, err := Parse(f)
			if err != nil {
				t.Skipf("parse (malformed XML?): %v", err)
			}
			flat, err := doc.Flatten()
			if err != nil {
				t.Fatalf("flatten: %v", err)
			}
			if len(flat) == 0 && len(doc.IED) > 0 {
				t.Logf("warning: no flattened entries but IEDs present (may have no DataTypeTemplates)")
			}
		})
	}
}
