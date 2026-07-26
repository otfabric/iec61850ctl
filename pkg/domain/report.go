// SPDX-License-Identifier: MIT

package domain

import iec61850 "github.com/otfabric/go-iec61850"

// ReasonForInclusion describes why a data set member is included in a received report.
type ReasonForInclusion string

const (
	ReasonNotIncluded   ReasonForInclusion = ""
	ReasonDataChange    ReasonForInclusion = "data-change"
	ReasonQualityChange ReasonForInclusion = "quality-change"
	ReasonDataUpdate    ReasonForInclusion = "data-update"
	ReasonIntegrity     ReasonForInclusion = "integrity"
	ReasonGI            ReasonForInclusion = "gi"
	ReasonUnknown       ReasonForInclusion = "unknown"
)

// Report represents a received report event from a report control block.
type Report struct {
	RcbRef       string
	RptID        string
	SeqNum       *uint32
	SubSeqNum    *uint16
	MoreSegments bool
	DatSet       string
	ConfRev      *uint32
	BufOvfl      *bool
	EntryID      []byte // BRCB entry identifier when present
	Timestamp    *Timestamp
	Elements     []ReportElement
}

// ReportElement represents a single data set member value within a received report.
type ReportElement struct {
	Index  int
	Ref    string
	Reason ReasonForInclusion
	Value  *Value
}

// FromReportIndication converts a go-iec61850 ReportIndication to domain.Report.
func FromReportIndication(ind *iec61850.ReportIndication, includeValues bool) *Report {
	if ind == nil {
		return nil
	}
	r := &Report{
		RptID:        ind.RptID,
		DatSet:       ind.DatSet,
		MoreSegments: ind.MoreSegments,
	}
	seq := ind.SeqNum
	r.SeqNum = &seq
	if ind.SubSeqNum != 0 {
		sub := uint16(ind.SubSeqNum)
		r.SubSeqNum = &sub
	}
	if ind.ConfRev != 0 {
		cr := ind.ConfRev
		r.ConfRev = &cr
	}
	if ind.BufOvfl {
		b := true
		r.BufOvfl = &b
	}
	if len(ind.EntryID) > 0 {
		r.EntryID = append([]byte(nil), ind.EntryID...)
	}
	if !ind.Timestamp.IsZero() {
		ts := Timestamp{UnixMs: uint64(ind.Timestamp.UnixMilli())}
		r.Timestamp = &ts
	}
	if includeValues {
		for i, v := range ind.Values {
			elem := ReportElement{Index: i}
			if i < len(ind.DataReferences) {
				elem.Ref = ind.DataReferences[i]
			}
			if i < len(ind.ReasonCodes) {
				elem.Reason = reasonFromCode(ind.ReasonCodes[i])
			}
			if v != nil {
				elem.Value = ValueFromMMS(v.MMS())
			}
			r.Elements = append(r.Elements, elem)
		}
	}
	return r
}

func reasonFromCode(rc iec61850.ReasonCode) ReasonForInclusion {
	switch {
	case rc&iec61850.ReasonDataChanged != 0:
		return ReasonDataChange
	case rc&iec61850.ReasonQualityChanged != 0:
		return ReasonQualityChange
	case rc&iec61850.ReasonDataUpdate != 0:
		return ReasonDataUpdate
	case rc&iec61850.ReasonIntegrity != 0:
		return ReasonIntegrity
	case rc&iec61850.ReasonGI != 0:
		return ReasonGI
	default:
		return ReasonUnknown
	}
}
