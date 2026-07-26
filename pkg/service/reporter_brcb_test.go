// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"io"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
)

type brcbSetMock struct {
	mockConnection
	update iec61850.RCBUpdate
	err    error
}

func (m *brcbSetMock) SetReportControlBlock(_ context.Context, _, _ string, u iec61850.RCBUpdate) error {
	m.update = u
	return m.err
}

func TestReporter_ApplyBRCBPreEnable(t *testing.T) {
	m := &brcbSetMock{}
	resv := int32(30)
	r := NewReporter(m).WithConfig(ReporterConfig{
		PurgeBuf: true,
		EntryID:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
		ResvTms:  &resv,
		Writer:   io.Discard,
	})
	if err := r.applyBRCBPreEnable(context.Background(), "InteropLD", "LLN0$BR$brcb01"); err != nil {
		t.Fatal(err)
	}
	if m.update.Fields&(iec61850.RCBFieldPurgeBuf|iec61850.RCBFieldEntryID|iec61850.RCBFieldResvTms) == 0 {
		t.Fatalf("fields=%v", m.update.Fields)
	}
	if !m.update.PurgeBuf || m.update.ResvTms != 30 || len(m.update.EntryID) != 8 {
		t.Fatalf("update=%+v", m.update)
	}

	r2 := NewReporter(m).WithConfig(ReporterConfig{Writer: io.Discard})
	if err := r2.applyBRCBPreEnable(context.Background(), "InteropLD", "LLN0$BR$brcb01"); err != nil {
		t.Fatal(err)
	}

	m.err = errors.New("denied")
	r3 := NewReporter(m).WithConfig(ReporterConfig{PurgeBuf: true, Writer: io.Discard})
	if err := r3.applyBRCBPreEnable(context.Background(), "InteropLD", "LLN0$BR$brcb01"); err == nil {
		t.Fatal("expected configure error")
	}
}
