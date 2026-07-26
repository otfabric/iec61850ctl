// SPDX-License-Identifier: MIT

package service

import "fmt"

// debugEnabled controls whether low-level IEC 61850 calls are logged.
// It is configured once at the start of each CLI invocation via the
// top-level --debug flag (see cmd/root.go).
var debugEnabled bool

// SetDebug configures debug logging for IEC 61850 calls.
// This is intended to be called from the CLI layer before any
// connections are created or service methods are invoked.
func SetDebug(enabled bool) {
	debugEnabled = enabled
}

// debugf prints a formatted debug message when debug logging is enabled.
// All logging goes to stdout so that it is visible in CLI output.
func debugf(format string, args ...interface{}) {
	if !debugEnabled {
		return
	}
	fmt.Printf("[IEC61850 DEBUG] "+format+"\n", args...)
}
