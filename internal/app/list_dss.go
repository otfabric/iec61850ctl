// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// ListDataSetsInput specifies the LD and LN to list data sets for.
type ListDataSetsInput struct {
	LD string // Logical device name (required)
	LN string // Logical node name (required)
}

// ListDataSetNames returns data set names within a logical node.
func (a *App) ListDataSetNames(input ListDataSetsInput) ([]string, error) {
	return a.DataSetService().ListDataSets(input.LD, input.LN)
}

// GetDataSetInput specifies the data set to retrieve details for.
type GetDataSetInput struct {
	LD   string // Logical device name (required)
	LN   string // Logical node name (required)
	Name string // Data set name (required)
}

// GetDataSet returns detailed data set information with member references.
func (a *App) GetDataSet(input GetDataSetInput) (*view.DataSet, error) {
	ds, err := a.DataSetService().GetDataSetDetails(input.LD, input.LN, input.Name)
	if err != nil {
		return nil, err
	}

	vds := service.ProjectDataSet(*ds)
	return &vds, nil
}

// ListDataSets returns all data sets with details for a logical node.
func (a *App) ListDataSets(input ListDataSetsInput) ([]view.DataSet, error) {
	dsSvc := a.DataSetService()

	names, err := dsSvc.ListDataSets(input.LD, input.LN)
	if err != nil {
		return nil, err
	}

	result := make([]view.DataSet, 0, len(names))
	for _, name := range names {
		ds, err := dsSvc.GetDataSetDetails(input.LD, input.LN, name)
		if err != nil {
			continue // skip datasets that fail to load
		}
		result = append(result, service.ProjectDataSet(*ds))
	}
	return result, nil
}
