// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/otfabric/iec61850ctl/internal/app"

	"github.com/spf13/cobra"
)

var (
	journalListLD string
)

var listJournalCmd = &cobra.Command{
	Use:   "journals",
	Short: "List journals within a logical device",
	Long: `List all journals (logs) found in a specific logical device by querying all logical nodes.

Requires --ld or --domain to specify the logical device (MMS domain).
To list available logical devices, use 'iec61850ctl list lds' (or 'list domains').

Examples:
  iec61850ctl list journals --ld ZS1REF620A1LD0
  iec61850ctl list journals --domain ZS1REF620A1LD0`,
	RunE: runListJournal,
}

func init() {
	listCmd.AddCommand(listJournalCmd)
	listJournalCmd.Flags().StringVar(&journalListLD, "ld", "", "logical device (LD) name to list journals from")
	listJournalCmd.Flags().StringVar(&journalListLD, "domain", "", "MMS domain name (alias for --ld)")
	_ = listJournalCmd.MarkFlagRequired("ld")
}

func runListJournal(cmd *cobra.Command, args []string) error {
	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	a := app.New(conn)

	// List journals in the specified LD
	journals, err := a.ListJournals(app.ListJournalsInput{LD: journalListLD})
	if err != nil {
		return err
	}

	if len(journals) == 0 {
		fmt.Printf("No journals found in logical device '%s'.\n", journalListLD)
		fmt.Println("Note: Journal availability depends on device configuration.")
		return nil
	}

	fmt.Printf("Journals in '%s' (%d):\n", journalListLD, len(journals))
	for i, j := range journals {
		fmt.Printf("  %d. %s (in %s)\n", i+1, j.Name, j.LogicalNode)
	}
	fmt.Printf("\nUse '--journal %s' with 'get journal' to read entries.\n", journals[0].Name)
	return nil
}
