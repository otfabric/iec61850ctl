// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestExplorer_GetLogicalNodesAndDataObjects(t *testing.T) {
	mock := &mockConnection{
		logicalNodes: []string{"LLN0", "MMXU1"},
		dataObjects:  []string{"DO1", "Hz"},
	}
	ex := NewExplorer(mock)

	lns, err := ex.GetLogicalNodes("LD0")
	if err != nil {
		t.Fatalf("GetLogicalNodes: %v", err)
	}
	if len(lns) != 2 || lns[0].Name != "LLN0" || lns[0].DOCount != 2 {
		t.Fatalf("lns=%+v", lns)
	}

	dos, err := ex.GetDataObjects("LD0", "MMXU1")
	if err != nil {
		t.Fatalf("GetDataObjects: %v", err)
	}
	if len(dos) != 2 || dos[1].Name != "Hz" {
		t.Fatalf("dos=%+v", dos)
	}
}

func TestExplorer_GetDataAttributesAndDANames(t *testing.T) {
	mock := &mockConnection{}
	ex := NewExplorer(mock)

	attrs, err := ex.GetDataAttributes(ListDataAttributesInput{
		LogicalDevice: "LD0",
		LogicalNode:   "MMXU1",
		DataObject:    "DO1",
	})
	if err != nil {
		t.Fatalf("GetDataAttributes: %v", err)
	}
	if len(attrs) == 0 {
		t.Fatal("expected attributes")
	}
	sawMag := false
	for _, a := range attrs {
		if a.Name == "mag" || a.Name == "q" {
			sawMag = true
		}
		if a.FC == domain.FC_NONE {
			t.Fatalf("unexpected FC_NONE for %+v", a)
		}
		if a.Value == nil {
			t.Fatalf("expected value for %+v", a)
		}
	}
	if !sawMag {
		t.Fatalf("expected mag/q leaves, got %+v", attrs)
	}

	names, err := ex.GetDataObjectDANames("LD0", "MMXU1", "DO1")
	if err != nil {
		t.Fatalf("GetDataObjectDANames: %v", err)
	}
	if len(names) < 2 {
		t.Fatalf("DA names=%v", names)
	}

	grouped, err := ex.ListDataAttributes(ListDataAttributesInput{
		LogicalDevice: "LD0",
		LogicalNode:   "MMXU1",
		DataObject:    "DO1",
	})
	if err != nil {
		t.Fatalf("ListDataAttributes: %v", err)
	}
	if len(grouped["mag"]) == 0 && len(grouped["q"]) == 0 {
		t.Fatalf("grouped=%v", grouped)
	}
}

func TestListDataAttributesInput_Validate(t *testing.T) {
	if err := (ListDataAttributesInput{}).Validate(); err == nil {
		t.Fatal("expected validation error")
	}
	if err := (ListDataAttributesInput{LogicalDevice: "LD0"}).Validate(); err == nil {
		t.Fatal("expected LN error")
	}
	if err := (ListDataAttributesInput{LogicalDevice: "LD0", LogicalNode: "LN"}).Validate(); err == nil {
		t.Fatal("expected DO error")
	}
	if err := (ListDataAttributesInput{
		LogicalDevice: "LD0", LogicalNode: "LN", DataObject: "DO",
	}).Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestExplorer_GetDataAttributes_NilClient(t *testing.T) {
	ex := NewExplorer(nil)
	_, err := ex.GetDataAttributes(ListDataAttributesInput{
		LogicalDevice: "LD0", LogicalNode: "LN", DataObject: "DO",
	})
	if err == nil {
		t.Fatal("expected nil client error")
	}
}

func TestExplorer_GetLogicalDevices_WithCounts(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD0"},
		logicalNodes:   []string{"LLN0"},
		dataSets:       []string{"LLN0$ds1"},
		reports:        []string{"LLN0$BR$rcb1", "LLN0$RP$urcb1"},
	}
	lds, err := NewExplorer(mock).GetLogicalDevices()
	if err != nil {
		t.Fatalf("GetLogicalDevices: %v", err)
	}
	if len(lds) != 1 {
		t.Fatalf("len=%d", len(lds))
	}
	if lds[0].LNCount != 1 || lds[0].DSCount != 1 || lds[0].BRCBCount != 1 || lds[0].URCBCount != 1 {
		t.Fatalf("counts=%+v", lds[0])
	}
}
