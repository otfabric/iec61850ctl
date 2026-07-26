// SPDX-License-Identifier: MIT

// Package server: seed ValueStore from serialized IED JSON leaves.

package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	mms "github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// utcTimeValue is the JSON shape for UTC_TIME in serialized IED (value.v).
type utcTimeValue struct {
	Seconds      int64  `json:"seconds"`
	Milliseconds uint16 `json:"milliseconds"`
	TimeQuality  uint8  `json:"time_quality"`
}

func seedValuesFromFile(srv *iec61850.Server, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read values file: %w", err)
	}
	var ied domain.IED
	if err := json.Unmarshal(data, &ied); err != nil {
		return fmt.Errorf("parse values JSON: %w", err)
	}
	SeedValues(srv, &ied)
	return nil
}

// SeedValues best-effort seeds the server ValueStore from IED JSON leaf entries.
// Leaf refs use dot notation (LD/LN.DO.attr); store keys use MMS format (LD/LN$FC$DO$attr).
// Skips leaves without values, unsupported types, or keys that cannot be mapped.
func SeedValues(srv *iec61850.Server, ied *domain.IED) {
	if srv == nil || ied == nil {
		return
	}
	vs := srv.ValueStore()
	for i := range ied.Leaves {
		leaf := &ied.Leaves[i]
		if leaf.ValueError != "" || leaf.Value == nil || leaf.Value.Raw == nil {
			continue
		}
		if leaf.Type == domain.TypeStructure || leaf.Type == domain.TypeArray {
			continue
		}
		key, err := leafRefToStoreKey(leaf.Ref, leaf.FC)
		if err != nil {
			continue
		}
		mv, ok := domainValueToMMS(leaf.Value)
		if !ok {
			continue
		}
		vs.Set(key, mv)
	}
}

func leafRefToStoreKey(ref string, fc domain.FunctionalConstraint) (string, error) {
	objRef, err := domain.ParseObjectReference(ref)
	if err != nil {
		return "", err
	}
	if len(objRef.Path) == 0 {
		return "", fmt.Errorf("not a leaf reference: %q", ref)
	}
	fcStr := string(fc)
	if fcStr == "" {
		return "", fmt.Errorf("missing FC for %q", ref)
	}
	itemID := objRef.LN + "$" + fcStr + "$" + strings.Join(objRef.Path, "$")
	return objRef.LD + "/" + itemID, nil
}

func domainValueToMMS(v *domain.Value) (*mms.Value, bool) {
	if v == nil || v.Raw == nil {
		return nil, false
	}

	switch v.Type {
	case domain.TypeBoolean:
		b, ok := asBool(v.Raw)
		if !ok {
			return nil, false
		}
		return mms.NewBoolean(b), true

	case domain.TypeInteger, domain.TypeInt8, domain.TypeInt16, domain.TypeInt32, domain.TypeInt64:
		i, ok := asInt64(v.Raw)
		if !ok {
			return nil, false
		}
		return mms.NewInteger(i), true

	case domain.TypeUnsigned, domain.TypeUint8, domain.TypeUint16, domain.TypeUint32:
		u, ok := asUint64(v.Raw)
		if !ok {
			return nil, false
		}
		return mms.NewUnsigned(u), true

	case domain.TypeFloat:
		f, ok := asFloat64(v.Raw)
		if !ok {
			return nil, false
		}
		return mms.NewFloat(f), true

	case domain.TypeVisibleString, domain.TypeMmsString:
		s, ok := v.Raw.(string)
		if !ok {
			return nil, false
		}
		if v.Type == domain.TypeMmsString {
			return mms.NewMmsString(s), true
		}
		return mms.NewVisibleString(s), true

	case domain.TypeUtcTime:
		t, quality, ok := parseUtcTime(v.Raw)
		if !ok {
			return nil, false
		}
		return mms.NewUTCTimeWithQuality(t, quality), true

	case domain.TypeBinaryTime:
		ms, ok := asInt64(v.Raw)
		if !ok {
			return nil, false
		}
		return mms.NewBinaryTime(ms), true

	case domain.TypeBitString:
		bits, bitLen, ok := parseBitString(v.Raw)
		if !ok {
			return nil, false
		}
		if bitLen > 0 {
			return mms.NewBitStringWithLength(bits, bitLen), true
		}
		return mms.NewBitString(bits), true

	case domain.TypeOctetString:
		data, ok := parseOctetString(v.Raw)
		if !ok {
			return nil, false
		}
		return mms.NewOctetString(data), true

	default:
		return nil, false
	}
}

func parseUtcTime(raw interface{}) (time.Time, uint8, bool) {
	switch v := raw.(type) {
	case utcTimeValue:
		return utcTimeFromParts(v.Seconds, v.Milliseconds, v.TimeQuality)
	case map[string]interface{}:
		sec, _ := asInt64(v["seconds"])
		ms, _ := asUint64(v["milliseconds"])
		tq, _ := asUint64(v["time_quality"])
		return utcTimeFromParts(sec, uint16(ms), uint8(tq))
	default:
		return time.Time{}, 0, false
	}
}

func utcTimeFromParts(seconds int64, millis uint16, quality uint8) (time.Time, uint8, bool) {
	epochMs := seconds*1000 + int64(millis)
	return time.UnixMilli(epochMs), quality, true
}

func parseBitString(raw interface{}) ([]byte, int, bool) {
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			s = s[2:]
		}
		if len(s) == 0 {
			return []byte{0, 0}, 16, true
		}
		n, err := strconv.ParseUint(s, 16, 16)
		if err != nil {
			return nil, 0, false
		}
		return []byte{byte(n >> 8), byte(n)}, 16, true
	case float64:
		n := uint16(v)
		return []byte{byte(n >> 8), byte(n)}, 16, true
	case int64:
		n := uint16(v)
		return []byte{byte(n >> 8), byte(n)}, 16, true
	default:
		return nil, 0, false
	}
}

func parseOctetString(raw interface{}) ([]byte, bool) {
	s, ok := raw.(string)
	if !ok {
		return nil, false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return []byte{}, true
	}
	data, err := hex.DecodeString(s)
	if err != nil {
		return nil, false
	}
	return data, true
}

func asBool(raw interface{}) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case float64:
		return v != 0, true
	default:
		return false, false
	}
}

func asInt64(raw interface{}) (int64, bool) {
	switch v := raw.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

func asUint64(raw interface{}) (uint64, bool) {
	switch v := raw.(type) {
	case uint64:
		return v, true
	case uint32:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	default:
		return 0, false
	}
}

func asFloat64(raw interface{}) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}
