// SPDX-License-Identifier: MIT

// Package formatter: structured output (JSON, CSV, Table, YAML) for IEC 61850 view types.
//
// The Formatter delegates to a Renderer selected by output format.
// All public Render* methods accept view types only.

package formatter

import (
	"fmt"
	"io"

	"github.com/otfabric/iec61850ctl/pkg/view"
)

// RenderDataAttributes renders view data attributes using the configured output format.
func (f *Formatter) RenderDataAttributes(attrs []view.DataAttribute, w io.Writer) error {
	return NewRenderer(f.outputFormat).RenderDataAttributes(attrs, w)
}

// RenderDataSet renders a view DataSet using the configured output format.
func (f *Formatter) RenderDataSet(ds *view.DataSet, w io.Writer) error {
	if ds == nil {
		return fmt.Errorf("dataset is nil")
	}
	return NewRenderer(f.outputFormat).RenderDataSet(ds, w)
}

// RenderReportControlBlock renders a view ReportControlBlock using the configured output format.
func (f *Formatter) RenderReportControlBlock(rcb *view.ReportControlBlock, w io.Writer) error {
	if rcb == nil {
		return fmt.Errorf("report control block is nil")
	}
	return NewRenderer(f.outputFormat).RenderReportControlBlock(rcb, w)
}

// RenderLogicalDevices renders view logical devices using the configured output format.
func (f *Formatter) RenderLogicalDevices(devices []view.LogicalDevice, w io.Writer) error {
	return NewRenderer(f.outputFormat).RenderLogicalDevices(devices, w)
}

// RenderLogicalNodes renders view logical nodes using the configured output format.
func (f *Formatter) RenderLogicalNodes(nodes []view.LogicalNode, w io.Writer) error {
	return NewRenderer(f.outputFormat).RenderLogicalNodes(nodes, w)
}

// RenderDataObjects renders view data objects using the configured output format.
func (f *Formatter) RenderDataObjects(objects []view.DataObject, w io.Writer) error {
	return NewRenderer(f.outputFormat).RenderDataObjects(objects, w)
}
