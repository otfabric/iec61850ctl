// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// It defines all commands (root, list, get, tree) and their flags, delegating
// business logic to the services package.
package cmd

import (
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List devices, nodes, and objects from the IEC 61850 server",
}

func init() {
	rootCmd.AddCommand(listCmd)
}
