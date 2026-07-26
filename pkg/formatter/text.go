// SPDX-License-Identifier: MIT

package formatter

import (
	"fmt"
	"io"

	"github.com/otfabric/iec61850ctl/pkg/view"
)

type textRenderer struct{}

func (r *textRenderer) RenderLogicalDevices(devices []view.LogicalDevice, w io.Writer) error {
	for _, ld := range devices {
		totalReports := ld.URCBCount + ld.BRCBCount
		_, _ = fmt.Fprintf(w, "%s  [LNs: %d, DataSets: %d, Reports: %d (UB:%d/B:%d)]\n",
			ld.Name, ld.LNCount, ld.DSCount, totalReports, ld.URCBCount, ld.BRCBCount)
	}
	return nil
}

func (r *textRenderer) RenderLogicalNodes(nodes []view.LogicalNode, w io.Writer) error {
	for _, ln := range nodes {
		_, _ = fmt.Fprintf(w, "%s  [DOs: %d, DataSets: %d, RCBs: %d]\n",
			ln.Name, ln.DOCount, ln.DSCount, ln.RCBCount)
	}
	return nil
}

func (r *textRenderer) RenderDataObjects(objects []view.DataObject, w io.Writer) error {
	for _, do := range objects {
		_, _ = fmt.Fprintf(w, "%s  [DAs: %d]\n", do.Name, do.DACount)
	}
	return nil
}

func (r *textRenderer) RenderDataAttributes(attrs []view.DataAttribute, w io.Writer) error {
	for _, attr := range attrs {
		line := attr.Name
		if attr.Type != "" {
			line += " [" + attr.Type + "]"
		}
		if attr.FC != "" {
			line += " (" + attr.FC + ")"
		}
		if attr.Value != "" {
			line += " = " + attr.Value
		}
		if attr.Time != "" {
			line += " @ " + attr.Time
		}
		if attr.Quality != "" {
			line += " {" + attr.Quality + "}"
		}
		_, _ = fmt.Fprintln(w, line)
	}
	return nil
}

func (r *textRenderer) RenderDataSet(ds *view.DataSet, w io.Writer) error {
	_, _ = fmt.Fprintf(w, "Dataset: %s\n", ds.Name)
	_, _ = fmt.Fprintf(w, "Deletable: %v\n", ds.IsDeletable)
	_, _ = fmt.Fprintf(w, "Members (%d):\n", ds.MemberCount)
	for _, m := range ds.Members {
		fcStr := ""
		if m.FC != "" {
			fcStr = fmt.Sprintf(" [%s]", m.FC)
		}
		valueStr := ""
		if m.Value != "" {
			valueStr = fmt.Sprintf(" = %s", m.Value)
		}
		_, _ = fmt.Fprintf(w, "  %d. %s%s%s\n", m.Index, m.Ref, fcStr, valueStr)
	}
	return nil
}

func (r *textRenderer) RenderReportControlBlock(rcb *view.ReportControlBlock, w io.Writer) error {
	bufferedStr := "Unbuffered"
	if rcb.Buffered {
		bufferedStr = "Buffered"
	}
	_, _ = fmt.Fprintf(w, "Report Control Block: %s (%s)\n", rcb.Name, bufferedStr)
	_, _ = fmt.Fprintf(w, "Reference: %s/%s.%s\n", rcb.LD, rcb.LN, rcb.Name)
	if rcb.Enabled != nil {
		_, _ = fmt.Fprintf(w, "Enabled: %v\n", *rcb.Enabled)
	}
	if rcb.RptID != "" {
		_, _ = fmt.Fprintf(w, "Report ID: %s\n", rcb.RptID)
	}
	if rcb.DatSet != "" {
		_, _ = fmt.Fprintf(w, "Dataset: %s\n", rcb.DatSet)
	}
	if rcb.IntgPd != nil {
		_, _ = fmt.Fprintf(w, "Integrity Period: %d ms\n", *rcb.IntgPd)
	}
	if rcb.Reserved != nil {
		_, _ = fmt.Fprintf(w, "Reserved: %v\n", *rcb.Reserved)
	}
	if rcb.PurgeBuf != nil {
		_, _ = fmt.Fprintf(w, "PurgeBuf: %v\n", *rcb.PurgeBuf)
	}
	if rcb.EntryID != "" {
		_, _ = fmt.Fprintf(w, "EntryID: %s\n", rcb.EntryID)
	}
	if rcb.ResvTms != nil {
		_, _ = fmt.Fprintf(w, "ResvTms: %d\n", *rcb.ResvTms)
	}
	_, _ = fmt.Fprintf(w, "\nTrigger Options:\n")
	printBoolOpt(w, "  Data Change", rcb.TriggerOptions.DataChange)
	printBoolOpt(w, "  Quality Change", rcb.TriggerOptions.QualityChange)
	printBoolOpt(w, "  Data Update", rcb.TriggerOptions.DataUpdate)
	printBoolOpt(w, "  Periodic", rcb.TriggerOptions.Periodic)
	printBoolOpt(w, "  GI (General Interrogation)", rcb.TriggerOptions.GI)
	if rcb.TriggerOptions.Transient {
		printBoolOpt(w, "  Transient", rcb.TriggerOptions.Transient)
	}
	_, _ = fmt.Fprintf(w, "\nOptional Fields:\n")
	printBoolOpt(w, "  Sequence Number", rcb.OptionalFields.SequenceNumber)
	printBoolOpt(w, "  Time Stamp", rcb.OptionalFields.TimeStamp)
	printBoolOpt(w, "  Reason Code", rcb.OptionalFields.ReasonCode)
	printBoolOpt(w, "  Dataset Name", rcb.OptionalFields.DataSetName)
	printBoolOpt(w, "  Data Reference", rcb.OptionalFields.DataReference)
	printBoolOpt(w, "  Buffer Overflow", rcb.OptionalFields.BufferOverflow)
	printBoolOpt(w, "  Entry ID", rcb.OptionalFields.EntryID)
	printBoolOpt(w, "  Config Revision", rcb.OptionalFields.ConfigRevision)
	return nil
}

func printBoolOpt(w io.Writer, name string, value bool) {
	if value {
		_, _ = fmt.Fprintf(w, "%s: ✓\n", name)
	} else {
		_, _ = fmt.Fprintf(w, "%s: ✗\n", name)
	}
}
