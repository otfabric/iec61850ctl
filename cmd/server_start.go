// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"strings"

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
	serverStartCmd.Flags().IntVarP(&serverStartPort, "port", "p", 102, "MMS port")
	serverStartCmd.Flags().IntVar(&serverStartMaxConn, "max-connections", 5, "maximum MMS client connections")
	serverStartCmd.Flags().BoolVar(&serverStartReadyJSON, "ready-json", false, "emit one JSON readiness event on stdout after bind")
	serverStartCmd.Flags().StringVar(&serverStartFixtureID, "fixture-id", "", "optional fixture identifier included in readiness JSON")
	_ = serverStartCmd.MarkFlagRequired("scl")
}

func runServerStart(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(serverStartSCL); err != nil {
		return fmt.Errorf("SCL file: %w", err)
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
	}

	if err := server.Run(cfg); err != nil {
		return err
	}

	return nil
}
