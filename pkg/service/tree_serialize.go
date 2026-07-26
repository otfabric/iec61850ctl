// SPDX-License-Identifier: MIT

package service

import (
	"fmt"
	"time"

	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// ValueToModel converts a leaf value and MMS type into a domain.Value.
// Returns the typed value container and an error string (empty if successful).
func ValueToModel(value interface{}, mmsType mms.ValueType) (*domain.Value, string) {
	if value == nil {
		return nil, ""
	}

	modelType := domain.FromMMSValueType(mmsType)
	var raw interface{}

	switch mmsType {
	case mms.ValueTypeBoolean:
		if b, ok := value.(bool); ok {
			raw = b
		}
	case mms.ValueTypeInteger:
		switch v := value.(type) {
		case int32:
			raw = v
		case int64:
			raw = v
		case int:
			raw = v
		default:
			raw = value
		}
	case mms.ValueTypeUnsigned:
		switch v := value.(type) {
		case uint32:
			raw = v
		case uint16:
			raw = v
		case uint8:
			raw = v
		default:
			raw = value
		}
	case mms.ValueTypeFloat, mms.ValueTypeReal:
		if f, ok := value.(float32); ok {
			raw = f
		} else if f, ok := value.(float64); ok {
			raw = f
		}
	case mms.ValueTypeVisibleString, mms.ValueTypeMmsString:
		if s, ok := value.(string); ok {
			raw = s
		}
	case mms.ValueTypeBitString:
		raw = formatBitStringForJSON(value)
	case mms.ValueTypeOctetString:
		raw = formatOctetStringForJSON(value)
	case mms.ValueTypeUTCTime:
		raw = formatUtcTimeForJSON(value)
	case mms.ValueTypeBinaryTime:
		raw = formatBinaryTimeForJSON(value)
	default:
		raw = value
	}

	if raw == nil {
		return nil, fmt.Sprintf("unsupported type or value: %T", value)
	}

	return domain.NewValue(raw, modelType), ""
}

func formatBitStringForJSON(value interface{}) string {
	var q uint16
	switch v := value.(type) {
	case uint16:
		q = v
	case int32:
		q = uint16(v)
	case uint32:
		q = uint16(v)
	case []byte:
		if len(v) >= 2 {
			q = uint16(v[0])<<8 | uint16(v[1])
		} else if len(v) == 1 {
			q = uint16(v[0])
		}
	default:
		return fmt.Sprintf("0x%04x", 0)
	}
	return fmt.Sprintf("0x%04x", q)
}

func formatOctetStringForJSON(value interface{}) string {
	if b, ok := value.([]byte); ok {
		return fmt.Sprintf("%x", b)
	}
	return ""
}

type utcTimeSerialized struct {
	Seconds      int64  `json:"seconds"`
	Milliseconds uint16 `json:"milliseconds"`
	TimeQuality  uint8  `json:"time_quality"`
}

func formatUtcTimeForJSON(value interface{}) interface{} {
	switch v := value.(type) {
	case time.Time:
		ms := v.UTC().UnixMilli()
		return utcTimeSerialized{
			Seconds:      ms / 1000,
			Milliseconds: uint16(ms % 1000),
		}
	case *time.Time:
		if v != nil {
			return formatUtcTimeForJSON(*v)
		}
	}
	return nil
}

func formatBinaryTimeForJSON(value interface{}) interface{} {
	switch v := value.(type) {
	case int64:
		return v
	case uint64:
		return v
	case int:
		return int64(v)
	default:
		return value
	}
}
