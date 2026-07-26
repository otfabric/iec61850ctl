// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"

	iec61850 "github.com/otfabric/go-iec61850"
	mms "github.com/otfabric/go-mms"
)

// registerInteropControls registers the fixture controllable objects used by
// reverse e2e / mms-interop controller adapters:
//
//	GGIO1.SPCSO1 — direct-with-normal-security
//	GGIO1.SPCSO2 — sbo-with-normal-security
//	GGIO1.SPCSO3 — sbo-with-enhanced-security
//
// OnOperate updates the corresponding stVal in the value store.
func registerInteropControls(srv *iec61850.Server) error {
	vs := srv.ValueStore()
	makeOperateHandler := func(stValKey string) iec61850.ControlHandler {
		return iec61850.ControlHandler{
			OnOperate: func(_ context.Context, req iec61850.ControlRequest) error {
				if req.CtlVal == nil {
					return fmt.Errorf("nil CtlVal in operate request")
				}
				boolVal, ok := req.CtlVal.Bool()
				if !ok {
					return fmt.Errorf("ctlVal is not a boolean")
				}
				vs.Set(stValKey, mms.NewBoolean(boolVal))
				return nil
			},
		}
	}

	type ctrl struct {
		doRef    string
		ctlModel iec61850.CtlModel
		stValKey string
	}
	controls := []ctrl{
		{"GGIO1.SPCSO1", iec61850.CtlModelDirectNormal, "InteropLD/GGIO1$ST$SPCSO1$stVal"},
		{"GGIO1.SPCSO2", iec61850.CtlModelSBONormal, "InteropLD/GGIO1$ST$SPCSO2$stVal"},
		{"GGIO1.SPCSO3", iec61850.CtlModelSBOEnhanced, "InteropLD/GGIO1$ST$SPCSO3$stVal"},
	}
	for _, c := range controls {
		if err := srv.RegisterControl("InteropLD", c.doRef, c.ctlModel, makeOperateHandler(c.stValKey)); err != nil {
			return fmt.Errorf("register control %s: %w", c.doRef, err)
		}
	}
	return nil
}
