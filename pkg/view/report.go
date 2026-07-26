// SPDX-License-Identifier: MIT

package view

// ReportControlBlock is the projection of domain.ReportControlBlock for display/API output.
type ReportControlBlock struct {
	Name     string `json:"name"`
	LD       string `json:"ld,omitempty"`
	LN       string `json:"ln,omitempty"`
	Buffered bool   `json:"buffered"`
	Ref      string `json:"ref,omitempty"`

	RptID    string  `json:"rpt_id,omitempty"`
	DatSet   string  `json:"dat_set,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
	ConfRev  *uint32 `json:"conf_rev,omitempty"`
	IntgPd   *uint32 `json:"intg_pd_ms,omitempty"`
	BufTm    *uint32 `json:"buf_tm_ms,omitempty"`
	Reserved *bool   `json:"reserved,omitempty"`
	SqNum    *uint16 `json:"sq_num,omitempty"`

	TriggerOptions TriggerOptions `json:"trigger_options"`
	OptionalFields OptionalFields `json:"optional_fields"`
}

// TriggerOptions describes the trigger conditions for a report.
type TriggerOptions struct {
	DataChange    bool `json:"data_change"`
	QualityChange bool `json:"quality_change"`
	DataUpdate    bool `json:"data_update"`
	Periodic      bool `json:"periodic"`
	GI            bool `json:"gi"`
	Transient     bool `json:"transient,omitempty"`
}

// OptionalFields describes which optional fields are included in reports.
type OptionalFields struct {
	SequenceNumber bool `json:"sequence_number"`
	TimeStamp      bool `json:"time_stamp"`
	ReasonCode     bool `json:"reason_code"`
	DataSetName    bool `json:"data_set_name"`
	DataReference  bool `json:"data_reference"`
	BufferOverflow bool `json:"buffer_overflow"`
	EntryID        bool `json:"entry_id"`
	ConfigRevision bool `json:"config_revision"`
}

// ReportControlBlockRef is the projection of domain.ReportControlBlockRef for listing.
type ReportControlBlockRef struct {
	LD       string `json:"ld,omitempty"`
	LN       string `json:"ln,omitempty"`
	Name     string `json:"name"`
	Buffered bool   `json:"buffered"`
	Ref      string `json:"ref,omitempty"`
}
