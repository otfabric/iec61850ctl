// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"testing"
)

func TestResolveReportRef(t *testing.T) {
	ld, item, buffered, err := resolveReportRef("LD0/LLN0.BR.rcb1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if ld != "LD0" || item != "LLN0$BR$rcb1" || !buffered {
		t.Fatalf("%q %q %v", ld, item, buffered)
	}
	ld, item, buffered, err = resolveReportRef("LD0/LLN0.RP.urcb1")
	if err != nil || buffered || item != "LLN0$RP$urcb1" {
		t.Fatalf("%q %q %v %v", ld, item, buffered, err)
	}
	if _, _, _, err := resolveReportRef("bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestReporter_WithWriterAndValidate(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(nil).WithWriter(&buf).WithReportRef("LD0/LLN0.BR.rcb1")
	if r.config.Writer != &buf {
		t.Fatal("writer not set")
	}
	if err := r.config.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := (ReporterConfig{}).Validate(); err == nil {
		t.Fatal("expected ReportRef required")
	}
	if err := (ReporterConfig{ReportRef: "x", Sync: true}).Validate(); err == nil {
		t.Fatal("expected sync DatasetRef")
	}
	if err := (ReporterConfig{ReportRef: "x", Duration: -1}).Validate(); err == nil {
		t.Fatal("expected duration error")
	}
	if err := (ReporterConfig{ReportRef: "x", MaxReports: -1}).Validate(); err == nil {
		t.Fatal("expected max reports error")
	}
}
