// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// subscribe.go defines the subscribe parent command for RCB and other subscriptions.
package cmd

import (
	"github.com/spf13/cobra"
)

var subscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Subscribe to server events (reports) with graceful cleanup",
	Long:  `Subscribe to report control block (RCB) notifications. Use Ctrl+C or --duration/--max-reports for clean shutdown; the report is always disabled before disconnect.`,
}

func init() {
	rootCmd.AddCommand(subscribeCmd)
}
