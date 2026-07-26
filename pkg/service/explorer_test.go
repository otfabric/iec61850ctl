// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// TestLogicalDeviceInfo_Structure verifies domain.LogicalDevice struct fields.
func TestLogicalDeviceInfo_Structure(t *testing.T) {
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
	explorer := NewExplorer(nil)
	if explorer == nil {
		t.Fatal("NewExplorer(nil) returned nil")
	}

	_, err := explorer.GetLogicalDevices()
	if err == nil {
		t.Error("GetLogicalDevices with nil client: expected error, got nil")
	}
}
