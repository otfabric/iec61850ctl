// SPDX-License-Identifier: MIT

package formatter

import (
	"fmt"
	"math"
	"strconv"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// ScalarJSONValue returns a typed JSON-native value for the fixture scalar baseline.
// Supported: boolean, integer, unsigned, float, visible/mms string, null.
// Complex encodings (timestamp, bit string, octet string, array, structure) are deferred;
// they currently return a string interim representation via Value.String().
func ScalarJSONValue(v *domain.Value) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch v.Type {
	case domain.TypeFloat:
		// JSON has no NaN/Inf: encoding/json rejects them, which would drop
		// the whole record. Emit the strconv form ("NaN", "+Inf", "-Inf").
		switch f := v.Raw.(type) {
		case float64:
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return strconv.FormatFloat(f, 'g', -1, 64), nil
			}
		case float32:
			if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
				return strconv.FormatFloat(float64(f), 'g', -1, 32), nil
			}
		}
		return v.Raw, nil
	case domain.TypeBoolean, domain.TypeInteger, domain.TypeUnsigned,
		domain.TypeVisibleString, domain.TypeMmsString:
		return v.Raw, nil
	case domain.TypeUnknown:
		if v.Raw == nil {
			return nil, nil
		}
		return v.Raw, nil
	case domain.TypeBitString, domain.TypeOctetString, domain.TypeUtcTime,
		domain.TypeBinaryTime, domain.TypeGeneralizedTime,
		domain.TypeArray, domain.TypeStructure:
		// Deferred stable public encodings — interim string form.
		return v.String(), nil
	default:
		if v.Raw == nil {
			return nil, nil
		}
		return fmt.Sprint(v.Raw), nil
	}
}

// JSONTypeName returns the stable type token used in get object JSON.
func JSONTypeName(t domain.MmsDataType) string {
	switch t {
	case domain.TypeBoolean:
		return "boolean"
	case domain.TypeInteger:
		return "integer"
	case domain.TypeUnsigned:
		return "unsigned"
	case domain.TypeFloat:
		return "float"
	case domain.TypeVisibleString:
		return "visible_string"
	case domain.TypeMmsString:
		return "mms_string"
	case domain.TypeBitString:
		return "bit_string"
	case domain.TypeOctetString:
		return "octet_string"
	case domain.TypeUtcTime:
		return "utc_time"
	case domain.TypeArray:
		return "array"
	case domain.TypeStructure:
		return "structure"
	default:
		if s := t.String(); s != "" {
			return s
		}
		return "unknown"
	}
}
