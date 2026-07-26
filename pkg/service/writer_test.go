// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

type writeMock struct {
	writeErr error
	readVal  *iec61850.Value
	readErr  error
	wrote    *mms.Value
	writeRef iec61850.Ref
}

func (m *writeMock) Write(_ context.Context, ref iec61850.Ref, v *mms.Value) error {
	m.writeRef = ref
	m.wrote = v
	return m.writeErr
}
func (m *writeMock) Read(_ context.Context, _ iec61850.Ref) (*iec61850.Value, error) {
	return m.readVal, m.readErr
}

func TestWriter_SetObject(t *testing.T) {
	m := &writeMock{readVal: iec61850.NewValue(mms.NewInteger(5))}
	w := NewWriter(m)
	res, err := w.SetObject(context.Background(), domain.WriteRequest{
		Object: "InteropLD/GGIO1.SetInt1.setVal",
		FC:     domain.FC_SP,
		Value:  domain.ScalarValue{Kind: domain.ScalarInt, Int: 5},
		Verify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.WriteOK || res.Verification == nil || !res.Verification.Matched {
		t.Fatalf("%+v", res)
	}
	if string(m.writeRef.FC) != "SP" {
		t.Fatalf("fc=%s", m.writeRef.FC)
	}
}

func TestWriter_RejectCO(t *testing.T) {
	w := NewWriter(&writeMock{})
	_, err := w.SetObject(context.Background(), domain.WriteRequest{
		Object: "InteropLD/GGIO1.SPCSO1.Oper",
		FC:     domain.FC_CO,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	if err == nil {
		t.Fatal("expected CO reject")
	}
}

func TestWriter_WriteFailure(t *testing.T) {
	m := &writeMock{writeErr: errors.New("object-access-denied")}
	w := NewWriter(m)
	res, err := w.SetObject(context.Background(), domain.WriteRequest{
		Object: "InteropLD/GGIO1.SetInt1.setVal",
		FC:     domain.FC_SP,
		Value:  domain.ScalarValue{Kind: domain.ScalarInt, Int: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.WriteOK {
		t.Fatal("expected write_ok=false")
	}
}

func TestWriter_VerifyMismatchAndReadError(t *testing.T) {
	m := &writeMock{readVal: iec61850.NewValue(mms.NewInteger(9))}
	w := NewWriter(m)
	res, err := w.SetObject(context.Background(), domain.WriteRequest{
		Object: "InteropLD/GGIO1.SetInt1.setVal",
		FC:     domain.FC_SP,
		Value:  domain.ScalarValue{Kind: domain.ScalarInt, Int: 5},
		Verify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verification == nil || res.Verification.Matched {
		t.Fatalf("expected mismatch: %+v", res.Verification)
	}

	m2 := &writeMock{readErr: errors.New("gone")}
	res2, err := NewWriter(m2).SetObject(context.Background(), domain.WriteRequest{
		Object: "InteropLD/GGIO1.SetInt1.setVal",
		FC:     domain.FC_SP,
		Value:  domain.ScalarValue{Kind: domain.ScalarInt, Int: 5},
		Verify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Verification == nil || res2.Verification.Error == "" {
		t.Fatalf("expected verify read error: %+v", res2.Verification)
	}
}

func TestWriter_Validation(t *testing.T) {
	w := NewWriter(&writeMock{})
	if _, err := w.SetObject(context.Background(), domain.WriteRequest{}); err == nil {
		t.Fatal("object required")
	}
	if _, err := w.SetObject(context.Background(), domain.WriteRequest{
		Object: "x", FC: domain.FunctionalConstraint("ZZ"),
		Value: domain.ScalarValue{Kind: domain.ScalarInt, Int: 1},
	}); err == nil {
		t.Fatal("invalid FC")
	}
	if _, err := w.SetObject(context.Background(), domain.WriteRequest{
		Object: "InteropLD/GGIO1.SetInt1.setVal[ST]",
		FC:     domain.FC_SP,
		Value:  domain.ScalarValue{Kind: domain.ScalarInt, Int: 1},
	}); err == nil {
		t.Fatal("FC conflict")
	}
}
