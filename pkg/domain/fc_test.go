// SPDX-License-Identifier: MIT

package domain

import (
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
)

func TestAllFCs(t *testing.T) {
	fcs := AllFCs()
	if len(fcs) != len(allFCs) {
		t.Fatalf("AllFCs() len = %d, want %d", len(fcs), len(allFCs))
	}
	for i, fc := range fcs {
		if fc != allFCs[i] {
			t.Fatalf("AllFCs()[%d] = %v, want %v", i, fc, allFCs[i])
		}
	}
	// Defensive copy: mutating result must not affect package slice.
	fcs[0] = FC_NONE
	if allFCs[0] == FC_NONE {
		t.Fatal("AllFCs() did not return a copy")
	}
}

func TestParseFC(t *testing.T) {
	tests := []struct {
		in   string
		want FunctionalConstraint
	}{
		{"ST", FC_ST},
		{"MX", FC_MX},
		{"SP", FC_SP},
		{"SV", FC_SV},
		{"CF", FC_CF},
		{"DC", FC_DC},
		{"SG", FC_SG},
		{"SE", FC_SE},
		{"SR", FC_SR},
		{"OR", FC_OR},
		{"BL", FC_BL},
		{"EX", FC_EX},
		{"CO", FC_CO},
		{"US", FC_US},
		{"MS", FC_MS},
		{"RP", FC_RP},
		{"BR", FC_BR},
		{"LG", FC_LG},
		{"GO", FC_GO},
		{"ALL", FC_ALL},
		{"mx", FC_MX},
		{"  rp  ", FC_RP},
		{"Br", FC_BR},
		{"", FC_NONE},
		{"XX", FC_NONE},
		{"none", FC_NONE},
		{"  ", FC_NONE},
	}
	for _, tt := range tests {
		t.Run(tt.in+"->"+string(tt.want), func(t *testing.T) {
			got := ParseFC(tt.in)
			if got != tt.want {
				t.Fatalf("ParseFC(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFunctionalConstraint_String_IsValid(t *testing.T) {
	if FC_NONE.String() != "NONE" {
		t.Errorf("FC_NONE.String() = %q, want NONE", FC_NONE.String())
	}
	if FC_MX.String() != "MX" {
		t.Errorf("FC_MX.String() = %q, want MX", FC_MX.String())
	}

	for _, fc := range AllFCs() {
		if !fc.IsValid() {
			t.Errorf("%s.IsValid() = false, want true", fc)
		}
	}
	if !FC_ALL.IsValid() {
		t.Error("FC_ALL.IsValid() = false, want true")
	}
	if FC_NONE.IsValid() {
		t.Error("FC_NONE.IsValid() = true, want false")
	}
	if FunctionalConstraint("ZZ").IsValid() {
		t.Error("unknown FC IsValid() = true, want false")
	}
}

func TestFunctionalConstraint_ToLibFC_FromLibFC(t *testing.T) {
	tests := []struct {
		name string
		fc   FunctionalConstraint
		want iec61850.FunctionalConstraint
	}{
		{"MX", FC_MX, iec61850.FunctionalConstraint("MX")},
		{"RP", FC_RP, iec61850.FunctionalConstraint("RP")},
		{"ALL maps empty", FC_ALL, ""},
		{"NONE maps empty", FC_NONE, ""},
		{"unknown maps empty", FunctionalConstraint("ZZ"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fc.ToLibFC()
			if got != tt.want {
				t.Fatalf("ToLibFC() = %q, want %q", got, tt.want)
			}
		})
	}

	if got := FromLibFC(iec61850.FunctionalConstraint("ST")); got != FC_ST {
		t.Errorf("FromLibFC(ST) = %q, want ST", got)
	}
	if got := FromLibFC(""); got != FC_NONE {
		t.Errorf("FromLibFC(\"\") = %q, want NONE", got)
	}
}
