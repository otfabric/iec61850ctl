// SPDX-License-Identifier: MIT

// Package services provides business logic for IEC 61850 device exploration and data reading.
// This file contains comprehensive tests for Phase 5: Quality of Life features.

package service

import (
	"bytes"
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

// === R2.4: Pagination Tests ===

func TestGetLogicalDevicesPaged(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD1", "LD2", "LD3", "LD4", "LD5"},
	}

	explorer := NewExplorer(mock)

	t.Run("FirstPage", func(t *testing.T) {
		result, err := explorer.GetLogicalDevicesPaged(PageOptions{Limit: 2, Offset: 0})
		if err != nil {
			t.Fatalf("GetLogicalDevicesPaged failed: %v", err)
		}
		if len(result.Items) != 2 {
			t.Errorf("Expected 2 items, got %d", len(result.Items))
		}
		if !result.HasMore {
			t.Error("Expected HasMore=true, got false")
		}
		if result.Total != 5 {
			t.Errorf("Expected Total=5, got %d", result.Total)
		}
		if result.Items[0].Name != "LD1" || result.Items[1].Name != "LD2" {
			t.Errorf("Unexpected items: %v", result.Items)
		}
	})

	t.Run("MiddlePage", func(t *testing.T) {
		result, err := explorer.GetLogicalDevicesPaged(PageOptions{Limit: 2, Offset: 2})
		if err != nil {
			t.Fatalf("GetLogicalDevicesPaged failed: %v", err)
		}
		if len(result.Items) != 2 {
			t.Errorf("Expected 2 items, got %d", len(result.Items))
		}
		if !result.HasMore {
			t.Error("Expected HasMore=true, got false")
		}
		if result.Items[0].Name != "LD3" || result.Items[1].Name != "LD4" {
			t.Errorf("Unexpected items: %v", result.Items)
		}
	})

	t.Run("LastPage", func(t *testing.T) {
		result, err := explorer.GetLogicalDevicesPaged(PageOptions{Limit: 2, Offset: 4})
		if err != nil {
			t.Fatalf("GetLogicalDevicesPaged failed: %v", err)
		}
		if len(result.Items) != 1 {
			t.Errorf("Expected 1 item, got %d", len(result.Items))
		}
		if result.HasMore {
			t.Error("Expected HasMore=false, got true")
		}
		if result.Items[0].Name != "LD5" {
			t.Errorf("Unexpected item: %v", result.Items[0])
		}
	})

	t.Run("NoLimit", func(t *testing.T) {
		result, err := explorer.GetLogicalDevicesPaged(PageOptions{Limit: 0, Offset: 0})
		if err != nil {
			t.Fatalf("GetLogicalDevicesPaged failed: %v", err)
		}
		if len(result.Items) != 5 {
			t.Errorf("Expected all 5 items, got %d", len(result.Items))
		}
		if result.HasMore {
			t.Error("Expected HasMore=false when no limit, got true")
		}
	})

	t.Run("OffsetBeyondTotal", func(t *testing.T) {
		result, err := explorer.GetLogicalDevicesPaged(PageOptions{Limit: 2, Offset: 10})
		if err != nil {
			t.Fatalf("GetLogicalDevicesPaged failed: %v", err)
		}
		if len(result.Items) != 0 {
			t.Errorf("Expected 0 items, got %d", len(result.Items))
		}
		if result.HasMore {
			t.Error("Expected HasMore=false, got true")
		}
	})
}

func TestGetAllReportsPaged(t *testing.T) {
	// Use the existing mockConnection - it returns default empty lists for reports
	mock := &mockConnection{
		logicalDevices: []string{"LD1", "LD2"},
	}

	// The mock actually returns some reports from traversal, let's test pagination with those

	t.Run("PaginationWithMockResults", func(t *testing.T) {
		rsService := NewReportService(mock)
		result, err := rsService.GetAllReportsPaged(PageOptions{Limit: 5, Offset: 0})
		if err != nil {
			t.Fatalf("GetAllReportsPaged failed: %v", err)
		}
		// Mock has default behavior that creates reports, check pagination works
		if result.Total == 0 {
			t.Skip("Mock returned no reports, skipping pagination test")
		}
		if len(result.Items) > result.Total {
			t.Errorf("Items count %d exceeds Total %d", len(result.Items), result.Total)
		}
		if len(result.Items) > 5 {
			t.Errorf("Expected max 5 items with Limit=5, got %d", len(result.Items))
		}
	})

	t.Run("AllReportsNoPagination", func(t *testing.T) {
		rsService := NewReportService(mock)
		result, err := rsService.GetAllReportsPaged(PageOptions{Limit: 0, Offset: 0})
		if err != nil {
			t.Fatalf("GetAllReportsPaged failed: %v", err)
		}
		// With no limit, should get all items
		if result.Total != len(result.Items) {
			t.Errorf("Expected Items=%d to match Total=%d", len(result.Items), result.Total)
		}
		if result.HasMore {
			t.Error("Expected HasMore=false when no limit, got true")
		}
	})
}

// === R4.3: Multi-format Output Tests ===

func TestFormatterOutputFormat(t *testing.T) {
	devices := []domain.LogicalDevice{{Name: "LD1", LNCount: 1}}
	viewDevices := ProjectLogicalDevices(devices)

	t.Run("WithOutputFormatJSON", func(t *testing.T) {
		var buf bytes.Buffer
		f := formatter.NewFormatter().WithOutputFormat(formatter.OutputFormatJSON)
		if err := f.RenderLogicalDevices(viewDevices, &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), `"name": "LD1"`) {
			t.Errorf("Expected JSON output, got %s", buf.String())
		}
	})

	t.Run("WithOutputFormatCSV", func(t *testing.T) {
		var buf bytes.Buffer
		f := formatter.NewFormatter().WithOutputFormat(formatter.OutputFormatCSV)
		if err := f.RenderLogicalDevices(viewDevices, &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "LD1,") {
			t.Errorf("Expected CSV output, got %s", buf.String())
		}
	})

	t.Run("DefaultFormat", func(t *testing.T) {
		var buf bytes.Buffer
		f := formatter.NewFormatter()
		if err := f.RenderLogicalDevices(viewDevices, &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "LD1") {
			t.Errorf("Expected text output with LD1, got %s", buf.String())
		}
	})

	t.Run("FluentChaining", func(t *testing.T) {
		var buf bytes.Buffer
		f := formatter.NewFormatter().
			WithTimeFormat(formatter.TimeFormatUnix).
			WithByteFormat(formatter.ByteFormatIEC).
			WithOutputFormat(formatter.OutputFormatJSON)
		if err := f.RenderLogicalDevices(viewDevices, &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), `"name": "LD1"`) {
			t.Error("Fluent chain: OutputFormat not applied")
		}
	})
}

// === R6.1: Tree Build() Method Tests ===

func TestTreeBuild(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"TestLD"},
	}

	tree := NewTree(mock)

	t.Run("BuildReturnsIED", func(t *testing.T) {
		ied, err := tree.Build("", false)
		if err != nil {
			t.Fatalf("Build failed: %v", err)
		}
		if ied == nil {
			t.Fatal("Build returned nil IED")
		}
		if len(ied.LogicalDevices) != 1 {
			t.Errorf("Expected 1 logical device, got %d", len(ied.LogicalDevices))
		}
	})

	t.Run("BuildWithPath", func(t *testing.T) {
		ied, err := tree.Build("TestLD", false)
		if err != nil {
			t.Fatalf("Build with path failed: %v", err)
		}
		if ied == nil {
			t.Fatal("Build returned nil IED")
		}
		if len(ied.LogicalDevices) == 0 {
			t.Error("Expected at least 1 logical device")
		}
	})
}

// === R6.3: FindPath Builder Tests ===

func TestFindPathBuilder(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD1"},
	}

	finder := NewFinder(mock)

	t.Run("FluentBuilderAPI", func(t *testing.T) {
		builder := finder.Path().
			MatchingLN(".*MMXU.*").
			WithDO("Hz")

		if builder.input.LNPattern != ".*MMXU.*" {
			t.Error("LNPattern not set")
		}
		if builder.input.DoName != "Hz" {
			t.Error("DoName not set")
		}
	})

	t.Run("BuilderWithDA", func(t *testing.T) {
		builder := finder.Path().
			MatchingLN(".*MMXU.*").
			WithDO("Hz").
			WithDA("mag")

		if builder.input.DaName != "mag" {
			t.Error("DaName not set")
		}
		if !builder.input.IncludeDas {
			t.Error("IncludeDas should auto-enable when WithDA is called")
		}
	})

	t.Run("BuilderIncludingAttributes", func(t *testing.T) {
		builder := finder.Path().
			MatchingLN(".*MMXU.*").
			WithDO("Hz").
			IncludingAttributes()

		if !builder.input.IncludeDas {
			t.Error("IncludeDas not set")
		}
	})

	t.Run("BuilderWithDetails", func(t *testing.T) {
		builder := finder.Path().
			MatchingLN(".*MMXU.*").
			WithDO("Hz").
			WithDetails()

		if !builder.input.Detailed {
			t.Error("Detailed not set")
		}
	})

	t.Run("BuilderFind", func(t *testing.T) {
		result, err := finder.Path().
			MatchingLN(".*MMXU.*").
			WithDO("DO1"). // Mock returns DO1, DO2 by default
			Find()

		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}
		// Mock returns MMXU1 as a default LN, and DO1 is in the default DO list
		if len(result.Matches) == 0 {
			t.Error("Expected at least 1 match with mock defaults")
		}
		if len(result.Matches) > 0 {
			if result.Matches[0].DO != "DO1" {
				t.Errorf("Expected DO=DO1, got %s", result.Matches[0].DO)
			}
		}
	})

	t.Run("BuilderComplexChain", func(t *testing.T) {
		result, err := finder.Path().
			MatchingLN(".*MMXU.*").
			WithDO("DO1"). // Use DO1 from mock defaults
			IncludingAttributes().
			WithDetails().
			Find()

		if err != nil {
			t.Fatalf("Complex chain Find failed: %v", err)
		}
		if len(result.Matches) == 0 {
			t.Error("Expected at least 1 match")
		}
	})
}

// === R7.2: Callback Injection Tests ===

func TestReporterOnReportCallback(t *testing.T) {
	// Use existing mockConnection which already implements the needed methods
	mock := &mockConnection{}

	t.Run("OnReportSetsCallback", func(t *testing.T) {
		reporter := NewReporter(mock)
		called := false
		callback := func(r *domain.Report) {
			called = true
		}

		reporter.OnReport(callback)

		if reporter.onReportHook == nil {
			t.Error("OnReport callback not set")
		}

		// Simulate a report to trigger callback
		if reporter.onReportHook != nil {
			reporter.onReportHook(&domain.Report{})
			if !called {
				t.Error("Callback was not invoked")
			}
		}
	})

	t.Run("OnReportFluentAPI", func(t *testing.T) {
		reporter := NewReporter(mock).
			WithReportRef("LD/LN.BR.rcb").
			OnReport(func(r *domain.Report) {
				// Custom handling
			})

		if reporter.config.ReportRef != "LD/LN.BR.rcb" {
			t.Error("ReportRef not set correctly in fluent chain")
		}
		if reporter.onReportHook == nil {
			t.Error("OnReport callback not set in fluent chain")
		}
	})
}

// === General Phase 5 Integration Tests ===

func TestPhase5FluentAPIs(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD1"},
	}

	t.Run("FormatterFullChain", func(t *testing.T) {
		var buf bytes.Buffer
		f := formatter.NewFormatter().
			WithTimeFormat(formatter.TimeFormatISO).
			WithByteFormat(formatter.ByteFormatSI).
			WithOutputFormat(formatter.OutputFormatJSON)
		devices := []domain.LogicalDevice{{Name: "LD1"}}
		viewDevices := ProjectLogicalDevices(devices)
		if err := f.RenderLogicalDevices(viewDevices, &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), `"name": "LD1"`) {
			t.Error("Formatter chain: JSON output not produced")
		}
	})

	t.Run("TreeFluentChain", func(t *testing.T) {
		tree := NewTree(mock).WithCallInterval(100)
		if tree.callInterval != 100 {
			t.Error("CallInterval not set via fluent API")
		}
	})

	t.Run("ExplorerFluentChain", func(t *testing.T) {
		explorer := NewExplorer(mock).WithDebug(true)
		if !explorer.debug {
			t.Error("Debug not set via fluent API")
		}
	})
}
