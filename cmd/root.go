// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// It defines all commands (root, list, get, tree) and their flags, delegating
// business logic to the services package.
package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/otfabric/iec61850ctl/pkg/service"

	"github.com/spf13/cobra"
)

var (
	host    string
	port    int
	debug   bool
	iedName string
)

var rootCmd = &cobra.Command{
	Use:   "iec61850ctl",
	Short: "IEC 61850 MMS CLI (browse, read, reports, SCL, local server)",
	Long: `Pure-Go command-line tool for IEC 61850 MMS: discover devices, browse and read
the data model, subscribe to reports, transfer files, query journals, parse SCL
offline, run a local MMS server from SCL, and optionally expose an HTTP/JSON API.

Environment Variables:
  IEC61850_HOST       Default host address (overridden by --host flag)
  IEC61850_PORT       Default port number (overridden by --port flag)
  IEC61850_IED_NAME   Default IED name for MMS domain normalisation (overridden by --ied-name)`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command and handles any execution errors.
// It exits with code 1 if an error occurs.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getHostPort returns the host and port to use, checking flags first then environment variables.
// Returns error if host is not provided via either method.
func getHostPort() (string, int, error) {
	finalHost := host
	finalPort := port

	// If host flag not set, try environment variable
	if finalHost == "" {
		if envHost := os.Getenv("IEC61850_HOST"); envHost != "" {
			finalHost = envHost
		} else {
			return "", 0, fmt.Errorf("host is required (use --host flag or IEC61850_HOST environment variable)")
		}
	}

	// If port is default (102) and env var is set, use env var
	if port == 102 && os.Getenv("IEC61850_PORT") != "" {
		if envPort, err := strconv.Atoi(os.Getenv("IEC61850_PORT")); err == nil {
			finalPort = envPort
		}
	}

	return finalHost, finalPort, nil
}

// getIEDName returns the IED name from --ied-name, else IEC61850_IED_NAME, else empty.
func getIEDName() string {
	if iedName != "" {
		return iedName
	}
	return os.Getenv("IEC61850_IED_NAME")
}

func init() {
	// Enable/disable low-level IEC 61850 debug logging for all commands
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		service.SetDebug(debug)
	}

	// Global flags for all subcommands
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "host address of the IEC 61850 server (env: IEC61850_HOST)")
	rootCmd.PersistentFlags().IntVar(&port, "port", 102, "port of the IEC 61850 server (env: IEC61850_PORT)")
	rootCmd.PersistentFlags().StringVar(&iedName, "ied-name", "", "IED name for MMS domain normalisation / SCL IED selection (env: IEC61850_IED_NAME)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging of IEC 61850 calls")
}
