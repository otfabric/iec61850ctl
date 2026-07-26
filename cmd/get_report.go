// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// It defines all commands (root, list, get, tree) and their flags, delegating
// business logic to the services package.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"

	"github.com/spf13/cobra"
)

var (
	reportLdFlag       string
	reportLnFlag       string
	reportNameFlag     string
	reportDetailedFlag bool
)

var getReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Get detailed configuration of a specific report control block",
	Long: `Retrieve and display the complete configuration of a specific report control block (RCB).

Reports can be either:
- Unbuffered Reports (URCB): Real-time reports sent immediately when data changes
- Buffered Reports (BRCB): Reports stored in a buffer and sent based on trigger conditions

The report configuration includes:
- Report ID and associated DataSet
- Enabled status and configuration revision
- Trigger options (data change, quality change, periodic, etc.)
- Optional fields included in reports
- Timing parameters (integrity period, buffer time)

Example:
  iec61850ctl get report --host <server> --ld ZS1REF620A1LD0 --ln LLN0 --report rcbMeasFltA01
  iec61850ctl get report --host <server> --ld ZS1550REX640LD0 --ln LLN0 --report rcbMeasReg01 --detailed`,
	RunE: runGetReport,
}

func init() {
	getCmd.AddCommand(getReportCmd)
	getReportCmd.Flags().StringVar(&reportLdFlag, "ld", "", "Logical device name (required)")
	getReportCmd.Flags().StringVar(&reportLnFlag, "ln", "", "Logical node name (required)")
	getReportCmd.Flags().StringVar(&reportNameFlag, "report", "", "Report name (required)")
	getReportCmd.Flags().BoolVar(&reportDetailedFlag, "detailed", false, "Fetch and print the report's dataset (members and current values)")
	_ = getReportCmd.MarkFlagRequired("ld")
	_ = getReportCmd.MarkFlagRequired("ln")
	_ = getReportCmd.MarkFlagRequired("report")
}

func runGetReport(cmd *cobra.Command, args []string) error {
	finalHost, finalPort, err := getHostPort()
	if err != nil {
		return err
	}
	printConnectionTarget(finalHost, finalPort)

	conn, err := client.NewConnection(client.ConnectionInput{
		Host:           finalHost,
		Port:           finalPort,
		ConnectTimeout: 10,
		RequestTimeout: 10,
	})
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	a := app.New(conn)

	result, err := a.GetReport(app.GetReportInput{
		LD:   reportLdFlag,
		LN:   reportLnFlag,
		Name: reportNameFlag,
	})
	if err != nil {
		return fmt.Errorf("failed to read report configuration: %w", err)
	}

	renderer := formatter.NewRenderer(formatter.OutputFormatText)
	if err := renderer.RenderReportControlBlock(&result.Report, os.Stdout); err != nil {
		return err
	}

	if reportDetailedFlag && result.DataSet != nil {
		fmt.Println()
		if err := renderer.RenderDataSet(result.DataSet, os.Stdout); err != nil {
			return err
		}
	}

	return nil
}
