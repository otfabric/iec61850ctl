// SPDX-License-Identifier: MIT

package app

import (
	"context"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// ControlInspect reads the control model for an object.
func (a *App) ControlInspect(ctx context.Context, object string) (*domain.ControlResult, error) {
	return service.NewController(a.conn).Inspect(ctx, object)
}

// ControlOperate executes an atomic control journey.
func (a *App) ControlOperate(ctx context.Context, req domain.ControlRequest) (*domain.ControlResult, error) {
	return service.NewController(a.conn).Operate(ctx, req)
}

// SetObject writes a scalar attribute.
func (a *App) SetObject(ctx context.Context, req domain.WriteRequest) (*domain.WriteResult, error) {
	return service.NewWriter(a.conn).SetObject(ctx, req)
}
