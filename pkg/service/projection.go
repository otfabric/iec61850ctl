// SPDX-License-Identifier: MIT

package service

import (
	"fmt"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// ProjectLogicalDevice maps a domain.LogicalDevice to its view projection.
// Summary counts are taken from pre-computed fields when available, falling back
// to counting child slices when the domain object was populated with full data.
func ProjectLogicalDevice(ld domain.LogicalDevice) view.LogicalDevice {
	lnCount := ld.LNCount
	if lnCount == 0 && len(ld.LogicalNodes) > 0 {
		lnCount = len(ld.LogicalNodes)
	}

	dsCount := ld.DSCount
	urcbCount := ld.URCBCount
	brcbCount := ld.BRCBCount

	// If counts are zero but we have full LN data, compute from children.
	if dsCount == 0 && urcbCount == 0 && brcbCount == 0 && len(ld.LogicalNodes) > 0 {
		for _, ln := range ld.LogicalNodes {
			dsCount += len(ln.DataSets)
			for _, rcb := range ln.ReportControlBlocks {
				if rcb.Buffered {
					brcbCount++
				} else {
					urcbCount++
				}
			}
		}
	}

	return view.LogicalDevice{
		Name:      ld.Name,
		LNCount:   lnCount,
		DSCount:   dsCount,
		URCBCount: urcbCount,
		BRCBCount: brcbCount,
	}
}

// ProjectLogicalDevices maps a slice of domain logical devices to view projections.
func ProjectLogicalDevices(lds []domain.LogicalDevice) []view.LogicalDevice {
	result := make([]view.LogicalDevice, len(lds))
	for i, ld := range lds {
		result[i] = ProjectLogicalDevice(ld)
	}
	return result
}

// ProjectDataAttribute maps a domain.DataAttribute to its view projection.
// Values, timestamps, and quality are rendered as display strings.
func ProjectDataAttribute(da domain.DataAttribute) view.DataAttribute {
	v := view.DataAttribute{
		Name: da.Name,
		Ref:  da.Ref,
	}

	if da.FC != domain.FC_NONE {
		v.FC = string(da.FC)
	}
	if da.Type != "" {
		v.Type = string(da.Type)
	}

	if da.Value != nil {
		if da.Value.Display != "" {
			v.Value = da.Value.Display
		} else if da.Value.Raw != nil {
			v.Value = fmt.Sprintf("%v", da.Value.Raw)
		}
	}
	if da.ValueError != "" && v.Value == "" {
		v.Value = fmt.Sprintf("error: %s", da.ValueError)
	}

	if da.Timestamp != nil && da.Timestamp.UnixMs > 0 {
		v.Time = da.Timestamp.RFC3339()
	}

	if da.Quality != nil {
		v.Quality = da.Quality.String()
	}

	if len(da.Children) > 0 {
		v.Children = ProjectDataAttributes(da.Children)
	}

	return v
}

// ProjectDataAttributes maps a slice of domain data attributes to view projections.
func ProjectDataAttributes(das []domain.DataAttribute) []view.DataAttribute {
	result := make([]view.DataAttribute, len(das))
	for i, da := range das {
		result[i] = ProjectDataAttribute(da)
	}
	return result
}

// ProjectDataSet maps a domain.DataSet to its view projection.
func ProjectDataSet(ds domain.DataSet) view.DataSet {
	members := make([]view.DataSetMember, len(ds.Members))
	for i, m := range ds.Members {
		members[i] = view.DataSetMember{
			Index: i + 1,
			Ref:   m.Ref,
		}
		if m.FC != domain.FC_NONE {
			members[i].FC = string(m.FC)
		}
		if m.Value != nil && m.Value.Raw != nil {
			members[i].Value = fmt.Sprintf("%v", m.Value.Raw)
		}
	}
	return view.DataSet{
		Name:        ds.Name,
		IsDeletable: ds.IsDeletable,
		MemberCount: len(ds.Members),
		Members:     members,
	}
}

// ProjectReportControlBlock maps a domain.ReportControlBlock to its view projection.
func ProjectReportControlBlock(rcb domain.ReportControlBlock) view.ReportControlBlock {
	return view.ReportControlBlock{
		Name:     rcb.Name,
		LD:       rcb.LD,
		LN:       rcb.LN,
		Buffered: rcb.Buffered,
		Ref:      rcb.Ref,
		RptID:    rcb.RptID,
		DatSet:   rcb.DatSet,
		Enabled:  rcb.Enabled,
		ConfRev:  rcb.ConfRev,
		IntgPd:   rcb.IntgPd,
		BufTm:    rcb.BufTm,
		Reserved: rcb.Reserved,
		SqNum:    rcb.SqNum,
		TriggerOptions: view.TriggerOptions{
			DataChange:    rcb.TriggerOptions.DataChange,
			QualityChange: rcb.TriggerOptions.QualityChange,
			DataUpdate:    rcb.TriggerOptions.DataUpdate,
			Periodic:      rcb.TriggerOptions.Periodic,
			GI:            rcb.TriggerOptions.GI,
			Transient:     rcb.TriggerOptions.Transient,
		},
		OptionalFields: view.OptionalFields{
			SequenceNumber: rcb.OptionalFields.SequenceNumber,
			TimeStamp:      rcb.OptionalFields.TimeStamp,
			ReasonCode:     rcb.OptionalFields.ReasonCode,
			DataSetName:    rcb.OptionalFields.DataSetName,
			DataReference:  rcb.OptionalFields.DataReference,
			BufferOverflow: rcb.OptionalFields.BufferOverflow,
			EntryID:        rcb.OptionalFields.EntryID,
			ConfigRevision: rcb.OptionalFields.ConfigRevision,
		},
	}
}

// ProjectReportControlBlockRef maps a domain.ReportControlBlockRef to its view projection.
func ProjectReportControlBlockRef(ref domain.ReportControlBlockRef) view.ReportControlBlockRef {
	return view.ReportControlBlockRef{
		LD:       ref.LD,
		LN:       ref.LN,
		Name:     ref.Name,
		Buffered: ref.Buffered,
		Ref:      ref.Ref,
	}
}

// ProjectReportControlBlockRefs maps a slice of domain report control block refs to view projections.
func ProjectReportControlBlockRefs(refs []domain.ReportControlBlockRef) []view.ReportControlBlockRef {
	result := make([]view.ReportControlBlockRef, len(refs))
	for i, ref := range refs {
		result[i] = ProjectReportControlBlockRef(ref)
	}
	return result
}

// ProjectFileEntry maps a domain.FileEntry to its view projection.
// Size and timestamp are formatted as human-readable strings.
func ProjectFileEntry(f domain.FileEntry, formatSize func(uint64) string, formatTime func(uint64) string) view.FileEntry {
	return view.FileEntry{
		Name:         f.Name,
		Size:         formatSize(uint64(f.Size)),
		SizeBytes:    f.Size,
		LastModified: formatTime(f.LastModified),
	}
}

// ProjectJournalInfo maps a domain.JournalInfo to its view projection.
func ProjectJournalInfo(j domain.JournalInfo) view.JournalInfo {
	return view.JournalInfo{
		Name:        j.Name,
		LogicalNode: j.LogicalNode,
		FullRef:     j.FullRef,
	}
}

// ProjectJournalInfos maps a slice of domain journal info to view projections.
func ProjectJournalInfos(infos []domain.JournalInfo) []view.JournalInfo {
	result := make([]view.JournalInfo, len(infos))
	for i, j := range infos {
		result[i] = ProjectJournalInfo(j)
	}
	return result
}

// ProjectJournalEntry maps a domain.JournalEntry to its view projection.
func ProjectJournalEntry(e domain.JournalEntry) view.JournalEntry {
	vars := make([]view.JournalVariable, len(e.Variables))
	for i, v := range e.Variables {
		vars[i] = view.JournalVariable{
			Tag:   v.Tag,
			Value: v.Value,
		}
	}
	return view.JournalEntry{
		EntryID:        e.EntryID,
		OccurrenceTime: e.OccurrenceTime,
		Variables:      vars,
	}
}

// ProjectJournalEntries maps a slice of domain journal entries to view projections.
func ProjectJournalEntries(entries []domain.JournalEntry) []view.JournalEntry {
	result := make([]view.JournalEntry, len(entries))
	for i, e := range entries {
		result[i] = ProjectJournalEntry(e)
	}
	return result
}
