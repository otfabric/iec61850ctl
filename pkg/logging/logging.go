// SPDX-License-Identifier: MIT

// Package logging builds the structured (log/slog) logger used by the CLI and
// passed to go-iec61850 / go-mms via their Logger options.
package logging

import (
	"io"
	"log/slog"
	"strings"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// Levels and formats accepted by New.
const (
	LevelOff   = "off"
	FormatText = "text"
	FormatJSON = "json"
)

// ParseLevel maps debug|info|warn|warning|error|off (case-insensitive) to a
// slog level. off reports ok=false with a nil error.
func ParseLevel(s string) (level slog.Level, enabled bool, err error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, true, nil
	case "info", "":
		return slog.LevelInfo, true, nil
	case "warn", "warning":
		return slog.LevelWarn, true, nil
	case "error":
		return slog.LevelError, true, nil
	case LevelOff, "none", "quiet":
		return 0, false, nil
	}
	return 0, false, errs.Invalidf("invalid --log-level %q (supported: debug, info, warn, error, off)", s)
}

// New returns a logger writing to w at level in format (text or json).
// Level "off" returns a logger that discards everything.
func New(w io.Writer, level, format string) (*slog.Logger, error) {
	lvl, enabled, err := ParseLevel(level)
	if err != nil {
		return nil, err
	}
	f := strings.ToLower(strings.TrimSpace(format))
	if f != "" && f != FormatText && f != FormatJSON {
		return nil, errs.Invalidf("invalid --log-format %q (supported: text, json)", format)
	}
	if !enabled || w == nil {
		return slog.New(slog.DiscardHandler), nil
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if f == FormatJSON {
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	}
	return slog.New(slog.NewTextHandler(w, opts)), nil
}
