// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-iec61850/scl"
	"github.com/otfabric/go-mms/transport/iso"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// RunConfig holds options for starting the MMS server from an SCL file.
type RunConfig struct {
	SclPath        string       // path to SCL/CID/ICD file (required)
	IEDName        string       // IED name in SCL; empty uses the first IED
	ValuesPath     string       // optional path to serialized IED JSON for value seeding
	FilesDir       string       // optional directory served read-only over MMS file services
	Host           string       // bind address (e.g. "0.0.0.0")
	Port           int          // MMS port; 0 binds an ephemeral port (see Running.Addr)
	MaxConnections int          // max MMS connections (reserved; not yet enforced by go-mms)
	ReadyJSON      bool         // emit one JSON readiness event on Stdout after bind
	FixtureID      string       // optional fixture identifier for readiness JSON
	Version        string       // build/version metadata for readiness JSON
	Logger         *slog.Logger // structured go-iec61850/go-mms server logging; nil disables
	Stdout         io.Writer    // readiness JSON destination (default os.Stdout)
	Stderr         io.Writer    // human status line destination (default os.Stderr)

	// GOOSE, when non-nil, publishes the model's GoCBs on this transport
	// and exposes them over MMS (LLN0$GO$<name>). GOOSEStartDisabled
	// leaves GoEna false until a client enables publishing.
	GOOSE              goose.Transport
	GOOSEStartDisabled bool
}

// Running is a started server. Close it to stop serving.
type Running struct {
	addr    string
	iedName string
	srv     *iec61850.Server
	ln      *iso.Listener
	files   *DirFileProvider
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
	err     error
}

// Addr returns the bound host:port (the real port when RunConfig.Port was 0).
func (r *Running) Addr() string { return r.addr }

// IEDName returns the IED served.
func (r *Running) IEDName() string { return r.iedName }

// Done is closed once the accept loop has stopped.
func (r *Running) Done() <-chan struct{} { return r.done }

// Close stops accepting, closes the listener and server, and waits for the
// accept loop. It is idempotent.
func (r *Running) Close() error {
	r.once.Do(func() {
		r.cancel()
		_ = r.ln.Close()
		<-r.done
		r.srv.Close()
		if r.files != nil {
			_ = r.files.Release()
		}
	})
	return r.err
}

// Start loads the model, binds the listener and serves in the background until
// ctx is done or Close is called.
func Start(ctx context.Context, cfg RunConfig) (*Running, error) {
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return nil, errs.Invalidf("port %d out of range 0..65535", cfg.Port)
	}
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 5
	}

	sclData, err := scl.ParseFile(cfg.SclPath)
	if err != nil {
		return nil, fmt.Errorf("parse SCL: %w", err)
	}

	iedName, err := resolveIEDName(sclData, cfg.IEDName)
	if err != nil {
		return nil, err
	}

	model, err := iec61850.NewServerModelFromSCL(sclData, iedName, "")
	if err != nil {
		return nil, fmt.Errorf("build server model: %w", err)
	}

	opts := iec61850.ServerOptions{
		Logger: cfg.Logger,
		Identity: &iec61850.ServerIdentity{
			Vendor:   "OTFabric",
			Model:    "iec61850ctl",
			Revision: "1.0",
		},
	}
	var files *DirFileProvider
	if cfg.FilesDir != "" {
		files, err = NewDirFileProvider(cfg.FilesDir)
		if err != nil {
			return nil, fmt.Errorf("files dir: %w", err)
		}
		opts.FileProvider = files
	}
	srv, err := iec61850.NewServer(model, opts)
	if err != nil {
		releaseFiles(files)
		return nil, fmt.Errorf("create server: %w", err)
	}
	fail := func(err error) (*Running, error) {
		srv.Close()
		releaseFiles(files)
		return nil, err
	}

	if cfg.ValuesPath != "" {
		if err := seedValuesFromFile(srv, cfg.ValuesPath); err != nil {
			return fail(fmt.Errorf("seed values: %w", err))
		}
	}

	if err := registerInteropControls(srv); err != nil {
		return fail(fmt.Errorf("register controls: %w", err))
	}

	srv.EnableReports()
	if cfg.GOOSE != nil {
		if _, err := srv.EnableGOOSE(iec61850.GOOSEOptions{Transport: cfg.GOOSE, StartDisabled: cfg.GOOSEStartDisabled, Logger: cfg.Logger}); err != nil {
			return fail(fmt.Errorf("enable GOOSE: %w", err))
		}
	}
	if srv.JournalEngine() == nil {
		srv.EnableJournals()
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	ln, err := iso.Listen(addr)
	if err != nil {
		return fail(errs.Mark(fmt.Errorf("listen on %s: %w", addr, err), errs.ErrConnectionFailed))
	}

	sctx, cancel := context.WithCancel(ctx)
	r := &Running{
		addr:    ln.Addr().String(),
		iedName: iedName,
		srv:     srv,
		ln:      ln,
		files:   files,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	go func() {
		defer close(r.done)
		if err := srv.ListenAndServe(sctx, ln); err != nil && sctx.Err() == nil && !errors.Is(err, net.ErrClosed) {
			r.err = err
		}
	}()
	return r, nil
}

// Run starts the server and blocks until SIGINT/SIGTERM.
func Run(cfg RunConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return RunContext(ctx, cfg)
}

// RunContext starts the server, prints the status line (and readiness JSON),
// and blocks until ctx is done.
func RunContext(ctx context.Context, cfg RunConfig) error {
	stdout, stderr := cfg.Stdout, cfg.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	r, err := Start(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	_, _ = fmt.Fprintf(stderr, "IEC 61850 MMS server listening on %s (IED %s, Ctrl+C to stop)\n", r.Addr(), r.IEDName())

	if cfg.ReadyJSON {
		line, err := EncodeReadyEvent(r.Addr(), cfg.FixtureID, cfg.Version, r.IEDName())
		if err != nil {
			return err
		}
		if _, err := stdout.Write(line); err != nil {
			return fmt.Errorf("write ready event: %w", err)
		}
	}

	select {
	case <-ctx.Done():
	case <-r.Done():
	}
	return r.Close()
}

func releaseFiles(p *DirFileProvider) {
	if p != nil {
		_ = p.Release()
	}
}

func resolveIEDName(sclData *scl.SCL, name string) (string, error) {
	if name != "" {
		if sclData.FindIED(name) == nil {
			return "", errs.Newf(errs.ErrNotFound, "IED %q not found in SCL", name)
		}
		return name, nil
	}
	if len(sclData.IEDs) == 0 {
		return "", errs.Newf(errs.ErrInvalidConfig, "SCL contains no IEDs")
	}
	// Prefer the first IED that actually hosts an MMS server: CIDs often list
	// client-only IEDs (no AccessPoint/Server) before the device itself.
	for _, ied := range sclData.IEDs {
		for _, ap := range ied.AccessPoints {
			if ap.Server != nil && len(ap.Server.LDevices) > 0 {
				return ied.Name, nil
			}
		}
	}
	return sclData.IEDs[0].Name, nil
}
