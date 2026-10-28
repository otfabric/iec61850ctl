// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"

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
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Connecting to IEC 61850 device at %s\n", net.JoinHostPort(httpIECHost, strconv.Itoa(httpIECPort)))

	srv, err := httpTransport.NewServer(cmd.Context(), httpTransport.Config{
		ListenAddr: httpListenAddr,
		Connection: client.ConnectionInput{
			Host:           httpIECHost,
			Port:           httpIECPort,
			ConnectTimeout: connectTimeoutFlag,
			RequestTimeout: requestTimeout(),
			IEDName:        getIEDName(),
			Retry:          client.RetryPolicy{Retries: retryFlag, MaxDelay: retryMaxDelayFlag},
			Logger:         cmdLogger(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	ln, err := net.Listen("tcp", httpListenAddr)
	if err != nil {
		srv.Close()
		return errs.Mark(fmt.Errorf("listen on %s: %w", httpListenAddr, err), errs.ErrConnectionFailed)
	}
	addr := ln.Addr().String()
	_, _ = fmt.Fprintf(out, "HTTP server listening on %s. Example requests:\n", addr)
	_, _ = fmt.Fprintf(out, "  curl http://%s/health\n", addr)
	_, _ = fmt.Fprintf(out, "  curl http://%s/api/logical-devices\n\n", addr)

	if err := srv.Serve(cmd.Context(), ln); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
