// SPDX-License-Identifier: MIT

package domain

import (
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
)

func TestReportControlBlock_Methods(t *testing.T) {
	empty := &ReportControlBlock{Name: "urcb01", Buffered: false}
	if empty.IsFullyPopulated() {
		t.Error("empty RCB IsFullyPopulated() = true")
	}
	if empty.FCString() != FC_RP {
		t.Errorf("unbuffered FCString() = %v, want RP", empty.FCString())
	}

	buffered := &ReportControlBlock{Name: "brcb01", Buffered: true}
	if buffered.FCString() != FC_BR {
		t.Errorf("buffered FCString() = %v, want BR", buffered.FCString())
	}

	withRptID := &ReportControlBlock{RptID: "rpt-1"}
	if !withRptID.IsFullyPopulated() {
		t.Error("RptID set IsFullyPopulated() = false")
	}

	enabled := true
	withEnabled := &ReportControlBlock{Enabled: &enabled}
	if !withEnabled.IsFullyPopulated() {
		t.Error("Enabled set IsFullyPopulated() = false")
	}

	cr := uint32(3)
	withConf := &ReportControlBlock{ConfRev: &cr}
	if !withConf.IsFullyPopulated() {
		t.Error("ConfRev set IsFullyPopulated() = false")
	}
}

func TestFromReportIndication(t *testing.T) {
	if got := FromReportIndication(nil, true); got != nil {
		t.Fatalf("FromReportIndication(nil) = %+v, want nil", got)
	}

	minimal := &iec61850.ReportIndication{
		RptID:        "rptA",
		DatSet:       "LD0/LLN0$ds1",
		MoreSegments: true,
		SeqNum:       7,
	}
	got := FromReportIndication(minimal, false)
	if got == nil {
		t.Fatal("FromReportIndication returned nil")
	}
	if got.RptID != "rptA" || got.DatSet != "LD0/LLN0$ds1" || !got.MoreSegments {
		t.Fatalf("basic fields = %+v", got)
	}
	if got.SeqNum == nil || *got.SeqNum != 7 {
		t.Fatalf("SeqNum = %v", got.SeqNum)
	}
	if got.SubSeqNum != nil || got.ConfRev != nil || got.BufOvfl != nil || got.Timestamp != nil {
		t.Fatalf("unexpected optional fields set: %+v", got)
	}
	if len(got.Elements) != 0 {
		t.Fatalf("includeValues=false Elements = %v", got.Elements)
	}

	ts := time.UnixMilli(1_700_000_000_000).UTC()
	full := &iec61850.ReportIndication{
		RptID:          "rptB",
		DatSet:         "LD0/LLN0$ds2",
		SeqNum:         1,
		SubSeqNum:      2,
		MoreSegments:   false,
		ConfRev:        9,
		BufOvfl:        true,
		Timestamp:      ts,
		DataReferences: []string{"LD0/GGIO1.stVal"},
		Values:         []*iec61850.Value{iec61850.NewValue(mms.NewBoolean(true)), nil},
		ReasonCodes: []iec61850.ReasonCode{
			iec61850.ReasonDataChanged,
			iec61850.ReasonQualityChanged,
		},
	}
	got = FromReportIndication(full, true)
	if got.SubSeqNum == nil || *got.SubSeqNum != 2 {
		t.Fatalf("SubSeqNum = %v", got.SubSeqNum)
	}
	if got.ConfRev == nil || *got.ConfRev != 9 {
		t.Fatalf("ConfRev = %v", got.ConfRev)
	}
	if got.BufOvfl == nil || !*got.BufOvfl {
		t.Fatalf("BufOvfl = %v", got.BufOvfl)
	}
	if got.Timestamp == nil || got.Timestamp.UnixMs != 1_700_000_000_000 {
		t.Fatalf("Timestamp = %+v", got.Timestamp)
	}
	if len(got.Elements) != 2 {
		t.Fatalf("Elements len = %d", len(got.Elements))
	}
	if got.Elements[0].Index != 0 || got.Elements[0].Ref != "LD0/GGIO1.stVal" {
		t.Fatalf("elem0 = %+v", got.Elements[0])
	}
	if got.Elements[0].Reason != ReasonDataChange {
		t.Fatalf("elem0.Reason = %q", got.Elements[0].Reason)
	}
	if got.Elements[0].Value == nil || got.Elements[0].Value.Raw != true {
		t.Fatalf("elem0.Value = %+v", got.Elements[0].Value)
	}
	if got.Elements[1].Ref != "" {
		t.Fatalf("elem1.Ref = %q, want empty (no data ref)", got.Elements[1].Ref)
	}
	if got.Elements[1].Reason != ReasonQualityChange {
		t.Fatalf("elem1.Reason = %q", got.Elements[1].Reason)
	}
	if got.Elements[1].Value != nil {
		t.Fatalf("elem1.Value = %+v, want nil", got.Elements[1].Value)
	}
}

func TestReasonFromCode(t *testing.T) {
	tests := []struct {
		rc   iec61850.ReasonCode
		want ReasonForInclusion
	}{
		{iec61850.ReasonDataChanged, ReasonDataChange},
		{iec61850.ReasonQualityChanged, ReasonQualityChange},
		{iec61850.ReasonDataUpdate, ReasonDataUpdate},
		{iec61850.ReasonIntegrity, ReasonIntegrity},
		{iec61850.ReasonGI, ReasonGI},
		{0, ReasonUnknown},
		{iec61850.ReasonCode(1 << 6), ReasonUnknown},
		// data-change takes precedence when multiple bits set
		{iec61850.ReasonDataChanged | iec61850.ReasonGI, ReasonDataChange},
	}
	for _, tt := range tests {
		if got := reasonFromCode(tt.rc); got != tt.want {
			t.Errorf("reasonFromCode(%v) = %q, want %q", tt.rc, got, tt.want)
		}
	}
}
