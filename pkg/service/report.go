// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"strings"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// ReportService discovers and reads report control blocks.
type ReportService struct {
	conn IEC61850Connection
}

func NewReportService(conn IEC61850Connection) *ReportService {
	return &ReportService{conn: conn}
}

func (r *ReportService) ListUnbufferedReports(ldName, lnName string) ([]string, error) {
	return r.listByFC(ldName, lnName, "$RP$")
}

func (r *ReportService) ListBufferedReports(ldName, lnName string) ([]string, error) {
	return r.listByFC(ldName, lnName, "$BR$")
}

func (r *ReportService) listByFC(ldName, lnName, marker string) ([]string, error) {
	ctx := context.Background()
	all, err := r.conn.ListReports(ctx, ldName)
	if err != nil {
		return nil, err
	}
	prefix := lnName + marker
	var names []string
	for _, item := range all {
		if strings.HasPrefix(item, prefix) {
			names = append(names, strings.TrimPrefix(item, prefix))
		}
	}
	return names, nil
}

func (r *ReportService) GetAllReportsPaged(opts PageOptions) (*PagedResult[domain.ReportControlBlockRef], error) {
	all, err := r.GetAllReports()
	if err != nil {
		return nil, err
	}
	return paginate(all, opts), nil
}

func (r *ReportService) GetAllReports() ([]domain.ReportControlBlockRef, error) {
	ctx := context.Background()
	lds, err := r.conn.ListLogicalDevices(ctx)
	if err != nil {
		return nil, err
	}
	var entries []domain.ReportControlBlockRef
	for _, ld := range lds {
		reports, err := r.conn.ListReports(ctx, ld.Name)
		if err != nil {
			continue
		}
		for _, item := range reports {
			// item like LLN0$BR$rcb01
			parts := strings.Split(item, "$")
			if len(parts) < 3 {
				continue
			}
			ln, fc, name := parts[0], parts[1], parts[2]
			buffered := fc == "BR"
			entries = append(entries, domain.ReportControlBlockRef{
				LD: ld.Name, LN: ln, Name: name, Buffered: buffered,
				Ref: fmt.Sprintf("%s/%s.%s", ld.Name, ln, name),
			})
		}
	}
	return entries, nil
}

func (r *ReportService) GetReportDetails(ldName, lnName, reportName string, buffered bool) (*domain.ReportControlBlock, error) {
	ctx := context.Background()
	fc := "RP"
	if buffered {
		fc = "BR"
	}
	itemID := fmt.Sprintf("%s$%s$%s", lnName, fc, reportName)
	rcb, err := r.conn.GetReportControlBlock(ctx, ldName, itemID)
	if err != nil {
		return nil, err
	}
	return mapRCB(ldName, lnName, reportName, buffered, rcb), nil
}

// ResolveReportDetails tries BR then RP when the RCB type is unknown.
func (r *ReportService) ResolveReportDetails(ldName, lnName, reportName string) (*domain.ReportControlBlock, error) {
	rcb, err := r.GetReportDetails(ldName, lnName, reportName, true)
	if err == nil {
		return rcb, nil
	}
	return r.GetReportDetails(ldName, lnName, reportName, false)
}

func mapRCB(ld, ln, name string, buffered bool, rcb *iec61850.ReportControlBlock) *domain.ReportControlBlock {
	if rcb == nil {
		return nil
	}
	out := &domain.ReportControlBlock{
		Name:     name,
		LD:       ld,
		LN:       ln,
		Buffered: buffered,
		Ref:      fmt.Sprintf("%s/%s.%s", ld, ln, name),
		RptID:    rcb.RptID,
		DatSet:   rcb.DatSet,
	}
	ena := rcb.RptEna
	out.Enabled = &ena
	cr := rcb.ConfRev
	out.ConfRev = &cr
	intg := rcb.IntgPd
	out.IntgPd = &intg
	buf := rcb.BufTm
	out.BufTm = &buf
	resv := rcb.Resv
	out.Reserved = &resv
	sq := uint16(rcb.SqNum)
	out.SqNum = &sq
	out.TriggerOptions = domain.TriggerOptions{
		DataChange:    rcb.TrgOps.Has(iec61850.TrgOpDataChanged),
		QualityChange: rcb.TrgOps.Has(iec61850.TrgOpQualityChanged),
		DataUpdate:    rcb.TrgOps.Has(iec61850.TrgOpDataUpdate),
		Periodic:      rcb.TrgOps.Has(iec61850.TrgOpIntegrity),
		GI:            rcb.TrgOps.Has(iec61850.TrgOpGI),
	}
	out.OptionalFields = domain.OptionalFields{
		SequenceNumber: rcb.OptFlds.Has(iec61850.OptFldSeqNum),
		TimeStamp:      rcb.OptFlds.Has(iec61850.OptFldTimeStamp),
		ReasonCode:     rcb.OptFlds.Has(iec61850.OptFldReasonCode),
		DataSetName:    rcb.OptFlds.Has(iec61850.OptFldDataSet),
		DataReference:  rcb.OptFlds.Has(iec61850.OptFldDataRef),
		BufferOverflow: rcb.OptFlds.Has(iec61850.OptFldBufOvfl),
		EntryID:        rcb.OptFlds.Has(iec61850.OptFldEntryID),
		ConfigRevision: rcb.OptFlds.Has(iec61850.OptFldConfRev),
	}
	if buffered {
		pb := rcb.PurgeBuf
		out.PurgeBuf = &pb
		if len(rcb.EntryID) > 0 {
			out.EntryID = append([]byte(nil), rcb.EntryID...)
		}
		if rcb.ResvTms != 0 {
			rt := rcb.ResvTms
			out.ResvTms = &rt
		}
	}
	return out
}
