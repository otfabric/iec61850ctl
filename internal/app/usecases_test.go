// SPDX-License-Identifier: MIT

package app

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

func TestAppAccessors(t *testing.T) {
	m := &mockConn{}
	a := New(m)
	if a.Connection() != m {
		t.Fatal("Connection mismatch")
	}
	if a.Explorer() == nil || a.Reader() == nil || a.DataSetService() == nil || a.ReportService() == nil || a.JournalService() == nil {
		t.Fatal("nil service accessor")
	}
}

func TestListLogicalDevicesAndNames(t *testing.T) {
	a := New(&mockConn{lds: []string{"LD0", "LD1"}})
	names, err := a.ListLogicalDeviceNames()
	if err != nil || len(names) != 2 {
		t.Fatalf("names=%v err=%v", names, err)
	}
	devs, err := a.ListLogicalDevices()
	if err != nil || len(devs) != 2 {
		t.Fatalf("devs=%v err=%v", devs, err)
	}
}

func TestListLogicalDevices_Error(t *testing.T) {
	a := New(&mockConn{err: errors.New("boom")})
	if _, err := a.ListLogicalDevices(); err == nil {
		t.Fatal("expected error")
	}
}

func TestListLogicalNodes(t *testing.T) {
	a := New(&mockConn{})
	names, err := a.ListLogicalNodeNames(ListLogicalNodesInput{LD: "LD0"})
	if err != nil || len(names) < 1 {
		t.Fatalf("names=%v err=%v", names, err)
	}
	nodes, err := a.ListLogicalNodes(ListLogicalNodesInput{LD: "LD0"})
	if err != nil || len(nodes) < 1 {
		t.Fatalf("nodes=%v err=%v", nodes, err)
	}
}

func TestListDataObjectNames(t *testing.T) {
	a := New(&mockConn{})
	names, err := a.ListDataObjectNames(ListDataObjectsInput{LD: "LD0", LN: "MMXU1"})
	if err != nil || len(names) < 1 {
		t.Fatalf("names=%v err=%v", names, err)
	}
}

func TestGetObject(t *testing.T) {
	a := New(&mockConn{})
	obj, err := a.GetObject(GetObjectInput{Object: "LD0/MMXU1.Hz.mag.f", FC: domain.FC_MX})
	if err != nil || obj == nil {
		t.Fatalf("obj=%v err=%v", obj, err)
	}
}

func TestListFilesAndDownload(t *testing.T) {
	a := New(&mockConn{})
	files, err := a.ListFiles("")
	if err != nil || len(files) < 1 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	var buf bytes.Buffer
	entry, err := a.DownloadFile("conf.xml", &buf)
	if err != nil || entry == nil || buf.String() != "data" {
		t.Fatalf("entry=%v buf=%q err=%v", entry, buf.String(), err)
	}
}

func TestListAndGetReports(t *testing.T) {
	a := New(&mockConn{reports: []string{"LLN0$BR$rcb1", "LLN0$RP$urcb1"}})
	refs, err := a.ListAllReports()
	if err != nil || len(refs) < 1 {
		t.Fatalf("refs=%v err=%v", refs, err)
	}
	u, b, err := a.ListReportNames(ListReportsInput{LD: "LD0", LN: "LLN0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 || len(u) != 1 {
		t.Fatalf("u=%v b=%v", u, b)
	}
	buffered := true
	rep, err := a.GetReport(GetReportInput{LD: "LD0", LN: "LLN0", Name: "rcb1", Buffered: &buffered})
	if err != nil || rep == nil || rep.Report.RptID == "" {
		t.Fatalf("rep=%+v err=%v", rep, err)
	}
}

func TestDataSetNames(t *testing.T) {
	a := New(&mockConn{datasets: []string{"LLN0$Meas"}})
	names, err := a.ListDataSetNames(ListDataSetsInput{LD: "LD0", LN: "LLN0"})
	if err != nil || len(names) < 1 {
		t.Fatalf("names=%v err=%v", names, err)
	}
	ds, err := a.GetDataSet(GetDataSetInput{LD: "LD0", LN: "LLN0", Name: "Meas"})
	if err != nil || ds == nil {
		t.Fatalf("ds=%v err=%v", ds, err)
	}
}

func TestJournals(t *testing.T) {
	a := New(&mockConn{journals: []string{"LLN0$EventLog"}})
	infos, err := a.ListJournals(ListJournalsInput{LD: "LD0"})
	if err != nil || len(infos) < 1 {
		t.Fatalf("infos=%v err=%v", infos, err)
	}
	to := uint64(2000)
	res, err := a.GetJournalEntries(GetJournalEntriesInput{
		DomainID: "LD0", JournalName: "LLN0$EventLog", FromMs: 1000, ToMs: &to,
	})
	if err != nil || res == nil {
		t.Fatalf("res=%v err=%v", res, err)
	}
}

func TestFindPath(t *testing.T) {
	a := New(&mockConn{lds: []string{"LD0"}, lns: []string{"MMXU1"}, dos: []string{"Hz", "Mod"}})
	res, err := a.FindPath(FindPathInput{LNPattern: "MMXU", DOName: "Hz"})
	if err != nil || res == nil || len(res.Matches) < 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestListDataAttributes(t *testing.T) {
	a := New(&mockConn{})
	attrs, err := a.ListDataAttributes(ListDataAttributesInput{LD: "LD0", LN: "MMXU1", DO: "Hz"})
	if err != nil {
		t.Fatal(err)
	}
	_ = attrs
	flat, err := a.GetDataAttributes(ListDataAttributesInput{LD: "LD0", LN: "MMXU1", DO: "Hz", Detailed: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = flat
}

func TestListDataSets(t *testing.T) {
	a := New(&mockConn{datasets: []string{"LLN0$Meas"}})
	list, err := a.ListDataSets(ListDataSetsInput{LD: "LD0", LN: "LLN0"})
	if err != nil || len(list) < 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
}

func TestRenderAndBuildTree(t *testing.T) {
	a := New(&mockConn{lds: []string{"LD0"}})
	var buf bytes.Buffer
	n, err := a.RenderTree(&buf, TreeInput{Host: "h", Port: 102, Path: "LD0/LLN0"})
	if err != nil {
		t.Fatal(err)
	}
	_ = n
	ied, err := a.BuildSerializableTree(TreeInput{Host: "h", Port: 102, Path: "LD0"})
	if err != nil {
		t.Fatal(err)
	}
	if ied == nil {
		t.Fatal("nil ied")
	}
}

func TestListDataObjects(t *testing.T) {
	a := New(&mockConn{dos: []string{"Hz", "Mod"}})
	dos, err := a.ListDataObjects(ListDataObjectsInput{LD: "LD0", LN: "MMXU1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(dos) != 2 {
		t.Fatalf("len=%d", len(dos))
	}
	if dos[0].Name != "Hz" {
		t.Fatalf("name=%q", dos[0].Name)
	}
}

func TestBulkFind(t *testing.T) {
	a := New(&mockConn{
		lds: []string{"LD0"},
		lns: []string{"LLN0", "FMMXU1"},
		dos: []string{"Hz", "DO1"},
	})
	empty, err := a.BulkFind(BulkFindInput{})
	if err != nil || empty == nil || empty.CallCount != 0 {
		t.Fatalf("empty: %+v err=%v", empty, err)
	}
	res, err := a.BulkFind(BulkFindInput{Mappings: []service.BulkMappingEntry{
		{ControlledPropertyId: "p1", BaseLn: "MMXU", DoDaPath: "Hz"},
		{ControlledPropertyId: "p2", BaseLn: "MMXU", DoDaPath: "DO1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("entries=%+v", res.Entries)
	}
	if res.CallCount == 0 {
		t.Fatal("expected device calls")
	}
}

func TestGetDataSetWithValues(t *testing.T) {
	a := New(&mockConn{datasets: []string{"LLN0$Meas"}})
	ds, err := a.GetDataSetWithValues(GetDataSetWithValuesInput{
		LD: "LD0", LN: "LLN0", Name: "Meas", ReadValues: false,
	})
	if err != nil || ds == nil {
		t.Fatalf("ds=%v err=%v", ds, err)
	}
	ds, err = a.GetDataSetWithValues(GetDataSetWithValuesInput{
		LD: "LD0", LN: "LLN0", Name: "Meas", ReadValues: true,
	})
	if err != nil || ds == nil {
		t.Fatalf("with values: ds=%v err=%v", ds, err)
	}
	if len(ds.Members) == 0 {
		t.Fatal("expected members")
	}
	if ds.Members[0].Value == "" {
		t.Fatal("expected formatted member value")
	}
}

func TestSubscribeReport(t *testing.T) {
	a := New(&mockConn{})
	var buf bytes.Buffer

	// Invalid config
	if _, err := a.SubscribeReport(SubscribeReportInput{}); err == nil {
		t.Fatal("expected validation error")
	}

	// Valid config, subscription fails in mock (covers Run error path)
	_, err := a.SubscribeReport(SubscribeReportInput{
		ReportRef: "LD0/LLN0.BR.rcb1",
		Duration:  10 * time.Millisecond,
		Writer:    &buf,
	})
	if err == nil {
		t.Fatal("expected subscribe error from mock")
	}
}
