// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestApp_ControlAndWrite(t *testing.T) {
	m := &mockConn{
		ctlModelSet: true,
		ctlModel:    iec61850.CtlModelDirectNormal,
		allowWrite:  true,
		readVal:     iec61850.NewValue(mms.NewInteger(5)),
	}
	a := New(m)
	if a.Controller() == nil || a.Writer() == nil {
		t.Fatal("nil accessors")
	}

	insp, err := a.ControlInspect(context.Background(), "InteropLD/GGIO1.SPCSO1")
	if err != nil || !insp.Controllable {
		t.Fatalf("inspect: %+v err=%v", insp, err)
	}

	op, err := a.ControlOperate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO1",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	if err != nil || op.Status != domain.ControlStatusOperated {
		t.Fatalf("operate: %+v err=%v", op, err)
	}

	wr, err := a.SetObject(context.Background(), domain.WriteRequest{
		Object: "InteropLD/GGIO1.SetInt1.setVal",
		FC:     domain.FC_SP,
		Value:  domain.ScalarValue{Kind: domain.ScalarInt, Int: 5},
		Verify: true,
	})
	if err != nil || !wr.WriteOK || wr.Verification == nil || !wr.Verification.Matched {
		t.Fatalf("set: %+v err=%v", wr, err)
	}
}
