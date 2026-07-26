//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"testing"
)

func TestBrowse_ListLDs(t *testing.T) {
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			res := runCtl(t, h.ctlEnv(), commandTimeout, "list", "lds", "--format", "json")
			requireExitZero(t, res, h)

			var devices []struct {
				Name string `json:"name"`
			}
			if err := decodeExactJSON(res.Stdout, &devices); err != nil {
				logCommandFailure(t, res, h)
				t.Fatalf("decode list lds JSON: %v", err)
			}
			names := make([]string, len(devices))
			for i, d := range devices {
				names[i] = d.Name
			}
			if !containsName(names, "InteropLD") {
				logCommandFailure(t, res, h)
				t.Fatalf("InteropLD not in list lds: %v", names)
			}
		})
	}
}

func TestBrowse_ListLNs(t *testing.T) {
	wantLNs := []string{"LLN0", "GGIO1", "MMXU1", "MMTR1"}
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			res := runCtl(t, h.ctlEnv(), commandTimeout,
				"list", "lns", "--ld", "InteropLD", "--format", "json")
			requireExitZero(t, res, h)

			var nodes []struct {
				Name string `json:"name"`
			}
			if err := decodeExactJSON(res.Stdout, &nodes); err != nil {
				logCommandFailure(t, res, h)
				t.Fatalf("decode list lns JSON: %v", err)
			}
			names := make([]string, len(nodes))
			for i, n := range nodes {
				names[i] = n.Name
			}
			for _, want := range wantLNs {
				if !containsName(names, want) {
					logCommandFailure(t, res, h)
					t.Fatalf("missing LN %q in %v", want, names)
				}
			}
		})
	}
}
