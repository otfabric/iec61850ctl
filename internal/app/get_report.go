// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// GetReportInput specifies the report control block to retrieve.
type GetReportInput struct {
	LD       string // Logical device name (required)
	LN       string // Logical node name (required)
	Name     string // Report name (required)
	Buffered *bool  // Optional; nil probes BR then RP
}

// GetReportResult contains report details and optionally the linked dataset.
type GetReportResult struct {
	Report  view.ReportControlBlock `json:"report"`
	DataSet *view.DataSet           `json:"dataset,omitempty"`
}

// GetReport returns report control block details with optional linked dataset.
func (a *App) GetReport(input GetReportInput) (*GetReportResult, error) {
	var (
		rcb *domain.ReportControlBlock
		err error
	)
	if input.Buffered != nil {
		rcb, err = a.ReportService().GetReportDetails(input.LD, input.LN, input.Name, *input.Buffered)
	} else {
		rcb, err = a.ReportService().ResolveReportDetails(input.LD, input.LN, input.Name)
	}
	if err != nil {
		return nil, err
	}

	vrcb := service.ProjectReportControlBlock(*rcb)
	result := &GetReportResult{Report: vrcb}

	if rcb.DatSet != "" {
		if ld, ln, dsName, ok := domain.ParseDataSetRef(rcb.DatSet); ok {
			if ds, err := a.DataSetService().GetDataSetDetails(ld, ln, dsName); err == nil {
				vds := service.ProjectDataSet(*ds)
				result.DataSet = &vds
			}
		}
	}

	return result, nil
}
