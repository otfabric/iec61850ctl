// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// scl.go defines the SCL (System Configuration Language) subcommand for
// parsing and converting CID/ICD/SCD files without touching the IEC 61850 client.
package cmd

import (
	"github.com/spf13/cobra"
)

var sclCmd = &cobra.Command{
	Use:   "scl",
	Short: "Parse and convert SCL/CID/ICD files (no server connection)",
	Long: `Parse IEC 61850 SCL XML (CID, ICD, SCD) and output tree or flat text, or convert to CSV.
Does not connect to any device; operates on local files only.`,
}

func init() {
	rootCmd.AddCommand(sclCmd)
}
