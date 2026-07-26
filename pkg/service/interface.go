// SPDX-License-Identifier: MIT

// Package service provides business logic for IEC 61850 device exploration and data reading.
package service

import (
	"context"
	"io"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
)

// IEC61850Connection is the client boundary used by services and the app layer.
type IEC61850Connection interface {
	ListLogicalDevices(ctx context.Context) ([]iec61850.LogicalDevice, error)
	ListLogicalNodes(ctx context.Context, ld string) ([]iec61850.LogicalNode, error)
	ListDataObjects(ctx context.Context, ld, ln string) ([]iec61850.DataObject, error)
	ListChildren(ctx context.Context, ref iec61850.Ref) ([]iec61850.BrowseNode, error)
	TreeWithOptions(ctx context.Context, opts iec61850.TreeOptions) (*iec61850.ModelNode, error)
	FindPaths(ctx context.Context, query iec61850.FindQuery) ([]iec61850.Ref, error)

	Read(ctx context.Context, ref iec61850.Ref) (*iec61850.Value, error)
	ReadMultiple(ctx context.Context, refs []iec61850.Ref) ([]iec61850.ReadResult, error)
	GetVariableType(ctx context.Context, ref iec61850.Ref) (*mms.TypeSpec, error)

	ListDataSets(ctx context.Context, ld string) ([]string, error)
	GetDataSet(ctx context.Context, ld, dsName string) (*iec61850.DataSet, error)
	ReadDataSet(ctx context.Context, ld, dsName string) ([]iec61850.DataSetValue, error)

	ListReports(ctx context.Context, ld string) ([]string, error)
	GetReportControlBlock(ctx context.Context, ld, rcbItemID string) (*iec61850.ReportControlBlock, error)
	SetReportControlBlock(ctx context.Context, ld, rcbItemID string, update iec61850.RCBUpdate) error
	TriggerGI(ctx context.Context, ld, rcbItemID string) error
	SubscribeReport(ctx context.Context, rptID string, opts iec61850.SubscribeReportOptions) (*iec61850.ReportSubscription, error)

	ListFiles(ctx context.Context, pattern string) ([]iec61850.FileEntry, error)
	DownloadFile(ctx context.Context, fileName string, w io.Writer) (*iec61850.FileEntry, error)
	GetFileAttributes(ctx context.Context, fileName string) (*iec61850.FileEntry, error)

	ListJournals(ctx context.Context, ld string) ([]string, error)
	ReadJournal(ctx context.Context, ld, journal string, start, stop time.Time) (*iec61850.JournalReadResult, error)
	ReadJournalAfter(ctx context.Context, ld, journal string, afterTime time.Time, afterID []byte) (*iec61850.JournalReadResult, error)

	Close(ctx context.Context) error
	Abort(ctx context.Context) error
}

// ClientAdapter wraps *iec61850.Client so it satisfies IEC61850Connection.
// Prefer passing *iec61850.Client through NewClientAdapter at the CLI boundary.
type ClientAdapter struct {
	*iec61850.Client
}

// NewClientAdapter returns an IEC61850Connection backed by c.
func NewClientAdapter(c *iec61850.Client) IEC61850Connection {
	return &ClientAdapter{Client: c}
}

var _ IEC61850Connection = (*ClientAdapter)(nil)
