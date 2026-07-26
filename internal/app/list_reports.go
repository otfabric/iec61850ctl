// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// ListReportsInput specifies filters for listing report control blocks.
type ListReportsInput struct {
	LD string // Logical device name (empty for all)
	LN string // Logical node name (empty for all)
}

// ListAllReports returns all report control block references across all LDs/LNs.
func (a *App) ListAllReports() ([]view.ReportControlBlockRef, error) {
	refs, err := a.ReportService().GetAllReports()
	if err != nil {
		return nil, err
	}
	return service.ProjectReportControlBlockRefs(refs), nil
}

// ListReportNames returns unbuffered and buffered report names for a specific LD/LN.
func (a *App) ListReportNames(input ListReportsInput) (unbuffered, buffered []string, err error) {
	rptSvc := a.ReportService()

	unbuffered, err = rptSvc.ListUnbufferedReports(input.LD, input.LN)
	if err != nil {
		return nil, nil, err
	}

	buffered, err = rptSvc.ListBufferedReports(input.LD, input.LN)
	if err != nil {
		return nil, nil, err
	}

	return unbuffered, buffered, nil
}
