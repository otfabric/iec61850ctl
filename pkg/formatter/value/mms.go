// SPDX-License-Identifier: MIT

package value

import (
	"fmt"
	"strings"
	"time"

	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// FormatLeafValue formats a leaf attribute value for display by MMS type.
// For UTC_TIME accepts domain.Timestamp, time.Time, or milliseconds (uint64/int64).
// attrNameOrPath is optional: when set, BIT_STRING is decoded as quality only if attrNameOrPath is "q" or ends with ".q".
func FormatLeafValue(value interface{}, mmsType domain.MmsDataType, attrNameOrPath string) string {
	if value == nil {
		return "(nil)"
	}
	switch mmsType {
	case domain.TypeUtcTime:
		return formatUtcTimeLeaf(value)
	case domain.TypeBinaryTime:
		return FormatBinaryTime(value)
	case domain.TypeBitString:
		if isQualityAttr(attrNameOrPath) {
			return FormatQualityValue(value)
		}
		return formatBitStringRaw(value)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func formatUtcTimeLeaf(value interface{}) string {
	switch v := value.(type) {
	case domain.Timestamp:
		return FormatUtcTimestamp(v)
	case *domain.Timestamp:
		if v == nil {
			return "(nil)"
		}
		return FormatUtcTimestamp(*v)
	case time.Time:
		return FormatUtcTimeFromTime(v, 0)
	case uint64:
		return FormatUtcTimeValue(v, 0)
	case int64:
		return FormatUtcTimeValue(uint64(v), 0)
	default:
		return fmt.Sprintf("<error: UTC_TIME value is not Timestamp or time.Time (got %T)>", value)
	}
}

// FormatAttributeValueWithError returns a display string for an attribute value, or an error message if err is non-nil.
func FormatAttributeValueWithError(value interface{}, mmsType domain.MmsDataType, err error, attrNameOrPath string) string {
	if err != nil {
		return fmt.Sprintf("<error: %v>", err)
	}
	return FormatLeafValue(value, mmsType, attrNameOrPath)
}

// FormatDataAttributeValue returns the display string for a domain.DataAttribute.
func FormatDataAttributeValue(da *domain.DataAttribute) string {
	if da == nil {
		return "(nil)"
	}
	if da.ValueError != "" {
		return fmt.Sprintf("<error: %s>", da.ValueError)
	}
	if da.Value != nil {
		if da.Value.Display != "" {
			return da.Value.Display
		}
		return da.Value.String()
	}
	return "(nil)"
}

// FormatMmsValue formats a go-mms Value for display using its type and payload.
func FormatMmsValue(mv *mms.Value) string {
	if mv == nil {
		return "(nil)"
	}
	switch mv.Type() {
	case mms.ValueTypeArray, mms.ValueTypeStructure:
		els, ok := arrayOrStructureElements(mv)
		if !ok {
			return mv.String()
		}
		parts := make([]string, 0, len(els))
		for _, el := range els {
			parts = append(parts, FormatMmsValue(el))
		}
		return "[" + strings.Join(parts, " ") + "]"
	case mms.ValueTypeUTCTime:
		t, ok := mv.UTCTime()
		if !ok {
			return "(nil)"
		}
		return FormatUtcTimeFromTime(t, mv.UTCTimeQuality())
	default:
		mmsType := domain.FromMMSValueType(mv.Type())
		raw := mmsLeafRaw(mv)
		return FormatLeafValue(raw, mmsType, "")
	}
}

// FormatDomainValue formats a domain.Value for display.
func FormatDomainValue(v *domain.Value) string {
	if v == nil {
		return "(nil)"
	}
	if v.Display != "" {
		return v.Display
	}
	switch v.Type {
	case domain.TypeArray, domain.TypeStructure:
		children, ok := v.Raw.([]*domain.Value)
		if !ok {
			return v.String()
		}
		parts := make([]string, 0, len(children))
		for _, child := range children {
			parts = append(parts, FormatDomainValue(child))
		}
		return "[" + strings.Join(parts, " ") + "]"
	default:
		return FormatLeafValue(v.Raw, v.Type, "")
	}
}

func arrayOrStructureElements(mv *mms.Value) ([]*mms.Value, bool) {
	if els, ok := mv.ArrayElements(); ok {
		return els, true
	}
	return mv.Structure()
}

func mmsLeafRaw(mv *mms.Value) interface{} {
	switch mv.Type() {
	case mms.ValueTypeBoolean:
		b, _ := mv.Bool()
		return b
	case mms.ValueTypeInteger:
		i, _ := mv.Int64()
		return i
	case mms.ValueTypeUnsigned:
		u, _ := mv.Uint64()
		return u
	case mms.ValueTypeFloat, mms.ValueTypeReal:
		f, _ := mv.Float64()
		return f
	case mms.ValueTypeBitString:
		bits, _ := mv.BitString()
		return bits
	case mms.ValueTypeOctetString:
		b, _ := mv.OctetString()
		return b
	case mms.ValueTypeVisibleString:
		s, _ := mv.VisibleString()
		return s
	case mms.ValueTypeMmsString:
		s, _ := mv.MmsString()
		return s
	case mms.ValueTypeBinaryTime:
		ms, ok := mv.BinaryTime()
		if ok {
			return ms
		}
		return nil
	case mms.ValueTypeGeneralizedTime:
		t, _ := mv.GeneralizedTime()
		return t
	case mms.ValueTypeBCD:
		bcd, _ := mv.BCD()
		return bcd
	case mms.ValueTypeObjectIdentifier:
		oid, _ := mv.ObjectIdentifier()
		return oid
	case mms.ValueTypeDataAccessError:
		code, _ := mv.DataAccessErr()
		return code
	default:
		return mv.String()
	}
}
