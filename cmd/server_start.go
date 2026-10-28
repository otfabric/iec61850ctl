// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/iec61850ctl/pkg/errs"

	"github.com/otfabric/iec61850ctl/pkg/stack/server"

	"github.com/spf13/cobra"
)

var (
	serverStartSCL       string
	serverStartValues    string
	serverStartHost      string
	serverStartPort      int
	serverStartMaxConn   int
	serverStartReadyJSON bool
	serverStartFixtureID string
	serverStartFilesDir  string
)

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start IEC 61850 MMS server from SCL file",
	Long:  `Load an SCL/CID/ICD file and start the MMS server. Optionally seed leaf values from serialized IED JSON.`,
	RunE:  runServerStart,
}

func init() {
	serverCmd.AddCommand(serverStartCmd)
	serverStartCmd.Flags().StringVar(&serverStartSCL, "scl", "", "path to SCL/CID/ICD file (required)")
	serverStartCmd.Flags().StringVar(&serverStartValues, "values", "", "path to serialized IED JSON (leaf values applied before start)")
	serverStartCmd.Flags().StringVar(&serverStartHost, "host", "0.0.0.0", "bind address")
	serverStartCmd.Flags().IntVarP(&serverStartPort, "port", "p", 102, "MMS port (0 = ephemeral; the bound address is printed and included in --ready-json)")
	serverStartCmd.Flags().IntVar(&serverStartMaxConn, "max-connections", 5, "reserved; not enforced yet")
	serverStartCmd.Flags().BoolVar(&serverStartReadyJSON, "ready-json", false, "emit one JSON readiness event on stdout after bind")
	serverStartCmd.Flags().StringVar(&serverStartFixtureID, "fixture-id", "", "optional fixture identifier included in readiness JSON")
	serverStartCmd.Flags().StringVar(&serverStartFilesDir, "files-dir", "", "directory served read-only over MMS file services (list files / get file)")
	_ = serverStartCmd.MarkFlagRequired("scl")
}

func runServerStart(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(serverStartSCL); err != nil {
		return fmt.Errorf("SCL file: %w", err)
	}
	if serverStartPort < 0 || serverStartPort > 65535 {
		return errs.Invalidf("--port %d out of range 0..65535 (0 = ephemeral)", serverStartPort)
	}

	ver := strings.TrimSpace(buildMeta.commit)
	if ver == "" {
		ver = strings.TrimSpace(buildMeta.version)
	}
	if ver == "" {
		ver = "dev"
	}

	cfg := server.RunConfig{
		SclPath:        serverStartSCL,
		IEDName:        getIEDName(),
		ValuesPath:     serverStartValues,
		Host:           serverStartHost,
		Port:           serverStartPort,
		MaxConnections: serverStartMaxConn,
		ReadyJSON:      serverStartReadyJSON,
		FixtureID:      serverStartFixtureID,
		Version:        ver,
		FilesDir:       serverStartFilesDir,
		Logger:         cmdLogger(),
		Stdout:         cmd.OutOrStdout(),
		Stderr:         cmd.ErrOrStderr(),
	}

	// Cancelling the command context (SIGINT/SIGTERM) shuts the server down cleanly.
	return server.RunContext(cmd.Context(), cfg)
}
