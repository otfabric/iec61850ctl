// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/errs"

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
		return errs.Mark(fmt.Errorf("--from: %w", err), errs.ErrInvalidInput)
	}

	// MMS BinaryTime cannot encode instants before 1984-01-01; "--from 0"
	// means "from the beginning", so clamp instead of failing on the wire.
	fromMs = max(fromMs, binaryTimeEpochMs)

	var toMs *uint64
	if journalGetTo != "" {
		t, err := domain.ParseTimeToUnixMs(journalGetTo)
		if err != nil {
			return errs.Mark(fmt.Errorf("--to: %w", err), errs.ErrInvalidInput)
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
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "--- Entry %d ---\n", i+1)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  EntryID:        %s\n", e.EntryID)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  OccurrenceTime: %s\n", e.OccurrenceTime)
		if len(e.Variables) > 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  Variables:")
			for _, v := range e.Variables {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s\n", v.Tag, v.Value)
			}
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Total entries: %d", result.EntryCount)
	if result.HasMore {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), " (more available)")
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	return nil
}

// binaryTimeEpochMs is 1984-01-01T00:00:00Z, the MMS BinaryTime epoch.
const binaryTimeEpochMs = uint64(441763200000)
