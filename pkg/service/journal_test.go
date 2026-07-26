// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
)

func TestNewJournal(t *testing.T) {
	j := NewJournal(&mockConnection{})
	if j == nil || j.conn == nil {
		t.Fatal("NewJournal failed")
	}
}

func TestJournal_ListJournals(t *testing.T) {
	mock := &mockConnection{
		journals: []string{"LLN0$EventLog", "LLN0.GeneralLog", "plainLog"},
	}
	infos, err := NewJournal(mock).ListJournals("LD0")
	if err != nil {
		t.Fatalf("ListJournals: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("want 3, got %d", len(infos))
	}
	if infos[0].LogicalNode != "LLN0" || infos[0].Name != "EventLog" {
		t.Fatalf("infos[0]=%+v", infos[0])
	}
	if infos[1].LogicalNode != "LLN0" || infos[1].Name != "GeneralLog" {
		t.Fatalf("infos[1]=%+v", infos[1])
	}
	if infos[2].Name != "plainLog" || infos[2].FullRef != "LD0/plainLog" {
		t.Fatalf("infos[2]=%+v", infos[2])
	}

	names, err := NewJournal(mock).ListJournalNames("LD0")
	if err != nil || len(names) != 3 {
		t.Fatalf("ListJournalNames: %v %v", names, err)
	}

	mockErr := &mockConnection{listJournalsErr: errors.New("no journals")}
	if _, err := NewJournal(mockErr).ListJournals("LD0"); err == nil {
		t.Fatal("expected error")
	}
}

func TestJournal_ReadTimeRangeAndStartAfter(t *testing.T) {
	mock := &mockConnection{}
	j := NewJournal(mock)

	entries, more, err := j.ReadTimeRange("LD0", "LLN0$EventLog", 0, 2_000_000_000_000)
	if err != nil {
		t.Fatalf("ReadTimeRange: %v", err)
	}
	if more || len(entries) != 1 || entries[0].EntryID != "01" {
		t.Fatalf("entries=%+v more=%v", entries, more)
	}
	if len(entries[0].Variables) != 1 || entries[0].Variables[0].Tag != "tag1" {
		t.Fatalf("variables=%+v", entries[0].Variables)
	}

	entries2, more2, err := j.ReadStartAfter("LD0", "LLN0$EventLog", 0, nil)
	if err != nil {
		t.Fatalf("ReadStartAfter: %v", err)
	}
	if more2 || len(entries2) != 1 || entries2[0].EntryID != "02" {
		t.Fatalf("entries=%+v more=%v", entries2, more2)
	}

	mockErr := &mockConnection{readJournalErr: errors.New("read fail")}
	if _, _, err := NewJournal(mockErr).ReadTimeRange("LD0", "j", 0, 1); err == nil {
		t.Fatal("expected ReadTimeRange error")
	}
	mockErr2 := &mockConnection{readJournalAfterErr: errors.New("after fail")}
	if _, _, err := NewJournal(mockErr2).ReadStartAfter("LD0", "j", 0, nil); err == nil {
		t.Fatal("expected ReadStartAfter error")
	}
}

func TestJournal_GetEntries(t *testing.T) {
	end := uint64(2_000_000_000_000)
	mock := &mockConnection{}
	res, err := NewJournal(mock).GetEntries("LD0", "LLN0$EventLog", 0, &end)
	if err != nil {
		t.Fatalf("GetEntries range: %v", err)
	}
	if len(res.Entries) != 1 || res.MoreFollows {
		t.Fatalf("got %+v", res)
	}

	res2, err := NewJournal(mock).GetEntries("LD0", "LLN0$EventLog", 0, nil)
	if err != nil {
		t.Fatalf("GetEntries start-after: %v", err)
	}
	if len(res2.Entries) != 1 {
		t.Fatalf("got %+v", res2)
	}

	// Paginate time range once then stop.
	page1 := &iec61850.JournalReadResult{
		MoreFollows: true,
		Entries: []iec61850.JournalEntry{{
			EntryID:        []byte{0xaa},
			OccurrenceTime: time.UnixMilli(1000).UTC(),
			Variables:      []iec61850.JournalVariable{{Tag: "a"}},
		}},
	}
	page2 := &iec61850.JournalReadResult{
		MoreFollows: false,
		Entries: []iec61850.JournalEntry{{
			EntryID:        []byte{0xbb},
			OccurrenceTime: time.UnixMilli(2000).UTC(),
		}},
	}
	paging := &pagingJournalConn{pages: []*iec61850.JournalReadResult{page1, page2}}
	end2 := uint64(10_000)
	paged, err := NewJournal(paging).GetEntries("LD0", "j", 0, &end2)
	if err != nil {
		t.Fatalf("paged GetEntries: %v", err)
	}
	if len(paged.Entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(paged.Entries))
	}
}

func TestMapJournalResult(t *testing.T) {
	entries, more, err := mapJournalResult(nil)
	if err != nil || more || entries != nil {
		t.Fatalf("nil result: %v %v %v", entries, more, err)
	}

	res := &iec61850.JournalReadResult{
		MoreFollows: true,
		Entries: []iec61850.JournalEntry{
			{
				EntryID:        []byte{0xde, 0xad},
				OccurrenceTime: time.Date(2024, 1, 2, 3, 4, 5, 6_000_000, time.UTC),
				Variables: []iec61850.JournalVariable{
					{Tag: "x", Value: iec61850.NewValue(mms.NewFloat(1.5))},
					{Tag: "y", Value: nil},
				},
			},
		},
	}
	out, more, err := mapJournalResult(res)
	if err != nil || !more || len(out) != 1 {
		t.Fatalf("map: %v more=%v out=%+v", err, more, out)
	}
	if out[0].EntryID != "dead" {
		t.Fatalf("EntryID=%q", out[0].EntryID)
	}
	if len(out[0].Variables) != 2 || out[0].Variables[0].Value == "" {
		t.Fatalf("variables=%+v", out[0].Variables)
	}
}

func TestJournal_GetEntries_StartAfterPaging(t *testing.T) {
	page1 := &iec61850.JournalReadResult{
		MoreFollows: true,
		Entries: []iec61850.JournalEntry{{
			EntryID:        []byte{0x01},
			OccurrenceTime: time.UnixMilli(1000).UTC(),
		}},
	}
	page2 := &iec61850.JournalReadResult{
		MoreFollows: false,
		Entries: []iec61850.JournalEntry{{
			EntryID:        []byte{0x02},
			OccurrenceTime: time.UnixMilli(2000).UTC(),
		}},
	}
	conn := &pagingJournalAfterConn{pages: []*iec61850.JournalReadResult{page1, page2}}
	res, err := NewJournal(conn).GetEntries("LD0", "j", 0, nil)
	if err != nil {
		t.Fatalf("GetEntries: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("want 2, got %d", len(res.Entries))
	}
}

type pagingJournalConn struct {
	mockConnection
	pages []*iec61850.JournalReadResult
	i     int
}

func (p *pagingJournalConn) ReadJournal(_ context.Context, _, _ string, _, _ time.Time) (*iec61850.JournalReadResult, error) {
	if p.i >= len(p.pages) {
		return &iec61850.JournalReadResult{}, nil
	}
	r := p.pages[p.i]
	p.i++
	return r, nil
}

type pagingJournalAfterConn struct {
	mockConnection
	pages []*iec61850.JournalReadResult
	i     int
}

func (p *pagingJournalAfterConn) ReadJournalAfter(_ context.Context, _, _ string, _ time.Time, _ []byte) (*iec61850.JournalReadResult, error) {
	if p.i >= len(p.pages) {
		return &iec61850.JournalReadResult{}, nil
	}
	r := p.pages[p.i]
	p.i++
	return r, nil
}
