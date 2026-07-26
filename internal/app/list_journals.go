// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// ListJournalsInput specifies the logical device to list journals for.
type ListJournalsInput struct {
	LD string // Logical device name (required)
}

// ListJournals returns journal metadata for a logical device.
func (a *App) ListJournals(input ListJournalsInput) ([]view.JournalInfo, error) {
	journals, err := a.JournalService().ListJournals(input.LD)
	if err != nil {
		return nil, err
	}
	return service.ProjectJournalInfos(journals), nil
}

// GetJournalEntriesInput specifies the journal and time range to query.
type GetJournalEntriesInput struct {
	DomainID    string  // MMS domain name (required)
	JournalName string  // Journal name (required)
	FromMs      uint64  // Start time in milliseconds since epoch (required)
	ToMs        *uint64 // End time in milliseconds since epoch (nil = now)
}

// GetJournalEntriesResult contains journal query results.
type GetJournalEntriesResult struct {
	Entries    []view.JournalEntry `json:"entries"`
	EntryCount int                 `json:"entry_count"`
	HasMore    bool                `json:"has_more"`
}

// GetJournalEntries returns journal entries within a time range.
func (a *App) GetJournalEntries(input GetJournalEntriesInput) (*GetJournalEntriesResult, error) {
	journalSvc := a.JournalService()

	result, err := journalSvc.GetEntries(input.DomainID, input.JournalName, input.FromMs, input.ToMs)
	if err != nil {
		return nil, err
	}

	entries := service.ProjectJournalEntries(result.Entries)
	return &GetJournalEntriesResult{
		Entries:    entries,
		EntryCount: len(entries),
		HasMore:    result.MoreFollows,
	}, nil
}
