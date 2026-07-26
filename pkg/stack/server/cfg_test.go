// SPDX-License-Identifier: MIT

package server

import (
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestIEDToCfg_NilAndEmpty(t *testing.T) {
	if _, err := IEDToCfg(nil, "X"); err == nil {
		t.Fatal("expected error for nil IED")
	}
	if _, err := IEDToCfg(&domain.IED{}, "X"); err == nil {
		t.Fatal("expected error for empty logical devices")
	}
}

func TestIEDToCfg_MinimalModel(t *testing.T) {
	ied := &domain.IED{
		LogicalDevices: []domain.LogicalDevice{
			{
				Name: "LD0",
				LogicalNodes: []domain.LogicalNode{
					{
						Name: "LLN0",
						DataObjects: []domain.DataObject{
							{
								Name: "Mod",
								Attributes: []domain.DataAttribute{
									{
										Name:  "stVal",
										FC:    domain.FC_ST,
										Type:  domain.TypeInt32,
										Value: domain.NewValue(int64(1), domain.TypeInt32),
									},
									{
										Name: "q",
										FC:   domain.FC_ST,
										Type: domain.TypeBitString,
									},
									{
										Name: "ctlModel",
										FC:   domain.FC_CF,
										Type: domain.TypeInt8,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cfg, err := IEDToCfg(ied, "TEST")
	if err != nil {
		t.Fatalf("IEDToCfg: %v", err)
	}
	s := string(cfg)

	mustContain := []string{
		"MODEL(TEST){",
		"LD(LD0){",
		"LN(LLN0){",
		"DO(Mod 0){",
		"DA(stVal 0 3 0 1 0)=1;",
		"DA(q 0 23 0 2 0);",
		"DA(ctlModel 0 12 4 4 0);",
	}
	for _, want := range mustContain {
		if !strings.Contains(s, want) {
			t.Errorf("cfg missing %q\nfull:\n%s", want, s)
		}
	}
}

func TestIEDToCfg_ConstructedAndValues(t *testing.T) {
	ied := &domain.IED{
		LogicalDevices: []domain.LogicalDevice{
			{
				Name: "CTRL",
				LogicalNodes: []domain.LogicalNode{
					{
						Name: "GGIO1",
						DataObjects: []domain.DataObject{
							{
								Name: "AnIn1",
								Attributes: []domain.DataAttribute{
									{
										Name: "mag",
										FC:   domain.FC_MX,
										Type: domain.TypeStructure,
										Children: []domain.DataAttribute{
											{
												Name:  "f",
												FC:    domain.FC_MX,
												Type:  domain.TypeFloat,
												Value: domain.NewValue(12.5, domain.TypeFloat),
											},
										},
									},
									{
										Name:  "d",
										FC:    domain.FC_DC,
										Type:  domain.TypeVisibleString,
										Value: domain.NewValue("desc", domain.TypeVisibleString),
									},
									{
										Name:  "enb",
										FC:    domain.FC_ST,
										Type:  domain.TypeBoolean,
										Value: domain.NewValue(true, domain.TypeBoolean),
									},
									{
										Name: "t",
										FC:   domain.FC_ST,
										Type: domain.TypeUtcTime,
										Value: domain.NewValue(map[string]interface{}{
											"seconds":      float64(1700000000),
											"milliseconds": float64(123),
										}, domain.TypeUtcTime),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cfg, err := IEDToCfg(ied, "")
	if err != nil {
		t.Fatalf("IEDToCfg: %v", err)
	}
	s := string(cfg)

	mustContain := []string{
		"MODEL(){",
		"LD(CTRL){",
		"LN(GGIO1){",
		"DO(AnIn1 0){",
		"DA(mag 0 27 1 1 0){",
		"DA(f 0 10 1 1 0)=12.5;",
		`DA(d 0 20 5 1 0)="desc";`,
		"DA(enb 0 0 0 1 0)=1;",
		"DA(t 0 22 0 1 0)=1700000000123;",
	}
	for _, want := range mustContain {
		if !strings.Contains(s, want) {
			t.Errorf("cfg missing %q\nfull:\n%s", want, s)
		}
	}
}

func TestDataAttributeTypeCode_Coverage(t *testing.T) {
	cases := []struct {
		da   domain.DataAttribute
		want int
	}{
		{domain.DataAttribute{Type: domain.TypeBoolean}, daTypeBoolean},
		{domain.DataAttribute{Type: domain.TypeInt8}, daTypeInt8},
		{domain.DataAttribute{Type: domain.TypeInt16}, daTypeInt16},
		{domain.DataAttribute{Type: domain.TypeInt32}, daTypeInt32},
		{domain.DataAttribute{Type: domain.TypeInt64}, daTypeInt64},
		{domain.DataAttribute{Type: domain.TypeUint8}, daTypeInt8U},
		{domain.DataAttribute{Type: domain.TypeUint16}, daTypeInt16U},
		{domain.DataAttribute{Type: domain.TypeUint32}, daTypeInt32U},
		{domain.DataAttribute{Type: domain.TypeFloat}, daTypeFloat32},
		{domain.DataAttribute{Type: domain.TypeMmsString}, daTypeVisStr},
		{domain.DataAttribute{Type: domain.TypeOctetString}, daTypeOctet8},
		{domain.DataAttribute{Type: domain.TypeBitString, Name: "other"}, daTypeBitString},
		{domain.DataAttribute{Type: domain.TypeBitString, Name: "q"}, daTypeQuality},
		{domain.DataAttribute{Name: "ctlModel", Type: domain.TypeInt8}, daTypeEnum},
		{domain.DataAttribute{Type: domain.TypeStructure}, daTypeConstructed},
		{domain.DataAttribute{Type: domain.TypeUnknown}, daTypeInt32},
		{domain.DataAttribute{Children: []domain.DataAttribute{{Name: "x"}}}, daTypeConstructed},
	}
	for _, tt := range cases {
		got := dataAttributeTypeCode(&tt.da)
		if got != tt.want {
			t.Errorf("dataAttributeTypeCode(%+v) = %d, want %d", tt.da, got, tt.want)
		}
	}
}

func TestFcOrdinalAndTrigger(t *testing.T) {
	if fcOrdinal(domain.FC_MX) != 1 {
		t.Errorf("FC_MX ordinal = %d, want 1", fcOrdinal(domain.FC_MX))
	}
	if fcOrdinal(domain.FC_CO) != 12 {
		t.Errorf("FC_CO ordinal = %d, want 12", fcOrdinal(domain.FC_CO))
	}
	if fcOrdinal(domain.FunctionalConstraint("ZZ")) != 0 {
		t.Errorf("unknown FC should be 0")
	}
	if daTriggerOption("q") != 2 {
		t.Errorf("trigger q = %d, want 2", daTriggerOption("q"))
	}
	if daTriggerOption("ctlModel") != 4 {
		t.Errorf("trigger ctlModel = %d, want 4", daTriggerOption("ctlModel"))
	}
	if daTriggerOption("stVal") != 1 {
		t.Errorf("trigger stVal = %d, want 1", daTriggerOption("stVal"))
	}
}

func TestFormatCfgValue(t *testing.T) {
	if formatCfgValue(&domain.DataAttribute{}) != "" {
		t.Error("nil value should format empty")
	}
	if formatCfgValue(&domain.DataAttribute{Value: domain.NewValue(nil, domain.TypeBoolean)}) != "" {
		t.Error("nil raw should format empty")
	}
	if got := formatCfgValue(&domain.DataAttribute{
		Value: domain.NewValue(false, domain.TypeBoolean),
	}); got != "0" {
		t.Errorf("bool false = %q, want 0", got)
	}
	if got := formatCfgValue(&domain.DataAttribute{
		Value: domain.NewValue(int(7), domain.TypeInt32),
	}); got != "7" {
		t.Errorf("int = %q, want 7", got)
	}
	if got := formatCfgValue(&domain.DataAttribute{
		Type:  domain.TypeUtcTime,
		Value: domain.NewValue("bad", domain.TypeUtcTime),
	}); got != "" {
		t.Errorf("bad utc map = %q, want empty", got)
	}
}
