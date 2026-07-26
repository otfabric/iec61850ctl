// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestReasonToken(t *testing.T) {
	cases := []struct {
		in   domain.ReasonForInclusion
		want string
	}{
		{domain.ReasonGI, "gi"},
		{domain.ReasonDataChange, "dchg"},
		{domain.ReasonQualityChange, "qchg"},
		{domain.ReasonDataUpdate, "dupd"},
		{domain.ReasonIntegrity, "integrity"},
		{domain.ReasonNotIncluded, ""},
	}
	for _, tc := range cases {
		if got := reasonToken(tc.in); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestReporterConfig_ValidateFormat(t *testing.T) {
	cfg := ReporterConfig{ReportRef: "LD/LLN0.RP.urcb01", Format: "yaml"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid format error")
	}
	cfg.Format = "jsonl"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestReasonTokens_Dedup(t *testing.T) {
	report := &domain.Report{
		Elements: []domain.ReportElement{
			{Reason: domain.ReasonGI},
			{Reason: domain.ReasonGI},
			{Reason: domain.ReasonDataChange},
			{Reason: domain.ReasonNotIncluded},
		},
	}
	got := reasonTokens(report)
	if len(got) != 2 || got[0] != "gi" || got[1] != "dchg" {
		t.Fatalf("got %#v", got)
	}
}

func TestWriteJSONLAndSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSONL(&buf, jsonlEvent{Event: "baseline", DataSet: "ds"}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONLSummary(&buf, 1, true, 1500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), buf.String())
	}
	var summary jsonlEvent
	if err := json.Unmarshal(lines[1], &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Event != "summary" || summary.ReportsReceived != 1 || summary.DurationMS != 1500 {
		t.Fatalf("summary: %+v", summary)
	}
	if summary.CleanDisable == nil || !*summary.CleanDisable {
		t.Fatal("expected clean_disable true")
	}
}

func TestReportValuesJSON(t *testing.T) {
	report := &domain.Report{
		Elements: []domain.ReportElement{
			{Value: domain.NewValue(true, domain.TypeBoolean)},
			{Value: domain.NewValue(int64(2), domain.TypeInteger)},
		},
	}
	if reportValuesJSON(report, false) != nil {
		t.Fatal("show=false should omit values")
	}
	got := reportValuesJSON(report, true)
	if len(got) != 2 || got[0] != true || got[1] != int64(2) {
		t.Fatalf("got %#v", got)
	}
}

func TestBaselineValuesJSON(t *testing.T) {
	if baselineValuesJSON(nil, false) != nil {
		t.Fatal("show=false should omit values")
	}
	if got := baselineValuesJSON(nil, true); len(got) != 0 {
		t.Fatalf("empty input: %#v", got)
	}
}
