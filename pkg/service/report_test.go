// SPDX-License-Identifier: MIT

package service

import (
	"errors"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
)

func TestReportService_ListReports(t *testing.T) {
	mock := &mockConnection{
		reports: []string{"LLN0$BR$rcb1", "LLN0$RP$urcb1", "MMXU1$BR$brcb01"},
	}
	svc := NewReportService(mock)

	br, err := svc.ListBufferedReports("LD0", "LLN0")
	if err != nil {
		t.Fatalf("ListBufferedReports: %v", err)
	}
	if len(br) != 1 || br[0] != "rcb1" {
		t.Fatalf("buffered=%v", br)
	}

	ur, err := svc.ListUnbufferedReports("LD0", "LLN0")
	if err != nil {
		t.Fatalf("ListUnbufferedReports: %v", err)
	}
	if len(ur) != 1 || ur[0] != "urcb1" {
		t.Fatalf("unbuffered=%v", ur)
	}

	mockErr := &mockConnection{listReportsErr: errors.New("fail")}
	if _, err := NewReportService(mockErr).ListBufferedReports("LD0", "LLN0"); err == nil {
		t.Fatal("expected error")
	}
}

func TestReportService_GetAllReports(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD0"},
		reports:        []string{"LLN0$BR$rcb1", "LLN0$RP$urcb1", "bad", "MMXU1$BR$brcb01"},
	}
	all, err := NewReportService(mock).GetAllReports()
	if err != nil {
		t.Fatalf("GetAllReports: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 valid reports, got %d (%+v)", len(all), all)
	}
	var sawBR, sawRP bool
	for _, r := range all {
		if r.Name == "rcb1" && r.Buffered {
			sawBR = true
		}
		if r.Name == "urcb1" && !r.Buffered {
			sawRP = true
		}
	}
	if !sawBR || !sawRP {
		t.Fatalf("missing buffered/unbuffered refs: %+v", all)
	}

	paged, err := NewReportService(mock).GetAllReportsPaged(PageOptions{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("GetAllReportsPaged: %v", err)
	}
	if len(paged.Items) != 1 || !paged.HasMore || paged.Total != 3 {
		t.Fatalf("paged=%+v", paged)
	}
}

func TestReportService_GetReportDetails(t *testing.T) {
	mock := &mockConnection{
		rcb: &iec61850.ReportControlBlock{
			RptID:    "id1",
			DatSet:   "LD0/LLN0$ds1",
			RptEna:   true,
			ConfRev:  2,
			BufTm:    50,
			SqNum:    3,
			IntgPd:   500,
			Resv:     false,
			PurgeBuf: false,
			EntryID:  []byte{0, 0, 0, 0, 0, 0, 0, 9},
			ResvTms:  15,
			TrgOps:   iec61850.TrgOpDataChanged | iec61850.TrgOpQualityChanged | iec61850.TrgOpIntegrity,
			OptFlds:  iec61850.OptFldSeqNum | iec61850.OptFldDataSet | iec61850.OptFldEntryID,
		},
	}
	svc := NewReportService(mock)
	rcb, err := svc.GetReportDetails("LD0", "LLN0", "rcb1", true)
	if err != nil {
		t.Fatalf("GetReportDetails: %v", err)
	}
	if rcb.RptID != "id1" || !rcb.Buffered || rcb.Enabled == nil || !*rcb.Enabled {
		t.Fatalf("rcb=%+v", rcb)
	}
	if !rcb.TriggerOptions.DataChange || !rcb.OptionalFields.SequenceNumber {
		t.Fatalf("flags=%+v %+v", rcb.TriggerOptions, rcb.OptionalFields)
	}
	if rcb.PurgeBuf == nil || *rcb.PurgeBuf || len(rcb.EntryID) != 8 || rcb.ResvTms == nil || *rcb.ResvTms != 15 {
		t.Fatalf("BRCB fields=%+v", rcb)
	}
	view := ProjectReportControlBlock(*rcb)
	if view.EntryID == "" || view.ResvTms == nil {
		t.Fatalf("projected BRCB=%+v", view)
	}

	mockErr := &mockConnection{getRCBErr: errors.New("missing")}
	if _, err := NewReportService(mockErr).GetReportDetails("LD0", "LLN0", "x", false); err == nil {
		t.Fatal("expected error")
	}
}

func TestReportService_ResolveReportDetails(t *testing.T) {
	brcb := &iec61850.ReportControlBlock{RptID: "br"}
	urcb := &iec61850.ReportControlBlock{RptID: "ur"}

	// Prefer buffered when available.
	mockBR := &mockConnection{
		getRCBByItemID: map[string]*iec61850.ReportControlBlock{
			"LLN0$BR$rcb1": brcb,
		},
	}
	got, err := NewReportService(mockBR).ResolveReportDetails("LD0", "LLN0", "rcb1")
	if err != nil || got.RptID != "br" || !got.Buffered {
		t.Fatalf("prefer BR: %+v err=%v", got, err)
	}

	// Fall back to unbuffered when BR fails.
	mockRP := &mockConnection{
		getRCBErrByItemID: map[string]error{
			"LLN0$BR$rcb1": errors.New("no br"),
		},
		getRCBByItemID: map[string]*iec61850.ReportControlBlock{
			"LLN0$RP$rcb1": urcb,
		},
	}
	got2, err := NewReportService(mockRP).ResolveReportDetails("LD0", "LLN0", "rcb1")
	if err != nil || got2.RptID != "ur" || got2.Buffered {
		t.Fatalf("fallback RP: %+v err=%v", got2, err)
	}
}

func TestMapRCB_Nil(t *testing.T) {
	if mapRCB("LD0", "LLN0", "r", true, nil) != nil {
		t.Fatal("expected nil")
	}
}
