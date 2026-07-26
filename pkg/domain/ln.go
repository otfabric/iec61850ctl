// SPDX-License-Identifier: MIT

package domain

// LogicalNode represents an IEC 61850 logical node (LN) within a logical device.
type LogicalNode struct {
	Name                string
	DataObjects         []DataObject
	DataSets            []string
	ReportControlBlocks []ReportControlBlockRef
	LogControlBlocks    []LogControlBlock
	SGCBs               []SettingGroupCB
	DOCount             int // optional summary when attributes are not loaded
}
