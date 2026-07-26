// SPDX-License-Identifier: MIT

package service

import (
	"testing"
	"time"
)

func TestReporterFluentAPI(t *testing.T) {
	reporter := NewReporter(nil).
		WithReportRef("LD0/LN0.BR.rcb1").
		WithDuration(10 * time.Second).
		WithMaxReports(100).
		WithShowValues(true)

	if reporter.config.ReportRef != "LD0/LN0.BR.rcb1" {
		t.Errorf("Expected ReportRef=LD0/LN0.BR.rcb1, got %s", reporter.config.ReportRef)
	}
	if reporter.config.Duration != 10*time.Second {
		t.Errorf("Expected Duration=10s, got %v", reporter.config.Duration)
	}
	if reporter.config.MaxReports != 100 {
		t.Errorf("Expected MaxReports=100, got %d", reporter.config.MaxReports)
	}
	if !reporter.config.ShowValues {
		t.Error("Expected ShowValues=true")
	}
}

func TestReporterWithConfig(t *testing.T) {
	config := ReporterConfig{
		ReportRef:  "LD0/LN0.BR.rcb1",
		Duration:   10 * time.Second,
		MaxReports: 100,
	}

	reporter := NewReporter(nil).WithConfig(config)

	if reporter.config.ReportRef != "LD0/LN0.BR.rcb1" {
		t.Errorf("Expected ReportRef=LD0/LN0.BR.rcb1, got %s", reporter.config.ReportRef)
	}
}

func TestExplorerFluentAPI(t *testing.T) {
	explorer := NewExplorer(nil).WithDebug(true)

	if !explorer.debug {
		t.Error("Expected debug=true")
	}
}

func TestTreeFluentAPI(t *testing.T) {
	tree := NewTree(nil).WithCallInterval(100 * time.Millisecond)

	if tree.callInterval != 100*time.Millisecond {
		t.Errorf("Expected callInterval=100ms, got %v", tree.callInterval)
	}
}
