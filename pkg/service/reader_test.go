// SPDX-License-Identifier: MIT

package service

import (
	"errors"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestReader_ReadObject_Errors(t *testing.T) {
	if _, err := NewReader(nil).ReadObject("LD0/LN.DO", domain.FC_MX); err == nil {
		t.Fatal("expected nil client error")
	}
	mock := &mockConnection{
		readFunc: func(ref iec61850.Ref) (*iec61850.Value, error) {
			return nil, errors.New("read fail")
		},
	}
	if _, err := NewReader(mock).ReadObject("LD0/LN.DO.attr", domain.FC_MX); err == nil {
		t.Fatal("expected read error")
	}
	if _, err := NewReader(mock).ReadObject("not a ref!!!", domain.FC_NONE); err == nil {
		t.Fatal("expected parse error")
	}
	// Bracket FC already present.
	mockOK := &mockConnection{}
	obj, err := NewReader(mockOK).ReadObject("LD0/LN.DO.attr[ST]", domain.FC_NONE)
	if err != nil || obj == nil {
		t.Fatalf("bracket FC: %v %+v", err, obj)
	}
}

func TestReader_ReadLeafValue(t *testing.T) {
	if _, err := NewReader(nil).ReadLeafValue("LD0/LN.DO.a", iec61850.FCMX, mms.ValueTypeFloat); err == nil {
		t.Fatal("expected nil client error")
	}
	mock := &mockConnection{
		readFunc: func(ref iec61850.Ref) (*iec61850.Value, error) {
			return iec61850.NewValue(mms.NewBoolean(true)), nil
		},
	}
	v, err := NewReader(mock).ReadLeafValue("LD0/LN.DO.a", "", mms.ValueTypeBoolean)
	if err != nil || v != true {
		t.Fatalf("got %v %v", v, err)
	}
	v2, err := NewReader(mock).ReadLeafValue("LD0/LN.DO.a[ST]", iec61850.FCST, mms.ValueTypeBoolean)
	if err != nil || v2 != true {
		t.Fatalf("bracket: %v %v", v2, err)
	}
}

func TestValueToInterface(t *testing.T) {
	if v, err := valueToInterface(nil, mms.ValueTypeFloat); v != nil || err != nil {
		t.Fatalf("nil: %v %v", v, err)
	}

	cases := []struct {
		name string
		val  *mms.Value
		typ  mms.ValueType
	}{
		{"bool", mms.NewBoolean(true), mms.ValueTypeBoolean},
		{"int", mms.NewInteger(42), mms.ValueTypeInteger},
		{"uint", mms.NewUnsigned(7), mms.ValueTypeUnsigned},
		{"float", mms.NewFloat(1.5), mms.ValueTypeFloat},
		{"vstring", mms.NewVisibleString("hi"), mms.ValueTypeVisibleString},
		{"mmsstring", mms.NewMmsString("ms"), mms.ValueTypeMmsString},
		{"octet", mms.NewOctetString([]byte{1, 2}), mms.ValueTypeOctetString},
		{"bit", mms.NewBitString([]byte{0xff}), mms.ValueTypeBitString},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := valueToInterface(iec61850.NewValue(tc.val), tc.typ)
			if err != nil || got == nil {
				t.Fatalf("got %v err=%v", got, err)
			}
		})
	}

	ts := time.Unix(1_700_000_000, 0).UTC()
	utcVal := mms.NewUTCTime(ts)
	gotTS, err := valueToInterface(iec61850.NewValue(utcVal), mms.ValueTypeUTCTime)
	if err != nil {
		t.Fatalf("utc: %v", err)
	}
	if _, ok := gotTS.(time.Time); !ok {
		t.Fatalf("utc type %T", gotTS)
	}

	binVal := mms.NewBinaryTime(12345)
	gotBin, err := valueToInterface(iec61850.NewValue(binVal), mms.ValueTypeBinaryTime)
	if err != nil || gotBin == nil {
		t.Fatalf("binary: %v %v", gotBin, err)
	}

	def, err := valueToInterface(iec61850.NewValue(mms.NewFloat(1)), mms.ValueTypeStructure)
	if err != nil || def == nil {
		t.Fatalf("default: %v %v", def, err)
	}
}
