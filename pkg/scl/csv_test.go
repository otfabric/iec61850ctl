// SPDX-License-Identifier: MIT

package scl

import (
	"bytes"
	"strings"
	"testing"
)

func TestEscapeCSVField(t *testing.T) {
	tests := []struct {
		s, sep, want string
	}{
		{"", ",", ""},
		{"plain", ",", "plain"},
		{"a,b", ",", `"a,b"`},
		{`say "hi"`, ",", `"say ""hi"""`},
		{"line\nbreak", ",", "\"line\nbreak\""},
		{"a|b", "|", `"a|b"`},
		{"ok", "|", "ok"},
	}
	for _, tt := range tests {
		got := EscapeCSVField(tt.s, tt.sep)
		if got != tt.want {
			t.Errorf("EscapeCSVField(%q, %q) = %q, want %q", tt.s, tt.sep, got, tt.want)
		}
	}
}

func TestCSVRow(t *testing.T) {
	ld, ln, do, da, fc, val, bType, enumVals := CSVRow(FlattenEntry{
		Path:     "TEST1LD0/LLN0.Beh.stVal",
		FC:       "ST",
		Value:    "on",
		BType:    "INT8",
		EnumVals: "1(on),2(off)",
	})
	if ld != "TEST1LD0" || ln != "LLN0" || do != "Beh" || da != "stVal" {
		t.Errorf("path parts = %q/%q.%q.%q", ld, ln, do, da)
	}
	if fc != "ST" || val != "on" || bType != "INT8" || enumVals != "1(on),2(off)" {
		t.Errorf("meta = fc=%q val=%q type=%q enum=%q", fc, val, bType, enumVals)
	}

	ld, ln, do, da, _, val, _, _ = CSVRow(FlattenEntry{Path: "ONLY", Value: ""})
	if ld != "ONLY" || ln != "" || do != "" || da != "" || val != "?" {
		t.Errorf("short path = ld=%q ln=%q do=%q da=%q val=%q", ld, ln, do, da, val)
	}

	ld, ln, do, da, _, _, _, _ = CSVRow(FlattenEntry{Path: "LD/LN"})
	if ld != "LD" || ln != "LN" || do != "" || da != "" {
		t.Errorf("two-part = %q/%q.%q.%q", ld, ln, do, da)
	}

	ld, ln, do, da, _, _, _, _ = CSVRow(FlattenEntry{Path: "LD/LN.DO"})
	if ld != "LD" || ln != "LN" || do != "DO" || da != "" {
		t.Errorf("three-part = %q/%q.%q.%q", ld, ln, do, da)
	}

	ld, ln, do, da, _, _, _, _ = CSVRow(FlattenEntry{Path: "LD/LN.DO.a.b"})
	if ld != "LD" || ln != "LN" || do != "DO" || da != "a.b" {
		t.Errorf("nested DA = %q/%q.%q.%q", ld, ln, do, da)
	}
}

func TestWriteCSV_Minimal(t *testing.T) {
	doc, err := Parse(strings.NewReader(minimalCID))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	entries, err := doc.Flatten()
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected flattened entries")
	}

	var buf bytes.Buffer
	if err := WriteCSV(&buf, ",", entries); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, CSVHeaders+"\n") {
		t.Fatalf("missing headers; got prefix %q", out[:min(80, len(out))])
	}
	if !strings.Contains(out, "TEST1LD0") {
		t.Errorf("CSV missing logical device; got:\n%s", out)
	}
	if !strings.Contains(out, "Beh") || !strings.Contains(out, "stVal") {
		t.Errorf("CSV missing DO/DA; got:\n%s", out)
	}

	var pipe bytes.Buffer
	if err := WriteCSV(&pipe, "|", entries[:1]); err != nil {
		t.Fatalf("WriteCSV pipe: %v", err)
	}
	if !strings.HasPrefix(pipe.String(), strings.ReplaceAll(CSVHeaders, ",", "|")+"\n") {
		t.Errorf("pipe headers wrong: %q", pipe.String())
	}
}

func TestWriteTree_Minimal(t *testing.T) {
	doc, err := Parse(strings.NewReader(minimalCID))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var simple bytes.Buffer
	if err := doc.WriteTree(&simple, false); err != nil {
		t.Fatalf("WriteTree simple: %v", err)
	}
	got := simple.String()
	if !strings.Contains(got, "TEST1LD0") {
		t.Errorf("tree missing root; got:\n%s", got)
	}
	if !strings.Contains(got, "LLN0") {
		t.Errorf("tree missing LLN0; got:\n%s", got)
	}
	if !strings.Contains(got, "Beh") {
		t.Errorf("tree missing Beh; got:\n%s", got)
	}
	if !strings.Contains(got, "stVal: ?") {
		t.Errorf("tree missing simple leaf; got:\n%s", got)
	}

	var detailed bytes.Buffer
	if err := doc.WriteTree(&detailed, true); err != nil {
		t.Fatalf("WriteTree detailed: %v", err)
	}
	dgot := detailed.String()
	if !strings.Contains(dgot, "[ST]") || !strings.Contains(dgot, "[type:") {
		t.Errorf("detailed tree missing FC/type; got:\n%s", dgot)
	}
}
