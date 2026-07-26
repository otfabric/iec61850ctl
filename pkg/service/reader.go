// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// Reader reads IEC 61850 data attribute values.
type Reader struct {
	conn IEC61850Connection
}

func NewReader(conn IEC61850Connection) *Reader {
	return &Reader{conn: conn}
}

// ObjectValue is a read result with metadata.
type ObjectValue struct {
	Ref   string
	FC    domain.FunctionalConstraint
	Type  domain.MmsDataType
	Value *domain.Value
}

// ReadObject reads a single object reference with optional FC.
// objectRef is LD/LN.DO.attr…; fc may be empty for auto-detect via ParseRef with bracket FC.
func (r *Reader) ReadObject(objectRef string, fc domain.FunctionalConstraint) (*ObjectValue, error) {
	if r.conn == nil {
		return nil, fmt.Errorf("reader: client is nil")
	}
	refStr := objectRef
	if fc != domain.FC_NONE && fc != domain.FC_ALL && !hasFCBracket(objectRef) {
		refStr = objectRef + "[" + string(fc) + "]"
	}
	ref, err := iec61850.ParseRef(refStr)
	if err != nil {
		return nil, fmt.Errorf("parse ref %q: %w", refStr, err)
	}
	ctx := context.Background()
	v, err := r.conn.Read(ctx, ref)
	if err != nil {
		return nil, err
	}
	out := &ObjectValue{
		Ref:   objectRef,
		FC:    domain.FromLibFC(ref.FC),
		Type:  domain.FromMMSValueType(v.Type()),
		Value: domain.ValueFromMMS(v.MMS()),
	}
	return out, nil
}

func hasFCBracket(s string) bool {
	return len(s) > 0 && s[len(s)-1] == ']'
}

// ReadLeafValue reads a leaf attribute and returns a Go value suitable for formatting/serialization.
func (r *Reader) ReadLeafValue(refStr string, fc iec61850.FunctionalConstraint, mmsType mms.ValueType) (interface{}, error) {
	if r.conn == nil {
		return nil, fmt.Errorf("reader: client is nil")
	}
	fcStr := string(fc)
	if fcStr == "" {
		fcStr = "MX"
	}
	if !hasFCBracket(refStr) {
		refStr = refStr + "[" + fcStr + "]"
	}
	ref, err := iec61850.ParseRef(refStr)
	if err != nil {
		return nil, fmt.Errorf("parse ref %q: %w", refStr, err)
	}
	v, err := r.conn.Read(context.Background(), ref)
	if err != nil {
		return nil, err
	}
	return valueToInterface(v, mmsType)
}

func valueToInterface(v *iec61850.Value, mmsType mms.ValueType) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	switch mmsType {
	case mms.ValueTypeBoolean:
		return v.Bool()
	case mms.ValueTypeInteger:
		if i, err := v.Int32(); err == nil {
			return i, nil
		}
		return v.Int64()
	case mms.ValueTypeUnsigned:
		if u, err := v.Uint32(); err == nil {
			return u, nil
		}
		return v.Uint64()
	case mms.ValueTypeFloat, mms.ValueTypeReal:
		return v.Float64()
	case mms.ValueTypeVisibleString:
		return v.VisibleString()
	case mms.ValueTypeMmsString:
		return v.MmsString()
	case mms.ValueTypeOctetString:
		return v.OctetString()
	case mms.ValueTypeBitString:
		return v.BitString()
	case mms.ValueTypeUTCTime:
		if ts, err := v.Timestamp(); err == nil {
			return ts.Time, nil
		}
		if mv := v.MMS(); mv != nil {
			if t, ok := mv.UTCTime(); ok {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("UTC time decode failed")
	case mms.ValueTypeBinaryTime:
		if mv := v.MMS(); mv != nil {
			if ms, ok := mv.BinaryTime(); ok {
				return ms, nil
			}
		}
		return int64(0), fmt.Errorf("binary time decode failed")
	default:
		if mv := v.MMS(); mv != nil {
			return mv.String(), nil
		}
		return nil, nil
	}
}
