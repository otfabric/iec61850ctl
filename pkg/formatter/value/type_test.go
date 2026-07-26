// SPDX-License-Identifier: MIT

package value

import (
	"testing"

	"github.com/otfabric/go-mms"
)

func TestFormatTypeSpec(t *testing.T) {
	tests := []struct {
		name string
		spec *mms.TypeSpec
		want string
	}{
		{"nil", nil, "UNKNOWN"},
		{"structure empty", &mms.TypeSpec{Type: mms.ValueTypeStructure}, "STRUCT"},
		{"structure with elements", &mms.TypeSpec{Type: mms.ValueTypeStructure, Elements: []mms.TypeSpecElement{{}, {}, {}}}, "STRUCT(3)"},
		{"array no count", &mms.TypeSpec{Type: mms.ValueTypeArray}, "ARRAY"},
		{"array with count", &mms.TypeSpec{Type: mms.ValueTypeArray, Count: 10}, "ARRAY[10]"},
		{"visible string", &mms.TypeSpec{Type: mms.ValueTypeVisibleString}, "STRING"},
		{"mms string", &mms.TypeSpec{Type: mms.ValueTypeMmsString}, "STRING"},
		{"boolean", &mms.TypeSpec{Type: mms.ValueTypeBoolean}, "BOOL"},
		{"float", &mms.TypeSpec{Type: mms.ValueTypeFloat}, "FLOAT"},
		{"utc time", &mms.TypeSpec{Type: mms.ValueTypeUTCTime}, "UTC_TIME"},
		{"binary time", &mms.TypeSpec{Type: mms.ValueTypeBinaryTime}, "BINARY_TIME"},
		{"bit string", &mms.TypeSpec{Type: mms.ValueTypeBitString}, "BIT_STRING"},
		{"octet string", &mms.TypeSpec{Type: mms.ValueTypeOctetString}, "OCTET_STRING"},
		{"int8", &mms.TypeSpec{Type: mms.ValueTypeInteger, Size: 8}, "INT8"},
		{"int16", &mms.TypeSpec{Type: mms.ValueTypeInteger, Size: 16}, "INT16"},
		{"int32", &mms.TypeSpec{Type: mms.ValueTypeInteger, Size: 32}, "INT32"},
		{"int64", &mms.TypeSpec{Type: mms.ValueTypeInteger, Size: 64}, "INT64"},
		{"integer default size", &mms.TypeSpec{Type: mms.ValueTypeInteger, Size: 0}, "INT"},
		{"uint8", &mms.TypeSpec{Type: mms.ValueTypeUnsigned, Size: 8}, "UINT8"},
		{"uint16", &mms.TypeSpec{Type: mms.ValueTypeUnsigned, Size: 16}, "UINT16"},
		{"uint32", &mms.TypeSpec{Type: mms.ValueTypeUnsigned, Size: 32}, "UINT32"},
		{"unsigned default size", &mms.TypeSpec{Type: mms.ValueTypeUnsigned, Size: 0}, "UINT"},
		{"unknown type", &mms.TypeSpec{Type: mms.ValueType(999)}, "TYPE_999"},
		{"bcd", &mms.TypeSpec{Type: mms.ValueTypeBCD}, "BCD"},
		{"obj id", &mms.TypeSpec{Type: mms.ValueTypeObjectIdentifier}, "OBJ_ID"},
		{"generalized time", &mms.TypeSpec{Type: mms.ValueTypeGeneralizedTime}, "GENERALIZED_TIME"},
		{"data access error", &mms.TypeSpec{Type: mms.ValueTypeDataAccessError}, "DATA_ACCESS_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTypeSpec(tt.spec)
			if got != tt.want {
				t.Errorf("FormatTypeSpec() = %q, want %q", got, tt.want)
			}
		})
	}
}
