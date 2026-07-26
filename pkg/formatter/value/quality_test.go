// SPDX-License-Identifier: MIT

package value

import (
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestFormatQuality(t *testing.T) {
	tests := []struct {
		name string
		q    *domain.Quality
		want string
	}{
		{"nil", nil, ""},
		{"good", ptrQuality(domain.Quality(domain.ValidityGood)), "good"},
		{"invalid", ptrQuality(domain.Quality(domain.ValidityInvalid)), "invalid"},
		{"reserved", ptrQuality(domain.Quality(domain.ValidityReserved)), "reserved"},
		{"questionable", ptrQuality(domain.Quality(domain.ValidityQuestionable)), "questionable"},
		{
			"good with overflow",
			ptrQuality(domain.Quality(domain.ValidityGood) | domain.QualityOverflow),
			"good,overflow",
		},
		{
			"all detail flags",
			ptrQuality(domain.Quality(domain.ValidityInvalid) |
				domain.QualityOverflow |
				domain.QualityOutOfRange |
				domain.QualityBadReference |
				domain.QualityOscillatory |
				domain.QualityFailure |
				domain.QualityOldData |
				domain.QualityInconsistent |
				domain.QualityInaccurate |
				domain.QualitySubstituted |
				domain.QualityTest |
				domain.QualityOperatorBlocked),
			"invalid,overflow,out-of-range,bad-ref,oscillatory,failure,old-data,inconsistent,inaccurate,substituted,test,operator-blocked",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatQuality(tt.q)
			if got != tt.want {
				t.Errorf("FormatQuality() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatQualityValue(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		wantSubstr []string
	}{
		{
			"uint16 good",
			uint16(0),
			[]string{"0x0000", "[good]", "details: none"},
		},
		{
			"int32 invalid",
			int32(domain.ValidityInvalid),
			[]string{"0x0001", "[invalid]", "details: none"},
		},
		{
			"uint32 reserved",
			uint32(domain.ValidityReserved),
			[]string{"0x0002", "[reserved]"},
		},
		{
			"int questionable",
			int(domain.ValidityQuestionable),
			[]string{"0x0003", "[questionable]"},
		},
		{
			"bytes two",
			[]byte{0x00, 0x04}, // overflow + good
			[]string{"0x0004", "[good]", "overflow"},
		},
		{
			"bytes one",
			[]byte{0x01},
			[]string{"0x0001", "[invalid]"},
		},
		{
			"bytes empty",
			[]byte{},
			[]string{"0x0000", "[good]", "details: none"},
		},
		{
			"all flags",
			uint16(domain.ValidityQuestionable) |
				uint16(domain.QualityOverflow) |
				uint16(domain.QualityOutOfRange) |
				uint16(domain.QualityBadReference) |
				uint16(domain.QualityOscillatory) |
				uint16(domain.QualityFailure) |
				uint16(domain.QualityOldData) |
				uint16(domain.QualityInconsistent) |
				uint16(domain.QualityInaccurate) |
				uint16(domain.QualitySubstituted) |
				uint16(domain.QualityTest) |
				uint16(domain.QualityOperatorBlocked),
			[]string{
				"[questionable]",
				"overflow", "out-of-range", "bad-ref", "oscillatory", "failure",
				"old-data", "inconsistent", "inaccurate", "substituted", "test", "operator-blocked",
			},
		},
		{
			"unsupported type",
			"not-a-quality",
			[]string{"not-a-quality (raw)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatQualityValue(tt.value)
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(got, sub) {
					t.Errorf("FormatQualityValue(%v) = %q, want containing %q", tt.value, got, sub)
				}
			}
		})
	}
}

func ptrQuality(q domain.Quality) *domain.Quality {
	return &q
}
