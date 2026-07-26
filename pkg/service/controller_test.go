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

type controlMock struct {
	model          iec61850.CtlModel
	selectRet      string
	selectErr      error
	selectWVErr    error
	operateErr     error
	cancelErr      error
	lastAppl       *iec61850.LastApplError
	lastApplErr    error
	readVal        *iec61850.Value
	readErr        error
	selectCalls    int
	selectWVCalls  int
	operateCalls   int
	cancelCalls    int
	lastOperateNum uint8
	lastSelectNum  uint8
}

func (m *controlMock) ReadCtlModel(_ context.Context, _ iec61850.Ref) (iec61850.CtlModel, error) {
	return m.model, nil
}
func (m *controlMock) Select(_ context.Context, _ iec61850.Ref) (string, error) {
	m.selectCalls++
	return m.selectRet, m.selectErr
}
func (m *controlMock) SelectWithValue(_ context.Context, _ iec61850.Ref, p iec61850.OperateParams) error {
	m.selectWVCalls++
	m.lastSelectNum = p.CtlNum
	return m.selectWVErr
}
func (m *controlMock) Operate(_ context.Context, _ iec61850.Ref, p iec61850.OperateParams) error {
	m.operateCalls++
	m.lastOperateNum = p.CtlNum
	return m.operateErr
}
func (m *controlMock) Cancel(_ context.Context, _ iec61850.Ref, _ iec61850.CancelParams) error {
	m.cancelCalls++
	return m.cancelErr
}
func (m *controlMock) ReadLastApplError(_ context.Context, _ iec61850.Ref) (*iec61850.LastApplError, error) {
	return m.lastAppl, m.lastApplErr
}
func (m *controlMock) Read(_ context.Context, _ iec61850.Ref) (*iec61850.Value, error) {
	return m.readVal, m.readErr
}

func TestController_AutoDirect(t *testing.T) {
	m := &controlMock{
		model:   iec61850.CtlModelDirectNormal,
		readVal: iec61850.NewValue(mms.NewBoolean(true)),
	}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object:     "InteropLD/GGIO1.SPCSO1",
		Mode:       domain.ControlModeAuto,
		Value:      domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
		ConfirmRef: "InteropLD/GGIO1.SPCSO1.stVal[ST]",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusConfirmed {
		t.Fatalf("status=%s", res.Status)
	}
	if m.operateCalls != 1 || m.selectCalls != 0 {
		t.Fatalf("calls operate=%d select=%d", m.operateCalls, m.selectCalls)
	}
}

func TestController_AutoSBO(t *testing.T) {
	m := &controlMock{
		model:     iec61850.CtlModelSBONormal,
		selectRet: "InteropLD/GGIO1.SPCSO2",
		readVal:   iec61850.NewValue(mms.NewBoolean(true)),
	}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object:     "InteropLD/GGIO1.SPCSO2",
		Mode:       domain.ControlModeAuto,
		Value:      domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
		ConfirmRef: "InteropLD/GGIO1.SPCSO2.stVal[ST]",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusConfirmed {
		t.Fatalf("status=%s", res.Status)
	}
	if m.selectCalls != 1 || m.operateCalls != 1 {
		t.Fatalf("select=%d operate=%d", m.selectCalls, m.operateCalls)
	}
}

func TestController_SBOwSameCtlNum(t *testing.T) {
	m := &controlMock{model: iec61850.CtlModelSBOEnhanced}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO3",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
		CtlNum: 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusOperated {
		t.Fatalf("status=%s", res.Status)
	}
	if m.lastSelectNum != 42 || m.lastOperateNum != 42 {
		t.Fatalf("ctlNum select=%d operate=%d", m.lastSelectNum, m.lastOperateNum)
	}
}

func TestController_DryRun(t *testing.T) {
	m := &controlMock{model: iec61850.CtlModelSBONormal}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO2",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusPlanned {
		t.Fatalf("status=%s", res.Status)
	}
	if m.selectCalls != 0 || m.operateCalls != 0 {
		t.Fatal("dry-run must not write")
	}
}

func TestController_OperateFailureCancels(t *testing.T) {
	m := &controlMock{
		model:      iec61850.CtlModelSBONormal,
		selectRet:  "ok",
		operateErr: errors.New("access denied"),
	}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO2",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusFailed {
		t.Fatalf("status=%s", res.Status)
	}
	if m.cancelCalls != 1 {
		t.Fatalf("cancelCalls=%d", m.cancelCalls)
	}
	if res.Cleanup == nil || !res.Cleanup.Attempted || !res.Cleanup.OK {
		t.Fatalf("cleanup=%+v", res.Cleanup)
	}
}

func TestController_ConfirmationMismatch(t *testing.T) {
	m := &controlMock{
		model:   iec61850.CtlModelDirectNormal,
		readVal: iec61850.NewValue(mms.NewBoolean(false)),
	}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object:     "InteropLD/GGIO1.SPCSO1",
		Mode:       domain.ControlModeAuto,
		Value:      domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
		ConfirmRef: "InteropLD/GGIO1.SPCSO1.stVal[ST]",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusConfirmationMismatch {
		t.Fatalf("status=%s", res.Status)
	}
	if !res.Status.ExitNonZero() {
		t.Fatal("expected non-zero exit status")
	}
}

func TestController_ModeMismatch(t *testing.T) {
	m := &controlMock{model: iec61850.CtlModelDirectNormal}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO1",
		Mode:   domain.ControlModeSBO,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusFailed {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestController_StatusOnly(t *testing.T) {
	m := &controlMock{model: iec61850.CtlModelStatusOnly}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/LLN0.Beh",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusFailed {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestController_Inspect(t *testing.T) {
	m := &controlMock{model: iec61850.CtlModelSBONormal}
	c := NewController(m)
	res, err := c.Inspect(context.Background(), "InteropLD/GGIO1.SPCSO2")
	if err != nil {
		t.Fatal(err)
	}
	if !res.RequiresSelect || res.ControlModel.Code != 2 {
		t.Fatalf("%+v", res)
	}
}

func TestController_OperatedUnconfirmed(t *testing.T) {
	m := &controlMock{
		model:   iec61850.CtlModelDirectNormal,
		readErr: errors.New("timeout"),
	}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object:     "InteropLD/GGIO1.SPCSO1",
		Mode:       domain.ControlModeAuto,
		Value:      domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
		ConfirmRef: "InteropLD/GGIO1.SPCSO1.stVal[ST]",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusOperatedUnconfirmed {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestController_SelectFailureNoOperate(t *testing.T) {
	m := &controlMock{
		model:     iec61850.CtlModelSBONormal,
		selectErr: errors.New("select denied"),
	}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO2",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusFailed || m.operateCalls != 0 {
		t.Fatalf("status=%s operate=%d", res.Status, m.operateCalls)
	}
}

func TestController_LastApplErrorMatched(t *testing.T) {
	m := &controlMock{
		model:      iec61850.CtlModelDirectNormal,
		operateErr: errors.New("failed"),
		lastAppl: &iec61850.LastApplError{
			CntrlObj: "InteropLD/GGIO1.SPCSO1",
			Error:    1,
		},
	}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO1",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.LastError == nil || !res.LastError.Matched {
		t.Fatalf("last error=%+v", res.LastError)
	}
}

func TestController_CancelCleanupError(t *testing.T) {
	m := &controlMock{
		model:      iec61850.CtlModelSBONormal,
		selectRet:  "ok",
		operateErr: errors.New("operate failed"),
		cancelErr:  errors.New("cancel failed"),
	}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO2",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Cleanup == nil || res.Cleanup.OK || res.Cleanup.Error == "" {
		t.Fatalf("cleanup=%+v", res.Cleanup)
	}
}

func TestController_DirectEnhancedAndOriginCheck(t *testing.T) {
	m := &controlMock{model: iec61850.CtlModelDirectEnhanced}
	c := NewController(m)
	res, err := c.Operate(context.Background(), domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO1",
		Mode:   domain.ControlModeDirect,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
		Origin: domain.ControlOrigin{Category: domain.OrCatBayControl, Ident: "tester"},
		Check:  domain.CheckConditions{Synchro: true, Interlock: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ControlStatusOperated {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestParseControlObjectRefAndValidate(t *testing.T) {
	if _, err := parseControlObjectRef("InteropLD/GGIO1.SPCSO1[ST]"); err == nil {
		t.Fatal("FC not allowed")
	}
	if _, err := parseControlObjectRef("bad"); err == nil {
		t.Fatal("bad ref")
	}
	if err := validateControlRequest(domain.ControlRequest{}); err == nil {
		t.Fatal("object required")
	}
	if err := validateControlRequest(domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO1",
		Value:  domain.ScalarValue{Kind: domain.ScalarBool},
		Origin: domain.ControlOrigin{Ident: string(make([]byte, domain.MaxOrIdentBytes+1))},
	}); err == nil {
		t.Fatal("or-ident too long")
	}
}

func TestResolveControlModeAndPlanned(t *testing.T) {
	m, err := resolveControlMode(domain.ControlModeAuto, iec61850.CtlModelDirectEnhanced)
	if err != nil || m != domain.ControlModeDirect {
		t.Fatalf("auto direct-enhanced: %v %v", m, err)
	}
	if _, err := resolveControlMode(domain.ControlModeSBOw, iec61850.CtlModelSBONormal); err == nil {
		t.Fatal("expected sbow mismatch")
	}
	if ops := plannedOperations(domain.ControlModeSBOw); len(ops) != 2 {
		t.Fatalf("planned=%v", ops)
	}
}

func TestLastApplErrorMatches(t *testing.T) {
	if !lastApplErrorMatches("InteropLD/GGIO1.SPCSO1", "InteropLD/GGIO1.SPCSO1") {
		t.Fatal("exact")
	}
	if !lastApplErrorMatches("InteropLD/GGIO1.SPCSO1.Oper", "InteropLD/GGIO1.SPCSO1") {
		t.Fatal("prefix")
	}
	if lastApplErrorMatches("", "x") || lastApplErrorMatches("a", "b") {
		t.Fatal("negative")
	}
}
