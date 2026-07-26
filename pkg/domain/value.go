// SPDX-License-Identifier: MIT

package domain

import (
	"fmt"

	"github.com/otfabric/go-mms"
)

// Value is a typed container for any MMS leaf or constructed value.
type Value struct {
	Raw     interface{}
	Type    MmsDataType
	Display string
}

// NewValue creates a Value from a raw Go value and its MMS type.
func NewValue(raw interface{}, mmsType MmsDataType) *Value {
	return &Value{Raw: raw, Type: mmsType}
}

// ValueFromMMS converts a go-mms Value to domain.Value.
func ValueFromMMS(mv *mms.Value) *Value {
	if mv == nil {
		return nil
	}
	mmsType := FromMMSValueType(mv.Type())

	switch mv.Type() {
	case mms.ValueTypeArray:
		els, ok := mv.ArrayElements()
		if !ok {
			return NewValue(nil, mmsType)
		}
		children := make([]*Value, 0, len(els))
		for _, el := range els {
			children = append(children, ValueFromMMS(el))
		}
		return NewValue(children, mmsType)
	case mms.ValueTypeStructure:
		els, ok := mv.Structure()
		if !ok {
			return NewValue(nil, mmsType)
		}
		children := make([]*Value, 0, len(els))
		for _, el := range els {
			children = append(children, ValueFromMMS(el))
		}
		return NewValue(children, mmsType)
	case mms.ValueTypeBoolean:
		b, _ := mv.Bool()
		return NewValue(b, mmsType)
	case mms.ValueTypeInteger:
		i, _ := mv.Int64()
		return NewValue(i, mmsType)
	case mms.ValueTypeUnsigned:
		u, _ := mv.Uint64()
		return NewValue(u, mmsType)
	case mms.ValueTypeFloat, mms.ValueTypeReal:
		f, _ := mv.Float64()
		return NewValue(f, mmsType)
	case mms.ValueTypeVisibleString:
		s, _ := mv.VisibleString()
		return NewValue(s, mmsType)
	case mms.ValueTypeMmsString:
		s, _ := mv.MmsString()
		return NewValue(s, mmsType)
	case mms.ValueTypeOctetString:
		b, _ := mv.OctetString()
		return NewValue(b, mmsType)
	case mms.ValueTypeBitString:
		bits, _ := mv.BitString()
		return NewValue(bits, mmsType)
	case mms.ValueTypeUTCTime:
		t, ok := mv.UTCTime()
		if ok {
			return NewValue(t, mmsType)
		}
		return NewValue(nil, mmsType)
	case mms.ValueTypeBinaryTime:
		ms, ok := mv.BinaryTime()
		if ok {
			return NewValue(ms, mmsType)
		}
		return NewValue(nil, mmsType)
	default:
		return NewValue(mv.String(), mmsType)
	}
}

// ValueFromMmsValue is retained as an alias for ValueFromMMS.
func ValueFromMmsValue(mv *mms.Value) *Value {
	return ValueFromMMS(mv)
}

func (v Value) String() string {
	if v.Raw == nil {
		return "<nil>"
	}
	if v.Display != "" {
		return v.Display
	}
	return fmt.Sprintf("%v", v.Raw)
}
