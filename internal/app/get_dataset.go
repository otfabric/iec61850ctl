// SPDX-License-Identifier: MIT

package app

import (
	"context"

	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// GetDataSetWithValuesInput specifies a data set to retrieve with live values.
type GetDataSetWithValuesInput struct {
	LD         string // Logical device name (required)
	LN         string // Logical node name (required)
	Name       string // Data set name (required)
	ReadValues bool   // Whether to read live values for each member
}

// GetDataSetWithValues returns data set details with optional live member values.
func (a *App) GetDataSetWithValues(input GetDataSetWithValuesInput) (*view.DataSet, error) {
	dsSvc := a.DataSetService()

	ds, err := dsSvc.GetDataSetDetails(input.LD, input.LN, input.Name)
	if err != nil {
		return nil, err
	}

	vds := service.ProjectDataSet(*ds)

	if input.ReadValues {
		lookup := input.Name
		if input.LN != "" {
			lookup = input.LN + "$" + input.Name
		}
		values, err := a.Connection().ReadDataSet(context.Background(), input.LD, lookup)
		if err != nil {
			values, err = a.Connection().ReadDataSet(context.Background(), input.LD, input.Name)
		}
		if err == nil {
			for i, mv := range values {
				if i < len(vds.Members) && mv.Value != nil {
					vds.Members[i].Value = formatter.FormatMmsValue(mv.Value.MMS())
				}
			}
		}
	}

	return &vds, nil
}
