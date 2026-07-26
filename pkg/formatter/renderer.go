// SPDX-License-Identifier: MIT

package formatter

import (
	"io"

	"github.com/otfabric/iec61850ctl/pkg/view"
)

// Renderer defines the contract for rendering view projections in a specific output format.
// Each output format (JSON, CSV, Table, YAML, Text) implements this interface.
// Renderers only accept view types — never domain types.
type Renderer interface {
	RenderLogicalDevices(devices []view.LogicalDevice, w io.Writer) error
	RenderLogicalNodes(nodes []view.LogicalNode, w io.Writer) error
	RenderDataObjects(objects []view.DataObject, w io.Writer) error
	RenderDataAttributes(attrs []view.DataAttribute, w io.Writer) error
	RenderDataSet(ds *view.DataSet, w io.Writer) error
	RenderReportControlBlock(rcb *view.ReportControlBlock, w io.Writer) error
}

// NewRenderer returns a Renderer for the given output format.
func NewRenderer(format OutputFormat) Renderer {
	switch format {
	case OutputFormatJSON:
		return &jsonRenderer{}
	case OutputFormatCSV:
		return &csvRenderer{}
	case OutputFormatTable:
		return &tableRenderer{}
	case OutputFormatYAML:
		return &yamlRenderer{}
	default:
		return &textRenderer{}
	}
}
