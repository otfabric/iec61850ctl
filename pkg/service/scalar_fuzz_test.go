// SPDX-License-Identifier: MIT

package service

import (
	"errors"
	"math"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// FuzzParseScalarValue exercises the `set object --type/--value` path:
// domain.ParseScalarKind + ParseScalarValue. Invariant: a successful parse
// encodes via ScalarToMMS / ScalarToCtlVal without panicking, and the MMS
// value round-trips (ScalarMatchesMMS) for every finite value.
func FuzzParseScalarValue(f *testing.F) {
	seeds := []struct{ kind, raw string }{
		{"bool", "true"}, {"boolean", "0"}, {"bool", "yes"},
		{"int", "-42"}, {"integer", "9223372036854775807"}, {"int", "9223372036854775808"},
		{"int", "2147483648"}, {"enum", "3"}, {"enum", "-2147483649"},
		{"uint", "18446744073709551615"}, {"unsigned", "-1"}, {"uint", "2147483648"},
		{"float", "1.5"}, {"double", "1e308"}, {"float", "NaN"}, {"float", "+Inf"},
		{"float", "0x1p-1074"}, {"float", "3.4e39"},
		{"string", ""}, {"string", "héllo\x00"}, {"str", "x"}, {" INT ", " 7 "},
	}
	for _, s := range seeds {
		f.Add(s.kind, s.raw)
	}
	f.Fuzz(func(t *testing.T, kindStr, raw string) {
		kind, err := domain.ParseScalarKind(kindStr)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("ParseScalarKind error %v not ErrInvalidInput", err)
			}
			return
		}
		v, err := ParseScalarValue(raw, kind)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("ParseScalarValue error %v not ErrInvalidInput", err)
			}
			return
		}
		if v.Kind != kind {
			t.Fatalf("kind = %q, want %q", v.Kind, kind)
		}
		if ScalarRequestedAny(v) == nil {
			t.Fatalf("ScalarRequestedAny(%+v) = nil", v)
		}
		mv, err := ScalarToMMS(v)
		if err != nil || mv == nil {
			t.Fatalf("ScalarToMMS(%+v) = %v, %v", v, mv, err)
		}
		finite := kind != domain.ScalarFloat || (!math.IsNaN(v.Float) && !math.IsInf(v.Float, 0))
		if finite && !ScalarMatchesMMS(v, mv) {
			t.Fatalf("ScalarMatchesMMS(%+v, %v) = false", v, mv)
		}
		if got := ScalarFromIECValue(iec61850.NewValue(mv)); got == nil {
			t.Fatalf("ScalarFromIECValue(%v) = nil", mv)
		}
		ctl, err := ScalarToCtlVal(v)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("ScalarToCtlVal error %v not ErrInvalidInput", err)
			}
		} else if ctl == nil {
			t.Fatalf("ScalarToCtlVal(%+v) = nil, nil", v)
		}
	})
}
