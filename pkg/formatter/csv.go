// SPDX-License-Identifier: MIT

package formatter

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/otfabric/iec61850ctl/pkg/view"
)

type csvRenderer struct{}

func (r *csvRenderer) RenderLogicalDevices(devices []view.LogicalDevice, w io.Writer) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"Name", "LNCount", "DSCount", "URCBCount", "BRCBCount"}); err != nil {
		return err
	}
	for _, ld := range devices {
		if err := cw.Write([]string{
			ld.Name,
			fmt.Sprintf("%d", ld.LNCount),
			fmt.Sprintf("%d", ld.DSCount),
			fmt.Sprintf("%d", ld.URCBCount),
			fmt.Sprintf("%d", ld.BRCBCount),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *csvRenderer) RenderLogicalNodes(nodes []view.LogicalNode, w io.Writer) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"Name", "DOCount", "DSCount", "RCBCount"}); err != nil {
		return err
	}
	for _, ln := range nodes {
		if err := cw.Write([]string{
			ln.Name,
			fmt.Sprintf("%d", ln.DOCount),
			fmt.Sprintf("%d", ln.DSCount),
			fmt.Sprintf("%d", ln.RCBCount),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *csvRenderer) RenderDataObjects(objects []view.DataObject, w io.Writer) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"Name", "DACount"}); err != nil {
		return err
	}
	for _, do := range objects {
		if err := cw.Write([]string{
			do.Name,
			fmt.Sprintf("%d", do.DACount),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *csvRenderer) RenderDataAttributes(attrs []view.DataAttribute, w io.Writer) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"Name", "Type", "FC", "Value"}); err != nil {
		return err
	}
	for _, attr := range attrs {
		if err := cw.Write([]string{attr.Name, attr.Type, attr.FC, attr.Value}); err != nil {
			return err
		}
	}
	return nil
}

func (r *csvRenderer) RenderDataSet(ds *view.DataSet, w io.Writer) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"Field", "Value"})
	_ = cw.Write([]string{"Name", ds.Name})
	_ = cw.Write([]string{"Deletable", fmt.Sprintf("%v", ds.IsDeletable)})
	_ = cw.Write([]string{"MemberCount", fmt.Sprintf("%d", ds.MemberCount)})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Index", "Reference", "FC", "Value"})
	for _, m := range ds.Members {
		_ = cw.Write([]string{fmt.Sprintf("%d", m.Index), m.Ref, m.FC, m.Value})
	}
	return nil
}

func (r *csvRenderer) RenderReportControlBlock(rcb *view.ReportControlBlock, w io.Writer) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"Field", "Value"})
	_ = cw.Write([]string{"Name", rcb.Name})
	_ = cw.Write([]string{"LD", rcb.LD})
	_ = cw.Write([]string{"LN", rcb.LN})
	_ = cw.Write([]string{"Ref", rcb.Ref})
	_ = cw.Write([]string{"Buffered", fmt.Sprintf("%v", rcb.Buffered)})
	if rcb.Enabled != nil {
		_ = cw.Write([]string{"Enabled", fmt.Sprintf("%v", *rcb.Enabled)})
	}
	if rcb.RptID != "" {
		_ = cw.Write([]string{"RptID", rcb.RptID})
	}
	if rcb.DatSet != "" {
		_ = cw.Write([]string{"DatSet", rcb.DatSet})
	}
	if rcb.IntgPd != nil {
		_ = cw.Write([]string{"IntgPd", fmt.Sprintf("%d ms", *rcb.IntgPd)})
	}
	if rcb.Reserved != nil {
		_ = cw.Write([]string{"Reserved", fmt.Sprintf("%v", *rcb.Reserved)})
	}
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Trigger Options", ""})
	_ = cw.Write([]string{"DataChange", fmt.Sprintf("%v", rcb.TriggerOptions.DataChange)})
	_ = cw.Write([]string{"QualityChange", fmt.Sprintf("%v", rcb.TriggerOptions.QualityChange)})
	_ = cw.Write([]string{"DataUpdate", fmt.Sprintf("%v", rcb.TriggerOptions.DataUpdate)})
	_ = cw.Write([]string{"Periodic", fmt.Sprintf("%v", rcb.TriggerOptions.Periodic)})
	_ = cw.Write([]string{"GI", fmt.Sprintf("%v", rcb.TriggerOptions.GI)})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Optional Fields", ""})
	_ = cw.Write([]string{"SequenceNumber", fmt.Sprintf("%v", rcb.OptionalFields.SequenceNumber)})
	_ = cw.Write([]string{"TimeStamp", fmt.Sprintf("%v", rcb.OptionalFields.TimeStamp)})
	_ = cw.Write([]string{"ReasonCode", fmt.Sprintf("%v", rcb.OptionalFields.ReasonCode)})
	_ = cw.Write([]string{"DataSetName", fmt.Sprintf("%v", rcb.OptionalFields.DataSetName)})
	_ = cw.Write([]string{"DataReference", fmt.Sprintf("%v", rcb.OptionalFields.DataReference)})
	_ = cw.Write([]string{"BufferOverflow", fmt.Sprintf("%v", rcb.OptionalFields.BufferOverflow)})
	_ = cw.Write([]string{"EntryID", fmt.Sprintf("%v", rcb.OptionalFields.EntryID)})
	_ = cw.Write([]string{"ConfigRevision", fmt.Sprintf("%v", rcb.OptionalFields.ConfigRevision)})
	return nil
}
