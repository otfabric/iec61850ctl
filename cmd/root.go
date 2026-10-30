// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// It defines all commands (root, list, get, tree) and their flags, delegating
// business logic to the services package.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/logging"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

var (
	host    string
	port    int
	debug   bool
	iedName string

	logLevelFlag       string
	logFormatFlag      string
	connectTimeoutFlag time.Duration
	requestTimeoutFlag time.Duration
	retryFlag          int
	retryMaxDelayFlag  time.Duration
)

// runState is the per-invocation state built in PersistentPreRunE.
var runState struct {
	entered bool         // PersistentPreRunE ran (errors before it are usage errors)
	logger  *slog.Logger // structured logger on stderr; never nil after pre-run
}

var rootCmd = &cobra.Command{
	Use:   "iec61850ctl",
	Short: "IEC 61850 CLI: MMS client and server, GOOSE, Sampled Values, SCL",
	Long: `Pure-Go command-line tool for IEC 61850.

  MMS client    discover devices, browse and read the data model, write and
                operate objects, subscribe to reports, transfer files, query
                journals
  Station bus   GOOSE and Sampled Values on raw Ethernet: sniff, supervised
                subscribe, guarded publish, control block access over MMS
  Engineering   parse and convert SCL offline
  Server        run a local MMS server from SCL, or an HTTP/JSON API in front
                of one device

Environment Variables:
  IEC61850_HOST       Default host address (overridden by --host flag)
  IEC61850_PORT       Default port number (overridden by --port flag)
  IEC61850_IED_NAME   Default IED name for MMS domain normalisation (overridden by --ied-name)

Exit codes:
  0 success            4 timeout              7 write verification mismatch
  1 failure            5 not found            8 unsupported service
  2 usage/invalid      6 rejected/denied    130 interrupted (SIGINT/SIGTERM)
  3 connection failed`,
	SilenceUsage:      true,
	SilenceErrors:     true,
	PersistentPreRunE: persistentPreRun,
}

// applyFormatEnv fills a command's --format, when it has one and it was not
// given, from IEC61850CTL_FORMAT (canonical) or IEC61850CTL_OUTPUT_FORMAT.
func applyFormatEnv(cmd *cobra.Command) error {
	f := cmd.Flags().Lookup("format")
	if f == nil || f.Changed {
		return nil
	}
	v := os.Getenv("IEC61850CTL_FORMAT")
	if v == "" {
		v = os.Getenv("IEC61850CTL_OUTPUT_FORMAT")
	}
	if v == "" {
		return nil
	}
	return f.Value.Set(v)
}

func persistentPreRun(cmd *cobra.Command, _ []string) error {
	// Cobra validates required flags and flag groups only after this hook;
	// do it here so those failures classify as usage errors (exit 2).
	if err := cmd.ValidateRequiredFlags(); err != nil {
		return errs.Mark(err, errs.ErrInvalidInput)
	}
	if err := cmd.ValidateFlagGroups(); err != nil {
		return errs.Mark(err, errs.ErrInvalidInput)
	}
	runState.entered = true
	if err := applyFormatEnv(cmd); err != nil {
		return err
	}
	level := logLevelFlag
	if debug && !cmd.Flags().Changed("log-level") {
		level = "debug"
	}
	logger, err := logging.New(cmd.ErrOrStderr(), level, logFormatFlag)
	if err != nil {
		return err
	}
	runState.logger = logger
	service.SetLogger(logger)
	switch {
	case connectTimeoutFlag < 0:
		return errs.Invalidf("--connect-timeout must be >= 0")
	case requestTimeoutFlag < 0:
		return errs.Invalidf("--timeout must be >= 0")
	case retryFlag < 0:
		return errs.Invalidf("--retry must be >= 0")
	case retryMaxDelayFlag < 0:
		return errs.Invalidf("--retry-max-delay must be >= 0")
	}
	return nil
}

// Execute runs the CLI with os.Args and exits with the code from ExitCode.
// SIGINT/SIGTERM cancel the command context (exit 130).
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	code := ExecuteContext(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// ExecuteContext runs the CLI in-process with the given arguments and writers
// and returns the process exit code (see errs.ExitCode). Errors are printed
// to stderr. Flag values persist between calls; callers that run several
// invocations in one process must reset them (tests do).
func ExecuteContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	runState.entered = false
	runState.logger = nil
	rootCmd.SetArgs(args)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	// Cobra keeps the first context it hands to a subcommand; reset it so
	// repeated in-process invocations each see their own ctx.
	setContextTree(rootCmd, ctx)
	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return errs.ExitOK
	}
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		err = fmt.Errorf("interrupted: %w", err)
	}
	_, _ = fmt.Fprintln(stderr, err)
	if !runState.entered {
		// Cobra rejected the command line (unknown command/flag, bad args,
		// missing required flag) before any command logic ran.
		return errs.ExitUsage
	}
	return errs.ExitCode(err)
}

func setContextTree(c *cobra.Command, ctx context.Context) {
	c.SetContext(ctx)
	for _, sub := range c.Commands() {
		setContextTree(sub, ctx)
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
			return "", 0, errs.Invalidf("host is required (use --host flag or IEC61850_HOST environment variable)")
		}
	}

	// If port flag not set and env var is set, use env var
	if !rootCmd.PersistentFlags().Changed("port") {
		if env := os.Getenv("IEC61850_PORT"); env != "" {
			envPort, err := strconv.Atoi(env)
			if err != nil || envPort <= 0 || envPort > 65535 {
				return "", 0, errs.Invalidf("invalid IEC61850_PORT %q", env)
			}
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

// cmdLogger returns the invocation logger (discarding before pre-run).
func cmdLogger() *slog.Logger {
	if runState.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return runState.logger
}

// requestTimeout is the per-request bound from --timeout.
func requestTimeout() time.Duration {
	if requestTimeoutFlag <= 0 {
		return client.DefaultRequestTimeout
	}
	return requestTimeoutFlag
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&host, "host", "", "host address of the IEC 61850 server (env: IEC61850_HOST)")
	pf.IntVar(&port, "port", 102, "port of the IEC 61850 server (env: IEC61850_PORT)")
	pf.StringVar(&iedName, "ied-name", "", "IED name for MMS domain normalisation / SCL IED selection (env: IEC61850_IED_NAME)")
	pf.BoolVar(&debug, "debug", false, "shorthand for --log-level=debug")
	pf.StringVar(&logLevelFlag, "log-level", "warn", "structured log level on stderr: debug, info, warn, error, off")
	pf.StringVar(&logFormatFlag, "log-format", "text", "structured log format: text, json")
	pf.DurationVar(&connectTimeoutFlag, "connect-timeout", client.DefaultConnectTimeout, "timeout for each connect attempt (TCP + COTP + MMS association)")
	pf.DurationVar(&requestTimeoutFlag, "timeout", client.DefaultRequestTimeout, "timeout for each MMS request")
	pf.IntVar(&retryFlag, "retry", 0, "number of connect retries with exponential backoff and jitter")
	pf.DurationVar(&retryMaxDelayFlag, "retry-max-delay", client.DefaultRetryMaxDelay, "maximum delay between connect retries")
}
