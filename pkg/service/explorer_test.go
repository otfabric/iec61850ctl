// SPDX-License-Identifier: MIT

package service

import (
	"errors"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// TestLogicalDeviceInfo_Structure verifies domain.LogicalDevice struct fields.
func TestLogicalDeviceInfo_Structure(t *testing.T) {
	t.Parallel()
	info := domain.LogicalDevice{
		Name:      "MNSREF615CTRL",
		LNCount:   42,
		DSCount:   5,
		URCBCount: 3,
		BRCBCount: 2,
	}

	if info.Name != "MNSREF615CTRL" {
		t.Errorf("Name = %q, want %q", info.Name, "MNSREF615CTRL")
	}
	if info.LNCount != 42 {
		t.Errorf("LNCount = %d, want %d", info.LNCount, 42)
	}
	if info.DSCount != 5 {
		t.Errorf("DSCount = %d, want %d", info.DSCount, 5)
	}
	if info.URCBCount != 3 {
		t.Errorf("URCBCount = %d, want %d", info.URCBCount, 3)
	}
	if info.BRCBCount != 2 {
		t.Errorf("BRCBCount = %d, want %d", info.BRCBCount, 2)
	}
}

// TestGetLogicalDevices_NilClient tests that GetLogicalDevices returns an error when client is nil.
func TestGetLogicalDevices_NilClient(t *testing.T) {
	t.Parallel()
	explorer := NewExplorer(nil)
	if explorer == nil {
		t.Fatal("NewExplorer(nil) returned nil")
	}

	_, err := explorer.GetLogicalDevices()
	if err == nil {
		t.Error("GetLogicalDevices with nil client: expected error, got nil")
	}
}

func TestExplorer_EmptyLDIsNotFound(t *testing.T) {
	t.Parallel()
	m := &mockConnection{logicalNodes: []string{}, logicalDevices: []string{"LD0"}}
	e := NewExplorer(m)
	if _, err := e.ListLogicalNodes("NoSuchLD"); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("ListLogicalNodes unknown LD: want ErrNotFound, got %v", err)
	}
	if _, err := e.GetLogicalNodes("NoSuchLD"); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("GetLogicalNodes unknown LD: want ErrNotFound, got %v", err)
	}
	if names, err := e.ListLogicalNodes("LD0"); err != nil || len(names) != 0 {
		t.Fatalf("known empty LD: names=%v err=%v", names, err)
	}

	// A failed directory probe keeps the empty list.
	m.listLDsErr = errors.New("probe failed")
	if names, err := e.ListLogicalNodes("NoSuchLD"); err != nil || len(names) != 0 {
		t.Fatalf("probe failure: names=%v err=%v", names, err)
	}
}
