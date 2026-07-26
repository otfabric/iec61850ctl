// SPDX-License-Identifier: MIT

package domain

// DataObject represents an IEC 61850 data object (DO) within a logical node.
type DataObject struct {
	Name       string
	Attributes []DataAttribute
}
