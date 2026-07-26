// SPDX-License-Identifier: MIT

package domain

// IED is the top-level aggregate for a full IEC 61850 device model.
// Used by "tree --serialize" to produce a complete JSON representation.
type IED struct {
	Meta           IEDMeta
	LogicalDevices []LogicalDevice
	Leaves         []LeafEntry
}

// IEDMeta holds metadata about the serialized IED snapshot.
type IEDMeta struct {
	SourceHost   string
	SourcePort   int
	SerializedAt string
	Generator    string
}

// LeafEntry represents a single leaf value in the flat index.
type LeafEntry struct {
	Ref        string
	FC         FunctionalConstraint
	Type       MmsDataType
	Value      *Value
	ValueError string
}

// LogControlBlock represents a log control block (LCB) entry.
type LogControlBlock struct {
	Name    string
	LD      string
	LN      string
	FullRef string
}

// SettingGroupCB represents a setting group control block (SGCB).
type SettingGroupCB struct {
	NumOfSG int
	ActSG   int
	EditSG  int
	CnfEdit bool
}
