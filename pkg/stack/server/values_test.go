// SPDX-License-Identifier: MIT

package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otfabric/go-iec61850/scl"
	mms "github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestResolveIEDName(t *testing.T) {
	doc := &scl.SCL{IEDs: []scl.IED{{Name: "IED1"}, {Name: "IED2"}}}

	got, err := resolveIEDName(doc, "IED2")
	if err != nil || got != "IED2" {
		t.Fatalf("named = %q, %v", got, err)
	}
	got, err = resolveIEDName(doc, "")
	if err != nil || got != "IED1" {
		t.Fatalf("default = %q, %v", got, err)
	}
	if _, err := resolveIEDName(doc, "MISSING"); err == nil {
		t.Fatal("expected missing IED error")
	}
	if _, err := resolveIEDName(&scl.SCL{}, ""); err == nil {
		t.Fatal("expected empty IEDs error")
	}
}

func TestLeafRefToStoreKey(t *testing.T) {
	key, err := leafRefToStoreKey("LD0/LLN0.Mod.stVal", domain.FC_ST)
	if err != nil {
		t.Fatalf("leafRefToStoreKey: %v", err)
	}
	if key != "LD0/LLN0$ST$Mod$stVal" {
		t.Errorf("key = %q", key)
	}
	if _, err := leafRefToStoreKey("bad", domain.FC_ST); err == nil {
		t.Fatal("expected invalid ref error")
	}
	if _, err := leafRefToStoreKey("LD0/LLN0", domain.FC_ST); err == nil {
		t.Fatal("expected not-a-leaf error")
	}
	if _, err := leafRefToStoreKey("LD0/LLN0.Mod.stVal", ""); err == nil {
		t.Fatal("expected missing FC error")
	}
}

func TestDomainValueToMMS(t *testing.T) {
	tests := []struct {
		name string
		v    *domain.Value
		ok   bool
		kind mms.ValueType
	}{
		{"nil", nil, false, 0},
		{"nil raw", domain.NewValue(nil, domain.TypeBoolean), false, 0},
		{"bool", domain.NewValue(true, domain.TypeBoolean), true, mms.ValueTypeBoolean},
		{"int", domain.NewValue(int64(3), domain.TypeInt32), true, mms.ValueTypeInteger},
		{"uint", domain.NewValue(uint64(4), domain.TypeUint16), true, mms.ValueTypeUnsigned},
		{"float", domain.NewValue(1.25, domain.TypeFloat), true, mms.ValueTypeFloat},
		{"visstr", domain.NewValue("hi", domain.TypeVisibleString), true, mms.ValueTypeVisibleString},
		{"mmsstr", domain.NewValue("hi", domain.TypeMmsString), true, mms.ValueTypeMmsString},
		{"bintime", domain.NewValue(int64(100), domain.TypeBinaryTime), true, mms.ValueTypeBinaryTime},
		{"bitstr hex", domain.NewValue("0x00ff", domain.TypeBitString), true, mms.ValueTypeBitString},
		{"octet", domain.NewValue("deadbeef", domain.TypeOctetString), true, mms.ValueTypeOctetString},
		{"utc map", domain.NewValue(map[string]interface{}{
			"seconds": float64(10), "milliseconds": float64(5), "time_quality": float64(0),
		}, domain.TypeUtcTime), true, mms.ValueTypeUTCTime},
		{"unsupported", domain.NewValue(1, domain.TypeStructure), false, 0},
		{"bad bool", domain.NewValue("x", domain.TypeBoolean), false, 0},
		{"bad string", domain.NewValue(1, domain.TypeVisibleString), false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mv, ok := domainValueToMMS(tt.v)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && mv.Type() != tt.kind {
				t.Errorf("type = %v, want %v", mv.Type(), tt.kind)
			}
		})
	}
}

func TestParseHelpers(t *testing.T) {
	if _, _, ok := parseUtcTime("nope"); ok {
		t.Error("parseUtcTime bad type")
	}
	if _, q, ok := parseUtcTime(utcTimeValue{Seconds: 1, Milliseconds: 2, TimeQuality: 3}); !ok || q != 3 {
		t.Errorf("parseUtcTime struct failed q=%d ok=%v", q, ok)
	}

	bits, n, ok := parseBitString("")
	if !ok || n != 16 || len(bits) != 2 {
		t.Errorf("empty bitstring = %v %d %v", bits, n, ok)
	}
	if _, _, ok := parseBitString("zz"); ok {
		t.Error("invalid hex bitstring should fail")
	}
	if _, _, ok := parseBitString(float64(255)); !ok {
		t.Error("float bitstring should work")
	}
	if _, _, ok := parseBitString(int64(1)); !ok {
		t.Error("int64 bitstring should work")
	}
	if _, _, ok := parseBitString(true); ok {
		t.Error("bool bitstring should fail")
	}

	if data, ok := parseOctetString(""); !ok || len(data) != 0 {
		t.Errorf("empty octet = %v %v", data, ok)
	}
	if _, ok := parseOctetString(1); ok {
		t.Error("non-string octet should fail")
	}
	if _, ok := parseOctetString("zz"); ok {
		t.Error("bad hex octet should fail")
	}

	if b, ok := asBool(float64(2)); !ok || !b {
		t.Error("asBool float")
	}
	if _, ok := asBool("x"); ok {
		t.Error("asBool string")
	}
	if i, ok := asInt64(int(5)); !ok || i != 5 {
		t.Error("asInt64 int")
	}
	if i, ok := asInt64(int32(6)); !ok || i != 6 {
		t.Error("asInt64 int32")
	}
	if _, ok := asInt64("x"); ok {
		t.Error("asInt64 string")
	}
	if u, ok := asUint64(uint32(7)); !ok || u != 7 {
		t.Error("asUint64 uint32")
	}
	if u, ok := asUint64(uint16(8)); !ok || u != 8 {
		t.Error("asUint64 uint16")
	}
	if u, ok := asUint64(uint8(9)); !ok || u != 9 {
		t.Error("asUint64 uint8")
	}
	if _, ok := asUint64(int64(-1)); ok {
		t.Error("asUint64 negative int64")
	}
	if _, ok := asUint64(float64(-1)); ok {
		t.Error("asUint64 negative float")
	}
	if f, ok := asFloat64(float32(1.5)); !ok || f < 1.4 {
		t.Error("asFloat64 float32")
	}
	if f, ok := asFloat64(int(2)); !ok || f != 2 {
		t.Error("asFloat64 int")
	}
	if _, ok := asFloat64("x"); ok {
		t.Error("asFloat64 string")
	}
}

func TestSeedValues_NilSafe(t *testing.T) {
	SeedValues(nil, nil)
	SeedValues(nil, &domain.IED{})
}

func TestSeedValuesFromFile_Errors(t *testing.T) {
	if err := seedValuesFromFile(nil, "/no/such/file.json"); err == nil {
		t.Fatal("expected read error")
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := seedValuesFromFile(nil, bad); err == nil {
		t.Fatal("expected parse error")
	}
}
