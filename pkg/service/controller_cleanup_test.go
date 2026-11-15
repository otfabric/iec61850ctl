// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// endingControl is a device whose Select or Operate does not answer before
// the context ends. Cancel and ReadLastApplError record the state of their
// context; with blockCleanup they too wait for it to end.
type endingControl struct {
	controlMock
	blockSelect  bool
	blockCleanup bool

	mu        sync.Mutex
	cancelCtx []error
	laeCtx    []error
}

func (m *endingControl) Select(ctx context.Context, ref iec61850.Ref) (string, error) {
	if m.blockSelect {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return m.controlMock.Select(ctx, ref)
}

func (m *endingControl) SelectWithValue(ctx context.Context, ref iec61850.Ref, p iec61850.OperateParams) error {
	if m.blockSelect {
		<-ctx.Done()
		return ctx.Err()
	}
	return m.controlMock.SelectWithValue(ctx, ref, p)
}

func (m *endingControl) Operate(ctx context.Context, _ iec61850.Ref, _ iec61850.OperateParams) error {
	<-ctx.Done()
	return ctx.Err()
}

func (m *endingControl) Cancel(ctx context.Context, _ iec61850.Ref, _ iec61850.CancelParams) error {
	if m.blockCleanup {
		<-ctx.Done()
	}
	m.mu.Lock()
	m.cancelCtx = append(m.cancelCtx, ctx.Err())
	m.mu.Unlock()
	return ctx.Err()
}

func (m *endingControl) ReadLastApplError(ctx context.Context, _ iec61850.Ref) (*iec61850.LastApplError, error) {
	if m.blockCleanup {
		<-ctx.Done()
	}
	m.mu.Lock()
	m.laeCtx = append(m.laeCtx, ctx.Err())
	m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &iec61850.LastApplError{CntrlObj: "InteropLD/GGIO1.SPCSO2", Error: 1}, nil
}

// TestControllerCleanupAfterContextEnd: when select or operate fails
// because the journey's context ended (its deadline or Ctrl-C), the Cancel
// of a selected control and the read of LastApplError still run, each on a
// live context of its own bounded by cleanupTimeout.
func TestControllerCleanupAfterContextEnd(t *testing.T) {
	prev := cleanupTimeout
	cleanupTimeout = 200 * time.Millisecond
	t.Cleanup(func() { cleanupTimeout = prev })

	for _, c := range []struct {
		name        string
		model       iec61850.CtlModel
		blockSelect bool
		cancel      bool // the journey ends by cancel (Ctrl-C) rather than its deadline
		wantCancel  bool // a Cancel is due (select succeeded, operate failed)
	}{
		{"direct, deadline", iec61850.CtlModelDirectNormal, false, false, false},
		{"SBO operate, deadline", iec61850.CtlModelSBONormal, false, false, true},
		{"SBO operate, cancel", iec61850.CtlModelSBONormal, false, true, true},
		{"SBOw operate, deadline", iec61850.CtlModelSBOEnhanced, false, false, true},
		{"SBOw operate, cancel", iec61850.CtlModelSBOEnhanced, false, true, true},
		{"SBO select, deadline", iec61850.CtlModelSBONormal, true, false, false},
		{"SBOw select, cancel", iec61850.CtlModelSBOEnhanced, true, true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := &endingControl{controlMock: controlMock{model: c.model, selectRet: "InteropLD/GGIO1.SPCSO2"}, blockSelect: c.blockSelect}
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			if c.cancel {
				time.AfterFunc(20*time.Millisecond, cancel)
			}
			res, err := NewController(m).Operate(ctx, domain.ControlRequest{
				Object: "InteropLD/GGIO1.SPCSO2",
				Mode:   domain.ControlModeAuto,
				Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
			})
			if err != nil {
				t.Fatal(err)
			}
			if res.Status != domain.ControlStatusFailed || res.Err == nil {
				t.Fatalf("status %s, err %v; want failed", res.Status, res.Err)
			}
			if c.wantCancel {
				if len(m.cancelCtx) != 1 || m.cancelCtx[0] != nil {
					t.Fatalf("Cancel saw context errors %v, want one live context", m.cancelCtx)
				}
				if res.Cleanup == nil || !res.Cleanup.Attempted || !res.Cleanup.OK {
					t.Fatalf("cleanup %+v, want attempted and ok", res.Cleanup)
				}
			} else if len(m.cancelCtx) != 0 || res.Cleanup != nil {
				t.Fatalf("Cancel sent (%v, cleanup %+v) without a selected control", m.cancelCtx, res.Cleanup)
			}
			if len(m.laeCtx) != 1 || m.laeCtx[0] != nil {
				t.Fatalf("ReadLastApplError saw context errors %v, want one live context", m.laeCtx)
			}
			if res.LastError == nil || !res.LastError.Matched {
				t.Fatalf("LastError %+v, want the device's LastApplError", res.LastError)
			}
		})
	}
}

// TestControllerCleanupBounded: a device that does not answer Cancel or the
// LastApplError read holds each of them for cleanupTimeout at the most.
func TestControllerCleanupBounded(t *testing.T) {
	prev := cleanupTimeout
	cleanupTimeout = 100 * time.Millisecond
	t.Cleanup(func() { cleanupTimeout = prev })

	m := &endingControl{controlMock: controlMock{model: iec61850.CtlModelSBONormal, selectRet: "InteropLD/GGIO1.SPCSO2"}, blockCleanup: true}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	res, err := NewController(m).Operate(ctx, domain.ControlRequest{
		Object: "InteropLD/GGIO1.SPCSO2",
		Mode:   domain.ControlModeAuto,
		Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	// 20 ms of operate and two steps of 100 ms; the rest is slack for a
	// loaded machine.
	if elapsed > 2*time.Second {
		t.Fatalf("the failed journey took %v", elapsed)
	}
	if res.Cleanup == nil || res.Cleanup.OK || len(m.cancelCtx) != 1 || !errors.Is(m.cancelCtx[0], context.DeadlineExceeded) {
		t.Fatalf("cleanup %+v, Cancel context %v; want a Cancel stopped by its own deadline", res.Cleanup, m.cancelCtx)
	}
	if res.LastError != nil || len(m.laeCtx) != 1 || !errors.Is(m.laeCtx[0], context.DeadlineExceeded) {
		t.Fatalf("LastError %+v, read context %v; want a read stopped by its own deadline", res.LastError, m.laeCtx)
	}
}
