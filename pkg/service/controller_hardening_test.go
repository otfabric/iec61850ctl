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

// Phase 6: connection fault, concurrency, short endurance.

func TestController_ConnectionFaultAfterSelect(t *testing.T) {
	m := &controlMock{
		model:      iec61850.CtlModelSBONormal,
		selectRet:  "ok",
		operateErr: errors.New("connection reset by peer"),
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
		t.Fatalf("cancelCalls=%d want 1 (best-effort cleanup)", m.cancelCalls)
	}
}

func TestController_ConcurrentDirectOperate(t *testing.T) {
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := &controlMock{model: iec61850.CtlModelDirectNormal}
			c := NewController(m)
			res, err := c.Operate(context.Background(), domain.ControlRequest{
				Object: "InteropLD/GGIO1.SPCSO1",
				Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: true},
				Mode:   domain.ControlModeAuto,
			})
			if err != nil {
				errs <- err
				return
			}
			if res.Status != domain.ControlStatusOperated {
				errs <- errors.New("status not operated: " + string(res.Status))
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestController_EnduranceDirectOperate(t *testing.T) {
	m := &controlMock{model: iec61850.CtlModelDirectNormal}
	c := NewController(m)
	ctx := context.Background()
	deadline := time.Now().Add(200 * time.Millisecond)
	var ops int
	for time.Now().Before(deadline) {
		res, err := c.Operate(ctx, domain.ControlRequest{
			Object: "InteropLD/GGIO1.SPCSO1",
			Value:  domain.ScalarValue{Kind: domain.ScalarBool, Bool: ops%2 == 0},
			Mode:   domain.ControlModeAuto,
		})
		if err != nil {
			t.Fatalf("op %d: %v", ops, err)
		}
		if res.Status != domain.ControlStatusOperated {
			t.Fatalf("op %d status=%s", ops, res.Status)
		}
		ops++
	}
	if ops < 5 {
		t.Fatalf("endurance ops=%d want >=5", ops)
	}
}
