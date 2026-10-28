// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// FuzzParseObjectReference checks that parsing never panics, that errors are
// classified as ErrInvalidReference, and that successful parses round-trip
// through String().
func FuzzParseObjectReference(f *testing.F) {
	for _, s := range []string{
		"", "/", "LD0", "LD0/", "LD0/LLN0", "LD0/LLN0.", "LD0/LLN0.Mod.stVal",
		"ZX2REX640A1LD0/FMMXU1.Hz.mag.f", "LD/LN..x", "/LN.DO", "a/b/c.d",
		"LD0/LLN0.Mod.stVal[ST]", "LD0/LLN0$ST$Mod$stVal", "ü/ñ.ß",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := ParseObjectReference(s)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidReference) {
				t.Fatalf("error %v not classified as ErrInvalidReference", err)
			}
			return
		}
		if r.String() != strings.TrimSuffix(s, ".") && r.String() != s {
			t.Fatalf("String() = %q, input %q", r.String(), s)
		}
		again, err := ParseObjectReference(r.String())
		if err != nil {
			t.Fatalf("reparse %q: %v", r.String(), err)
		}
		if !reflect.DeepEqual(again, r) {
			t.Fatalf("round trip mismatch: %+v vs %+v", again, r)
		}
		_ = r.DOName()
		_ = r.DAPath()
		_ = r.Parent()
		if c := r.Child("x"); len(c.Path) != len(r.Path)+1 {
			t.Fatalf("Child path len %d", len(c.Path))
		}
	})
}

// FuzzParseRCBDataSetJournalRefs fuzzes the report, dataset and journal
// reference parsers: success implies the builder reproduces the input.
func FuzzParseRCBDataSetJournalRefs(f *testing.F) {
	for _, s := range []string{
		"", "LD0/LLN0.BR.rcb1", "LD0/LLN0.RP.urcb01", "LD0/LLN0..rcb", "LD0/LLN0.BR.",
		"LD0/LLN0$ds1", "LD0/LLN0.ds1", "InteropLD/LLN0.dsInterop", "LD0/LLN0$EventLog",
		"LD0/$x", "/LLN0$x", "LD0/LLN0$", "LD0/LLN0.BR.rcb.extra", "a/b/c$d",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if ld, ln, fc, rcb, ok := ParseReportRef(s); ok {
			if ld == "" || ln == "" || rcb == "" {
				t.Fatalf("ParseReportRef(%q) ok with empty part", s)
			}
			if got := ReportRef(ld, ln, fc, rcb); got != s {
				t.Fatalf("ReportRef rebuild = %q, input %q", got, s)
			}
		}
		if ld, ln, ds, ok := ParseDataSetRef(s); ok {
			if ld == "" || ln == "" || ds == "" {
				t.Fatalf("ParseDataSetRef(%q) ok with empty part", s)
			}
			if a, b := ld+"/"+ln+"$"+ds, ld+"/"+ln+"."+ds; a != s && b != s {
				t.Fatalf("ParseDataSetRef rebuild mismatch for %q", s)
			}
		}
		if ld, ln, logName, ok := ParseJournalRef(s); ok {
			if got := JournalRef(ld, ln, logName); got != s {
				t.Fatalf("JournalRef rebuild = %q, input %q", got, s)
			}
		}
	})
}

// FuzzParseTimeToUnixMs checks the journal time parser never panics, that
// errors are ErrInvalidInput, and that successes are consistent.
func FuzzParseTimeToUnixMs(f *testing.F) {
	for _, s := range []string{
		"", " ", "0", "1700000000000", "18446744073709551615", "18446744073709551616",
		"99999999999999999999999", "2024-10-30T12:00:00Z", "2024-10-30T12:00:00.123456789+02:00",
		"2024-10-30T12:00:00", "2024-10-30 12:00:00.000", "2024-10-30 12:00:00",
		"1969-12-31T23:59:59Z", "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z",
		"2262-04-12T00:00:00Z", "-1", "12ab", "２０２４",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ms, err := ParseTimeToUnixMs(s)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("ParseTimeToUnixMs(%q) error %v not ErrInvalidInput", s, err)
			}
			return
		}
		if ms > math.MaxInt64 {
			t.Fatalf("ParseTimeToUnixMs(%q) = %d overflows int64 (wrapped negative time?)", s, ms)
		}
		trimmed := strings.TrimSpace(s)
		if isDigitsOnly(trimmed) {
			want, perr := strconv.ParseUint(trimmed, 10, 64)
			if perr != nil || want != ms {
				t.Fatalf("digits %q -> %d, strconv %d (%v)", s, ms, want, perr)
			}
		}
		if ms > 0 && FormatTimestampMs(ms) == "" {
			t.Fatalf("FormatTimestampMs(%d) empty", ms)
		}
		_ = FromUtcTimeValue(ms, 0x80).String()
	})
}

// FuzzControlParsers fuzzes the control flag parsers (mode, or-cat, type).
func FuzzControlParsers(f *testing.F) {
	for _, s := range []string{
		"", "auto", "DIRECT", " sbo ", "sbow", "sbo-with", "remote-control", "process",
		"maintenance", "bay-control", "bool", "Boolean", "int", "integer", "uint", "unsigned",
		"float", "double", "enum", "string", "str", "\x00", "ＳＢＯ",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		norm := strings.ToLower(strings.TrimSpace(s))

		mode, err := ParseControlMode(s)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("ParseControlMode error %v not ErrInvalidInput", err)
			}
		} else {
			switch mode {
			case ControlModeAuto, ControlModeDirect, ControlModeSBO, ControlModeSBOw:
			default:
				t.Fatalf("ParseControlMode(%q) = %q", s, mode)
			}
			if again, err := ParseControlMode(string(mode)); err != nil || again != mode {
				t.Fatalf("mode round trip %q: %q %v", mode, again, err)
			}
		}

		cat, err := ParseOriginCategory(s)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("ParseOriginCategory error %v not ErrInvalidInput", err)
			}
		} else {
			code := cat.OrCatCode()
			if code < 0 || code > 8 {
				t.Fatalf("OrCatCode(%q) = %d", cat, code)
			}
			if again, err := ParseOriginCategory(string(cat)); err != nil || again != cat {
				t.Fatalf("or-cat round trip %q: %q %v", cat, again, err)
			}
			if norm != "" && string(cat) != norm {
				t.Fatalf("or-cat %q parsed as %q", s, cat)
			}
		}

		kind, err := ParseScalarKind(s)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("ParseScalarKind error %v not ErrInvalidInput", err)
			}
		} else if again, err := ParseScalarKind(string(kind)); err != nil || again != kind {
			t.Fatalf("kind round trip %q: %q %v", kind, again, err)
		}
	})
}
