// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// get_ds.go implements 'get ds' (alias: dataset) to retrieve a single data set by name.
package cmd

import (
	"fmt"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"

	"github.com/spf13/cobra"
)

var (
	getDsLdFlag       string
	getDsLnFlag       string
	getDsNameFlag     string
	getDsDetailedFlag bool
	getDsFormatFlag   string
)

var getDsCmd = &cobra.Command{
	Use:     "ds",
	Aliases: []string{"dataset"},
	Short:   "Get a specific data set by name",
	Long: `Retrieve and display a data set (DS) by name within a logical device and logical node.
Use --detailed to also read and display current values for each member.

Example:
  iec61850ctl get ds --host <server> --ld ZS1550REX640LD0 --ln LLN0 --name MeasReg
  iec61850ctl get dataset --host <server> --ld ZS1550REX640LD0 --ln LLN0 --name MeasReg --detailed`,
	RunE: runGetDs,
}

func init() {
	getCmd.AddCommand(getDsCmd)
	getDsCmd.Flags().StringVar(&getDsLdFlag, "ld", "", "Logical device name (required)")
	getDsCmd.Flags().StringVar(&getDsLnFlag, "ln", "", "Logical node name (required)")
	getDsCmd.Flags().StringVar(&getDsNameFlag, "name", "", "Data set name (required)")
	getDsCmd.Flags().BoolVar(&getDsDetailedFlag, "detailed", false, "Read and show current values for each member")
	getDsCmd.Flags().StringVar(&getDsFormatFlag, "format", "text", "Output format: text, json")
	_ = getDsCmd.MarkFlagRequired("ld")
	_ = getDsCmd.MarkFlagRequired("ln")
	_ = getDsCmd.MarkFlagRequired("name")
}

func runGetDs(cmd *cobra.Command, args []string) error {
	format, err := parseCLIFormatFlag(getDsFormatFlag)
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

	dsView, err := a.GetDataSetWithValues(app.GetDataSetWithValuesInput{
		LD:         getDsLdFlag,
		LN:         getDsLnFlag,
		Name:       getDsNameFlag,
		ReadValues: getDsDetailedFlag,
	})
	if err != nil {
		return fmt.Errorf("failed to get data set: %w", err)
	}

	if format == formatter.OutputFormatJSON {
		return writeJSON(cmd, dsView)
	}

	out := cmd.OutOrStdout()
	dsRef := fmt.Sprintf("%s/%s.%s", getDsLdFlag, getDsLnFlag, getDsNameFlag)
	_, _ = fmt.Fprintf(out, "Data set: %s\n", dsRef)
	_, _ = fmt.Fprintf(out, "Deletable: %t\n", dsView.IsDeletable)
	_, _ = fmt.Fprintf(out, "Members (%d):\n", dsView.MemberCount)

	for i, member := range dsView.Members {
		if getDsDetailedFlag && member.Value != "" {
			_, _ = fmt.Fprintf(out, "  %d. %s = %s\n", i+1, member.Ref, member.Value)
		} else {
			_, _ = fmt.Fprintf(out, "  %d. %s\n", i+1, member.Ref)
		}
	}

	return nil
}
