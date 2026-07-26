// SPDX-License-Identifier: MIT

// Package cmd provides the command-line interface for the iec61850ctl tool.
package cmd

import (
	"fmt"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/view"

	"github.com/spf13/cobra"
)

var (
	dssLdFlag       string
	dssLnFlag       string
	dssDetailedFlag bool
	dssFormatFlag   string
)

// listDssCmd represents the 'list dss' command for listing data sets within a logical node.
var listDssCmd = &cobra.Command{
	Use:   "dss",
	Short: "List all data sets in a logical node",
	Long: `List all data sets (DS) within a specific logical node.

Data sets are named collections of data attributes that are used by report 
control blocks to define what data to include in reports.

Example:
  iec61850ctl list dss --host <server> --ld MNSREF615LD0 --ln LLN0`,
	RunE: runListDss,
}

func init() {
	listCmd.AddCommand(listDssCmd)
	listDssCmd.Flags().StringVar(&dssLdFlag, "ld", "", "Logical device name (required)")
	listDssCmd.Flags().StringVar(&dssLnFlag, "ln", "", "Logical node name (required)")
	listDssCmd.Flags().BoolVar(&dssDetailedFlag, "detailed", false, "Show detailed information including DataSet members")
	listDssCmd.Flags().StringVar(&dssFormatFlag, "format", "text", "Output format: text, json")
	_ = listDssCmd.MarkFlagRequired("ld")
	_ = listDssCmd.MarkFlagRequired("ln")
}

func runListDss(cmd *cobra.Command, args []string) error {
	format, err := parseCLIFormatFlag(dssFormatFlag)
	if err != nil {
		return err
	}

	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	a := app.New(conn)

	dataSets, err := a.ListDataSetNames(app.ListDataSetsInput{LD: dssLdFlag, LN: dssLnFlag})
	if err != nil {
		return err
	}

	if format == formatter.OutputFormatJSON {
		entries := make([]view.DataSetName, 0, len(dataSets))
		for _, name := range dataSets {
			entry := view.DataSetName{Name: name}
			if dssDetailedFlag {
				dsView, err := a.GetDataSet(app.GetDataSetInput{LD: dssLdFlag, LN: dssLnFlag, Name: name})
				if err == nil && dsView != nil {
					entry.IsDeletable = dsView.IsDeletable
					entry.MemberCount = dsView.MemberCount
					entry.Members = dsView.Members
				}
			}
			entries = append(entries, entry)
		}
		return writeJSON(cmd, entries)
	}

	out := cmd.OutOrStdout()
	if len(dataSets) == 0 {
		_, _ = fmt.Fprintf(out, "No data sets found in '%s/%s'\n", dssLdFlag, dssLnFlag)
		return nil
	}

	_, _ = fmt.Fprintf(out, "Found %d data set(s) in '%s/%s':\n", len(dataSets), dssLdFlag, dssLnFlag)

	if !dssDetailedFlag {
		for i, ds := range dataSets {
			_, _ = fmt.Fprintf(out, "  %d. %s\n", i+1, ds)
		}
		_, _ = fmt.Fprintln(out, "\nUse --detailed flag to see DataSet members")
		return nil
	}

	for i, ds := range dataSets {
		_, _ = fmt.Fprintf(out, "\n  %d. %s\n", i+1, ds)

		dsView, err := a.GetDataSet(app.GetDataSetInput{LD: dssLdFlag, LN: dssLnFlag, Name: ds})
		if err != nil {
			_, _ = fmt.Fprintf(out, "     Error getting details: %v\n", err)
			continue
		}

		_, _ = fmt.Fprintf(out, "     Deletable: %t\n", dsView.IsDeletable)
		_, _ = fmt.Fprintf(out, "     Members (%d):\n", dsView.MemberCount)
		for j, member := range dsView.Members {
			_, _ = fmt.Fprintf(out, "       %d. %s\n", j+1, member.Ref)
		}
	}

	return nil
}
