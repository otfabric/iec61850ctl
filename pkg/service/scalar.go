// SPDX-License-Identifier: MIT

package service

import (
	"fmt"
	"strconv"
	"strings"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// ParseScalarValue parses a CLI --value / --type pair into a typed scalar.
func ParseScalarValue(raw string, kind domain.ScalarKind) (domain.ScalarValue, error) {
	v := domain.ScalarValue{Kind: kind}
	switch kind {
	case domain.ScalarBool:
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return v, fmt.Errorf("invalid bool value %q", raw)
		}
		v.Bool = b
	case domain.ScalarInt, domain.ScalarEnum:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return v, fmt.Errorf("invalid int value %q", raw)
		}
		v.Int = n
	case domain.ScalarUint:
		n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return v, fmt.Errorf("invalid uint value %q", raw)
		}
		v.Uint = n
	case domain.ScalarFloat:
		f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return v, fmt.Errorf("invalid float value %q", raw)
		}
		v.Float = f
	case domain.ScalarString:
		v.String = raw
	default:
		return v, fmt.Errorf("unsupported scalar kind %q", kind)
	}
	return v, nil
}

// ScalarRequestedAny returns a JSON-native projection of the scalar.
func ScalarRequestedAny(v domain.ScalarValue) any {
	switch v.Kind {
	case domain.ScalarBool:
		return v.Bool
	case domain.ScalarInt, domain.ScalarEnum:
		return v.Int
	case domain.ScalarUint:
		return v.Uint
	case domain.ScalarFloat:
		return v.Float
	case domain.ScalarString:
		return v.String
	default:
		return nil
	}
}

// ScalarToMMS converts a domain scalar to an MMS value for Write.
func ScalarToMMS(v domain.ScalarValue) (*mms.Value, error) {
	switch v.Kind {
	case domain.ScalarBool:
		return mms.NewBoolean(v.Bool), nil
	case domain.ScalarInt, domain.ScalarEnum:
		return mms.NewInteger(v.Int), nil
	case domain.ScalarUint:
		return mms.NewUnsigned(v.Uint), nil
	case domain.ScalarFloat:
		return mms.NewFloat(v.Float), nil
	case domain.ScalarString:
		return mms.NewVisibleString(v.String), nil
	default:
		return nil, fmt.Errorf("unsupported scalar kind %q", v.Kind)
	}
}

// ScalarToCtlVal converts a domain scalar to a control CtlVal.
func ScalarToCtlVal(v domain.ScalarValue) (*mms.Value, error) {
	switch v.Kind {
	case domain.ScalarBool:
		return iec61850.BoolCtlVal(v.Bool), nil
	case domain.ScalarInt:
		if v.Int < -2147483648 || v.Int > 2147483647 {
			return nil, fmt.Errorf("int value %d out of int32 range", v.Int)
		}
		return iec61850.IntCtlVal(int32(v.Int)), nil
	case domain.ScalarEnum:
		if v.Int < -2147483648 || v.Int > 2147483647 {
			return nil, fmt.Errorf("enum value %d out of int32 range", v.Int)
		}
		return iec61850.EnumCtlVal(int32(v.Int)), nil
	case domain.ScalarFloat:
		return iec61850.FloatCtlVal(float32(v.Float)), nil
	case domain.ScalarString:
		return iec61850.StringCtlVal(v.String), nil
	case domain.ScalarUint:
		if v.Uint > 2147483647 {
			return nil, fmt.Errorf("uint value %d out of int32 range for control", v.Uint)
		}
		return iec61850.IntCtlVal(int32(v.Uint)), nil
	default:
		return nil, fmt.Errorf("unsupported control scalar kind %q", v.Kind)
	}
}

// ScalarMatchesMMS reports whether the read MMS value matches the requested scalar.
func ScalarMatchesMMS(want domain.ScalarValue, got *mms.Value) bool {
	if got == nil {
		return false
	}
	switch want.Kind {
	case domain.ScalarBool:
		b, ok := got.Bool()
		return ok && b == want.Bool
	case domain.ScalarInt, domain.ScalarEnum:
		n, ok := got.Int64()
		return ok && n == want.Int
	case domain.ScalarUint:
		n, ok := got.Uint64()
		if ok {
			return n == want.Uint
		}
		// Some servers encode small unsigned as integer.
		i, ok2 := got.Int64()
		return ok2 && i >= 0 && uint64(i) == want.Uint
	case domain.ScalarFloat:
		f, ok := got.Float64()
		if !ok {
			return false
		}
		diff := f - want.Float
		if diff < 0 {
			diff = -diff
		}
		return diff < 1e-6
	case domain.ScalarString:
		if s, ok := got.VisibleString(); ok {
			return s == want.String
		}
		if s, ok := got.MmsString(); ok {
			return s == want.String
		}
		return false
	default:
		return false
	}
}

// ScalarFromIECValue projects an iec61850.Value to a JSON-native any for confirmation.
func ScalarFromIECValue(v *iec61850.Value) any {
	if v == nil || v.MMS() == nil {
		return nil
	}
	mv := v.MMS()
	if b, ok := mv.Bool(); ok {
		return b
	}
	if n, ok := mv.Int64(); ok {
		return n
	}
	if n, ok := mv.Uint64(); ok {
		return n
	}
	if f, ok := mv.Float64(); ok {
		return f
	}
	if s, ok := mv.VisibleString(); ok {
		return s
	}
	if s, ok := mv.MmsString(); ok {
		return s
	}
	return nil
}
