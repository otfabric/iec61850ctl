// SPDX-License-Identifier: MIT

package domain

// SampledValueStream represents an SV stream (placeholder for future "sv listen" command).
type SampledValueStream struct {
	SvID     string
	DatSet   string
	ConfRev  uint32
	SmpCnt   uint32
	SmpMod   uint8
	SmpRate  uint16
	SmpSynch uint8
	AppID    uint16
	SrcMAC   string
	DstMAC   string
	Values   []SampledValue
}

// SampledValue represents a single sample within an SV stream.
type SampledValue struct {
	Index   int
	Value   *Value
	Quality *Quality
}
