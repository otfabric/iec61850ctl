// SPDX-License-Identifier: MIT

package domain

// DataSet represents an IEC 61850 data set.
type DataSet struct {
	Name        string
	IsDeletable bool
	Members     []DataSetMember
}

// DataSetMember represents a single entry in a data set.
type DataSetMember struct {
	Ref   string
	FC    FunctionalConstraint
	Value *Value
}
