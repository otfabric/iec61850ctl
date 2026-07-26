// SPDX-License-Identifier: MIT

package view

// LogicalNode is the projection of domain.LogicalNode for display/API output.
type LogicalNode struct {
	Name     string `json:"name"`
	DOCount  int    `json:"do_count"`
	DSCount  int    `json:"ds_count"`
	RCBCount int    `json:"rcb_count"`
}

// DataObject is the projection of domain.DataObject for display/API output.
type DataObject struct {
	Name    string `json:"name"`
	DACount int    `json:"da_count"`
}
