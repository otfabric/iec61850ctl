// SPDX-License-Identifier: MIT

// Package cmd provides the command-line interface for the iec61850ctl tool.
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
	reportsLdFlag       string
	reportsLnFlag       string
	reportsDetailedFlag bool
	reportsAllFlag      bool
)

// listReportsCmd represents the 'list reports' command for listing report control blocks.
var listReportsCmd = &cobra.Command{
	Use:   "reports",
	Short: "List all report control blocks in a logical node or across the server",
	Long: `List report control blocks (RCB) in a specific logical node, or use --all to list
reports in every LD/LN on the server.

Reports are categorized as:
- Unbuffered Reports (URCB): Real-time reports sent immediately when data changes
- Buffered Reports (BRCB): Reports stored in a buffer and sent based on trigger conditions

--all is mutually exclusive with --ld and --ln. --detailed can be used with either mode.

Examples:
  iec61850ctl list reports --host <server> --ld MNSREF615LD0 --ln LLN0
  iec61850ctl list reports --host <server> --all
  iec61850ctl list reports --host <server> --all --detailed`,
	RunE: runListReports,
}

func init() {
	listCmd.AddCommand(listReportsCmd)
	listReportsCmd.Flags().StringVar(&reportsLdFlag, "ld", "", "Logical device name (required unless --all)")
	listReportsCmd.Flags().StringVar(&reportsLnFlag, "ln", "", "Logical node name (required unless --all)")
	listReportsCmd.Flags().BoolVar(&reportsDetailedFlag, "detailed", false, "Show detailed report configuration")
	listReportsCmd.Flags().BoolVar(&reportsAllFlag, "all", false, "List reports in all logical devices and logical nodes (mutually exclusive with --ld and --ln)")
}

func runListReports(cmd *cobra.Command, args []string) error {
	if reportsAllFlag {
		if reportsLdFlag != "" || reportsLnFlag != "" {
			return fmt.Errorf("--all cannot be combined with --ld or --ln")
		}
	} else {
		if reportsLdFlag == "" || reportsLnFlag == "" {
			return fmt.Errorf("--ld and --ln are required (or use --all to list reports across the server)")
		}
	}

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

	if reportsAllFlag {
		refs, err := a.ListAllReports()
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			fmt.Println("No reports found on the server.")
			return nil
		}
		fmt.Printf("Found %d report(s) across all logical devices:\n\n", len(refs))

		curLD, curLN := "", ""
		for _, ref := range refs {
			if ref.LD != curLD || ref.LN != curLN {
				curLD, curLN = ref.LD, ref.LN
				if reportsDetailedFlag {
					fmt.Printf("\n--- %s/%s ---\n\n", ref.LD, ref.LN)
				} else {
					fmt.Printf("%s/%s:\n", ref.LD, ref.LN)
				}
			}
			kind := "URCB"
			if ref.Buffered {
				kind = "BRCB"
			}
			fmt.Printf("  - %s (%s)\n", ref.Name, kind)

			if reportsDetailedFlag {
				result, err := a.GetReport(app.GetReportInput{LD: ref.LD, LN: ref.LN, Name: ref.Name})
				if err != nil {
					fmt.Printf("     Error reading configuration: %v\n\n", err)
				} else {
					renderer := formatter.NewRenderer(formatter.OutputFormatText)
					_ = renderer.RenderReportControlBlock(&result.Report, os.Stdout)
					fmt.Println()
				}
			}
		}
		if !reportsDetailedFlag {
			fmt.Println("\nUse --detailed flag to see report configuration")
		}
		return nil
	}

	unbuffered, buffered, err := a.ListReportNames(app.ListReportsInput{
		LD: reportsLdFlag,
		LN: reportsLnFlag,
	})
	if err != nil {
		return err
	}

	total := len(unbuffered) + len(buffered)
	if total == 0 {
		fmt.Printf("No reports found in '%s/%s'\n", reportsLdFlag, reportsLnFlag)
		return nil
	}
	fmt.Printf("Found %d report(s) in '%s/%s':\n\n", total, reportsLdFlag, reportsLnFlag)

	if len(unbuffered) > 0 {
		fmt.Printf("Unbuffered Reports (%d):\n", len(unbuffered))
		for i, name := range unbuffered {
			fmt.Printf("  %d. %s\n", i+1, name)
			if reportsDetailedFlag {
				result, err := a.GetReport(app.GetReportInput{LD: reportsLdFlag, LN: reportsLnFlag, Name: name})
				if err != nil {
					fmt.Printf("     Error reading configuration: %v\n\n", err)
				} else {
					renderer := formatter.NewRenderer(formatter.OutputFormatText)
					_ = renderer.RenderReportControlBlock(&result.Report, os.Stdout)
					fmt.Println()
				}
			}
		}
		if !reportsDetailedFlag {
			fmt.Println()
		}
	}
	if len(buffered) > 0 {
		fmt.Printf("Buffered Reports (%d):\n", len(buffered))
		for i, name := range buffered {
			fmt.Printf("  %d. %s\n", i+1, name)
			if reportsDetailedFlag {
				result, err := a.GetReport(app.GetReportInput{LD: reportsLdFlag, LN: reportsLnFlag, Name: name})
				if err != nil {
					fmt.Printf("     Error reading configuration: %v\n\n", err)
				} else {
					renderer := formatter.NewRenderer(formatter.OutputFormatText)
					_ = renderer.RenderReportControlBlock(&result.Report, os.Stdout)
					fmt.Println()
				}
			}
		}
	}
	if !reportsDetailedFlag {
		fmt.Println("Use --detailed flag to see report configuration")
	}
	return nil
}
