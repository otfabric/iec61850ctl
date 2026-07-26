// SPDX-License-Identifier: MIT

package value

import (
	"fmt"

	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// FormatTypeSpec formats a go-mms TypeSpec into a human-readable type string.
// For structures and arrays, it includes element counts. Returns "UNKNOWN" if spec is nil.
func FormatTypeSpec(spec *mms.TypeSpec) string {
	if spec == nil {
		return "UNKNOWN"
	}

	switch spec.Type {
	case mms.ValueTypeStructure:
		if len(spec.Elements) > 0 {
			return fmt.Sprintf("STRUCT(%d)", len(spec.Elements))
		}
		return "STRUCT"
	case mms.ValueTypeArray:
		if spec.Count > 0 {
			return fmt.Sprintf("ARRAY[%d]", spec.Count)
		}
		return "ARRAY"
	case mms.ValueTypeVisibleString, mms.ValueTypeMmsString:
		return "STRING"
	default:
		typeStr := formatTypeSpecBaseType(spec.Type, spec.Size)
		if typeStr == "UNKNOWN" {
			return fmt.Sprintf("TYPE_%d", spec.Type)
		}
		return typeStr
	}
}

func formatTypeSpecBaseType(t mms.ValueType, size int) string {
	switch t {
	case mms.ValueTypeInteger:
		switch size {
		case 8:
			return string(domain.TypeInt8)
		case 16:
			return string(domain.TypeInt16)
		case 32:
			return string(domain.TypeInt32)
		case 64:
			return string(domain.TypeInt64)
		default:
			return string(domain.TypeInteger)
		}
	case mms.ValueTypeUnsigned:
		switch size {
		case 8:
			return string(domain.TypeUint8)
		case 16:
			return string(domain.TypeUint16)
		case 32:
			return string(domain.TypeUint32)
		default:
			return string(domain.TypeUnsigned)
		}
	default:
		return domain.FromMMSValueType(t).String()
	}
}
