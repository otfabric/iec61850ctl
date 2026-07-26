// SPDX-License-Identifier: MIT

package view

// ObjectRead is the machine-readable projection of a get object result.
type ObjectRead struct {
	Object string `json:"object"`
	FC     string `json:"fc"`
	Type   string `json:"type"`
	Value  any    `json:"value"`
}

// ReportDetails is a single JSON envelope for get report [--detailed].
type ReportDetails struct {
	Report  ReportControlBlock `json:"report"`
	DataSet *DataSet           `json:"data_set"`
}

// DataSetName is a minimal list-dss entry.
type DataSetName struct {
	Name        string          `json:"name"`
	IsDeletable bool            `json:"is_deletable,omitempty"`
	MemberCount int             `json:"member_count,omitempty"`
	Members     []DataSetMember `json:"members,omitempty"`
}
