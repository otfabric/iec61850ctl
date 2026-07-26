// SPDX-License-Identifier: MIT

package domain

import (
	"strings"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
)

func TestQuality_GetValidity_IsGood(t *testing.T) {
	tests := []struct {
		q        Quality
		validity Validity
		good     bool
	}{
		{0, ValidityGood, true},
		{Quality(ValidityInvalid), ValidityInvalid, false},
		{Quality(ValidityReserved), ValidityReserved, false},
		{Quality(ValidityQuestionable), ValidityQuestionable, false},
		{QualityOverflow | Quality(ValidityGood), ValidityGood, true},
		{QualityFailure | Quality(ValidityInvalid), ValidityInvalid, false},
	}
	for _, tt := range tests {
		if got := tt.q.GetValidity(); got != tt.validity {
			t.Errorf("Quality(%#x).GetValidity() = %d, want %d", tt.q, got, tt.validity)
		}
		if got := tt.q.IsGood(); got != tt.good {
			t.Errorf("Quality(%#x).IsGood() = %v, want %v", tt.q, got, tt.good)
		}
	}
}

func TestQuality_DetailFlags(t *testing.T) {
	all := QualityOverflow | QualityOutOfRange | QualityBadReference | QualityOscillatory |
		QualityFailure | QualityOldData | QualityInconsistent | QualityInaccurate |
		QualitySubstituted | QualityTest | QualityOperatorBlocked | QualityDerived

	flags := all.DetailFlags()
	want := []string{
		"overflow", "out-of-range", "bad-reference", "oscillatory",
		"failure", "old-data", "inconsistent", "inaccurate",
		"substituted", "test", "operator-blocked", "derived",
	}
	if len(flags) != len(want) {
		t.Fatalf("DetailFlags() len = %d, want %d (%v)", len(flags), len(want), flags)
	}
	for i := range want {
		if flags[i] != want[i] {
			t.Fatalf("DetailFlags()[%d] = %q, want %q", i, flags[i], want[i])
		}
	}

	if got := Quality(0).DetailFlags(); len(got) != 0 {
		t.Errorf("DetailFlags() empty = %v, want empty", got)
	}
}

func TestQuality_String(t *testing.T) {
	tests := []struct {
		name string
		q    Quality
		want string
	}{
		{"good", Quality(ValidityGood), "good"},
		{"invalid", Quality(ValidityInvalid), "invalid"},
		{"reserved", Quality(ValidityReserved), "reserved"},
		{"questionable", Quality(ValidityQuestionable), "questionable"},
		{"good with flags", QualityOverflow | QualityTest, "good [overflow, test]"},
		{"invalid with flag", Quality(ValidityInvalid) | QualityFailure, "invalid [failure]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.q.String()
			if got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}

	// Ensure joined flags contain expected tokens without relying on order beyond DetailFlags.
	s := (QualityOutOfRange | QualityOldData).String()
	if !strings.Contains(s, "out-of-range") || !strings.Contains(s, "old-data") {
		t.Errorf("String() = %q, want detail flags", s)
	}
}

func TestFromLibQuality(t *testing.T) {
	lib := iec61850.Quality(QualityOverflow | Quality(ValidityQuestionable))
	got := FromLibQuality(lib)
	if got != Quality(lib) {
		t.Errorf("FromLibQuality() = %#x, want %#x", got, Quality(lib))
	}
	if got.GetValidity() != ValidityQuestionable {
		t.Errorf("validity = %d, want questionable", got.GetValidity())
	}
}
