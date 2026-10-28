// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// PageOptions configures pagination for expensive list operations.
type PageOptions struct {
	Limit  int
	Offset int
}

// PagedResult contains paginated results with metadata.
type PagedResult[T any] struct {
	Items   []T
	HasMore bool
	Total   int
}

// Explorer discovers and lists IEC 61850 device model elements.
type Explorer struct {
	conn  IEC61850Connection
	debug bool
}

func NewExplorer(conn IEC61850Connection) *Explorer {
	return &Explorer{conn: conn}
}

func (e *Explorer) WithDebug(enabled bool) *Explorer {
	e.debug = enabled
	return e
}

func (e *Explorer) ListLogicalDevices() ([]string, error) {
	if e.conn == nil {
		return nil, fmt.Errorf("explorer: client is nil")
	}
	ctx := context.Background()
	devices, err := e.conn.ListLogicalDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get logical devices: %w", err)
	}
	names := make([]string, len(devices))
	for i, d := range devices {
		names[i] = d.Name
	}
	return names, nil
}

func (e *Explorer) GetLogicalDevices() ([]domain.LogicalDevice, error) {
	if e.conn == nil {
		return nil, fmt.Errorf("explorer: client is nil")
	}
	ctx := context.Background()
	deviceNames, err := e.conn.ListLogicalDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get logical devices: %w", err)
	}

	result := make([]domain.LogicalDevice, 0, len(deviceNames))
	for _, ld := range deviceNames {
		info := domain.LogicalDevice{Name: ld.Name}
		lns, err := e.conn.ListLogicalNodes(ctx, ld.Name)
		if err == nil {
			info.LNCount = len(lns)
		}
		if dss, err := e.conn.ListDataSets(ctx, ld.Name); err == nil {
			info.DSCount = len(dss)
		}
		if reports, err := e.conn.ListReports(ctx, ld.Name); err == nil {
			for _, r := range reports {
				if strings.Contains(r, "$BR$") {
					info.BRCBCount++
				} else {
					info.URCBCount++
				}
			}
		}
		result = append(result, info)
	}
	return result, nil
}

func (e *Explorer) GetLogicalDevicesPaged(opts PageOptions) (*PagedResult[domain.LogicalDevice], error) {
	all, err := e.GetLogicalDevices()
	if err != nil {
		return nil, err
	}
	return paginate(all, opts), nil
}

func (e *Explorer) ListLogicalNodes(ldName string) ([]string, error) {
	if e.conn == nil {
		return nil, fmt.Errorf("explorer: client is nil")
	}
	ctx := context.Background()
	nodes, err := e.conn.ListLogicalNodes(ctx, ldName)
	if err != nil {
		return nil, fmt.Errorf("failed to get logical nodes: %w", err)
	}
	if err := e.checkEmptyLD(ctx, ldName, len(nodes)); err != nil {
		return nil, err
	}
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	return names, nil
}

// checkEmptyLD turns an empty logical node list into ErrNotFound when the
// LD is not in the server directory. Some servers (dscsystems) answer
// GetNameList for an unknown domain with an empty list instead of an
// error; a real LD always has LLN0. A failed probe keeps the empty list.
func (e *Explorer) checkEmptyLD(ctx context.Context, ldName string, n int) error {
	if n > 0 {
		return nil
	}
	devices, probeErr := e.conn.ListLogicalDevices(ctx)
	known := probeErr != nil // unknown directory: keep the empty list
	for _, d := range devices {
		known = known || d.Name == ldName
	}
	if known {
		return nil
	}
	return errs.Newf(errs.ErrNotFound, "logical device %q not found", ldName)
}

func (e *Explorer) GetLogicalNodes(ldName string) ([]domain.LogicalNode, error) {
	ctx := context.Background()
	nodes, err := e.conn.ListLogicalNodes(ctx, ldName)
	if err != nil {
		return nil, err
	}
	if err := e.checkEmptyLD(ctx, ldName, len(nodes)); err != nil {
		return nil, err
	}
	out := make([]domain.LogicalNode, 0, len(nodes))
	for _, n := range nodes {
		ln := domain.LogicalNode{Name: n.Name}
		if dos, err := e.conn.ListDataObjects(ctx, ldName, n.Name); err == nil {
			ln.DOCount = len(dos)
		}
		out = append(out, ln)
	}
	return out, nil
}

func (e *Explorer) ListDataObjects(ldName, lnName string) ([]string, error) {
	ctx := context.Background()
	dos, err := e.conn.ListDataObjects(ctx, ldName, lnName)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(dos))
	for i, d := range dos {
		names[i] = d.Name
	}
	return names, nil
}

func (e *Explorer) GetDataObjects(ldName, lnName string) ([]domain.DataObject, error) {
	ctx := context.Background()
	dos, err := e.conn.ListDataObjects(ctx, ldName, lnName)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DataObject, 0, len(dos))
	for _, d := range dos {
		out = append(out, domain.DataObject{Name: d.Name})
	}
	return out, nil
}

func paginate[T any](all []T, opts PageOptions) *PagedResult[T] {
	total := len(all)
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if opts.Limit > 0 && offset+opts.Limit < total {
		end = offset + opts.Limit
	}
	return &PagedResult[T]{
		Items:   all[offset:end],
		Total:   total,
		HasMore: end < total,
	}
}
