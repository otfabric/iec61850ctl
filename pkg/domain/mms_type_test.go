// SPDX-License-Identifier: MIT

package domain

import (
	"testing"

	"github.com/otfabric/go-mms"
)

func TestMmsDataType_String(t *testing.T) {
	if got := TypeFloat.String(); got != "FLOAT" {
		t.Errorf("TypeFloat.String() = %q", got)
	}
	if got := MmsDataType("").String(); got != "UNKNOWN" {
		t.Errorf("empty String() = %q, want UNKNOWN", got)
	}
}

func TestMmsDataType_Predicates(t *testing.T) {
	leafCases := []struct {
		t    MmsDataType
		leaf bool
	}{
		{TypeBoolean, true},
		{TypeFloat, true},
		{TypeStructure, false},
		{TypeArray, false},
		{TypeUnknown, false},
		{"", false},
	}
	for _, tt := range leafCases {
		if got := tt.t.IsLeaf(); got != tt.leaf {
			t.Errorf("%q.IsLeaf() = %v, want %v", tt.t, got, tt.leaf)
		}
	}

	numeric := []MmsDataType{
		TypeInteger, TypeUnsigned, TypeFloat, TypeBcd,
		TypeInt8, TypeInt16, TypeInt32, TypeInt64,
		TypeUint8, TypeUint16, TypeUint32,
	}
	for _, nt := range numeric {
		if !nt.IsNumeric() {
			t.Errorf("%s.IsNumeric() = false", nt)
		}
	}
	if TypeBoolean.IsNumeric() {
		t.Error("TypeBoolean.IsNumeric() = true")
	}

	for _, st := range []MmsDataType{TypeVisibleString, TypeMmsString, TypeObjId} {
		if !st.IsString() {
			t.Errorf("%s.IsString() = false", st)
		}
	}
	if TypeOctetString.IsString() {
		t.Error("TypeOctetString.IsString() = true")
	}

	for _, tt := range []MmsDataType{TypeUtcTime, TypeBinaryTime, TypeGeneralizedTime} {
		if !tt.IsTime() {
			t.Errorf("%s.IsTime() = false", tt)
		}
	}
	if TypeFloat.IsTime() {
		t.Error("TypeFloat.IsTime() = true")
	}
}

func TestFromMMSValueType_FromLibMmsType(t *testing.T) {
	tests := []struct {
		in   mms.ValueType
		want MmsDataType
	}{
		{mms.ValueTypeArray, TypeArray},
		{mms.ValueTypeStructure, TypeStructure},
		{mms.ValueTypeBoolean, TypeBoolean},
		{mms.ValueTypeBitString, TypeBitString},
		{mms.ValueTypeInteger, TypeInteger},
		{mms.ValueTypeUnsigned, TypeUnsigned},
		{mms.ValueTypeFloat, TypeFloat},
		{mms.ValueTypeReal, TypeFloat},
		{mms.ValueTypeOctetString, TypeOctetString},
		{mms.ValueTypeVisibleString, TypeVisibleString},
		{mms.ValueTypeGeneralizedTime, TypeGeneralizedTime},
		{mms.ValueTypeBinaryTime, TypeBinaryTime},
		{mms.ValueTypeBCD, TypeBcd},
		{mms.ValueTypeObjectIdentifier, TypeObjId},
		{mms.ValueTypeMmsString, TypeMmsString},
		{mms.ValueTypeUTCTime, TypeUtcTime},
		{mms.ValueTypeDataAccessError, TypeDataAccessError},
		{mms.ValueType(999), TypeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.want.String(), func(t *testing.T) {
			got := FromMMSValueType(tt.in)
			if got != tt.want {
				t.Fatalf("FromMMSValueType(%v) = %q, want %q", tt.in, got, tt.want)
			}
			if alias := FromLibMmsType(tt.in); alias != tt.want {
				t.Fatalf("FromLibMmsType(%v) = %q, want %q", tt.in, alias, tt.want)
			}
		})
	}
}
