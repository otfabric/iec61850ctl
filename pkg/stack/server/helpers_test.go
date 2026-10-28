// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/scl"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestFcOrdinalAll(t *testing.T) {
	t.Parallel()
	fcs := []domain.FunctionalConstraint{domain.FC_ST, domain.FC_MX, domain.FC_SP, domain.FC_SV, domain.FC_CF, domain.FC_DC,
		domain.FC_SG, domain.FC_SE, domain.FC_SR, domain.FC_OR, domain.FC_BL, domain.FC_EX, domain.FC_CO, domain.FC_US,
		domain.FC_MS, domain.FC_RP, domain.FC_BR, domain.FC_LG}
	for i, fc := range fcs {
		if got := fcOrdinal(fc); got != i {
			t.Errorf("fcOrdinal(%s) = %d, want %d", fc, got, i)
		}
	}
}

func TestToInt64(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in   any
		want int64
	}{{nil, 0}, {float64(3.9), 3}, {int(4), 4}, {int64(5), 5}, {int32(6), 6}, {uint64(7), 7}, {"x", 0}} {
		if got := toInt64(tt.in); got != tt.want {
			t.Errorf("toInt64(%#v) = %d", tt.in, got)
		}
	}
}

func TestFormatCfgValueKinds(t *testing.T) {
	t.Parallel()
	da := func(raw any, typ domain.MmsDataType) *domain.DataAttribute {
		return &domain.DataAttribute{Type: typ, Value: domain.NewValue(raw, typ)}
	}
	tests := []struct {
		da   *domain.DataAttribute
		want string
	}{
		{da(true, domain.TypeBoolean), "1"},
		{da(float64(2), domain.TypeFloat), "2"},
		{da(2.5, domain.TypeFloat), "2.5"},
		{da(int32(-3), domain.TypeInt32), "-3"},
		{da(int64(9), domain.TypeInt64), "9"},
		{da(uint32(8), domain.TypeUint32), "8"},
		{da("ab", domain.TypeVisibleString), `"ab"`},
		{da([]int{1}, domain.TypeArray), "[1]"},
		{da(map[string]any{"seconds": float64(2), "milliseconds": float64(5)}, domain.TypeUtcTime), "2005"},
		{da(map[string]any{}, domain.TypeUtcTime), ""},
	}
	for _, tt := range tests {
		if got := formatCfgValue(tt.da); got != tt.want {
			t.Errorf("formatCfgValue(%#v) = %q, want %q", tt.da.Value.Raw, got, tt.want)
		}
	}
}

func TestDomainValueToMMSKinds(t *testing.T) {
	t.Parallel()
	ok := []*domain.Value{
		domain.NewValue(true, domain.TypeBoolean),
		domain.NewValue(float64(-4), domain.TypeInt16),
		domain.NewValue(uint8(4), domain.TypeUint8),
		domain.NewValue(uint16(4), domain.TypeUint16),
		domain.NewValue(uint32(4), domain.TypeUint32),
		domain.NewValue(uint64(4), domain.TypeUnsigned),
		domain.NewValue(int64(4), domain.TypeUnsigned),
		domain.NewValue(float64(4), domain.TypeUnsigned),
		domain.NewValue(float32(1.5), domain.TypeFloat),
		domain.NewValue(int64(1), domain.TypeFloat),
		domain.NewValue(int(1), domain.TypeFloat),
		domain.NewValue("s", domain.TypeMmsString),
		domain.NewValue("s", domain.TypeVisibleString),
		domain.NewValue(float64(-1), domain.TypeBinaryTime),
		domain.NewValue(float64(1e18), domain.TypeBinaryTime),
		domain.NewValue(float64(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()), domain.TypeBinaryTime),
	}
	for _, v := range ok {
		if _, good := domainValueToMMS(v); !good {
			t.Errorf("domainValueToMMS(%#v %s) failed", v.Raw, v.Type)
		}
	}
	bad := []*domain.Value{
		nil,
		domain.NewValue(nil, domain.TypeBoolean),
		domain.NewValue("x", domain.TypeBoolean),
		domain.NewValue("x", domain.TypeInt32),
		domain.NewValue(int64(-1), domain.TypeUnsigned),
		domain.NewValue(float64(-1), domain.TypeUnsigned),
		domain.NewValue("x", domain.TypeUnsigned),
		domain.NewValue("x", domain.TypeFloat),
		domain.NewValue(1, domain.TypeVisibleString),
		domain.NewValue("x", domain.TypeUtcTime),
		domain.NewValue("x", domain.TypeBinaryTime),
		domain.NewValue(1, domain.TypeBitString),
		domain.NewValue(1, domain.TypeOctetString),
		domain.NewValue(1, domain.TypeStructure),
	}
	for _, v := range bad {
		if _, good := domainValueToMMS(v); good {
			t.Errorf("domainValueToMMS(%#v) unexpectedly ok", v)
		}
	}
}

func newInteropServer(t *testing.T) *iec61850.Server {
	t.Helper()
	sclData, err := scl.ParseFile(interopICD(t))
	if err != nil {
		t.Fatal(err)
	}
	model, err := iec61850.NewServerModelFromSCL(sclData, "InteropIED", "")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := iec61850.NewServer(model, iec61850.ServerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func TestSeedValuesSkipsAndSets(t *testing.T) {
	t.Parallel()
	srv := newInteropServer(t)
	ied := &domain.IED{Leaves: []domain.LeafEntry{
		{Ref: "InteropLD/GGIO1.SPCSO1.stVal", FC: domain.FC_ST, Type: domain.TypeBoolean, Value: domain.NewValue(true, domain.TypeBoolean)},
		{Ref: "InteropLD/GGIO1.SPCSO1.stVal", FC: domain.FC_ST, ValueError: "x"},
		{Ref: "InteropLD/GGIO1.SPCSO1.stVal", FC: domain.FC_ST, Type: domain.TypeBoolean},
		{Ref: "InteropLD/GGIO1.SPCSO1", FC: domain.FC_ST, Type: domain.TypeStructure, Value: domain.NewValue(1, domain.TypeStructure)},
		{Ref: "bad", FC: domain.FC_ST, Type: domain.TypeBoolean, Value: domain.NewValue(true, domain.TypeBoolean)},
		{Ref: "InteropLD/GGIO1.SPCSO1.stVal", FC: domain.FC_ST, Type: domain.TypeBoolean, Value: domain.NewValue("nope", domain.TypeBoolean)},
	}}
	SeedValues(srv, ied)
	v := srv.ValueStore().Get("InteropLD/GGIO1$ST$SPCSO1$stVal")
	if v == nil {
		t.Fatal("stVal not seeded")
	}
	if b, _ := v.Bool(); !b {
		t.Fatalf("stVal = %v", v)
	}
}

func TestInteropControlOperate(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := Start(ctx, RunConfig{SclPath: interopICD(t), Host: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	c, err := iec61850.Dial(ctx, r.Addr(), iec61850.DialOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	ref, err := iec61850.ParseRef("InteropLD/GGIO1.SPCSO1")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Operate(ctx, ref, iec61850.OperateParams{CtlVal: iec61850.BoolCtlVal(true)}); err != nil {
		t.Fatalf("operate: %v", err)
	}
	if err := c.Operate(ctx, ref, iec61850.OperateParams{CtlVal: iec61850.IntCtlVal(1)}); err == nil {
		t.Fatal("non-boolean ctlVal accepted")
	}
}

func TestRegisterInteropControlsSkipsForeignModel(t *testing.T) {
	t.Parallel()
	sclData, err := scl.ParseFile(filepath.Join("testdata", "client-first.icd"))
	if err != nil {
		t.Fatal(err)
	}
	model, err := iec61850.NewServerModelFromSCL(sclData, "OtherIED", "")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := iec61850.NewServer(model, iec61850.ServerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	if hasLogicalDevice(srv, interopLD) {
		t.Fatal("unexpected InteropLD")
	}
	if err := registerInteropControls(srv); err != nil {
		t.Fatal(err)
	}
}
