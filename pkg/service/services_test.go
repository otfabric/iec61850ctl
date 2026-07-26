// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestNewReader(t *testing.T) {
	var conn IEC61850Connection
	reader := NewReader(conn)

	if reader == nil {
		t.Fatal("NewReader() returned nil")
	}
	if reader.conn != conn {
		t.Error("NewReader() did not set conn field correctly")
	}
}

func TestNewExplorer(t *testing.T) {
	var conn IEC61850Connection
	explorer := NewExplorer(conn)

	if explorer == nil {
		t.Fatal("NewExplorer() returned nil")
	}
	if explorer.conn != conn {
		t.Error("NewExplorer() did not set conn field correctly")
	}
}

func TestNewTree(t *testing.T) {
	var conn IEC61850Connection
	tree := NewTree(conn)

	if tree == nil {
		t.Fatal("NewTree() returned nil")
	}
	if tree.conn != conn {
		t.Error("NewTree() did not set conn field correctly")
	}
}

func TestDataAttribute_Structure(t *testing.T) {
	da := domain.DataAttribute{
		Name:     "mag.f",
		Ref:      "LD0/LN0.DO.mag.f",
		FC:       domain.FC_MX,
		Type:     domain.TypeFloat,
		Value:    domain.NewValue(123.45, domain.TypeFloat),
		Children: nil,
	}

	if da.Name != "mag.f" {
		t.Errorf("Name = %q, want %q", da.Name, "mag.f")
	}
	if da.Ref != "LD0/LN0.DO.mag.f" {
		t.Errorf("Ref = %q, want %q", da.Ref, "LD0/LN0.DO.mag.f")
	}
	if da.FC != domain.FC_MX {
		t.Errorf("FC = %s, want MX", da.FC)
	}
	if da.Type != domain.TypeFloat {
		t.Errorf("Type = %q, want FLOAT", da.Type.String())
	}
	if da.Value == nil || da.Value.Raw != 123.45 {
		t.Errorf("Value = %v, want 123.45", da.Value)
	}
	if da.Children != nil {
		t.Errorf("Children = %v, want nil", da.Children)
	}
}

func TestListDataAttributesInput_Structure(t *testing.T) {
	input := ListDataAttributesInput{
		LogicalDevice: "DEV03LD0",
		LogicalNode:   "CUS1_GGIO3",
		DataObject:    "AnIn1",
	}

	if input.LogicalDevice != "DEV03LD0" {
		t.Errorf("LogicalDevice = %q, want %q", input.LogicalDevice, "DEV03LD0")
	}
	if input.LogicalNode != "CUS1_GGIO3" {
		t.Errorf("LogicalNode = %q, want %q", input.LogicalNode, "CUS1_GGIO3")
	}
	if input.DataObject != "AnIn1" {
		t.Errorf("DataObject = %q, want %q", input.DataObject, "AnIn1")
	}
}
