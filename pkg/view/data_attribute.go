// SPDX-License-Identifier: MIT

package view

// DataAttribute is the projection of domain.DataAttribute for display/API output.
// Leaf attributes contain a flat value representation; compound attributes list children.
type DataAttribute struct {
	Name     string          `json:"name"`
	Ref      string          `json:"ref,omitempty"`
	FC       string          `json:"fc,omitempty"`
	Type     string          `json:"type,omitempty"`
	Value    string          `json:"value,omitempty"`
	Time     string          `json:"time,omitempty"`
	Quality  string          `json:"quality,omitempty"`
	Children []DataAttribute `json:"children,omitempty"`
}
