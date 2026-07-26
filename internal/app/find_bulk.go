// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// BulkFindInput contains the mapping entries to resolve.
type BulkFindInput struct {
	Mappings []service.BulkMappingEntry `json:"mappings"`
}

// BulkFindResult contains the resolved paths for all mapping entries.
type BulkFindResult struct {
	Entries   []service.BulkResultEntry `json:"entries"`
	CallCount int                       `json:"call_count"`
}

// BulkFind resolves paths for multiple mapping entries with minimal device calls.
func (a *App) BulkFind(input BulkFindInput) (*BulkFindResult, error) {
	finder := service.NewFinder(a.conn)

	result, err := finder.BulkFind(input.Mappings)
	if err != nil {
		return nil, err
	}

	return &BulkFindResult{
		Entries:   result.Entries,
		CallCount: result.CallCount,
	}, nil
}
