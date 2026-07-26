// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI for iec61850ctl.
// server.go defines the server parent command (generate-config, start).
package cmd

import (
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run MMS server from SCL, or export libiec61850 .cfg",
	Long: `Start a pure-Go MMS server from an SCL/CID/ICD file (server start --scl).
Optionally export a libiec61850 .cfg from tree --serialize JSON (server generate-config); export-only, not used at runtime.`,
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
