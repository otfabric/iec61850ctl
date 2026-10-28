//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"fmt"
	"strconv"
	"strings"
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

// TestRead_ST_DPS1AndTotVAh reads the Dbpos and INT64U ST leaves on every
// forward stack. Since iec61850-interop v1.0.0 every adapter serves
// DPS1.stVal = 2 (on) and TotVAh.actVal = 42; no stack is exempt and the
// value, not just the type, must match.
func TestRead_ST_DPS1AndTotVAh(t *testing.T) {
	leaves := []struct {
		key, typ string
		value    func(any) (any, error)
	}{
		{"InteropLD/GGIO1.DPS1.stVal", "bit_string", dbposValue},
		{"InteropLD/MMTR1.TotVAh.actVal", "unsigned", func(v any) (any, error) { return v, nil }},
	}
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			for _, leaf := range leaves {
				res := runCtl(t, h.ctlEnv(), commandTimeout,
					"get", "object", "--object", leaf.key, "--fc", "ST", "--format", "json")
				requireExitZero(t, res, h)
				var out struct {
					Type  string `json:"type"`
					Value any    `json:"value"`
				}
				if err := decodeExactJSON(res.Stdout, &out); err != nil {
					logCommandFailure(t, res, h)
					t.Fatalf("decode get object JSON: %v", err)
				}
				if out.Type != leaf.typ {
					logCommandFailure(t, res, h)
					t.Fatalf("%s type: got %q, want %q", leaf.key, out.Type, leaf.typ)
				}
				got, err := leaf.value(out.Value)
				if err != nil {
					logCommandFailure(t, res, h)
					t.Fatalf("%s: %v", leaf.key, err)
				}
				if want := expectedValue(t, leaf.key); !valuesEqual(got, want) {
					logCommandFailure(t, res, h)
					t.Fatalf("%s value: got %#v (raw %#v), want %#v", leaf.key, got, out.Value, want)
				}
			}
		})
	}
}

// dbposValue decodes a Dbpos from the interim bit-string JSON form "[b0 b1 ...]"
// (the octets, MSB first): the state is the two leading bits of the first octet.
func dbposValue(v any) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("bit string value %#v is not a string", v)
	}
	fields := strings.Fields(strings.Trim(s, "[]"))
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty bit string %q", s)
	}
	b, err := strconv.Atoi(fields[0])
	if err != nil || b < 0 || b > 255 {
		return nil, fmt.Errorf("bit string %q: first octet not a byte", s)
	}
	return b >> 6, nil
}
