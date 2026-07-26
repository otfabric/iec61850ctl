//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"testing"
)

func TestNegative_UnknownLD(t *testing.T) {
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			res := runCtl(t, h.ctlEnv(), commandTimeout,
				"list", "lns",
				"--ld", "NoSuchLD",
				"--format", "json",
			)
			if res.ExitCode == 0 {
				logCommandFailure(t, res, h)
			}
			requireExitNonZero(t, res)
		})
	}
}

func TestNegative_UnknownObject(t *testing.T) {
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			res := runCtl(t, h.ctlEnv(), commandTimeout,
				"get", "object",
				"--object", "InteropLD/LLN0.NoSuchAttr.stVal",
				"--fc", "ST",
				"--format", "json",
			)
			requireExitNonZero(t, res)
			if len(res.Stderr) == 0 {
				t.Fatalf("expected non-empty stderr for unknown object")
			}
		})
	}
}
