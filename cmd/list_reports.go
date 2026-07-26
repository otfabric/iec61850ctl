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
	reportsLdFlag       string
	reportsLnFlag       string
	reportsDetailedFlag bool
	reportsAllFlag      bool
	reportsFormatFlag   string
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
	listReportsCmd.Flags().StringVar(&reportsFormatFlag, "format", "text", "Output format: text, json")
}

func runListReports(cmd *cobra.Command, args []string) error {
	format, err := parseCLIFormatFlag(reportsFormatFlag)
	if err != nil {
		return err
	}

	if reportsAllFlag {
		if reportsLdFlag != "" || reportsLnFlag != "" {
			return fmt.Errorf("--all cannot be combined with --ld or --ln")
		}
	} else {
		if reportsLdFlag == "" || reportsLnFlag == "" {
			return fmt.Errorf("--ld and --ln are required (or use --all to list reports across the server)")
		}
	}

	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	a := app.New(conn)

	if reportsAllFlag {
		refs, err := a.ListAllReports()
		if err != nil {
			return err
		}
		if format == formatter.OutputFormatJSON {
			entries := make([]view.ReportControlBlockRef, 0, len(refs))
			entries = append(entries, refs...)
			return writeJSON(cmd, entries)
		}
		printAllReportsText(cmd, a, refs)
		return nil
	}

	unbuffered, buffered, err := a.ListReportNames(app.ListReportsInput{
		LD: reportsLdFlag,
		LN: reportsLnFlag,
	})
	if err != nil {
		return err
	}

	if format == formatter.OutputFormatJSON {
		entries := make([]view.ReportControlBlockRef, 0, len(unbuffered)+len(buffered))
		for _, name := range unbuffered {
			entries = append(entries, view.ReportControlBlockRef{
				LD:       reportsLdFlag,
				LN:       reportsLnFlag,
				Name:     name,
				Buffered: false,
				Ref:      fmt.Sprintf("%s/%s.RP.%s", reportsLdFlag, reportsLnFlag, name),
			})
		}
		for _, name := range buffered {
			entries = append(entries, view.ReportControlBlockRef{
				LD:       reportsLdFlag,
				LN:       reportsLnFlag,
				Name:     name,
				Buffered: true,
				Ref:      fmt.Sprintf("%s/%s.BR.%s", reportsLdFlag, reportsLnFlag, name),
			})
		}
		return writeJSON(cmd, entries)
	}

	printScopedReportsText(cmd, a, unbuffered, buffered)
	return nil
}

func printAllReportsText(cmd *cobra.Command, a *app.App, refs []view.ReportControlBlockRef) {
	out := cmd.OutOrStdout()
	if len(refs) == 0 {
		_, _ = fmt.Fprintln(out, "No reports found on the server.")
		return
	}
	_, _ = fmt.Fprintf(out, "Found %d report(s) across all logical devices:\n\n", len(refs))

	curLD, curLN := "", ""
	for _, ref := range refs {
		if ref.LD != curLD || ref.LN != curLN {
			curLD, curLN = ref.LD, ref.LN
			if reportsDetailedFlag {
				_, _ = fmt.Fprintf(out, "\n--- %s/%s ---\n\n", ref.LD, ref.LN)
			} else {
				_, _ = fmt.Fprintf(out, "%s/%s:\n", ref.LD, ref.LN)
			}
		}
		kind := "URCB"
		if ref.Buffered {
			kind = "BRCB"
		}
		_, _ = fmt.Fprintf(out, "  - %s (%s)\n", ref.Name, kind)

		if reportsDetailedFlag {
			result, err := a.GetReport(app.GetReportInput{LD: ref.LD, LN: ref.LN, Name: ref.Name})
			if err != nil {
				_, _ = fmt.Fprintf(out, "     Error reading configuration: %v\n\n", err)
			} else {
				renderer := formatter.NewRenderer(formatter.OutputFormatText)
				_ = renderer.RenderReportControlBlock(&result.Report, out)
				_, _ = fmt.Fprintln(out)
			}
		}
	}
	if !reportsDetailedFlag {
		_, _ = fmt.Fprintln(out, "\nUse --detailed flag to see report configuration")
	}
}

func printScopedReportsText(cmd *cobra.Command, a *app.App, unbuffered, buffered []string) {
	out := cmd.OutOrStdout()
	total := len(unbuffered) + len(buffered)
	if total == 0 {
		_, _ = fmt.Fprintf(out, "No reports found in '%s/%s'\n", reportsLdFlag, reportsLnFlag)
		return
	}
	_, _ = fmt.Fprintf(out, "Found %d report(s) in '%s/%s':\n\n", total, reportsLdFlag, reportsLnFlag)

	if len(unbuffered) > 0 {
		_, _ = fmt.Fprintf(out, "Unbuffered Reports (%d):\n", len(unbuffered))
		for i, name := range unbuffered {
			_, _ = fmt.Fprintf(out, "  %d. %s\n", i+1, name)
			if reportsDetailedFlag {
				result, err := a.GetReport(app.GetReportInput{LD: reportsLdFlag, LN: reportsLnFlag, Name: name})
				if err != nil {
					_, _ = fmt.Fprintf(out, "     Error reading configuration: %v\n\n", err)
				} else {
					renderer := formatter.NewRenderer(formatter.OutputFormatText)
					_ = renderer.RenderReportControlBlock(&result.Report, out)
					_, _ = fmt.Fprintln(out)
				}
			}
		}
		if !reportsDetailedFlag {
			_, _ = fmt.Fprintln(out)
		}
	}
	if len(buffered) > 0 {
		_, _ = fmt.Fprintf(out, "Buffered Reports (%d):\n", len(buffered))
		for i, name := range buffered {
			_, _ = fmt.Fprintf(out, "  %d. %s\n", i+1, name)
			if reportsDetailedFlag {
				result, err := a.GetReport(app.GetReportInput{LD: reportsLdFlag, LN: reportsLnFlag, Name: name})
				if err != nil {
					_, _ = fmt.Fprintf(out, "     Error reading configuration: %v\n\n", err)
				} else {
					renderer := formatter.NewRenderer(formatter.OutputFormatText)
					_ = renderer.RenderReportControlBlock(&result.Report, out)
					_, _ = fmt.Fprintln(out)
				}
			}
		}
	}
	if !reportsDetailedFlag {
		_, _ = fmt.Fprintln(out, "Use --detailed flag to see report configuration")
	}
}
