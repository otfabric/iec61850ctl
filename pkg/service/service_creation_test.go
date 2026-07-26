// SPDX-License-Identifier: MIT

// Package services provides business logic for IEC 61850 device exploration and data reading.
// This file contains tests for Phase 2 service consolidation features.

package service

import (
	"testing"
)

// TestBulkFindResult verifies the BulkFindResult struct structure.
func TestBulkFindResult(t *testing.T) {
	result := BulkFindResult{
		Entries: []BulkResultEntry{
			{ControlledPropertyId: "prop1", Paths: []string{"LD0/LN1.DO1"}},
			{ControlledPropertyId: "prop2", Paths: []string{"LD0/LN2.DO2", "LD0/LN2.DO3"}},
		},
		CallCount: 42,
	}

	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result.Entries))
	}
	if result.CallCount != 42 {
		t.Errorf("Expected CallCount=42, got %d", result.CallCount)
	}
	if result.Entries[0].ControlledPropertyId != "prop1" {
		t.Errorf("Expected first entry ID=prop1, got %s", result.Entries[0].ControlledPropertyId)
	}
}

// TestReportServiceCreation verifies NewReportService creates a service.
func TestReportServiceCreation(t *testing.T) {
	// Can't test with real connection, just verify constructor
	service := NewReportService(nil)
	if service == nil {
		t.Fatal("NewReportService returned nil")
	}
}

// TestDataSetServiceCreation verifies NewDataSetService creates a service.
func TestDataSetServiceCreation(t *testing.T) {
	// Can't test with real connection, just verify constructor
	service := NewDataSetService(nil)
	if service == nil {
		t.Fatal("NewDataSetService returned nil")
	}
}
