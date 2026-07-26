// SPDX-License-Identifier: MIT

package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestValueToModel_UTCTime_SerializesTimeQuality(t *testing.T) {
	ts := time.UnixMilli(1662761017061) // 2022-09-09 22:03:37.061 UTC
	val, errMsg := ValueToModel(ts, mms.ValueTypeUTCTime)
	if errMsg != "" {
		t.Fatalf("ValueToModel: %s", errMsg)
	}
	if val == nil || val.Type != domain.TypeUtcTime {
		t.Fatalf("expected type UTC_TIME, got %v", val)
	}
	jsonBytes, err := json.Marshal(val.Raw)
	if err != nil {
		t.Fatalf("marshal UTC_TIME from Raw: %v", err)
	}
	var decoded struct {
		Seconds      int64  `json:"seconds"`
		Milliseconds uint16 `json:"milliseconds"`
		TimeQuality  uint8  `json:"time_quality"`
	}
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("unmarshal UTC_TIME JSON: %v", err)
	}
	if decoded.Seconds != 1662761017 {
		t.Errorf("seconds = %d, want 1662761017", decoded.Seconds)
	}
	if decoded.Milliseconds != 61 {
		t.Errorf("milliseconds = %d, want 61", decoded.Milliseconds)
	}
}

func TestValueToModel_BitStringOctetBinary(t *testing.T) {
	bs, errMsg := ValueToModel(uint16(0x1234), mms.ValueTypeBitString)
	if errMsg != "" || bs == nil || bs.Raw != "0x1234" {
		t.Fatalf("bitstring uint16: %+v %q", bs, errMsg)
	}
	bs2, _ := ValueToModel([]byte{0xab, 0xcd}, mms.ValueTypeBitString)
	if bs2 == nil || bs2.Raw != "0xabcd" {
		t.Fatalf("bitstring bytes: %+v", bs2)
	}
	bs3, _ := ValueToModel(int32(0xff), mms.ValueTypeBitString)
	if bs3 == nil || bs3.Raw != "0x00ff" {
		t.Fatalf("bitstring int32: %+v", bs3)
	}
	bs4, _ := ValueToModel("nope", mms.ValueTypeBitString)
	if bs4 == nil || bs4.Raw != "0x0000" {
		t.Fatalf("bitstring default: %+v", bs4)
	}

	os, errMsg := ValueToModel([]byte{0xde, 0xad}, mms.ValueTypeOctetString)
	if errMsg != "" || os == nil || os.Raw != "dead" {
		t.Fatalf("octet: %+v %q", os, errMsg)
	}
	osEmpty, _ := ValueToModel("not-bytes", mms.ValueTypeOctetString)
	if osEmpty == nil || osEmpty.Raw != "" {
		t.Fatalf("octet non-bytes: %+v", osEmpty)
	}

	bin, errMsg := ValueToModel(int64(99), mms.ValueTypeBinaryTime)
	if errMsg != "" || bin == nil || bin.Raw != int64(99) {
		t.Fatalf("binary int64: %+v %q", bin, errMsg)
	}
	bin2, _ := ValueToModel(uint64(7), mms.ValueTypeBinaryTime)
	if bin2 == nil || bin2.Raw != uint64(7) {
		t.Fatalf("binary uint64: %+v", bin2)
	}
	bin3, _ := ValueToModel(5, mms.ValueTypeBinaryTime)
	if bin3 == nil || bin3.Raw != int64(5) {
		t.Fatalf("binary int: %+v", bin3)
	}
}

func TestValueToModel_UTCTimePointerAndNil(t *testing.T) {
	ts := time.UnixMilli(1_000).UTC()
	val, errMsg := ValueToModel(&ts, mms.ValueTypeUTCTime)
	if errMsg != "" || val == nil {
		t.Fatalf("ptr: %q %+v", errMsg, val)
	}
	var nilTS *time.Time
	bad, errMsg := ValueToModel(nilTS, mms.ValueTypeUTCTime)
	if bad != nil || errMsg == "" {
		t.Fatalf("nil ptr should error: %+v %q", bad, errMsg)
	}
	if v, msg := ValueToModel(nil, mms.ValueTypeFloat); v != nil || msg != "" {
		t.Fatalf("nil value: %v %q", v, msg)
	}
}

func TestValueToModel_CommonScalars(t *testing.T) {
	cases := []struct {
		val  interface{}
		typ  mms.ValueType
		want interface{}
	}{
		{true, mms.ValueTypeBoolean, true},
		{int32(3), mms.ValueTypeInteger, int32(3)},
		{uint32(4), mms.ValueTypeUnsigned, uint32(4)},
		{float32(1.5), mms.ValueTypeFloat, float32(1.5)},
		{"hello", mms.ValueTypeVisibleString, "hello"},
	}
	for _, tc := range cases {
		v, errMsg := ValueToModel(tc.val, tc.typ)
		if errMsg != "" || v == nil || v.Raw != tc.want {
			t.Fatalf("%v/%v -> %+v %q", tc.val, tc.typ, v, errMsg)
		}
	}
}

func TestFormatHelpersDirect(t *testing.T) {
	if formatBitStringForJSON([]byte{0x01}) != "0x0001" {
		t.Fatal(formatBitStringForJSON([]byte{0x01}))
	}
	if formatOctetStringForJSON([]byte{0x0a}) != "0a" {
		t.Fatal(formatOctetStringForJSON([]byte{0x0a}))
	}
	if formatBinaryTimeForJSON("x") != "x" {
		t.Fatal(formatBinaryTimeForJSON("x"))
	}
}
