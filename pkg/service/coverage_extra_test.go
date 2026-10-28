// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestExplorer_ListErrorsPropagate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mut  func(*modelConn)
		call func(*Explorer) error
	}{
		{"ListLogicalDevices", func(m *modelConn) { m.ldErr = true }, func(e *Explorer) error { _, err := e.ListLogicalDevices(); return err }},
		{"GetLogicalDevices", func(m *modelConn) { m.ldErr = true }, func(e *Explorer) error { _, err := e.GetLogicalDevices(); return err }},
		{"GetLogicalDevicesPaged", func(m *modelConn) { m.ldErr = true }, func(e *Explorer) error {
			_, err := e.GetLogicalDevicesPaged(PageOptions{Limit: 1})
			return err
		}},
		{"ListLogicalNodes", func(m *modelConn) { m.lnErr = true }, func(e *Explorer) error { _, err := e.ListLogicalNodes("LD0"); return err }},
		{"GetLogicalNodes", func(m *modelConn) { m.lnErr = true }, func(e *Explorer) error { _, err := e.GetLogicalNodes("LD0"); return err }},
		{"ListDataObjects", func(m *modelConn) { m.doErr = true }, func(e *Explorer) error { _, err := e.ListDataObjects("LD0", "LN"); return err }},
		{"GetDataObjects", func(m *modelConn) { m.doErr = true }, func(e *Explorer) error { _, err := e.GetDataObjects("LD0", "LN"); return err }},
		{"GetAllReportsPaged", func(m *modelConn) { m.ldErr = true }, func(e *Explorer) error {
			_, err := NewReportService(e.conn).GetAllReportsPaged(PageOptions{})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newTreeModel()
			tt.mut(m)
			if err := tt.call(NewExplorer(m)); err == nil {
				t.Fatal("expected error")
			}
		})
	}

	nilEx := NewExplorer(nil)
	if _, err := nilEx.ListLogicalDevices(); err == nil {
		t.Fatal("nil conn ListLogicalDevices")
	}
	if _, err := nilEx.GetLogicalDevices(); err == nil {
		t.Fatal("nil conn GetLogicalDevices")
	}
	if _, err := nilEx.ListLogicalNodes("x"); err == nil {
		t.Fatal("nil conn ListLogicalNodes")
	}
}

func TestExplorer_PartialChildErrorsTolerated(t *testing.T) {
	t.Parallel()
	m := newTreeModel()
	m.doErr = true
	lns, err := NewExplorer(m).GetLogicalNodes("LD0")
	if err != nil || len(lns) != 1 || lns[0].DOCount != 0 {
		t.Fatalf("lns=%+v err=%v", lns, err)
	}
	m2 := newTreeModel()
	m2.lnErr = true
	m2.listDataSetsErr = errors.New("x")
	m2.listReportsErr = errors.New("y")
	lds, err := NewExplorer(m2).GetLogicalDevices()
	if err != nil || len(lds) != 1 || lds[0].LNCount != 0 {
		t.Fatalf("lds=%+v err=%v", lds, err)
	}
	reps, err := NewReportService(&mockConnection{listReportsErr: errors.New("z")}).GetAllReports()
	if err != nil || len(reps) != 0 {
		t.Fatalf("reps=%+v err=%v", reps, err)
	}
	reps, err = NewReportService(&mockConnection{reports: []string{"bad", "LLN0$RP$u1"}}).GetAllReports()
	if err != nil || len(reps) != 2 { // one per default LD (LD0, LD1)
		t.Fatalf("reps=%+v err=%v", reps, err)
	}
}

func TestPaginateBounds(t *testing.T) {
	t.Parallel()
	all := []int{1, 2, 3}
	tests := []struct {
		opts    PageOptions
		want    int
		hasMore bool
	}{
		{PageOptions{Offset: -5}, 3, false},
		{PageOptions{Offset: 10}, 0, false},
		{PageOptions{Offset: 1, Limit: 1}, 1, true},
		{PageOptions{Limit: 5}, 3, false},
	}
	for _, tt := range tests {
		got := paginate(all, tt.opts)
		if len(got.Items) != tt.want || got.HasMore != tt.hasMore || got.Total != 3 {
			t.Fatalf("paginate(%+v) = %+v", tt.opts, got)
		}
	}
}

func TestValueToInterface_Table(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0).UTC()
	tests := []struct {
		name    string
		v       *mms.Value
		typ     mms.ValueType
		want    any
		wantErr bool
	}{
		{"bool", mms.NewBoolean(true), mms.ValueTypeBoolean, true, false},
		{"int32", mms.NewInteger(5), mms.ValueTypeInteger, int32(5), false},
		{"int64", mms.NewInteger(1 << 40), mms.ValueTypeInteger, int64(1 << 40), false},
		{"uint32", mms.NewUnsigned(5), mms.ValueTypeUnsigned, uint32(5), false},
		{"uint64", mms.NewUnsigned(1 << 40), mms.ValueTypeUnsigned, uint64(1 << 40), false},
		{"float", mms.NewFloat(1.5), mms.ValueTypeFloat, 1.5, false},
		{"vstring", mms.NewVisibleString("a"), mms.ValueTypeVisibleString, "a", false},
		{"mstring", mms.NewMmsString("b"), mms.ValueTypeMmsString, "b", false},
		{"utc", mms.NewUTCTime(now), mms.ValueTypeUTCTime, now, false},
		{"utc bad", mms.NewBoolean(true), mms.ValueTypeUTCTime, nil, true},
		{"binary time", mms.NewBinaryTime(42), mms.ValueTypeBinaryTime, int64(42), false},
		{"binary time bad", mms.NewBoolean(true), mms.ValueTypeBinaryTime, nil, true},
		{"default", mms.NewBCD(12), mms.ValueTypeBCD, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := valueToInterface(iec61850.NewValue(tt.v), tt.typ)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.want != nil {
				if tm, ok := tt.want.(time.Time); ok {
					if !got.(time.Time).Equal(tm) {
						t.Fatalf("got %v", got)
					}
					return
				}
				if got != tt.want {
					t.Fatalf("got %#v, want %#v", got, tt.want)
				}
			}
		})
	}
	if got, err := valueToInterface(nil, mms.ValueTypeBoolean); got != nil || err != nil {
		t.Fatalf("nil: %v %v", got, err)
	}
}

func TestValueToModel_Types(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 5_000_000).UTC()
	tests := []struct {
		name    string
		v       any
		typ     mms.ValueType
		wantNil bool
		wantErr bool
	}{
		{"nil", nil, mms.ValueTypeBoolean, true, false},
		{"bool wrong", "x", mms.ValueTypeBoolean, true, true},
		{"int", 3, mms.ValueTypeInteger, false, false},
		{"int64", int64(3), mms.ValueTypeInteger, false, false},
		{"int other", "3", mms.ValueTypeInteger, false, false},
		{"uint16", uint16(3), mms.ValueTypeUnsigned, false, false},
		{"uint8", uint8(3), mms.ValueTypeUnsigned, false, false},
		{"uint other", uint64(3), mms.ValueTypeUnsigned, false, false},
		{"float32", float32(1), mms.ValueTypeFloat, false, false},
		{"float wrong", "x", mms.ValueTypeReal, true, true},
		{"string wrong", 1, mms.ValueTypeVisibleString, true, true},
		{"bits", []byte{0x80}, mms.ValueTypeBitString, false, false},
		{"bits u16", uint16(1), mms.ValueTypeBitString, false, false},
		{"bits i32", int32(1), mms.ValueTypeBitString, false, false},
		{"bits u32", uint32(1), mms.ValueTypeBitString, false, false},
		{"bits other", "x", mms.ValueTypeBitString, false, false},
		{"octets", []byte{1}, mms.ValueTypeOctetString, false, false},
		{"utc", now, mms.ValueTypeUTCTime, false, false},
		{"utc ptr", &now, mms.ValueTypeUTCTime, false, false},
		{"utc wrong", "x", mms.ValueTypeUTCTime, true, true},
		{"btime int", 5, mms.ValueTypeBinaryTime, false, false},
		{"btime u64", uint64(5), mms.ValueTypeBinaryTime, false, false},
		{"btime other", "x", mms.ValueTypeBinaryTime, false, false},
		{"default", "x", mms.ValueTypeBCD, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, errMsg := ValueToModel(tt.v, tt.typ)
			if (got == nil) != tt.wantNil || (errMsg != "") != tt.wantErr {
				t.Fatalf("got=%+v err=%q", got, errMsg)
			}
		})
	}
}

func TestScalarMatchesAndProjection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want domain.ScalarValue
		got  *mms.Value
		ok   bool
	}{
		{"nil", domain.ScalarValue{Kind: domain.ScalarBool}, nil, false},
		{"uint as int", domain.ScalarValue{Kind: domain.ScalarUint, Uint: 4}, mms.NewInteger(4), true},
		{"uint neg int", domain.ScalarValue{Kind: domain.ScalarUint, Uint: 4}, mms.NewInteger(-4), false},
		{"float wrong type", domain.ScalarValue{Kind: domain.ScalarFloat, Float: 1}, mms.NewBoolean(true), false},
		{"float neg diff", domain.ScalarValue{Kind: domain.ScalarFloat, Float: 2}, mms.NewFloat(1), false},
		{"mms string", domain.ScalarValue{Kind: domain.ScalarString, String: "a"}, mms.NewMmsString("a"), true},
		{"string wrong type", domain.ScalarValue{Kind: domain.ScalarString, String: "a"}, mms.NewBoolean(true), false},
		{"unknown kind", domain.ScalarValue{Kind: "x"}, mms.NewBoolean(true), false},
	}
	for _, tt := range tests {
		if got := ScalarMatchesMMS(tt.want, tt.got); got != tt.ok {
			t.Errorf("%s: ScalarMatchesMMS = %v, want %v", tt.name, got, tt.ok)
		}
	}

	for _, v := range []*mms.Value{mms.NewUnsigned(1), mms.NewMmsString("s"), mms.NewFloat(1.5)} {
		if ScalarFromIECValue(iec61850.NewValue(v)) == nil {
			t.Errorf("ScalarFromIECValue(%v) = nil", v)
		}
	}
	if ScalarFromIECValue(nil) != nil || ScalarFromIECValue(iec61850.NewValue(mms.NewOctetString([]byte{1}))) != nil {
		t.Error("expected nil projection")
	}
	if ScalarRequestedAny(domain.ScalarValue{Kind: "x"}) != nil {
		t.Error("unknown kind projection")
	}
	if _, err := ScalarToMMS(domain.ScalarValue{Kind: "x"}); err == nil {
		t.Error("ScalarToMMS unknown kind")
	}
	if _, err := ParseScalarValue("1", "x"); err == nil {
		t.Error("ParseScalarValue unknown kind")
	}
	for _, v := range []domain.ScalarValue{
		{Kind: domain.ScalarEnum, Int: 1 << 40},
		{Kind: domain.ScalarUint, Uint: 1 << 40},
		{Kind: "x"},
	} {
		if _, err := ScalarToCtlVal(v); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("ScalarToCtlVal(%+v) err = %v", v, err)
		}
	}
}

// baselineConn returns dataset values with an error and a nil entry.
type baselineConn struct {
	mockConnection
	calls []string
}

func (m *baselineConn) ReadDataSet(_ context.Context, _, ds string) ([]iec61850.DataSetValue, error) {
	m.calls = append(m.calls, ds)
	if strings.Contains(ds, "$") {
		return nil, errors.New("not by LN-qualified name")
	}
	return []iec61850.DataSetValue{
		{Value: iec61850.NewValue(mms.NewFloat(1.5))},
		{Err: errors.New("denied")},
		{},
	}, nil
}

func TestReporter_RunBaselineSync_TextAndJSONL(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"text", "jsonl"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			conn := &baselineConn{}
			var out, diag bytes.Buffer
			r := NewReporter(conn).WithConfig(ReporterConfig{
				DatasetRef: "LD0/LLN0$ds1", ShowValues: true,
				Format: format, Writer: &out, ErrWriter: &diag,
			})
			r.runBaselineSync(context.Background(), &out)
			if len(conn.calls) != 2 || conn.calls[0] != "LLN0$ds1" || conn.calls[1] != "ds1" {
				t.Fatalf("ReadDataSet calls = %v (want LN-qualified then bare)", conn.calls)
			}
			s := out.String()
			if format == "text" {
				for _, want := range []string{"Baseline snapshot: 3 values", "[1] 1.5", "[2] (error: denied)", "[3] (nil)"} {
					if !strings.Contains(s, want) {
						t.Errorf("missing %q in %q", want, s)
					}
				}
				return
			}
			if !strings.Contains(s, `"event":"baseline"`) || !strings.Contains(s, `"values":[1.5,null,null]`) {
				t.Fatalf("jsonl baseline = %q", s)
			}
		})
	}
	conn := &mockConnection{readDataSetErr: errors.New("boom")}
	var out bytes.Buffer
	r := NewReporter(conn).WithConfig(ReporterConfig{DatasetRef: "LD0/LLN0$ds1", Writer: &out})
	r.runBaselineSync(context.Background(), &out)
	if !strings.Contains(out.String(), "baseline sync failed (boom)") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestReasonTokenUnknownAndCustom(t *testing.T) {
	t.Parallel()
	if reasonToken(domain.ReasonUnknown) != "" || reasonToken("") != "" {
		t.Fatal("unknown reasons must map to empty")
	}
	if reasonToken("custom") != "custom" {
		t.Fatal("custom reason passthrough")
	}
	if baselineValuesJSON(nil, false) != nil {
		t.Fatal("hidden values must be nil")
	}
}

func TestScalarToCtlValFloat32Range(t *testing.T) {
	t.Parallel()
	if _, err := ScalarToCtlVal(domain.ScalarValue{Kind: domain.ScalarFloat, Float: 1e39}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("1e39: err = %v", err)
	}
	if _, err := ScalarToCtlVal(domain.ScalarValue{Kind: domain.ScalarFloat, Float: 3.4e38}); err != nil {
		t.Fatalf("3.4e38: %v", err)
	}
}
