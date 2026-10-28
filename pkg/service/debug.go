// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
)

// logger receives low-level service diagnostics. It is configured once per
// CLI invocation from the global --log-level/--log-format flags (see cmd/root.go).
// A nil value (the default) discards all diagnostics.
var logger atomic.Pointer[slog.Logger]

// SetLogger installs the logger used for service-level diagnostics.
// Passing nil disables them. Safe for concurrent use.
func SetLogger(l *slog.Logger) {
	logger.Store(l)
}

// Logger returns the logger installed with SetLogger, or nil.
func Logger() *slog.Logger {
	return logger.Load()
}

// debugf logs a formatted message at debug level when a logger is installed
// and debug logging is enabled. Diagnostics never go to stdout, so they cannot
// corrupt json/csv/yaml output.
func debugf(format string, args ...any) {
	l := logger.Load()
	if l == nil || !l.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	l.Debug(fmt.Sprintf(format, args...), "component", "service")
}
