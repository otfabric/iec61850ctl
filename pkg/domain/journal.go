// SPDX-License-Identifier: MIT

package domain

// JournalInfo describes a journal (log) available on a logical node.
type JournalInfo struct {
	Name        string
	LogicalNode string
	FullRef     string
}

// JournalEntry represents a single entry in an IEC 61850 journal/log.
type JournalEntry struct {
	EntryID        string
	OccurrenceTime string
	Variables      []JournalVariable
}

// JournalVariable represents a single variable within a journal entry.
type JournalVariable struct {
	Tag   string
	Value string
}

// JournalQueryResult wraps the result of a journal query.
type JournalQueryResult struct {
	Entries     []JournalEntry
	MoreFollows bool
}
