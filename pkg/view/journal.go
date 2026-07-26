// SPDX-License-Identifier: MIT

package view

// JournalInfo is the projection of domain.JournalInfo for display/API output.
type JournalInfo struct {
	Name        string `json:"name"`
	LogicalNode string `json:"logical_node"`
	FullRef     string `json:"full_ref"`
}

// JournalEntry is the projection of domain.JournalEntry for display/API output.
type JournalEntry struct {
	EntryID        string            `json:"entry_id"`
	OccurrenceTime string            `json:"occurrence_time"`
	Variables      []JournalVariable `json:"variables"`
}

// JournalVariable is the projection of domain.JournalVariable for display/API output.
type JournalVariable struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}
