// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"io"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// timeoutConn bounds every request with a deadline (when the caller's context
// has none) and cancels in-flight requests when the base context is done
// (SIGINT). Service-layer code may therefore pass context.Background() and
// still never hang on a silent peer.
type timeoutConn struct {
	service.IEC61850Connection
	base    context.Context
	timeout time.Duration
}

var _ service.IEC61850Connection = (*timeoutConn)(nil)

// bound derives the per-request context. withDeadline=false only links the
// base cancellation (long-lived operations).
func (t *timeoutConn) bound(ctx context.Context, withDeadline bool) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	if _, has := ctx.Deadline(); withDeadline && !has && t.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	if t.base != nil && t.base.Done() != nil {
		stop := context.AfterFunc(t.base, cancel)
		return ctx, func() { stop(); cancel() }
	}
	return ctx, cancel
}

func call[T any](t *timeoutConn, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := t.bound(ctx, true)
	defer cancel()
	return fn(ctx)
}

func call0(t *timeoutConn, ctx context.Context, fn func(context.Context) error) error {
	ctx, cancel := t.bound(ctx, true)
	defer cancel()
	return fn(ctx)
}

func (t *timeoutConn) ReadCtlModel(ctx context.Context, ref iec61850.Ref) (iec61850.CtlModel, error) {
	return call(t, ctx, func(c context.Context) (iec61850.CtlModel, error) { return t.IEC61850Connection.ReadCtlModel(c, ref) })
}

func (t *timeoutConn) Select(ctx context.Context, ref iec61850.Ref) (string, error) {
	return call(t, ctx, func(c context.Context) (string, error) { return t.IEC61850Connection.Select(c, ref) })
}

func (t *timeoutConn) SelectWithValue(ctx context.Context, ref iec61850.Ref, p iec61850.OperateParams) error {
	return call0(t, ctx, func(c context.Context) error { return t.IEC61850Connection.SelectWithValue(c, ref, p) })
}

func (t *timeoutConn) Operate(ctx context.Context, ref iec61850.Ref, p iec61850.OperateParams) error {
	return call0(t, ctx, func(c context.Context) error { return t.IEC61850Connection.Operate(c, ref, p) })
}

func (t *timeoutConn) Cancel(ctx context.Context, ref iec61850.Ref, p iec61850.CancelParams) error {
	return call0(t, ctx, func(c context.Context) error { return t.IEC61850Connection.Cancel(c, ref, p) })
}

func (t *timeoutConn) ReadLastApplError(ctx context.Context, ref iec61850.Ref) (*iec61850.LastApplError, error) {
	return call(t, ctx, func(c context.Context) (*iec61850.LastApplError, error) {
		return t.IEC61850Connection.ReadLastApplError(c, ref)
	})
}

func (t *timeoutConn) Read(ctx context.Context, ref iec61850.Ref) (*iec61850.Value, error) {
	return call(t, ctx, func(c context.Context) (*iec61850.Value, error) { return t.IEC61850Connection.Read(c, ref) })
}

func (t *timeoutConn) Write(ctx context.Context, ref iec61850.Ref, v *mms.Value) error {
	return call0(t, ctx, func(c context.Context) error { return t.IEC61850Connection.Write(c, ref, v) })
}

func (t *timeoutConn) ListLogicalDevices(ctx context.Context) ([]iec61850.LogicalDevice, error) {
	return call(t, ctx, t.IEC61850Connection.ListLogicalDevices)
}

func (t *timeoutConn) ListLogicalNodes(ctx context.Context, ld string) ([]iec61850.LogicalNode, error) {
	return call(t, ctx, func(c context.Context) ([]iec61850.LogicalNode, error) {
		return t.IEC61850Connection.ListLogicalNodes(c, ld)
	})
}

func (t *timeoutConn) ListDataObjects(ctx context.Context, ld, ln string) ([]iec61850.DataObject, error) {
	return call(t, ctx, func(c context.Context) ([]iec61850.DataObject, error) {
		return t.IEC61850Connection.ListDataObjects(c, ld, ln)
	})
}

func (t *timeoutConn) ListChildren(ctx context.Context, ref iec61850.Ref) ([]iec61850.BrowseNode, error) {
	return call(t, ctx, func(c context.Context) ([]iec61850.BrowseNode, error) {
		return t.IEC61850Connection.ListChildren(c, ref)
	})
}

func (t *timeoutConn) TreeWithOptions(ctx context.Context, o iec61850.TreeOptions) (*iec61850.ModelNode, error) {
	return call(t, ctx, func(c context.Context) (*iec61850.ModelNode, error) {
		return t.IEC61850Connection.TreeWithOptions(c, o)
	})
}

func (t *timeoutConn) FindPaths(ctx context.Context, q iec61850.FindQuery) ([]iec61850.Ref, error) {
	return call(t, ctx, func(c context.Context) ([]iec61850.Ref, error) { return t.IEC61850Connection.FindPaths(c, q) })
}

func (t *timeoutConn) ReadMultiple(ctx context.Context, refs []iec61850.Ref) ([]iec61850.ReadResult, error) {
	return call(t, ctx, func(c context.Context) ([]iec61850.ReadResult, error) {
		return t.IEC61850Connection.ReadMultiple(c, refs)
	})
}

func (t *timeoutConn) GetVariableType(ctx context.Context, ref iec61850.Ref) (*mms.TypeSpec, error) {
	return call(t, ctx, func(c context.Context) (*mms.TypeSpec, error) { return t.IEC61850Connection.GetVariableType(c, ref) })
}

func (t *timeoutConn) ListDataSets(ctx context.Context, ld string) ([]string, error) {
	return call(t, ctx, func(c context.Context) ([]string, error) { return t.IEC61850Connection.ListDataSets(c, ld) })
}

func (t *timeoutConn) GetDataSet(ctx context.Context, ld, ds string) (*iec61850.DataSet, error) {
	return call(t, ctx, func(c context.Context) (*iec61850.DataSet, error) { return t.IEC61850Connection.GetDataSet(c, ld, ds) })
}

func (t *timeoutConn) ReadDataSet(ctx context.Context, ld, ds string) ([]iec61850.DataSetValue, error) {
	return call(t, ctx, func(c context.Context) ([]iec61850.DataSetValue, error) {
		return t.IEC61850Connection.ReadDataSet(c, ld, ds)
	})
}

func (t *timeoutConn) ListReports(ctx context.Context, ld string) ([]string, error) {
	return call(t, ctx, func(c context.Context) ([]string, error) { return t.IEC61850Connection.ListReports(c, ld) })
}

// GetGoCB forwards to the wrapped connection when it supports GoCB access.
func (t *timeoutConn) GetGoCB(ctx context.Context, ld, id string) (*iec61850.GoCB, error) {
	g, ok := t.IEC61850Connection.(service.GoCBConnection)
	if !ok {
		return nil, errs.Newf(errs.ErrUnsupported, "connection does not support GoCB access")
	}
	return call(t, ctx, func(c context.Context) (*iec61850.GoCB, error) { return g.GetGoCB(c, ld, id) })
}

// SetGoCB forwards to the wrapped connection when it supports GoCB access.
func (t *timeoutConn) SetGoCB(ctx context.Context, ld, id string, upd iec61850.GoCBUpdate) error {
	g, ok := t.IEC61850Connection.(service.GoCBConnection)
	if !ok {
		return errs.Newf(errs.ErrUnsupported, "connection does not support GoCB access")
	}
	return call0(t, ctx, func(c context.Context) error { return g.SetGoCB(c, ld, id, upd) })
}

// GetMSVCB forwards to the wrapped connection when it supports MSVCB access.
func (t *timeoutConn) GetMSVCB(ctx context.Context, ld, id string) (*iec61850.MSVCB, error) {
	m, ok := t.IEC61850Connection.(service.MSVCBConnection)
	if !ok {
		return nil, errs.Newf(errs.ErrUnsupported, "connection does not support MSVCB access")
	}
	return call(t, ctx, func(c context.Context) (*iec61850.MSVCB, error) { return m.GetMSVCB(c, ld, id) })
}

// SetMSVCB forwards to the wrapped connection when it supports MSVCB access.
func (t *timeoutConn) SetMSVCB(ctx context.Context, ld, id string, upd iec61850.MSVCBUpdate) error {
	m, ok := t.IEC61850Connection.(service.MSVCBConnection)
	if !ok {
		return errs.Newf(errs.ErrUnsupported, "connection does not support MSVCB access")
	}
	return call0(t, ctx, func(c context.Context) error { return m.SetMSVCB(c, ld, id, upd) })
}

func (t *timeoutConn) GetReportControlBlock(ctx context.Context, ld, id string) (*iec61850.ReportControlBlock, error) {
	return call(t, ctx, func(c context.Context) (*iec61850.ReportControlBlock, error) {
		return t.IEC61850Connection.GetReportControlBlock(c, ld, id)
	})
}

func (t *timeoutConn) SetReportControlBlock(ctx context.Context, ld, id string, u iec61850.RCBUpdate) error {
	return call0(t, ctx, func(c context.Context) error { return t.IEC61850Connection.SetReportControlBlock(c, ld, id, u) })
}

func (t *timeoutConn) TriggerGI(ctx context.Context, ld, id string) error {
	return call0(t, ctx, func(c context.Context) error { return t.IEC61850Connection.TriggerGI(c, ld, id) })
}

// SubscribeReport ties the subscription lifetime to ctx, so only base
// cancellation is linked; no request deadline is applied.
func (t *timeoutConn) SubscribeReport(ctx context.Context, rptID string, o iec61850.SubscribeReportOptions) (*iec61850.ReportSubscription, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t.base != nil && t.base.Done() != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		stop := context.AfterFunc(t.base, cancel)
		sub, err := t.IEC61850Connection.SubscribeReport(ctx, rptID, o)
		if err != nil {
			stop()
			cancel()
		}
		return sub, err
	}
	return t.IEC61850Connection.SubscribeReport(ctx, rptID, o)
}

func (t *timeoutConn) ListFiles(ctx context.Context, pattern string) ([]iec61850.FileEntry, error) {
	return call(t, ctx, func(c context.Context) ([]iec61850.FileEntry, error) {
		return t.IEC61850Connection.ListFiles(c, pattern)
	})
}

// DownloadFile may legitimately exceed a single request timeout, so only base
// cancellation is linked.
func (t *timeoutConn) DownloadFile(ctx context.Context, name string, w io.Writer) (*iec61850.FileEntry, error) {
	ctx, cancel := t.bound(ctx, false)
	defer cancel()
	return t.IEC61850Connection.DownloadFile(ctx, name, w)
}

func (t *timeoutConn) GetFileAttributes(ctx context.Context, name string) (*iec61850.FileEntry, error) {
	return call(t, ctx, func(c context.Context) (*iec61850.FileEntry, error) {
		return t.IEC61850Connection.GetFileAttributes(c, name)
	})
}

func (t *timeoutConn) ListJournals(ctx context.Context, ld string) ([]string, error) {
	return call(t, ctx, func(c context.Context) ([]string, error) { return t.IEC61850Connection.ListJournals(c, ld) })
}

func (t *timeoutConn) ReadJournal(ctx context.Context, ld, j string, start, stop time.Time) (*iec61850.JournalReadResult, error) {
	return call(t, ctx, func(c context.Context) (*iec61850.JournalReadResult, error) {
		return t.IEC61850Connection.ReadJournal(c, ld, j, start, stop)
	})
}

func (t *timeoutConn) ReadJournalAfter(ctx context.Context, ld, j string, after time.Time, id []byte) (*iec61850.JournalReadResult, error) {
	return call(t, ctx, func(c context.Context) (*iec61850.JournalReadResult, error) {
		return t.IEC61850Connection.ReadJournalAfter(c, ld, j, after, id)
	})
}
