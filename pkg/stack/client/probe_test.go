// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"io"
	"sync"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
)

// probeConn implements service.IEC61850Connection and records, per method,
// whether the context it received carried a deadline. Generated from
// pkg/service/interface.go; keep in sync.
type probeConn struct {
	mu       sync.Mutex
	deadline map[string]bool
	block    bool // block until ctx is done
}

func (p *probeConn) rec(ctx context.Context, name string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, has := ctx.Deadline()
	p.mu.Lock()
	if p.deadline == nil {
		p.deadline = map[string]bool{}
	}
	p.deadline[name] = has
	block := p.block
	p.mu.Unlock()
	if block {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (p *probeConn) ReadCtlModel(ctx context.Context, _ iec61850.Ref) (iec61850.CtlModel, error) {
	var zero iec61850.CtlModel
	return zero, p.rec(ctx, "ReadCtlModel")
}

func (p *probeConn) Select(ctx context.Context, _ iec61850.Ref) (string, error) {
	var zero string
	return zero, p.rec(ctx, "Select")
}

func (p *probeConn) SelectWithValue(ctx context.Context, _ iec61850.Ref, _ iec61850.OperateParams) error {
	return p.rec(ctx, "SelectWithValue")
}

func (p *probeConn) Operate(ctx context.Context, _ iec61850.Ref, _ iec61850.OperateParams) error {
	return p.rec(ctx, "Operate")
}

func (p *probeConn) Cancel(ctx context.Context, _ iec61850.Ref, _ iec61850.CancelParams) error {
	return p.rec(ctx, "Cancel")
}

func (p *probeConn) ReadLastApplError(ctx context.Context, _ iec61850.Ref) (*iec61850.LastApplError, error) {
	var zero *iec61850.LastApplError
	return zero, p.rec(ctx, "ReadLastApplError")
}

func (p *probeConn) Read(ctx context.Context, _ iec61850.Ref) (*iec61850.Value, error) {
	var zero *iec61850.Value
	return zero, p.rec(ctx, "Read")
}

func (p *probeConn) Write(ctx context.Context, _ iec61850.Ref, _ *mms.Value) error {
	return p.rec(ctx, "Write")
}

func (p *probeConn) ListLogicalDevices(ctx context.Context) ([]iec61850.LogicalDevice, error) {
	var zero []iec61850.LogicalDevice
	return zero, p.rec(ctx, "ListLogicalDevices")
}

func (p *probeConn) ListLogicalNodes(ctx context.Context, _ string) ([]iec61850.LogicalNode, error) {
	var zero []iec61850.LogicalNode
	return zero, p.rec(ctx, "ListLogicalNodes")
}

func (p *probeConn) ListDataObjects(ctx context.Context, _, _ string) ([]iec61850.DataObject, error) {
	var zero []iec61850.DataObject
	return zero, p.rec(ctx, "ListDataObjects")
}

func (p *probeConn) ListChildren(ctx context.Context, _ iec61850.Ref) ([]iec61850.BrowseNode, error) {
	var zero []iec61850.BrowseNode
	return zero, p.rec(ctx, "ListChildren")
}

func (p *probeConn) TreeWithOptions(ctx context.Context, _ iec61850.TreeOptions) (*iec61850.ModelNode, error) {
	var zero *iec61850.ModelNode
	return zero, p.rec(ctx, "TreeWithOptions")
}

func (p *probeConn) FindPaths(ctx context.Context, _ iec61850.FindQuery) ([]iec61850.Ref, error) {
	var zero []iec61850.Ref
	return zero, p.rec(ctx, "FindPaths")
}

func (p *probeConn) ReadMultiple(ctx context.Context, _ []iec61850.Ref) ([]iec61850.ReadResult, error) {
	var zero []iec61850.ReadResult
	return zero, p.rec(ctx, "ReadMultiple")
}

func (p *probeConn) GetVariableType(ctx context.Context, _ iec61850.Ref) (*mms.TypeSpec, error) {
	var zero *mms.TypeSpec
	return zero, p.rec(ctx, "GetVariableType")
}

func (p *probeConn) ListDataSets(ctx context.Context, _ string) ([]string, error) {
	var zero []string
	return zero, p.rec(ctx, "ListDataSets")
}

func (p *probeConn) GetDataSet(ctx context.Context, _, _ string) (*iec61850.DataSet, error) {
	var zero *iec61850.DataSet
	return zero, p.rec(ctx, "GetDataSet")
}

func (p *probeConn) ReadDataSet(ctx context.Context, _, _ string) ([]iec61850.DataSetValue, error) {
	var zero []iec61850.DataSetValue
	return zero, p.rec(ctx, "ReadDataSet")
}

func (p *probeConn) ListReports(ctx context.Context, _ string) ([]string, error) {
	var zero []string
	return zero, p.rec(ctx, "ListReports")
}

func (p *probeConn) GetReportControlBlock(ctx context.Context, _, _ string) (*iec61850.ReportControlBlock, error) {
	var zero *iec61850.ReportControlBlock
	return zero, p.rec(ctx, "GetReportControlBlock")
}

func (p *probeConn) SetReportControlBlock(ctx context.Context, _, _ string, _ iec61850.RCBUpdate) error {
	return p.rec(ctx, "SetReportControlBlock")
}

func (p *probeConn) TriggerGI(ctx context.Context, _, _ string) error {
	return p.rec(ctx, "TriggerGI")
}

func (p *probeConn) SubscribeReport(ctx context.Context, _ string, _ iec61850.SubscribeReportOptions) (*iec61850.ReportSubscription, error) {
	var zero *iec61850.ReportSubscription
	return zero, p.rec(ctx, "SubscribeReport")
}

func (p *probeConn) ListFiles(ctx context.Context, _ string) ([]iec61850.FileEntry, error) {
	var zero []iec61850.FileEntry
	return zero, p.rec(ctx, "ListFiles")
}

func (p *probeConn) DownloadFile(ctx context.Context, _ string, _ io.Writer) (*iec61850.FileEntry, error) {
	var zero *iec61850.FileEntry
	return zero, p.rec(ctx, "DownloadFile")
}

func (p *probeConn) GetFileAttributes(ctx context.Context, _ string) (*iec61850.FileEntry, error) {
	var zero *iec61850.FileEntry
	return zero, p.rec(ctx, "GetFileAttributes")
}

func (p *probeConn) ListJournals(ctx context.Context, _ string) ([]string, error) {
	var zero []string
	return zero, p.rec(ctx, "ListJournals")
}

func (p *probeConn) ReadJournal(ctx context.Context, _, _ string, _, _ time.Time) (*iec61850.JournalReadResult, error) {
	var zero *iec61850.JournalReadResult
	return zero, p.rec(ctx, "ReadJournal")
}

func (p *probeConn) ReadJournalAfter(ctx context.Context, _, _ string, _ time.Time, _ []byte) (*iec61850.JournalReadResult, error) {
	var zero *iec61850.JournalReadResult
	return zero, p.rec(ctx, "ReadJournalAfter")
}

func (p *probeConn) Close(ctx context.Context) error {
	return p.rec(ctx, "Close")
}

func (p *probeConn) Abort(ctx context.Context) error {
	return p.rec(ctx, "Abort")
}

var probeMethods = []string{"ReadCtlModel", "Select", "SelectWithValue", "Operate", "Cancel", "ReadLastApplError", "Read", "Write", "ListLogicalDevices", "ListLogicalNodes", "ListDataObjects", "ListChildren", "TreeWithOptions", "FindPaths", "ReadMultiple", "GetVariableType", "ListDataSets", "GetDataSet", "ReadDataSet", "ListReports", "GetReportControlBlock", "SetReportControlBlock", "TriggerGI", "SubscribeReport", "ListFiles", "DownloadFile", "GetFileAttributes", "ListJournals", "ReadJournal", "ReadJournalAfter", "Close", "Abort"}
