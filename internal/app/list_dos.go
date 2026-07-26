// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// ListDataObjectsInput specifies the LD and LN to list data objects for.
type ListDataObjectsInput struct {
	LD string // Logical device name (required)
	LN string // Logical node name (required)
}

// ListDataObjectNames returns data object names within a logical node.
func (a *App) ListDataObjectNames(input ListDataObjectsInput) ([]string, error) {
	return a.Explorer().ListDataObjects(input.LD, input.LN)
}

// ListDataObjects returns data objects with optional DA counts.
func (a *App) ListDataObjects(input ListDataObjectsInput) ([]view.DataObject, error) {
	explorer := a.Explorer()
	names, err := explorer.ListDataObjects(input.LD, input.LN)
	if err != nil {
		return nil, err
	}

	result := make([]view.DataObject, len(names))
	for i, name := range names {
		// Count data attributes
		daCount := 0
		attrsMap, err := a.ListDataAttributes(ListDataAttributesInput{
			LD:       input.LD,
			LN:       input.LN,
			DO:       name,
			Detailed: false,
		})
		if err == nil {
			for _, attrs := range attrsMap {
				daCount += len(attrs)
			}
		}

		result[i] = view.DataObject{
			Name:    name,
			DACount: daCount,
		}
	}
	return result, nil
}
