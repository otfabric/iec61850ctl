// SPDX-License-Identifier: MIT

package value

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestFormatLeafValue(t *testing.T) {
	ts := domain.Timestamp{UnixMs: 1737646499123, TimeQuality: 0x00}
	tm := time.UnixMilli(1737646499123).UTC()

	tests := []struct {
		name       string
		value      interface{}
		mmsType    domain.MmsDataType
		attr       string
		wantExact  string
		wantSubstr []string
	}{
		{"nil value", nil, domain.TypeBoolean, "", "(nil)", nil},
		{"default bool", true, domain.TypeBoolean, "", "true", nil},
		{"default int", 42, domain.TypeInteger, "", "42", nil},
		{
			"utc timestamp",
			ts, domain.TypeUtcTime, "",
			"", []string{"UTC", "leap-unknown"},
		},
		{
			"utc timestamp pointer",
			&ts, domain.TypeUtcTime, "",
			"", []string{"UTC", "leap-unknown"},
		},
		{
			"utc nil pointer",
			(*domain.Timestamp)(nil), domain.TypeUtcTime, "",
			"(nil)", nil,
		},
		{
			"utc time.Time",
			tm, domain.TypeUtcTime, "",
			"", []string{"2025-01-23 15:34:59.123 UTC"},
		},
		{
			"utc uint64 ms",
			uint64(1737646499123), domain.TypeUtcTime, "",
			"", []string{"2025-01-23 15:34:59.123 UTC"},
		},
		{
			"utc int64 ms",
			int64(1737646499123), domain.TypeUtcTime, "",
			"", []string{"2025-01-23 15:34:59.123 UTC"},
		},
		{
			"utc bad type",
			"bad", domain.TypeUtcTime, "",
			"", []string{"<error:", "UTC_TIME"},
		},
		{
			"binary time",
			uint64(1737646499123), domain.TypeBinaryTime, "",
			"", []string{"(ms=1737646499123)"},
		},
		{
			"bitstring quality by name",
			uint16(0), domain.TypeBitString, "q",
			"", []string{"0x0000", "[good]", "details: none"},
		},
		{
			"bitstring quality by path",
			uint16(1), domain.TypeBitString, "MX.q",
			"", []string{"0x0001", "[invalid]"},
		},
		{
			"bitstring raw uint16",
			uint16(0x1234), domain.TypeBitString, "stVal",
			"0x1234", nil,
		},
		{
			"bitstring raw int32",
			int32(0x00ab), domain.TypeBitString, "other",
			"0x00ab", nil,
		},
		{
			"bitstring raw uint32",
			uint32(0x00cd), domain.TypeBitString, "x",
			"0x00cd", nil,
		},
		{
			"bitstring raw int",
			int(0x00ef), domain.TypeBitString, "x",
			"0x00ef", nil,
		},
		{
			"bitstring raw two bytes",
			[]byte{0x12, 0x34}, domain.TypeBitString, "stVal",
			"0x1234", nil,
		},
		{
			"bitstring raw one byte",
			[]byte{0xab}, domain.TypeBitString, "stVal",
			"0x00ab", nil,
		},
		{
			"bitstring raw empty bytes",
			[]byte{}, domain.TypeBitString, "stVal",
			"0x0000", nil,
		},
		{
			"bitstring raw unsupported",
			"bits", domain.TypeBitString, "stVal",
			"bits", nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLeafValue(tt.value, tt.mmsType, tt.attr)
			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("FormatLeafValue() = %q, want %q", got, tt.wantExact)
			}
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(got, sub) {
					t.Errorf("FormatLeafValue() = %q, want containing %q", got, sub)
				}
			}
		})
	}
}

func TestFormatAttributeValueWithError(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		mmsType domain.MmsDataType
		err     error
		attr    string
		want    string
	}{
		{"with error", true, domain.TypeBoolean, errors.New("read failed"), "", "<error: read failed>"},
		{"no error", true, domain.TypeBoolean, nil, "", "true"},
		{"nil value no error", nil, domain.TypeBoolean, nil, "", "(nil)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAttributeValueWithError(tt.value, tt.mmsType, tt.err, tt.attr)
			if got != tt.want {
				t.Errorf("FormatAttributeValueWithError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDataAttributeValue(t *testing.T) {
	tests := []struct {
		name string
		da   *domain.DataAttribute
		want string
	}{
		{"nil da", nil, "(nil)"},
		{"value error", &domain.DataAttribute{ValueError: "boom"}, "<error: boom>"},
		{
			"display set",
			&domain.DataAttribute{Value: &domain.Value{Raw: 1, Display: "custom"}},
			"custom",
		},
		{
			"raw string",
			&domain.DataAttribute{Value: &domain.Value{Raw: 42, Type: domain.TypeInteger}},
			"42",
		},
		{"nil value", &domain.DataAttribute{}, "(nil)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDataAttributeValue(tt.da)
			if got != tt.want {
				t.Errorf("FormatDataAttributeValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatMmsValue(t *testing.T) {
	tm := time.UnixMilli(1737646499123).UTC()

	tests := []struct {
		name       string
		mv         *mms.Value
		wantExact  string
		wantSubstr []string
	}{
		{"nil", nil, "(nil)", nil},
		{
			"structure",
			mms.NewStructure([]*mms.Value{
				mms.NewInteger(42),
				mms.NewVisibleString("hello"),
			}),
			"",
			[]string{"[", "42", "hello", "]"},
		},
		{
			"array",
			mms.NewArray([]*mms.Value{
				mms.NewBoolean(true),
				mms.NewBoolean(false),
			}),
			"",
			[]string{"[", "true", "false", "]"},
		},
		{
			"utc time",
			mms.NewUTCTimeWithQuality(tm, 0x00),
			"",
			[]string{"UTC", "leap-unknown"},
		},
		{"boolean", mms.NewBoolean(true), "true", nil},
		{"integer", mms.NewInteger(-7), "-7", nil},
		{"unsigned", mms.NewUnsigned(99), "99", nil},
		{"float", mms.NewFloat(1.5), "1.5", nil},
		{"visible string", mms.NewVisibleString("abc"), "abc", nil},
		{"mms string", mms.NewMmsString("mms"), "mms", nil},
		{
			"bit string",
			mms.NewBitString([]byte{0x12, 0x34}),
			"",
			[]string{"0x"},
		},
		{
			"octet string",
			mms.NewOctetString([]byte{0xde, 0xad}),
			"",
			[]string{"222", "173"},
		},
		{
			"binary time",
			mms.NewBinaryTime(1737646499123),
			"",
			[]string{"(ms=1737646499123)"},
		},
		{
			"generalized time",
			mms.NewGeneralizedTime(tm),
			"",
			[]string{"2025"},
		},
		{"bcd", mms.NewBCD(12), "12", nil},
		{
			"object identifier",
			mms.NewObjectIdentifier([]int{1, 3, 6}),
			"",
			[]string{"1", "3", "6"},
		},
		{
			"data access error",
			mms.NewDataAccessError(mms.DataAccessErrorObjectInvalidated),
			"",
			[]string{"ObjectInvalidated"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMmsValue(tt.mv)
			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("FormatMmsValue() = %q, want %q", got, tt.wantExact)
			}
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(got, sub) {
					t.Errorf("FormatMmsValue() = %q, want containing %q", got, sub)
				}
			}
		})
	}
}

func TestFormatDomainValue(t *testing.T) {
	tests := []struct {
		name       string
		v          *domain.Value
		wantExact  string
		wantSubstr []string
	}{
		{"nil", nil, "(nil)", nil},
		{
			"display preferred",
			&domain.Value{Raw: 1, Type: domain.TypeInteger, Display: "shown"},
			"shown",
			nil,
		},
		{
			"structure children",
			&domain.Value{
				Type: domain.TypeStructure,
				Raw: []*domain.Value{
					{Raw: true, Type: domain.TypeBoolean},
					{Raw: int64(5), Type: domain.TypeInteger},
				},
			},
			"",
			[]string{"[", "true", "5", "]"},
		},
		{
			"array children",
			&domain.Value{
				Type: domain.TypeArray,
				Raw: []*domain.Value{
					{Raw: "a", Type: domain.TypeVisibleString},
					{Raw: "b", Type: domain.TypeVisibleString},
				},
			},
			"",
			[]string{"[", "a", "b", "]"},
		},
		{
			"structure bad raw",
			&domain.Value{Type: domain.TypeStructure, Raw: "not-children"},
			"not-children",
			nil,
		},
		{
			"leaf integer",
			&domain.Value{Type: domain.TypeInteger, Raw: int64(9)},
			"9",
			nil,
		},
		{
			"leaf utc",
			&domain.Value{Type: domain.TypeUtcTime, Raw: uint64(1737646499123)},
			"",
			[]string{"UTC"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDomainValue(tt.v)
			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("FormatDomainValue() = %q, want %q", got, tt.wantExact)
			}
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(got, sub) {
					t.Errorf("FormatDomainValue() = %q, want containing %q", got, sub)
				}
			}
		})
	}
}
