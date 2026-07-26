// SPDX-License-Identifier: MIT

package domain

import (
	"reflect"
	"testing"
	"time"

	"github.com/otfabric/go-mms"
)

func TestNewValue(t *testing.T) {
	v := NewValue(42, TypeInteger)
	if v == nil {
		t.Fatal("NewValue returned nil")
	}
	if v.Raw != 42 || v.Type != TypeInteger {
		t.Fatalf("NewValue = %+v", v)
	}
}

func TestValue_String(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want string
	}{
		{"nil raw", Value{Type: TypeInteger}, "<nil>"},
		{"display override", Value{Raw: 1, Display: "ON", Type: TypeBoolean}, "ON"},
		{"int", Value{Raw: int64(7), Type: TypeInteger}, "7"},
		{"float", Value{Raw: 1.5, Type: TypeFloat}, "1.5"},
		{"string", Value{Raw: "hello", Type: TypeVisibleString}, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValueFromMMS(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()

	tests := []struct {
		name     string
		mv       *mms.Value
		wantNil  bool
		wantType MmsDataType
		check    func(t *testing.T, v *Value)
	}{
		{
			name:    "nil",
			mv:      nil,
			wantNil: true,
		},
		{
			name:     "boolean",
			mv:       mms.NewBoolean(true),
			wantType: TypeBoolean,
			check: func(t *testing.T, v *Value) {
				if v.Raw != true {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "integer",
			mv:       mms.NewInteger(-9),
			wantType: TypeInteger,
			check: func(t *testing.T, v *Value) {
				if v.Raw != int64(-9) {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "unsigned",
			mv:       mms.NewUnsigned(99),
			wantType: TypeUnsigned,
			check: func(t *testing.T, v *Value) {
				if v.Raw != uint64(99) {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "float",
			mv:       mms.NewFloat(3.25),
			wantType: TypeFloat,
			check: func(t *testing.T, v *Value) {
				if v.Raw != 3.25 {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "real",
			mv:       mms.NewReal(2.5),
			wantType: TypeFloat,
		},
		{
			name:     "visible string",
			mv:       mms.NewVisibleString("abc"),
			wantType: TypeVisibleString,
			check: func(t *testing.T, v *Value) {
				if v.Raw != "abc" {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "mms string",
			mv:       mms.NewMmsString("mms"),
			wantType: TypeMmsString,
			check: func(t *testing.T, v *Value) {
				if v.Raw != "mms" {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "octet string",
			mv:       mms.NewOctetString([]byte{1, 2}),
			wantType: TypeOctetString,
			check: func(t *testing.T, v *Value) {
				b, ok := v.Raw.([]byte)
				if !ok || !reflect.DeepEqual(b, []byte{1, 2}) {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "bit string",
			mv:       mms.NewBitString([]byte{0xF0}),
			wantType: TypeBitString,
			check: func(t *testing.T, v *Value) {
				b, ok := v.Raw.([]byte)
				if !ok || !reflect.DeepEqual(b, []byte{0xF0}) {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "utc time",
			mv:       mms.NewUTCTime(now),
			wantType: TypeUtcTime,
			check: func(t *testing.T, v *Value) {
				tm, ok := v.Raw.(time.Time)
				if !ok || !tm.Equal(now) {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "binary time",
			mv:       mms.NewBinaryTime(12345),
			wantType: TypeBinaryTime,
			check: func(t *testing.T, v *Value) {
				if v.Raw != int64(12345) {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "array",
			mv:       mms.NewArray([]*mms.Value{mms.NewInteger(1), mms.NewInteger(2)}),
			wantType: TypeArray,
			check: func(t *testing.T, v *Value) {
				children, ok := v.Raw.([]*Value)
				if !ok || len(children) != 2 {
					t.Fatalf("Raw = %v", v.Raw)
				}
				if children[0].Raw != int64(1) || children[1].Raw != int64(2) {
					t.Fatalf("children = %+v", children)
				}
			},
		},
		{
			name:     "structure",
			mv:       mms.NewStructure([]*mms.Value{mms.NewBoolean(false), mms.NewVisibleString("x")}),
			wantType: TypeStructure,
			check: func(t *testing.T, v *Value) {
				children, ok := v.Raw.([]*Value)
				if !ok || len(children) != 2 {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "empty array",
			mv:       mms.NewArray(nil),
			wantType: TypeArray,
			check: func(t *testing.T, v *Value) {
				children, ok := v.Raw.([]*Value)
				if !ok || len(children) != 0 {
					t.Fatalf("Raw = %v", v.Raw)
				}
			},
		},
		{
			name:     "default / data access error",
			mv:       mms.NewDataAccessError(mms.DataAccessErrorObjectNonExistent),
			wantType: TypeDataAccessError,
			check: func(t *testing.T, v *Value) {
				if v.Raw == nil {
					t.Fatal("Raw is nil for default branch")
				}
			},
		},
		{
			name:     "generalized time default branch",
			mv:       mms.NewGeneralizedTime(now),
			wantType: TypeGeneralizedTime,
		},
		{
			name:     "bcd default branch",
			mv:       mms.NewBCD(12),
			wantType: TypeBcd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValueFromMMS(tt.mv)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("ValueFromMMS(nil) = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("ValueFromMMS returned nil")
			}
			if got.Type != tt.wantType {
				t.Fatalf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if tt.check != nil {
				tt.check(t, got)
			}

			alias := ValueFromMmsValue(tt.mv)
			if alias == nil || alias.Type != got.Type {
				t.Fatalf("ValueFromMmsValue alias mismatch: %+v vs %+v", alias, got)
			}
		})
	}
}
