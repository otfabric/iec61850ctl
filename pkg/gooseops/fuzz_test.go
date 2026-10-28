// SPDX-License-Identifier: MIT

package gooseops

import (
	"encoding/json"
	"testing"

	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-mms"
)

func FuzzParseAppID(f *testing.F) {
	for _, s := range []string{"0x1001", "1001", "d:4097", "", "0x", "d:", "ffff", "10000"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseAppID(s)
		if err != nil {
			return
		}
		back, err := ParseAppID(FormatAppID(id))
		if err != nil || back != id {
			t.Fatalf("round trip %q -> %d -> %q: %d %v", s, id, FormatAppID(id), back, err)
		}
	})
}

var fuzzTypes = []mms.TypeSpec{
	{Type: mms.ValueTypeBoolean}, {Type: mms.ValueTypeInteger, Size: 8}, {Type: mms.ValueTypeUnsigned, Size: 32},
	{Type: mms.ValueTypeFloat, FormatWidth: 32}, {Type: mms.ValueTypeVisibleString}, {Type: mms.ValueTypeBitString, Size: 13},
	{Type: mms.ValueTypeOctetString}, {Type: mms.ValueTypeUTCTime},
}

// FuzzParseValue checks that every accepted --value encodes as GOOSE data
// and renders as JSON.
func FuzzParseValue(f *testing.F) {
	for _, s := range []string{"true", "-1", "1.5", "c0/2", "0xdead", "now", "2026-01-02T03:04:05Z", "1e39"} {
		for i := range fuzzTypes {
			f.Add(s, uint8(i))
		}
	}
	f.Fuzz(func(t *testing.T, s string, k uint8) {
		ts := fuzzTypes[int(k)%len(fuzzTypes)]
		v, err := ParseValue(s, ts)
		if err != nil {
			return
		}
		if ts.Type == mms.ValueTypeUTCTime {
			if tm, _ := v.UTCTime(); tm.Year() < 1970 || tm.Year() > 2106 {
				return // outside the UtcTime range; the encoder rejects it
			}
		}
		b, err := goose.EncodeData(v)
		if err != nil {
			t.Fatalf("accepted %q as %v but cannot encode: %v", s, ts.Type, err)
		}
		if _, err := goose.DecodeData(b); err != nil {
			t.Fatalf("decode own encoding of %q: %v", s, err)
		}
		if _, err := json.Marshal(ValueJSON(v)); err != nil {
			t.Fatalf("json %q: %v", s, err)
		}
	})
}
