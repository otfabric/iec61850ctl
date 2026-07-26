// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
)

func TestLnMatches(t *testing.T) {
	if !lnMatches("LLN0$ds1", "LLN0") {
		t.Error("expected prefix match")
	}
	if !lnMatches("LD0/LLN0$ds1", "LLN0") {
		t.Error("expected /LN$ match")
	}
	if lnMatches("MMXU1$ds1", "LLN0") {
		t.Error("expected non-match")
	}
}

func TestDataSetService_ListDataSets(t *testing.T) {
	mock := &mockConnection{
		dataSets: []string{"LLN0$ds1", "LLN0$ds2", "MMXU1$meas", "orphan"},
	}
	svc := NewDataSetService(mock)

	all, err := svc.ListDataSets("LD0", "")
	if err != nil {
		t.Fatalf("ListDataSets: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("want 4, got %d", len(all))
	}

	filtered, err := svc.ListDataSets("LD0", "LLN0")
	if err != nil {
		t.Fatalf("ListDataSets LN filter: %v", err)
	}
	if len(filtered) < 2 {
		t.Fatalf("want at least ds1/ds2, got %v", filtered)
	}
	for _, name := range filtered {
		if name == "meas" {
			t.Fatalf("MMXU1 dataset leaked into LLN0 filter: %v", filtered)
		}
	}

	// Fallback path when LN filter yields nothing.
	mock2 := &mockConnection{dataSets: []string{"other$dsX", "bare"}}
	fallback, err := NewDataSetService(mock2).ListDataSets("LD0", "LLN0")
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if len(fallback) != 2 || fallback[0] != "dsX" || fallback[1] != "bare" {
		t.Fatalf("fallback got %v", fallback)
	}

	mock3 := &mockConnection{listDataSetsErr: errors.New("boom")}
	if _, err := NewDataSetService(mock3).ListDataSets("LD0", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestDataSetService_GetDataSet(t *testing.T) {
	ref, _ := iec61850.ParseRef("LD0/LLN0.DO1.mag.f")
	ds := &iec61850.DataSet{
		Deletable: true,
		Members:   []iec61850.DataSetMember{{Ref: ref}},
	}
	vals := []iec61850.DataSetValue{
		{Value: iec61850.NewValue(mms.NewFloat(3.14))},
	}
	mock := &mockConnection{dataSet: ds, dataSetValues: vals}
	svc := NewDataSetService(mock)

	details, err := svc.GetDataSet("LD0", "LLN0", "ds1", true)
	if err != nil {
		t.Fatalf("GetDataSet: %v", err)
	}
	if details.Name != "ds1" || !details.Deletable || len(details.Members) != 1 {
		t.Fatalf("unexpected details: %+v", details)
	}
	if details.Members[0].Value == nil {
		t.Fatal("expected member value")
	}

	detailsNoVal, err := svc.GetDataSet("LD0", "LLN0", "ds1", false)
	if err != nil {
		t.Fatalf("GetDataSet no values: %v", err)
	}
	if detailsNoVal.Members[0].Value != nil {
		t.Fatal("expected nil value when withValues=false")
	}

	failOnce := &failThenOKDataSet{ok: ds, vals: vals}
	got, err := NewDataSetService(failOnce).GetDataSet("LD0", "LLN0", "ds1", true)
	if err != nil {
		t.Fatalf("retry GetDataSet: %v", err)
	}
	if got == nil || len(got.Members) != 1 {
		t.Fatalf("retry got %+v", got)
	}

	mockErr := &mockConnection{getDataSetErr: errors.New("missing")}
	if _, err := NewDataSetService(mockErr).GetDataSet("LD0", "LLN0", "ds1", false); err == nil {
		t.Fatal("expected get error")
	}

	mockValErr := &mockConnection{dataSet: ds, readDataSetErr: errors.New("read failed")}
	partial, err := NewDataSetService(mockValErr).GetDataSet("LD0", "LLN0", "ds1", true)
	if err == nil {
		t.Fatal("expected values error")
	}
	if partial == nil || len(partial.Members) != 1 {
		t.Fatalf("expected metadata despite value error, got %+v", partial)
	}
}

func TestDataSetService_GetDataSetDetails(t *testing.T) {
	ref, _ := iec61850.ParseRef("LD0/LLN0.DO1.mag.f")
	mock := &mockConnection{
		dataSet: &iec61850.DataSet{
			Deletable: false,
			Members:   []iec61850.DataSetMember{{Ref: ref}},
		},
	}
	ds, err := NewDataSetService(mock).GetDataSetDetails("LD0", "LLN0", "ds1")
	if err != nil {
		t.Fatalf("GetDataSetDetails: %v", err)
	}
	if ds.Name != "ds1" || len(ds.Members) != 1 {
		t.Fatalf("got %+v", ds)
	}
}

// failThenOKDataSet fails the first GetDataSet (LN$Name) then succeeds on bare name.
type failThenOKDataSet struct {
	mockConnection
	ok   *iec61850.DataSet
	vals []iec61850.DataSetValue
	n    int
}

func (m *failThenOKDataSet) GetDataSet(_ context.Context, _, dsName string) (*iec61850.DataSet, error) {
	m.n++
	if m.n == 1 {
		return nil, errors.New("not found as LN$Name")
	}
	return m.ok, nil
}

func (m *failThenOKDataSet) ReadDataSet(_ context.Context, _, _ string) ([]iec61850.DataSetValue, error) {
	return m.vals, nil
}
