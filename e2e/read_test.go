//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"testing"
)

func TestRead_ST_ModStVal(t *testing.T) {
	const key = "InteropLD/LLN0.Mod.stVal"
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			res := runCtl(t, h.ctlEnv(), commandTimeout,
				"get", "object",
				"--object", key,
				"--fc", "ST",
				"--format", "json",
			)
			requireExitZero(t, res, h)

			var out struct {
				Object string `json:"object"`
				FC     string `json:"fc"`
				Type   string `json:"type"`
				Value  any    `json:"value"`
			}
			if err := decodeExactJSON(res.Stdout, &out); err != nil {
				logCommandFailure(t, res, h)
				t.Fatalf("decode get object JSON: %v", err)
			}
			want := expectedValue(t, key)
			if !valuesEqual(out.Value, want) {
				logCommandFailure(t, res, h)
				t.Fatalf("value: got %#v (%T), want %#v (%T)", out.Value, out.Value, want, want)
			}
			if out.FC != "ST" {
				t.Fatalf("fc: got %q, want ST", out.FC)
			}
		})
	}
}

func TestRead_ST_SPS1StVal(t *testing.T) {
	const key = "InteropLD/GGIO1.SPS1.stVal"
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			res := runCtl(t, h.ctlEnv(), commandTimeout,
				"get", "object",
				"--object", key,
				"--fc", "ST",
				"--format", "json",
			)
			requireExitZero(t, res, h)

			var out struct {
				Value any `json:"value"`
			}
			if err := decodeExactJSON(res.Stdout, &out); err != nil {
				logCommandFailure(t, res, h)
				t.Fatalf("decode get object JSON: %v", err)
			}
			want := expectedValue(t, key)
			if !valuesEqual(out.Value, want) {
				logCommandFailure(t, res, h)
				t.Fatalf("value: got %#v, want %#v", out.Value, want)
			}
		})
	}
}

func TestRead_MX_TotWMagF(t *testing.T) {
	const key = "InteropLD/MMXU1.TotW.mag.f"
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			res := runCtl(t, h.ctlEnv(), commandTimeout,
				"get", "object",
				"--object", key,
				"--fc", "MX",
				"--format", "json",
			)
			requireExitZero(t, res, h)

			var out struct {
				Type  string `json:"type"`
				Value any    `json:"value"`
			}
			if err := decodeExactJSON(res.Stdout, &out); err != nil {
				logCommandFailure(t, res, h)
				t.Fatalf("decode get object JSON: %v", err)
			}
			want := expectedValue(t, key)
			if !valuesEqual(out.Value, want) {
				logCommandFailure(t, res, h)
				t.Fatalf("value: got %#v, want %#v", out.Value, want)
			}
		})
	}
}
