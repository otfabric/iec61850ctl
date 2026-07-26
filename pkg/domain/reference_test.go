// SPDX-License-Identifier: MIT

package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseObjectReference(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		want    ObjectReference
		wantErr string
	}{
		{
			name: "full path",
			ref:  "ZX2REX640A1LD0/FMMXU1.Hz.mag.f",
			want: ObjectReference{LD: "ZX2REX640A1LD0", LN: "FMMXU1", Path: []string{"Hz", "mag", "f"}},
		},
		{
			name: "LN only",
			ref:  "LD0/LLN0",
			want: ObjectReference{LD: "LD0", LN: "LLN0"},
		},
		{
			name: "DO only",
			ref:  "LD0/GGIO1.AnIn1",
			want: ObjectReference{LD: "LD0", LN: "GGIO1", Path: []string{"AnIn1"}},
		},
		{
			name: "trailing dot yields empty path",
			ref:  "LD0/GGIO1.",
			want: ObjectReference{LD: "LD0", LN: "GGIO1"},
		},
		{
			name:    "missing slash",
			ref:     "LD0GGIO1.AnIn1",
			wantErr: "missing '/' separator",
		},
		{
			name:    "empty after slash",
			ref:     "LD0/",
			wantErr: "missing logical node after '/'",
		},
		{
			name:    "empty string",
			ref:     "",
			wantErr: "missing '/' separator",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseObjectReference(tt.ref)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseObjectReference(%q) error = nil, want containing %q", tt.ref, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseObjectReference(%q) error = %v, want containing %q", tt.ref, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseObjectReference(%q) unexpected error: %v", tt.ref, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseObjectReference(%q) = %+v, want %+v", tt.ref, got, tt.want)
			}
		})
	}
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name string
		ld   string
		ln   string
		path []string
		want string
	}{
		{"no path", "LD0", "LLN0", nil, "LD0/LLN0"},
		{"with path", "LD0", "GGIO1", []string{"AnIn1", "mag", "f"}, "LD0/GGIO1.AnIn1.mag.f"},
		{"empty path slice", "A", "B", []string{}, "A/B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Build(tt.ld, tt.ln, tt.path...)
			if got != tt.want {
				t.Fatalf("Build() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestObjectReference_String_DOName_DAPath_WithFC(t *testing.T) {
	r := ObjectReference{LD: "LD0", LN: "MMXU1", Path: []string{"Hz", "mag", "f"}}

	if got := r.String(); got != "LD0/MMXU1.Hz.mag.f" {
		t.Errorf("String() = %q, want LD0/MMXU1.Hz.mag.f", got)
	}
	if got := r.DOName(); got != "Hz" {
		t.Errorf("DOName() = %q, want Hz", got)
	}
	if got := r.DAPath(); !reflect.DeepEqual(got, []string{"mag", "f"}) {
		t.Errorf("DAPath() = %v, want [mag f]", got)
	}

	empty := ObjectReference{LD: "LD0", LN: "LLN0"}
	if got := empty.DOName(); got != "" {
		t.Errorf("DOName() empty path = %q, want \"\"", got)
	}
	if got := empty.DAPath(); got != nil {
		t.Errorf("DAPath() empty path = %v, want nil", got)
	}
	doOnly := ObjectReference{LD: "LD0", LN: "LLN0", Path: []string{"Mod"}}
	if got := doOnly.DAPath(); got != nil {
		t.Errorf("DAPath() DO-only = %v, want nil", got)
	}

	withFC := r.WithFC(FC_MX)
	if withFC.FC != FC_MX {
		t.Errorf("WithFC() FC = %v, want MX", withFC.FC)
	}
	if r.FC != FC_NONE {
		t.Errorf("WithFC mutated original FC = %v", r.FC)
	}
}

func TestDataSetRef_ReportRef_JournalRef(t *testing.T) {
	if got := DataSetRef("LD0", "LLN0", "dsEvents"); got != "LD0/LLN0$dsEvents" {
		t.Errorf("DataSetRef() = %q", got)
	}
	if got := ReportRef("LD0", "LLN0", FC_RP, "urcb01"); got != "LD0/LLN0.RP.urcb01" {
		t.Errorf("ReportRef() = %q", got)
	}
	if got := JournalRef("LD0", "LLN0", "log1"); got != "LD0/LLN0$log1" {
		t.Errorf("JournalRef() = %q", got)
	}
}

func TestParseDataSetRef(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		wantLD string
		wantLN string
		wantDS string
		wantOK bool
	}{
		{"domain format", "LD0/LLN0$dsEvents", "LD0", "LLN0", "dsEvents", true},
		{"directory format", "LD0/LLN0.dsEvents", "LD0", "LLN0", "dsEvents", true},
		{"empty", "", "", "", "", false},
		{"no slash", "LLN0$ds", "", "", "", false},
		{"leading slash", "/LLN0$ds", "", "", "", false},
		{"trailing slash", "LD0/", "", "", "", false},
		{"no separator", "LD0/LLN0", "", "", "", false},
		{"empty name", "LD0/LLN0$", "", "", "", false},
		{"empty LN", "LD0/$ds", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld, ln, ds, ok := ParseDataSetRef(tt.in)
			if ok != tt.wantOK || ld != tt.wantLD || ln != tt.wantLN || ds != tt.wantDS {
				t.Fatalf("ParseDataSetRef(%q) = (%q,%q,%q,%v), want (%q,%q,%q,%v)",
					tt.in, ld, ln, ds, ok, tt.wantLD, tt.wantLN, tt.wantDS, tt.wantOK)
			}
		})
	}
}

func TestParseReportRef(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantLD  string
		wantLN  string
		wantFC  FunctionalConstraint
		wantRCB string
		wantOK  bool
	}{
		{"valid RP", "LD0/LLN0.RP.urcb01", "LD0", "LLN0", FC_RP, "urcb01", true},
		{"valid BR", "IED1LD0/MMXU1.BR.brcb1", "IED1LD0", "MMXU1", FC_BR, "brcb1", true},
		{"empty", "", "", "", "", "", false},
		{"no slash", "LLN0.RP.urcb01", "", "", "", "", false},
		{"leading slash", "/LLN0.RP.urcb01", "", "", "", "", false},
		{"trailing slash", "LD0/", "", "", "", "", false},
		{"too few parts", "LD0/LLN0.RP", "", "", "", "", false},
		{"empty LN", "LD0/.RP.urcb01", "", "", "", "", false},
		{"empty rcb", "LD0/LLN0.RP.", "", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld, ln, fc, rcb, ok := ParseReportRef(tt.in)
			if ok != tt.wantOK || ld != tt.wantLD || ln != tt.wantLN || fc != tt.wantFC || rcb != tt.wantRCB {
				t.Fatalf("ParseReportRef(%q) = (%q,%q,%q,%q,%v), want (%q,%q,%q,%q,%v)",
					tt.in, ld, ln, fc, rcb, ok, tt.wantLD, tt.wantLN, tt.wantFC, tt.wantRCB, tt.wantOK)
			}
		})
	}
}

func TestParseJournalRef(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantLD  string
		wantLN  string
		wantLog string
		wantOK  bool
	}{
		{"valid", "LD0/LLN0$GeneralLog", "LD0", "LLN0", "GeneralLog", true},
		{"empty", "", "", "", "", false},
		{"no slash", "LLN0$log", "", "", "", false},
		{"leading slash", "/LLN0$log", "", "", "", false},
		{"trailing slash", "LD0/", "", "", "", false},
		{"no dollar", "LD0/LLN0", "", "", "", false},
		{"empty LN", "LD0/$log", "", "", "", false},
		{"empty log", "LD0/LLN0$", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld, ln, logName, ok := ParseJournalRef(tt.in)
			if ok != tt.wantOK || ld != tt.wantLD || ln != tt.wantLN || logName != tt.wantLog {
				t.Fatalf("ParseJournalRef(%q) = (%q,%q,%q,%v), want (%q,%q,%q,%v)",
					tt.in, ld, ln, logName, ok, tt.wantLD, tt.wantLN, tt.wantLog, tt.wantOK)
			}
		})
	}
}

func TestObjectReference_Parent_Child(t *testing.T) {
	r := ObjectReference{LD: "LD0", LN: "MMXU1", Path: []string{"Hz", "mag", "f"}, FC: FC_MX}

	parent := r.Parent()
	if parent.LD != "LD0" || parent.LN != "MMXU1" || parent.FC != FC_MX {
		t.Fatalf("Parent() identity = %+v", parent)
	}
	if !reflect.DeepEqual(parent.Path, []string{"Hz", "mag"}) {
		t.Fatalf("Parent() Path = %v, want [Hz mag]", parent.Path)
	}

	grand := parent.Parent()
	if !reflect.DeepEqual(grand.Path, []string{"Hz"}) {
		t.Fatalf("Parent() twice Path = %v, want [Hz]", grand.Path)
	}

	// Path length <= 1 clears path.
	root := grand.Parent()
	if root.Path != nil {
		t.Fatalf("Parent() of DO Path = %v, want nil", root.Path)
	}
	stillRoot := root.Parent()
	if stillRoot.Path != nil {
		t.Fatalf("Parent() of LN Path = %v, want nil", stillRoot.Path)
	}

	child := r.Child("q")
	if !reflect.DeepEqual(child.Path, []string{"Hz", "mag", "f", "q"}) {
		t.Fatalf("Child() Path = %v", child.Path)
	}
	if child.FC != FC_MX {
		t.Fatalf("Child() FC = %v, want MX", child.FC)
	}

	fromEmpty := ObjectReference{LD: "LD0", LN: "LLN0"}.Child("Mod")
	if !reflect.DeepEqual(fromEmpty.Path, []string{"Mod"}) {
		t.Fatalf("Child() from empty Path = %v", fromEmpty.Path)
	}
}
