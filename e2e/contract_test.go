//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Stack-independent CLI contract tests (run once; no Docker servers required).

func TestContract_MissingHost(t *testing.T) {
	requireCtlBin(t)

	res := runCtl(t, map[string]string{
		"IEC61850_HOST": "",
		"IEC61850_PORT": "102",
	}, commandTimeout, "list", "lds", "--format", "json")
	requireExitNonZero(t, res)
	if !strings.Contains(strings.ToLower(string(res.Stderr)), "host") {
		t.Fatalf("stderr should mention host; got %q", res.Stderr)
	}
}

func TestContract_InvalidFC(t *testing.T) {
	requireCtlBin(t)

	res := runCtl(t, map[string]string{
		"IEC61850_HOST": "127.0.0.1",
		"IEC61850_PORT": "1",
	}, commandTimeout,
		"get", "object",
		"--object", "InteropLD/LLN0.Mod.stVal",
		"--fc", "NOTAFC",
		"--format", "json",
	)
	requireExitNonZero(t, res)
	if !strings.Contains(strings.ToLower(string(res.Stderr)), "functional constraint") &&
		!strings.Contains(strings.ToLower(string(res.Stderr)), "invalid") {
		t.Fatalf("stderr should mention invalid FC; got %q", res.Stderr)
	}
}

func TestContract_InvalidFormat(t *testing.T) {
	requireCtlBin(t)

	res := runCtl(t, map[string]string{
		"IEC61850_HOST": "127.0.0.1",
		"IEC61850_PORT": "1",
	}, commandTimeout,
		"get", "object",
		"--object", "InteropLD/LLN0.Mod.stVal",
		"--fc", "ST",
		"--format", "xml",
	)
	requireExitNonZero(t, res)
	out := string(res.Stdout) + string(res.Stderr)
	if strings.Contains(out, "Value:") {
		t.Fatalf("invalid format must not fall back to text output; got stdout=%q stderr=%q", res.Stdout, res.Stderr)
	}
	if !strings.Contains(strings.ToLower(string(res.Stderr)), "format") {
		t.Fatalf("stderr should mention format; got %q", res.Stderr)
	}
}

func TestContract_ClosedPort(t *testing.T) {
	requireCtlBin(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	res := runCtl(t, map[string]string{
		"IEC61850_HOST":     "127.0.0.1",
		"IEC61850_PORT":     strconv.Itoa(port),
		"IEC61850_IED_NAME": "InteropIED",
	}, commandTimeout, "list", "lds", "--format", "json")
	requireExitNonZero(t, res)
	if res.Duration > 20*time.Second {
		t.Fatalf("closed-port command took %s, want completion within ~20s", res.Duration)
	}
}

func TestContract_FlagOverridesEnvHost(t *testing.T) {
	requireCtlBin(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	// Env points at a nonsense host; flag must win and attempt 127.0.0.1:<closed>.
	res := runCtl(t, map[string]string{
		"IEC61850_HOST":     "env-host-must-not-be-used.invalid",
		"IEC61850_PORT":     strconv.Itoa(port),
		"IEC61850_IED_NAME": "InteropIED",
	}, commandTimeout,
		"--host", "127.0.0.1",
		"list", "lds", "--format", "json",
	)
	requireExitNonZero(t, res)
	combined := strings.ToLower(string(res.Stdout) + string(res.Stderr))
	if strings.Contains(combined, "env-host-must-not-be-used.invalid") {
		t.Fatalf("flag should override env host; output still mentions env host: %q", combined)
	}
}
