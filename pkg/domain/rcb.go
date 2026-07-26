// SPDX-License-Identifier: MIT

package domain

// ReportControlBlockRef identifies a report control block (URCB or BRCB) by location and name.
// Used for listing and as the identity portion of ReportControlBlock.
// Ref is "LD/LN.Name" when LD and LN are set; empty in tree context when only Name+Buffered are known.
type ReportControlBlockRef struct {
	LD       string
	LN       string
	Name     string
	Buffered bool
	Ref      string // LD/LN.Name when LD and LN are set
}

// ReportControlBlock represents an IEC 61850 report control block (URCB or BRCB).
// Pointer fields distinguish "not fetched" from zero-value.
type ReportControlBlock struct {
	Name     string
	LD       string
	LN       string
	Buffered bool
	Ref      string

	RptID    string
	DatSet   string
	Enabled  *bool
	ConfRev  *uint32
	IntgPd   *uint32
	BufTm    *uint32
	Reserved *bool
	SqNum    *uint16

	TriggerOptions TriggerOptions
	OptionalFields OptionalFields
}

// TriggerOptions represents the trigger conditions for a report control block.
type TriggerOptions struct {
	DataChange    bool
	QualityChange bool
	DataUpdate    bool
	Periodic      bool
	GI            bool
	Transient     bool
}

// OptionalFields represents the optional fields in RCB reports.
type OptionalFields struct {
	SequenceNumber bool
	TimeStamp      bool
	ReasonCode     bool
	DataSetName    bool
	DataReference  bool
	BufferOverflow bool
	EntryID        bool
	ConfigRevision bool
}

// IsFullyPopulated returns true if the RCB has configuration details beyond name/buffered.
func (rcb *ReportControlBlock) IsFullyPopulated() bool {
	return rcb.Enabled != nil || rcb.ConfRev != nil || rcb.RptID != ""
}

// FCString returns the FC for this RCB type ("BR" for buffered, "RP" for unbuffered).
func (rcb *ReportControlBlock) FCString() FunctionalConstraint {
	if rcb.Buffered {
		return FC_BR
	}
	return FC_RP
}
