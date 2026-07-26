// SPDX-License-Identifier: MIT

package service

import (
	"math"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestParseScalarValue(t *testing.T) {
	b, err := ParseScalarValue("true", domain.ScalarBool)
	if err != nil || !b.Bool {
		t.Fatalf("bool: %+v %v", b, err)
	}
	i, err := ParseScalarValue("-3", domain.ScalarInt)
	if err != nil || i.Int != -3 {
		t.Fatalf("int: %+v %v", i, err)
	}
	e, err := ParseScalarValue("2", domain.ScalarEnum)
	if err != nil || e.Int != 2 {
		t.Fatalf("enum: %+v %v", e, err)
	}
	u, err := ParseScalarValue("9", domain.ScalarUint)
	if err != nil || u.Uint != 9 {
		t.Fatalf("uint: %+v %v", u, err)
	}
	f, err := ParseScalarValue("1.5", domain.ScalarFloat)
	if err != nil || f.Float != 1.5 {
		t.Fatalf("float: %+v %v", f, err)
	}
	s, err := ParseScalarValue("hi", domain.ScalarString)
	if err != nil || s.String != "hi" {
		t.Fatalf("string: %+v %v", s, err)
	}
	if _, err := ParseScalarValue("x", domain.ScalarBool); err == nil {
		t.Fatal("expected bool parse error")
	}
	if _, err := ParseScalarValue("x", domain.ScalarKind("nope")); err == nil {
		t.Fatal("expected unsupported kind")
	}
}

func TestScalarConversions(t *testing.T) {
	kinds := []domain.ScalarValue{
		{Kind: domain.ScalarBool, Bool: true},
		{Kind: domain.ScalarInt, Int: 7},
		{Kind: domain.ScalarEnum, Int: 1},
		{Kind: domain.ScalarUint, Uint: 8},
		{Kind: domain.ScalarFloat, Float: 2.5},
		{Kind: domain.ScalarString, String: "a"},
	}
	for _, v := range kinds {
		if ScalarRequestedAny(v) == nil {
			t.Fatalf("requested any nil for %s", v.Kind)
		}
		mv, err := ScalarToMMS(v)
		if err != nil || mv == nil {
			t.Fatalf("to MMS %s: %v", v.Kind, err)
		}
		if !ScalarMatchesMMS(v, mv) {
			t.Fatalf("match failed for %s", v.Kind)
		}
		cv, err := ScalarToCtlVal(v)
		if err != nil || cv == nil {
			t.Fatalf("to ctlVal %s: %v", v.Kind, err)
		}
	}
	if ScalarRequestedAny(domain.ScalarValue{}) != nil {
		t.Fatal("empty kind should request nil")
	}
	if _, err := ScalarToMMS(domain.ScalarValue{}); err == nil {
		t.Fatal("expected MMS unsupported")
	}
	if _, err := ScalarToCtlVal(domain.ScalarValue{}); err == nil {
		t.Fatal("expected ctlVal unsupported")
	}
	if _, err := ScalarToCtlVal(domain.ScalarValue{Kind: domain.ScalarInt, Int: math.MaxInt64}); err == nil {
		t.Fatal("expected int overflow")
	}
	if _, err := ScalarToCtlVal(domain.ScalarValue{Kind: domain.ScalarUint, Uint: math.MaxUint32}); err == nil {
		t.Fatal("expected uint overflow for control")
	}
}

func TestScalarMatchesMMS_EdgeCases(t *testing.T) {
	if ScalarMatchesMMS(domain.ScalarValue{Kind: domain.ScalarBool}, nil) {
		t.Fatal("nil got")
	}
	// unsigned encoded as integer
	want := domain.ScalarValue{Kind: domain.ScalarUint, Uint: 5}
	if !ScalarMatchesMMS(want, mms.NewInteger(5)) {
		t.Fatal("uint via int")
	}
	if ScalarMatchesMMS(domain.ScalarValue{Kind: domain.ScalarFloat, Float: 1}, mms.NewBoolean(true)) {
		t.Fatal("float vs bool")
	}
	if !ScalarMatchesMMS(domain.ScalarValue{Kind: domain.ScalarFloat, Float: 1}, mms.NewFloat(1+1e-9)) {
		t.Fatal("float epsilon")
	}
}

func TestScalarFromIECValue(t *testing.T) {
	if ScalarFromIECValue(nil) != nil {
		t.Fatal("nil")
	}
	if ScalarFromIECValue(iec61850.NewValue(mms.NewBoolean(true))) != true {
		t.Fatal("bool")
	}
	if ScalarFromIECValue(iec61850.NewValue(mms.NewInteger(3))).(int64) != 3 {
		t.Fatal("int")
	}
	if ScalarFromIECValue(iec61850.NewValue(mms.NewUnsigned(4))).(uint64) != 4 {
		t.Fatal("uint")
	}
	if ScalarFromIECValue(iec61850.NewValue(mms.NewFloat(1.25))).(float64) != 1.25 {
		t.Fatal("float")
	}
	if ScalarFromIECValue(iec61850.NewValue(mms.NewVisibleString("x"))) != "x" {
		t.Fatal("string")
	}
}
