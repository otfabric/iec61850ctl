// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"time"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/domain"

	"github.com/spf13/cobra"
)

var (
	journalGetDomain string
	journalGetItem   string
	journalGetFrom   string
	journalGetTo     string
)

var getJournalCmd = &cobra.Command{
	Use:   "journal",
	Short: "Read journal entries by time range or from a start time",
	Long: `Read MMS journal entries. Times use the same format as elsewhere in the tool:
  - RFC3339: 2024-10-30T12:00:00Z or 2024-10-30T12:00:00.000Z
  - Space-separated UTC: 2024-10-30 12:00:00 or 2024-10-30 12:00:00.000
  - Raw milliseconds since Unix epoch (e.g. 1730289600000)
With --to: entries in [from, to]. Without --to: entries after --from (ReadJournalStartAfter).`,
	RunE: runGetJournal,
}

func init() {
	getJournalCmd.Flags().StringVar(&journalGetDomain, "domain", "", "MMS domain (required); use 'list journals' or try an LD name from 'list lds'")
	getJournalCmd.Flags().StringVar(&journalGetItem, "journal", "", "journal/log name within the domain (required)")
	getJournalCmd.Flags().StringVar(&journalGetFrom, "from", "", "start time (required): RFC3339, 2006-01-02 15:04:05.000 UTC, or ms since epoch")
	getJournalCmd.Flags().StringVar(&journalGetTo, "to", "", "end time (optional): same format as --from; if omitted, entries after --from are returned")
	_ = getJournalCmd.MarkFlagRequired("domain")
	_ = getJournalCmd.MarkFlagRequired("journal")
	_ = getJournalCmd.MarkFlagRequired("from")
	getCmd.AddCommand(getJournalCmd)
}

func runGetJournal(cmd *cobra.Command, args []string) error {
	fromMs, err := domain.ParseTimeToUnixMs(journalGetFrom)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}

	var toMs *uint64
	if journalGetTo != "" {
		t, err := domain.ParseTimeToUnixMs(journalGetTo)
		if err != nil {
			return fmt.Errorf("--to: %w", err)
		}
		toMs = &t
	}

	session, err := openClientSession(cmd, clientSessionOptions{RequestTimeout: 30 * time.Second})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	a := app.New(conn)
	result, err := a.GetJournalEntries(app.GetJournalEntriesInput{
		DomainID:    journalGetDomain,
		JournalName: journalGetItem,
		FromMs:      fromMs,
		ToMs:        toMs,
	})
	if err != nil {
		return fmt.Errorf("get journal: %w", err)
	}

	for i, e := range result.Entries {
		fmt.Printf("--- Entry %d ---\n", i+1)
		fmt.Printf("  EntryID:        %s\n", e.EntryID)
		fmt.Printf("  OccurrenceTime: %s\n", e.OccurrenceTime)
		if len(e.Variables) > 0 {
			fmt.Println("  Variables:")
			for _, v := range e.Variables {
				fmt.Printf("    %s: %s\n", v.Tag, v.Value)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Total entries: %d", result.EntryCount)
	if result.HasMore {
		fmt.Print(" (more available)")
	}
	fmt.Println()
	return nil
}
