// SPDX-License-Identifier: MIT

package view

// DataSet is the projection of domain.DataSet for display/API output.
type DataSet struct {
	Name        string          `json:"name"`
	IsDeletable bool            `json:"is_deletable"`
	MemberCount int             `json:"member_count"`
	Members     []DataSetMember `json:"members"`
}

// DataSetMember is the projection of domain.DataSetMember for display/API output.
type DataSetMember struct {
	Index int    `json:"index"`
	Ref   string `json:"ref"`
	FC    string `json:"fc,omitempty"`
	Value string `json:"value,omitempty"`
}
