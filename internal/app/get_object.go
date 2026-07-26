// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// GetObjectInput specifies an object read.
type GetObjectInput struct {
	Object string
	FC     domain.FunctionalConstraint
}

// GetObject reads a single object reference.
func (a *App) GetObject(input GetObjectInput) (*service.ObjectValue, error) {
	return a.Reader().ReadObject(input.Object, input.FC)
}
