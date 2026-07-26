// SPDX-License-Identifier: MIT

package formatter

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/otfabric/iec61850ctl/pkg/view"
)

type tableRenderer struct{}

func (r *tableRenderer) RenderLogicalDevices(devices []view.LogicalDevice, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()
	_, _ = fmt.Fprintf(tw, "NAME\tLNs\tDataSets\tUB\tBR\n")
	_, _ = fmt.Fprintf(tw, "----\t---\t--------\t--\t--\n")
	for _, ld := range devices {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\n", ld.Name, ld.LNCount, ld.DSCount, ld.URCBCount, ld.BRCBCount)
	}
	return nil
}

func (r *tableRenderer) RenderLogicalNodes(nodes []view.LogicalNode, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()
	_, _ = fmt.Fprintf(tw, "NAME\tDOs\tDataSets\tRCBs\n")
	_, _ = fmt.Fprintf(tw, "----\t---\t--------\t----\n")
	for _, ln := range nodes {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\t%d\n", ln.Name, ln.DOCount, ln.DSCount, ln.RCBCount)
	}
	return nil
}

func (r *tableRenderer) RenderDataObjects(objects []view.DataObject, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()
	_, _ = fmt.Fprintf(tw, "NAME\tDAs\n")
	_, _ = fmt.Fprintf(tw, "----\t---\n")
	for _, do := range objects {
		_, _ = fmt.Fprintf(tw, "%s\t%d\n", do.Name, do.DACount)
	}
	return nil
}

func (r *tableRenderer) RenderDataAttributes(attrs []view.DataAttribute, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()
	_, _ = fmt.Fprintf(tw, "NAME\tTYPE\tFC\tVALUE\n")
	_, _ = fmt.Fprintf(tw, "----\t----\t--\t-----\n")
	for _, attr := range attrs {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", attr.Name, attr.Type, attr.FC, attr.Value)
	}
	return nil
}

func (r *tableRenderer) RenderDataSet(ds *view.DataSet, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()
	_, _ = fmt.Fprintf(tw, "Dataset: %s\n", ds.Name)
	_, _ = fmt.Fprintf(tw, "Deletable: %v\n", ds.IsDeletable)
	_, _ = fmt.Fprintf(tw, "Members: %d\n\n", ds.MemberCount)
	_, _ = fmt.Fprintf(tw, "INDEX\tREFERENCE\tFC\tVALUE\n")
	_, _ = fmt.Fprintf(tw, "-----\t---------\t--\t-----\n")
	for _, m := range ds.Members {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", m.Index, m.Ref, m.FC, m.Value)
	}
	return nil
}

func (r *tableRenderer) RenderReportControlBlock(rcb *view.ReportControlBlock, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()
	_, _ = fmt.Fprintf(tw, "FIELD\tVALUE\n")
	_, _ = fmt.Fprintf(tw, "-----\t-----\n")
	_, _ = fmt.Fprintf(tw, "Name\t%s\n", rcb.Name)
	_, _ = fmt.Fprintf(tw, "LD\t%s\n", rcb.LD)
	_, _ = fmt.Fprintf(tw, "LN\t%s\n", rcb.LN)
	_, _ = fmt.Fprintf(tw, "Ref\t%s\n", rcb.Ref)
	_, _ = fmt.Fprintf(tw, "Buffered\t%v\n", rcb.Buffered)
	if rcb.Enabled != nil {
		_, _ = fmt.Fprintf(tw, "Enabled\t%v\n", *rcb.Enabled)
	}
	if rcb.RptID != "" {
		_, _ = fmt.Fprintf(tw, "RptID\t%s\n", rcb.RptID)
	}
	if rcb.DatSet != "" {
		_, _ = fmt.Fprintf(tw, "DatSet\t%s\n", rcb.DatSet)
	}
	if rcb.IntgPd != nil {
		_, _ = fmt.Fprintf(tw, "IntgPd\t%d ms\n", *rcb.IntgPd)
	}
	if rcb.Reserved != nil {
		_, _ = fmt.Fprintf(tw, "Reserved\t%v\n", *rcb.Reserved)
	}
	_, _ = fmt.Fprintf(tw, "\nTRIGGER OPTIONS\t\n")
	_, _ = fmt.Fprintf(tw, "---------------\t\n")
	_, _ = fmt.Fprintf(tw, "DataChange\t%v\n", rcb.TriggerOptions.DataChange)
	_, _ = fmt.Fprintf(tw, "QualityChange\t%v\n", rcb.TriggerOptions.QualityChange)
	_, _ = fmt.Fprintf(tw, "DataUpdate\t%v\n", rcb.TriggerOptions.DataUpdate)
	_, _ = fmt.Fprintf(tw, "Periodic\t%v\n", rcb.TriggerOptions.Periodic)
	_, _ = fmt.Fprintf(tw, "GI\t%v\n", rcb.TriggerOptions.GI)
	_, _ = fmt.Fprintf(tw, "\nOPTIONAL FIELDS\t\n")
	_, _ = fmt.Fprintf(tw, "---------------\t\n")
	_, _ = fmt.Fprintf(tw, "SequenceNumber\t%v\n", rcb.OptionalFields.SequenceNumber)
	_, _ = fmt.Fprintf(tw, "TimeStamp\t%v\n", rcb.OptionalFields.TimeStamp)
	_, _ = fmt.Fprintf(tw, "ReasonCode\t%v\n", rcb.OptionalFields.ReasonCode)
	_, _ = fmt.Fprintf(tw, "DataSetName\t%v\n", rcb.OptionalFields.DataSetName)
	_, _ = fmt.Fprintf(tw, "DataReference\t%v\n", rcb.OptionalFields.DataReference)
	_, _ = fmt.Fprintf(tw, "BufferOverflow\t%v\n", rcb.OptionalFields.BufferOverflow)
	_, _ = fmt.Fprintf(tw, "EntryID\t%v\n", rcb.OptionalFields.EntryID)
	_, _ = fmt.Fprintf(tw, "ConfigRevision\t%v\n", rcb.OptionalFields.ConfigRevision)
	return nil
}
