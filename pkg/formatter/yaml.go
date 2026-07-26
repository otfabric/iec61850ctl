// SPDX-License-Identifier: MIT

package formatter

import (
	"io"

	"github.com/otfabric/iec61850ctl/pkg/view"

	"gopkg.in/yaml.v3"
)

type yamlRenderer struct{}

func (r *yamlRenderer) RenderLogicalDevices(devices []view.LogicalDevice, w io.Writer) error {
	return encodeYAML(devices, w)
}

func (r *yamlRenderer) RenderLogicalNodes(nodes []view.LogicalNode, w io.Writer) error {
	return encodeYAML(nodes, w)
}

func (r *yamlRenderer) RenderDataObjects(objects []view.DataObject, w io.Writer) error {
	return encodeYAML(objects, w)
}

func (r *yamlRenderer) RenderDataAttributes(attrs []view.DataAttribute, w io.Writer) error {
	return encodeYAML(attrs, w)
}

func (r *yamlRenderer) RenderDataSet(ds *view.DataSet, w io.Writer) error {
	return encodeYAML(ds, w)
}

func (r *yamlRenderer) RenderReportControlBlock(rcb *view.ReportControlBlock, w io.Writer) error {
	return encodeYAML(rcb, w)
}

func encodeYAML(v interface{}, w io.Writer) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(v)
}
