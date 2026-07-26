// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// ListDataAttributesInput specifies the LD, LN, and DO(s) to list data attributes for.
type ListDataAttributesInput struct {
	LD       string // Logical device name (required)
	LN       string // Logical node name (required)
	DO       string // Data object name (required)
	Detailed bool   // Include values, types, and quality
}

// ListDataAttributes returns data attributes grouped by functional constraint.
func (a *App) ListDataAttributes(input ListDataAttributesInput) (map[string][]view.DataAttribute, error) {
	explorer := a.Explorer()

	svcInput := service.ListDataAttributesInput{
		LogicalDevice: input.LD,
		LogicalNode:   input.LN,
		DataObject:    input.DO,
		Detailed:      input.Detailed,
	}

	domainResult, err := explorer.ListDataAttributes(svcInput)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]view.DataAttribute, len(domainResult))
	for fc, attrs := range domainResult {
		result[fc] = service.ProjectDataAttributes(attrs)
	}
	return result, nil
}

// GetDataAttributes returns a flat list of data attributes for a data object.
func (a *App) GetDataAttributes(input ListDataAttributesInput) ([]view.DataAttribute, error) {
	explorer := a.Explorer()

	svcInput := service.ListDataAttributesInput{
		LogicalDevice: input.LD,
		LogicalNode:   input.LN,
		DataObject:    input.DO,
		Detailed:      input.Detailed,
	}

	domainAttrs, err := explorer.GetDataAttributes(svcInput)
	if err != nil {
		return nil, err
	}

	return service.ProjectDataAttributes(domainAttrs), nil
}
