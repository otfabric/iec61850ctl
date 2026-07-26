// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

var (
	subscribeReportLd            string
	subscribeReportLn            string
	subscribeReportName          string
	subscribeReportType          string
	subscribeReportDuration      string
	subscribeReportMax           int
	subscribeReportValues        bool
	subscribeReportInterrogation bool
	subscribeReportSync          bool
	subscribeReportFormat        string
)

var subscribeReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Subscribe to RCB notifications with auto-cleanup",
	Long: `Subscribe to a report control block (RCB). The RCB reference must include the functional constraint (FC): use --type BR for buffered reports (BRCB) or --type RP for unbuffered reports (URCB). Exits on Ctrl+C, or after --duration (e.g. 30s, 5m, 1h), or after --max-reports. Always disables the report before disconnect.

Use --format jsonl for machine-readable JSON Lines on stdout (diagnostics on stderr).

Difference:
  --interrogation: GI → initial report (may be empty). Best for: SCADA-like event stream + report snapshot.
  --sync: ReadDataSet → full baseline snapshot (deterministic). Best for: operational consumers that must know state at startup/reconnect.

Examples:
  iec61850ctl subscribe report ... --interrogation
  iec61850ctl subscribe report ... --sync
  iec61850ctl subscribe report ... --interrogation --format jsonl`,
	RunE: runSubscribeReport,
}

func init() {
	subscribeCmd.AddCommand(subscribeReportCmd)
	subscribeReportCmd.Flags().StringVar(&subscribeReportLd, "ld", "", "Logical device name (required)")
	subscribeReportCmd.Flags().StringVar(&subscribeReportLn, "ln", "", "Logical node name (required)")
	subscribeReportCmd.Flags().StringVar(&subscribeReportName, "report", "", "Report control block name (required)")
	subscribeReportCmd.Flags().StringVar(&subscribeReportType, "type", "BR", "Report type: BR (buffered/BRCB) or RP (unbuffered/URCB)")
	subscribeReportCmd.Flags().StringVar(&subscribeReportDuration, "duration", "", "Auto-terminate after duration (e.g. 30s, 5m, 1h)")
	subscribeReportCmd.Flags().IntVar(&subscribeReportMax, "max-reports", 0, "Exit after receiving this many reports (0 = no limit)")
	subscribeReportCmd.Flags().BoolVar(&subscribeReportValues, "show-values", false, "Show data set values in each report")
	subscribeReportCmd.Flags().BoolVar(&subscribeReportInterrogation, "interrogation", false, "Trigger General Interrogation (GI) on the RCB after enabling to request an immediate report (may include 0 items)")
	subscribeReportCmd.Flags().BoolVar(&subscribeReportSync, "sync", false, "Read the report dataset once to obtain a full baseline snapshot (one-shot state sync)")
	subscribeReportCmd.Flags().StringVar(&subscribeReportFormat, "format", "text", "Output format: text, jsonl")
	_ = subscribeReportCmd.MarkFlagRequired("ld")
	_ = subscribeReportCmd.MarkFlagRequired("ln")
	_ = subscribeReportCmd.MarkFlagRequired("report")
}

func runSubscribeReport(cmd *cobra.Command, args []string) error {
	streamFmt, err := formatter.ParseStreamFormat(subscribeReportFormat)
	if err != nil {
		return err
	}
	var duration time.Duration
	if subscribeReportDuration != "" {
		duration, err = time.ParseDuration(subscribeReportDuration)
		if err != nil {
			return fmt.Errorf("--duration: %w (e.g. 30s, 5m, 1h)", err)
		}
		if duration <= 0 {
			return fmt.Errorf("--duration must be positive")
		}
	}
	if subscribeReportMax < 0 {
		return fmt.Errorf("--max-reports must be >= 0")
	}
	fc := strings.ToUpper(strings.TrimSpace(subscribeReportType))
	if fc != "BR" && fc != "RP" {
		return fmt.Errorf("--type must be BR (buffered) or RP (unbuffered), got %q", subscribeReportType)
	}

	session, err := openClientSession(cmd, clientSessionOptions{RequestTimeout: 30 * time.Second})
	if err != nil {
		return err
	}
	defer func() {
		if cerr := session.CloseStrict(); cerr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: session cleanup: %v\n", cerr)
		}
	}()
	conn := session.Conn()

	a := app.New(conn)
	reportRef := fmt.Sprintf("%s/%s.%s.%s", subscribeReportLd, subscribeReportLn, fc, subscribeReportName)

	var datasetRef string
	if subscribeReportSync {
		buffered := fc == "BR"
		details, err := a.ReportService().GetReportDetails(subscribeReportLd, subscribeReportLn, subscribeReportName, buffered)
		switch {
		case err != nil:
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: --sync requested but could not read report details (%v); skipping baseline sync.\n", err)
		case details.DatSet == "":
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Warning: --sync requested but report has no dataset reference; skipping baseline sync.")
		default:
			datasetRef = details.DatSet
		}
	}

	reporter, err := a.SubscribeReport(app.SubscribeReportInput{
		ReportRef:     reportRef,
		Duration:      duration,
		MaxReports:    subscribeReportMax,
		ShowValues:    subscribeReportValues,
		Interrogation: subscribeReportInterrogation,
		Sync:          subscribeReportSync,
		DatasetRef:    datasetRef,
		Writer:        cmd.OutOrStdout(),
		ErrWriter:     cmd.ErrOrStderr(),
		Format:        string(streamFmt),
	})
	if err != nil {
		return err
	}
	reporter.WriteStats(cmd.OutOrStdout())
	if streamFmt != formatter.StreamFormatJSONL {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Connection closed.")
	}
	return nil
}
