// SPDX-License-Identifier: MIT

package gooseops

import (
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// ParseValue parses raw as a value of the MMS type ts. Supported member
// types are boolean, integer, unsigned, float, visible/MMS string,
// bit string (hex, optionally "hex/len"), octet string (hex) and UTC time
// ("now" or RFC 3339). Structures and arrays are not parsed from text.
func ParseValue(raw string, ts mms.TypeSpec) (*mms.Value, error) {
	s := strings.TrimSpace(raw)
	bad := func(kind string) error {
		return errs.Invalidf("value %q is not a valid %s", raw, kind)
	}
	switch ts.Type {
	case mms.ValueTypeBoolean:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, bad("boolean")
		}
		return mms.NewBoolean(b), nil
	case mms.ValueTypeInteger:
		bits := ts.Size
		if bits <= 0 || bits > 64 {
			bits = 64
		}
		i, err := strconv.ParseInt(s, 0, bits)
		if err != nil {
			return nil, bad("integer of " + strconv.Itoa(bits) + " bits")
		}
		return mms.NewInteger(i), nil
	case mms.ValueTypeUnsigned:
		bits := ts.Size
		if bits <= 0 || bits > 64 {
			bits = 64
		}
		u, err := strconv.ParseUint(s, 0, bits)
		if err != nil {
			return nil, bad("unsigned of " + strconv.Itoa(bits) + " bits")
		}
		return mms.NewUnsigned(u), nil
	case mms.ValueTypeFloat:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, bad("float")
		}
		if (ts.FormatWidth == 0 || ts.FormatWidth == 32) && !math.IsInf(f, 0) && math.Abs(f) > math.MaxFloat32 {
			return nil, errs.Invalidf("value %q out of float32 range", raw)
		}
		return mms.NewFloat(f), nil
	case mms.ValueTypeVisibleString:
		for i := 0; i < len(raw); i++ {
			if raw[i] < 0x20 || raw[i] > 0x7e {
				return nil, errs.Invalidf("value %q: VisibleString allows printable ASCII only", raw)
			}
		}
		if ts.Size > 0 && len(raw) > ts.Size {
			return nil, errs.Invalidf("value %q longer than VisibleString%d", raw, ts.Size)
		}
		return mms.NewVisibleString(raw), nil
	case mms.ValueTypeMmsString:
		return mms.NewMmsString(raw), nil
	case mms.ValueTypeOctetString:
		b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
		if err != nil {
			return nil, bad("octet string (hex)")
		}
		return mms.NewOctetString(b), nil
	case mms.ValueTypeBitString:
		hexPart, lenPart, hasLen := strings.Cut(strings.TrimPrefix(s, "0x"), "/")
		b, err := hex.DecodeString(hexPart)
		if err != nil {
			return nil, bad("bit string (hex[/len])")
		}
		n := len(b) * 8
		if hasLen {
			n, err = strconv.Atoi(lenPart)
			if err != nil || n < 0 || n > len(b)*8 {
				return nil, bad("bit string (hex[/len])")
			}
		} else if ts.Size > 0 && ts.Size <= n {
			n = ts.Size
		}
		return mms.NewBitStringWithLength(b, n), nil
	case mms.ValueTypeUTCTime:
		if strings.EqualFold(s, "now") {
			return mms.NewUTCTime(time.Now()), nil
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, bad("UTC time (RFC 3339 or now)")
		}
		return mms.NewUTCTime(t), nil
	default:
		return nil, errs.Newf(errs.ErrUnsupported, "member type %s cannot be set from text", ts.Type)
	}
}

// ApplyOverrides replaces dataset values from "INDEX=VALUE" or
// "MEMBER-REF=VALUE" assignments, parsing each value as the member's type.
func ApplyOverrides(values []*mms.Value, members []goose.Member, assignments []string) error {
	for _, a := range assignments {
		key, raw, ok := strings.Cut(a, "=")
		if !ok {
			return errs.Invalidf("--value %q: expected INDEX=VALUE or MEMBER=VALUE", a)
		}
		idx, err := memberIndex(members, strings.TrimSpace(key))
		if err != nil {
			return err
		}
		if idx >= len(values) {
			return errs.Invalidf("--value %q: index %d out of range (dataset has %d members)", a, idx, len(values))
		}
		v, err := ParseValue(raw, members[idx].Type)
		if err != nil {
			return fmt.Errorf("--value %s: %w", key, err)
		}
		values[idx] = v
	}
	return nil
}

func memberIndex(members []goose.Member, key string) (int, error) {
	if i, err := strconv.Atoi(key); err == nil {
		if i < 0 || i >= len(members) {
			return 0, errs.Invalidf("--value index %d out of range (dataset has %d members)", i, len(members))
		}
		return i, nil
	}
	for i, m := range members {
		if m.Ref == key || m.Ref+"["+m.FC+"]" == key {
			return i, nil
		}
	}
	return 0, errs.Newf(errs.ErrNotFound, "--value: dataset has no member %q", key)
}
