// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

var (
	ldNameForLns    string
	lnsDetailedFlag bool
	lnsFormatFlag   string
)

var listLnsCmd = &cobra.Command{
	Use:   "lns",
	Short: "List all logical nodes within a specific logical device",
	Long: `Retrieves and displays the list of all logical nodes (LNs) within the specified logical device.
Requires the --ld flag to specify which logical device to query.`,
	RunE: runListLns,
}

func init() {
	listLnsCmd.Flags().StringVar(&ldNameForLns, "ld", "", "logical device name (required)")
	listLnsCmd.Flags().BoolVar(&lnsDetailedFlag, "detailed", false, "Show detailed LN information including DO/DataSet/Report counts")
	listLnsCmd.Flags().StringVar(&lnsFormatFlag, "format", "text", "Output format: text, json, csv, table, yaml")
	_ = listLnsCmd.MarkFlagRequired("ld")
	listCmd.AddCommand(listLnsCmd)
}

func runListLns(cmd *cobra.Command, args []string) error {
	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	a := app.New(conn)

	outputFormat, ok := formatter.ParseOutputFormat(lnsFormatFlag)
	if ok && outputFormat != formatter.OutputFormatText {
		nodes, err := a.ListLogicalNodes(app.ListLogicalNodesInput{LD: ldNameForLns})
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			if outputFormat == formatter.OutputFormatJSON {
				_, _ = io.WriteString(cmd.OutOrStdout(), "[]\n")
			}
			return nil
		}
		renderer := formatter.NewRenderer(outputFormat)
		return renderer.RenderLogicalNodes(nodes, cmd.OutOrStdout())
	}

	if !lnsDetailedFlag {
		lnNames, err := a.ListLogicalNodeNames(app.ListLogicalNodesInput{LD: ldNameForLns})
		if err != nil {
			return err
		}
		if len(lnNames) == 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No logical nodes found in '%s'\n", ldNameForLns)
			return nil
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d logical node(s) in '%s':\n", len(lnNames), ldNameForLns)
		for i, ln := range lnNames {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, ln)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nUse --detailed flag to see DO/DataSet/Report counts")
		return nil
	}

	nodes, err := a.ListLogicalNodes(app.ListLogicalNodesInput{LD: ldNameForLns})
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No logical nodes found in '%s'\n", ldNameForLns)
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d logical node(s) in '%s':\n", len(nodes), ldNameForLns)
	for i, ln := range nodes {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n  %d. %s\n", i+1, ln.Name)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "     Data Objects: %d\n", ln.DOCount)
		if ln.DSCount > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "     DataSets: %d\n", ln.DSCount)
		}
		if ln.RCBCount > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "     Reports: %d\n", ln.RCBCount)
		}
	}
	return nil
}
