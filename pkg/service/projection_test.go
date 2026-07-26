// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestProjectLogicalDevice_FromCounts(t *testing.T) {
	ld := domain.LogicalDevice{Name: "LD0", LNCount: 2, DSCount: 1, URCBCount: 3, BRCBCount: 4}
	v := ProjectLogicalDevice(ld)
	if v.Name != "LD0" || v.LNCount != 2 || v.DSCount != 1 || v.URCBCount != 3 || v.BRCBCount != 4 {
		t.Fatalf("unexpected projection: %+v", v)
	}
}

func TestProjectLogicalDevice_FromChildren(t *testing.T) {
	ld := domain.LogicalDevice{
		Name: "LD0",
		LogicalNodes: []domain.LogicalNode{
			{
				Name:     "LLN0",
				DataSets: []string{"ds1", "ds2"},
				ReportControlBlocks: []domain.ReportControlBlockRef{
					{Name: "u1", Buffered: false},
					{Name: "b1", Buffered: true},
				},
			},
		},
	}
	v := ProjectLogicalDevice(ld)
	if v.LNCount != 1 {
		t.Fatalf("LNCount=%d want 1", v.LNCount)
	}
	if v.DSCount != 2 || v.URCBCount != 1 || v.BRCBCount != 1 {
		t.Fatalf("counts ds=%d urcb=%d brcb=%d", v.DSCount, v.URCBCount, v.BRCBCount)
	}
}

func TestProjectLogicalDevices(t *testing.T) {
	out := ProjectLogicalDevices([]domain.LogicalDevice{{Name: "A"}, {Name: "B"}})
	if len(out) != 2 || out[0].Name != "A" || out[1].Name != "B" {
		t.Fatalf("got %+v", out)
	}
}

func TestProjectDataAttribute(t *testing.T) {
	da := domain.DataAttribute{
		Name:       "mag.f",
		Ref:        "LD0/LN0.DO.mag.f",
		FC:         domain.FC_MX,
		Type:       domain.TypeFloat,
		Value:      domain.NewValue(float32(1.5), domain.TypeFloat),
		ValueError: "",
	}
	v := ProjectDataAttribute(da)
	if v.Name != "mag.f" || v.FC != "MX" || v.Type == "" || v.Value == "" {
		t.Fatalf("unexpected: %+v", v)
	}

	errDA := domain.DataAttribute{Name: "q", ValueError: "timeout"}
	ev := ProjectDataAttribute(errDA)
	if ev.Value != "error: timeout" {
		t.Fatalf("Value=%q", ev.Value)
	}
}

func TestProjectDataAttributes(t *testing.T) {
	out := ProjectDataAttributes([]domain.DataAttribute{{Name: "a"}, {Name: "b"}})
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
}

func TestProjectDataSet(t *testing.T) {
	ds := domain.DataSet{
		Name: "Meas",
		Members: []domain.DataSetMember{
			{Ref: "LD0/LN0.DO.mag.f", FC: domain.FC_MX, Value: domain.NewValue(1.0, domain.TypeFloat)},
		},
	}
	v := ProjectDataSet(ds)
	if v.Name != "Meas" || len(v.Members) != 1 || v.Members[0].FC != "MX" {
		t.Fatalf("got %+v", v)
	}
}

func TestProjectReportControlBlockRefs(t *testing.T) {
	refs := []domain.ReportControlBlockRef{{LD: "LD0", LN: "LLN0", Name: "r1", Buffered: true}}
	out := ProjectReportControlBlockRefs(refs)
	if len(out) != 1 || out[0].Name != "r1" || !out[0].Buffered {
		t.Fatalf("got %+v", out)
	}
}

func TestProjectFileEntry(t *testing.T) {
	f := domain.FileEntry{Name: "a.bin", Size: 1024, LastModified: 1}
	v := ProjectFileEntry(f, func(u uint64) string { return "1KB" }, func(u uint64) string { return "t" })
	if v.Name != "a.bin" || v.Size != "1KB" || v.LastModified != "t" {
		t.Fatalf("got %+v", v)
	}
}

func TestProjectJournalInfos(t *testing.T) {
	out := ProjectJournalInfos([]domain.JournalInfo{{Name: "log1", LogicalNode: "LLN0", FullRef: "LD0/LLN0$log1"}})
	if len(out) != 1 || out[0].Name != "log1" {
		t.Fatalf("got %+v", out)
	}
}

func TestProjectJournalEntries(t *testing.T) {
	entries := []domain.JournalEntry{{
		EntryID: "01", OccurrenceTime: "t",
		Variables: []domain.JournalVariable{{Tag: "x", Value: "1"}},
	}}
	out := ProjectJournalEntries(entries)
	if len(out) != 1 || len(out[0].Variables) != 1 {
		t.Fatalf("got %+v", out)
	}
}

func TestProjectReportControlBlock(t *testing.T) {
	en := true
	rcb := domain.ReportControlBlock{
		Name:     "rcb1",
		RptID:    "id1",
		DatSet:   "LD0/LLN0$ds1",
		Buffered: true,
		Enabled:  &en,
	}
	v := ProjectReportControlBlock(rcb)
	if v.Name != "rcb1" || !v.Buffered || v.RptID != "id1" {
		t.Fatalf("got %+v", v)
	}
}

func TestProjectJournalEntry(t *testing.T) {
	e := domain.JournalEntry{EntryID: "aa", OccurrenceTime: "2024-01-01T00:00:00Z"}
	v := ProjectJournalEntry(e)
	if v.EntryID != "aa" {
		t.Fatalf("got %+v", v)
	}
}
