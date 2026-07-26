// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
)

func TestParsePath(t *testing.T) {
	ld, ln, do := parsePath("")
	if ld != "" || ln != "" || do != "" {
		t.Fatalf("empty: %q %q %q", ld, ln, do)
	}
	ld, ln, do = parsePath("LD0")
	if ld != "LD0" || ln != "" || do != "" {
		t.Fatalf("ld only: %q %q %q", ld, ln, do)
	}
	ld, ln, do = parsePath("LD0/LLN0")
	if ld != "LD0" || ln != "LLN0" || do != "" {
		t.Fatalf("ld/ln: %q %q %q", ld, ln, do)
	}
	ld, ln, do = parsePath("LD0/MMXU1.Hz.mag")
	if ld != "LD0" || ln != "MMXU1" || do != "Hz.mag" {
		t.Fatalf("full: %q %q %q", ld, ln, do)
	}
}

func TestDedupeStrings(t *testing.T) {
	got := dedupeStrings([]string{"a", "b", "a", "c", "b"})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("got %v", got)
	}
}

func TestLastSegment(t *testing.T) {
	if lastSegment("LD0/LN.DO.mag") != "mag" {
		t.Fatal(lastSegment("LD0/LN.DO.mag"))
	}
	if lastSegment("plain") != "plain" {
		t.Fatal(lastSegment("plain"))
	}
}

func TestRefWithFC(t *testing.T) {
	ref := iec61850.Ref{LD: "LD0", LN: "LN", Path: []string{"DO"}}
	got := refWithFC(ref, iec61850.FCMX)
	if got.FC != iec61850.FCMX || ref.FC != "" {
		t.Fatalf("got=%+v orig=%+v", got, ref)
	}
}

func TestFormatTypeSpec(t *testing.T) {
	if formatTypeSpec(nil) != "UNKNOWN" {
		t.Fatal(formatTypeSpec(nil))
	}
	if formatTypeSpec(&mms.TypeSpec{Type: mms.ValueTypeStructure}) != "STRUCT" {
		t.Fatal("STRUCT")
	}
	if formatTypeSpec(&mms.TypeSpec{
		Type:     mms.ValueTypeStructure,
		Elements: []mms.TypeSpecElement{{Name: "a"}, {Name: "b"}},
	}) != "STRUCT(2)" {
		t.Fatal("STRUCT(2)")
	}
	if formatTypeSpec(&mms.TypeSpec{Type: mms.ValueTypeArray, Count: 4}) != "ARRAY[4]" {
		t.Fatal("ARRAY[4]")
	}
	if formatTypeSpec(&mms.TypeSpec{Type: mms.ValueTypeArray}) != "ARRAY" {
		t.Fatal("ARRAY")
	}
	if formatTypeSpec(&mms.TypeSpec{Type: mms.ValueTypeFloat}) == "" {
		t.Fatal("float type empty")
	}
}

func TestFormatValueDisplay(t *testing.T) {
	if formatValueDisplay(nil) != "?" {
		t.Fatal("nil")
	}
	v := iec61850.NewValue(mms.NewFloat(1.25))
	if formatValueDisplay(v) == "?" {
		t.Fatal("expected formatted value")
	}
}

func TestTree_SetCallInterval(t *testing.T) {
	tr := NewTree(&mockConnection{})
	tr.SetCallInterval(0)
	if tr.callInterval != 0 {
		t.Fatal(tr.callInterval)
	}
}
