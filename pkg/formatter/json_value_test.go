// SPDX-License-Identifier: MIT

package formatter

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestScalarJSONValue_Baseline(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		v    *domain.Value
		want any
	}{
		{"nil", nil, nil},
		{"bool", domain.NewValue(false, domain.TypeBoolean), false},
		{"int", domain.NewValue(int64(1), domain.TypeInteger), int64(1)},
		{"float", domain.NewValue(1234.5, domain.TypeFloat), 1234.5},
		{"string", domain.NewValue("hi", domain.TypeVisibleString), "hi"},
	}
	for _, tc := range cases {
		got, err := ScalarJSONValue(tc.v)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %#v want %#v", tc.name, got, tc.want)
		}
	}
}

func TestParseCLIFormat(t *testing.T) {
	t.Parallel()
	f, err := ParseCLIFormat("json")
	if err != nil || f != OutputFormatJSON {
		t.Fatalf("json: %v %q", err, f)
	}
	if _, err := ParseCLIFormat("yaml"); err == nil {
		t.Fatal("expected error for yaml")
	}
	if _, err := ParseCLIFormat("csv"); err == nil {
		t.Fatal("expected error for csv")
	}
}

func TestParseStreamFormat(t *testing.T) {
	t.Parallel()
	f, err := ParseStreamFormat("jsonl")
	if err != nil || f != StreamFormatJSONL {
		t.Fatalf("jsonl: %v %q", err, f)
	}
	f, err = ParseStreamFormat("")
	if err != nil || f != StreamFormatText {
		t.Fatalf("default text: %v %q", err, f)
	}
	if _, err := ParseStreamFormat("json"); err == nil {
		t.Fatal("expected error for json")
	}
}

func TestJSONTypeName(t *testing.T) {
	t.Parallel()
	cases := map[domain.MmsDataType]string{
		domain.TypeBoolean:       "boolean",
		domain.TypeInteger:       "integer",
		domain.TypeUnsigned:      "unsigned",
		domain.TypeFloat:         "float",
		domain.TypeVisibleString: "visible_string",
		domain.TypeMmsString:     "mms_string",
		domain.TypeBitString:     "bit_string",
		domain.TypeOctetString:   "octet_string",
		domain.TypeUtcTime:       "utc_time",
		domain.TypeArray:         "array",
		domain.TypeStructure:     "structure",
	}
	for typ, want := range cases {
		if got := JSONTypeName(typ); got != want {
			t.Fatalf("%v: got %q want %q", typ, got, want)
		}
	}
}

func TestScalarJSONValue_DeferredComplex(t *testing.T) {
	t.Parallel()
	v := domain.NewValue([]byte{0x01}, domain.TypeOctetString)
	got, err := ScalarJSONValue(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(string); !ok {
		t.Fatalf("deferred complex should be interim string, got %T", got)
	}
}

// Regression (found by FuzzScalarJSONValue): NaN/Inf floats made json.Marshal
// fail, silently dropping jsonl report events.
func TestScalarJSONValue_NonFiniteFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw  any
		want any
	}{
		{math.NaN(), "NaN"},
		{math.Inf(1), "+Inf"},
		{math.Inf(-1), "-Inf"},
		{float32(math.Inf(1)), "+Inf"},
		{1.5, 1.5},
	}
	for _, tt := range tests {
		got, err := ScalarJSONValue(domain.NewValue(tt.raw, domain.TypeFloat))
		if err != nil || got != tt.want {
			t.Errorf("ScalarJSONValue(%v) = %v, %v; want %v", tt.raw, got, err, tt.want)
		}
		if _, err := json.Marshal(got); err != nil {
			t.Errorf("json.Marshal(%v): %v", got, err)
		}
	}
}
