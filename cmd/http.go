// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"log"

	httpTransport "github.com/otfabric/iec61850ctl/internal/transport/http"

	"github.com/spf13/cobra"
)

var (
	httpListenAddr string
	httpIECHost    string
	httpIECPort    int
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "Start HTTP/REST API server",
	Long: `Start an HTTP server that exposes IEC 61850 operations as RESTful endpoints.

The server connects to a single IEC 61850 device and exposes its data model
and operations via HTTP JSON APIs.

Example endpoints:
  GET  /api/logical-devices
  GET  /api/logical-nodes?ld=<device>
  GET  /api/datasets?ld=<device>&ln=<node>
  GET  /api/reports/all
  POST /api/find/path
  
See internal/transport/http/ for full endpoint documentation.`,
	RunE: runHTTP,
}

func init() {
	httpCmd.Flags().StringVar(&httpListenAddr, "listen", ":8080", "HTTP server listen address")
	httpCmd.Flags().StringVar(&httpIECHost, "iec-host", "", "IEC 61850 device host (required)")
	httpCmd.Flags().IntVar(&httpIECPort, "iec-port", 102, "IEC 61850 device port")
	_ = httpCmd.MarkFlagRequired("iec-host")

	rootCmd.AddCommand(httpCmd)
}

func runHTTP(cmd *cobra.Command, args []string) error {
	fmt.Printf("Connecting to IEC 61850 device at %s:%d\n", httpIECHost, httpIECPort)
	fmt.Printf("Starting HTTP server on %s\n\n", httpListenAddr)

	srv, err := httpTransport.NewServer(httpTransport.Config{
		ListenAddr: httpListenAddr,
		IECHost:    httpIECHost,
		IECPort:    httpIECPort,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	log.Printf("HTTP server ready. Example requests:")
	log.Printf("  curl http://localhost%s/health", httpListenAddr)
	log.Printf("  curl http://localhost%s/api/logical-devices", httpListenAddr)
	log.Println()

	if err := srv.Start(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
