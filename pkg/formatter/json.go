// SPDX-License-Identifier: MIT

package formatter

import (
	"encoding/json"
	"io"

	"github.com/otfabric/iec61850ctl/pkg/view"
)

type jsonRenderer struct{}

func (r *jsonRenderer) RenderLogicalDevices(devices []view.LogicalDevice, w io.Writer) error {
	return encodeJSON(devices, w)
}

func (r *jsonRenderer) RenderLogicalNodes(nodes []view.LogicalNode, w io.Writer) error {
	return encodeJSON(nodes, w)
}

func (r *jsonRenderer) RenderDataObjects(objects []view.DataObject, w io.Writer) error {
	return encodeJSON(objects, w)
}

func (r *jsonRenderer) RenderDataAttributes(attrs []view.DataAttribute, w io.Writer) error {
	return encodeJSON(attrs, w)
}

func (r *jsonRenderer) RenderDataSet(ds *view.DataSet, w io.Writer) error {
	return encodeJSON(ds, w)
}

func (r *jsonRenderer) RenderReportControlBlock(rcb *view.ReportControlBlock, w io.Writer) error {
	return encodeJSON(rcb, w)
}

func encodeJSON(v interface{}, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
