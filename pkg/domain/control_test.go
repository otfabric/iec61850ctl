// SPDX-License-Identifier: MIT

package domain

import "testing"

func TestParseControlMode(t *testing.T) {
	cases := []struct {
		in   string
		want ControlMode
		ok   bool
	}{
		{"", ControlModeAuto, true},
		{"auto", ControlModeAuto, true},
		{"DIRECT", ControlModeDirect, true},
		{"sbo", ControlModeSBO, true},
		{"sbow", ControlModeSBOw, true},
		{"force", "", false},
	}
	for _, tc := range cases {
		got, err := ParseControlMode(tc.in)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Fatalf("%q: got %q err=%v want %q", tc.in, got, err, tc.want)
			}
		} else if err == nil {
			t.Fatalf("%q: expected error", tc.in)
		}
	}
}

func TestControlStatus_ExitNonZero(t *testing.T) {
	if ControlStatusOperated.ExitNonZero() || ControlStatusConfirmed.ExitNonZero() || ControlStatusPlanned.ExitNonZero() {
		t.Fatal("success statuses must exit 0")
	}
	for _, s := range []ControlStatus{
		ControlStatusFailed,
		ControlStatusOperatedUnconfirmed,
		ControlStatusConfirmationMismatch,
	} {
		if !s.ExitNonZero() {
			t.Fatalf("%s should exit non-zero", s)
		}
	}
}

func TestParseOriginCategoryAndCode(t *testing.T) {
	cases := []struct {
		in   string
		code int
	}{
		{"", 3},
		{"not-supported", 0},
		{"bay-control", 1},
		{"station-control", 2},
		{"remote-control", 3},
		{"automatic-bay", 4},
		{"automatic-station", 5},
		{"automatic-remote", 6},
		{"maintenance", 7},
		{"process", 8},
	}
	for _, tc := range cases {
		got, err := ParseOriginCategory(tc.in)
		if err != nil || got.OrCatCode() != tc.code {
			t.Fatalf("%q: code=%d err=%v want %d", tc.in, got.OrCatCode(), err, tc.code)
		}
	}
	if _, err := ParseOriginCategory("nope"); err == nil {
		t.Fatal("expected invalid or-cat error")
	}
	if OriginCategory("x").OrCatCode() != 3 {
		t.Fatal("unknown category defaults to remote-control code 3")
	}
}

func TestParseScalarKind(t *testing.T) {
	cases := map[string]ScalarKind{
		"bool": "bool", "boolean": ScalarBool,
		"int": ScalarInt, "integer": ScalarInt,
		"uint": ScalarUint, "unsigned": ScalarUint,
		"float": ScalarFloat, "double": ScalarFloat,
		"enum": ScalarEnum, "string": ScalarString,
	}
	for in, want := range cases {
		got, err := ParseScalarKind(in)
		if err != nil || got != want {
			t.Fatalf("%q: got %q err=%v want %q", in, got, err, want)
		}
	}
	if _, err := ParseScalarKind("blob"); err == nil {
		t.Fatal("expected error")
	}
}
