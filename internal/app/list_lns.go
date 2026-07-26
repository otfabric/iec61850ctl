// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"strings"

	"github.com/otfabric/iec61850ctl/pkg/view"
)

// ListLogicalNodesInput specifies the logical device to list nodes for.
type ListLogicalNodesInput struct {
	LD string // Logical device name (required)
}

// ListLogicalNodeNames returns logical node names within a logical device.
func (a *App) ListLogicalNodeNames(input ListLogicalNodesInput) ([]string, error) {
	return a.Explorer().ListLogicalNodes(input.LD)
}

// ListLogicalNodes returns logical nodes with summary counts.
func (a *App) ListLogicalNodes(input ListLogicalNodesInput) ([]view.LogicalNode, error) {
	names, err := a.Explorer().ListLogicalNodes(input.LD)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	conn := a.Connection()
	result := make([]view.LogicalNode, len(names))
	for i, name := range names {
		doCount := 0
		if dos, err := conn.ListDataObjects(ctx, input.LD, name); err == nil {
			doCount = len(dos)
		}
		dsCount := 0
		if dss, err := conn.ListDataSets(ctx, input.LD); err == nil {
			prefix := name + "$"
			for _, ds := range dss {
				if strings.HasPrefix(ds, prefix) || (!strings.Contains(ds, "$") && name == "LLN0") {
					dsCount++
				} else if strings.HasPrefix(ds, prefix) {
					dsCount++
				}
			}
			// Count datasets whose MMS name starts with LN$
			n := 0
			for _, ds := range dss {
				if strings.HasPrefix(ds, prefix) {
					n++
				}
			}
			dsCount = n
		}
		rcbCount := 0
		if reports, err := conn.ListReports(ctx, input.LD); err == nil {
			marker := name + "$"
			for _, r := range reports {
				if strings.HasPrefix(r, marker) {
					rcbCount++
				}
			}
		}
		result[i] = view.LogicalNode{
			Name:     name,
			DOCount:  doCount,
			DSCount:  dsCount,
			RCBCount: rcbCount,
		}
	}
	return result, nil
}
