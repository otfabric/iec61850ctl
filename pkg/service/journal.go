// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// Journal reads MMS journal entries.
type Journal struct {
	conn IEC61850Connection
}

func NewJournal(conn IEC61850Connection) *Journal {
	return &Journal{conn: conn}
}

// ListJournals returns journal metadata for a logical device.
func (j *Journal) ListJournals(ldName string) ([]domain.JournalInfo, error) {
	names, err := j.conn.ListJournals(context.Background(), ldName)
	if err != nil {
		return nil, err
	}
	out := make([]domain.JournalInfo, 0, len(names))
	for _, name := range names {
		info := domain.JournalInfo{
			Name:    name,
			FullRef: ldName + "/" + name,
		}
		// Common form: LLN0$EventLog or LLN0.EventLog
		if i := strings.IndexAny(name, "$."); i >= 0 {
			info.LogicalNode = name[:i]
			info.Name = name[i+1:]
			info.FullRef = ldName + "/" + info.LogicalNode + "." + info.Name
		}
		out = append(out, info)
	}
	return out, nil
}

// ListJournalNames returns raw journal names for a logical device.
func (j *Journal) ListJournalNames(ldName string) ([]string, error) {
	return j.conn.ListJournals(context.Background(), ldName)
}

func (j *Journal) ReadTimeRange(ldName, journalName string, fromMs, toMs uint64) ([]domain.JournalEntry, bool, error) {
	ctx := context.Background()
	start := time.UnixMilli(int64(fromMs))
	stop := time.UnixMilli(int64(toMs))
	res, err := j.conn.ReadJournal(ctx, ldName, journalName, start, stop)
	if err != nil {
		return nil, false, err
	}
	return mapJournalResult(res)
}

func (j *Journal) ReadStartAfter(ldName, journalName string, fromMs uint64, entryID []byte) ([]domain.JournalEntry, bool, error) {
	ctx := context.Background()
	after := time.UnixMilli(int64(fromMs))
	res, err := j.conn.ReadJournalAfter(ctx, ldName, journalName, after, entryID)
	if err != nil {
		return nil, false, err
	}
	return mapJournalResult(res)
}

// GetEntries reads journal entries by time range or start-after with pagination.
// If endMs is nil, uses ReadJournalAfter until exhausted.
func (j *Journal) GetEntries(domainID, journalName string, startMs uint64, endMs *uint64) (*domain.JournalQueryResult, error) {
	var all []domain.JournalEntry
	more := false

	if endMs != nil {
		from := startMs
		to := *endMs
		for {
			entries, moreFollows, err := j.ReadTimeRange(domainID, journalName, from, to)
			if err != nil {
				return nil, err
			}
			all = append(all, entries...)
			more = moreFollows
			if !moreFollows || len(entries) == 0 {
				break
			}
			// Advance cursor by last occurrence time (+1ms).
			last := entries[len(entries)-1]
			ms, parseErr := domain.ParseTimeToUnixMs(last.OccurrenceTime)
			if parseErr != nil {
				return &domain.JournalQueryResult{Entries: all, MoreFollows: true}, fmt.Errorf("advance journal cursor: %w", parseErr)
			}
			from = ms + 1
		}
	} else {
		from := startMs
		var afterID []byte
		for {
			entries, moreFollows, err := j.ReadStartAfter(domainID, journalName, from, afterID)
			if err != nil {
				return nil, err
			}
			all = append(all, entries...)
			more = moreFollows
			if !moreFollows || len(entries) == 0 {
				break
			}
			last := entries[len(entries)-1]
			id, decodeErr := hex.DecodeString(last.EntryID)
			if decodeErr != nil {
				return &domain.JournalQueryResult{Entries: all, MoreFollows: true}, fmt.Errorf("decode journal entry id: %w", decodeErr)
			}
			afterID = id
			if ms, parseErr := domain.ParseTimeToUnixMs(last.OccurrenceTime); parseErr == nil {
				from = ms
			}
		}
	}

	return &domain.JournalQueryResult{Entries: all, MoreFollows: more}, nil
}

func mapJournalResult(res *iec61850.JournalReadResult) ([]domain.JournalEntry, bool, error) {
	if res == nil {
		return nil, false, nil
	}
	out := make([]domain.JournalEntry, 0, len(res.Entries))
	for _, e := range res.Entries {
		je := domain.JournalEntry{
			EntryID:        hex.EncodeToString(e.EntryID),
			OccurrenceTime: e.OccurrenceTime.UTC().Format(domain.TimeFormatForJournal),
		}
		for _, v := range e.Variables {
			valStr := ""
			if v.Value != nil && v.Value.MMS() != nil {
				valStr = v.Value.MMS().String()
			}
			je.Variables = append(je.Variables, domain.JournalVariable{
				Tag:   v.Tag,
				Value: valStr,
			})
		}
		out = append(out, je)
	}
	return out, res.MoreFollows, nil
}
