// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/scl"
	"github.com/otfabric/go-mms/transport/iso"
)

// RunConfig holds options for starting the MMS server from an SCL file.
type RunConfig struct {
	SclPath        string // path to SCL/CID/ICD file (required)
	IEDName        string // IED name in SCL; empty uses the first IED
	ValuesPath     string // optional path to serialized IED JSON for value seeding
	Host           string // bind address (e.g. "0.0.0.0")
	Port           int    // MMS port (typically 102)
	MaxConnections int    // max MMS connections (reserved; not yet enforced by go-mms)
	ReadyJSON      bool   // emit one JSON readiness event on stdout after bind
	FixtureID      string // optional fixture identifier for readiness JSON
	Version        string // build/version metadata for readiness JSON
}

// Run loads the model from an SCL file and starts the IEC 61850 MMS server until SIGINT/SIGTERM.
func Run(cfg RunConfig) error {
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port <= 0 {
		cfg.Port = 102
	}
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 5
	}

	sclData, err := scl.ParseFile(cfg.SclPath)
	if err != nil {
		return fmt.Errorf("parse SCL: %w", err)
	}

	iedName, err := resolveIEDName(sclData, cfg.IEDName)
	if err != nil {
		return err
	}

	model, err := iec61850.NewServerModelFromSCL(sclData, iedName, "")
	if err != nil {
		return fmt.Errorf("build server model: %w", err)
	}

	srv, err := iec61850.NewServer(model, iec61850.ServerOptions{
		Identity: &iec61850.ServerIdentity{
			Vendor:   "OTFabric",
			Model:    "iec61850ctl",
			Revision: "1.0",
		},
	})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	if cfg.ValuesPath != "" {
		if err := seedValuesFromFile(srv, cfg.ValuesPath); err != nil {
			srv.Close()
			return fmt.Errorf("seed values: %w", err)
		}
	}

	if err := registerInteropControls(srv); err != nil {
		srv.Close()
		return fmt.Errorf("register controls: %w", err)
	}

	srv.EnableReports()

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	ln, err := iso.Listen(addr)
	if err != nil {
		srv.Close()
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		_ = srv.ListenAndServe(ctx, ln)
	}()

	_, _ = fmt.Fprintf(os.Stderr, "IEC 61850 MMS server listening on %s (IED %s, Ctrl+C to stop)\n", addr, iedName)

	if cfg.ReadyJSON {
		line, err := EncodeReadyEvent(addr, cfg.FixtureID, cfg.Version, iedName)
		if err != nil {
			srv.Close()
			return err
		}
		if _, err := os.Stdout.Write(line); err != nil {
			srv.Close()
			return fmt.Errorf("write ready event: %w", err)
		}
	}

	<-ctx.Done()

	srv.Close()
	return nil
}

func resolveIEDName(sclData *scl.SCL, name string) (string, error) {
	if name != "" {
		if sclData.FindIED(name) == nil {
			return "", fmt.Errorf("IED %q not found in SCL", name)
		}
		return name, nil
	}
	if len(sclData.IEDs) == 0 {
		return "", fmt.Errorf("SCL contains no IEDs")
	}
	return sclData.IEDs[0].Name, nil
}
