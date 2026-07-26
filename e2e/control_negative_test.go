//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"strings"
	"testing"
)

// Phase 6 negative control / write journeys (isolated servers).
// Status-only ctlModel=0 is covered in unit tests (fixture has no status-only SPC).

func TestControl_NegativeConfirmationMismatch(t *testing.T) {
	for _, stack := range []string{stackLibIEC61850, stackIEC61850Bean} {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			requireStack(t, stack)
			requireCtlBin(t)
			readyTO := libReadyTimeout
			if stack == stackIEC61850Bean {
				readyTO = beanReadyTimeout
			}
			h, err := startDockerServer(stack, readyTO)
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			t.Cleanup(h.stop)

			// Operate true but confirm against a seeded-false SPS — mismatch.
			op := runCtl(t, h.ctlEnv(), commandTimeout,
				"control", "operate",
				"--object", "InteropLD/GGIO1.SPCSO1",
				"--value", "true",
				"--type", "bool",
				"--mode", "auto",
				"--confirm-ref", "InteropLD/GGIO1.SPS1.stVal[ST]",
				"--format", "json",
			)
			if op.ExitCode == 0 {
				logCommandFailure(t, op, h)
				t.Fatal("confirmation mismatch: expected non-zero exit")
			}
			var result struct {
				Status string `json:"status"`
			}
			if err := decodeExactJSON(op.Stdout, &result); err != nil {
				logCommandFailure(t, op, h)
				t.Fatalf("JSON-on-failure contract broken: %v", err)
			}
			if result.Status != "confirmation-mismatch" {
				t.Fatalf("status=%q want confirmation-mismatch", result.Status)
			}
		})
	}
}

func TestControl_NegativeModeMismatch(t *testing.T) {
	for _, stack := range []string{stackLibIEC61850, stackIEC61850Bean} {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			requireStack(t, stack)
			requireCtlBin(t)
			readyTO := libReadyTimeout
			if stack == stackIEC61850Bean {
				readyTO = beanReadyTimeout
			}
			h, err := startDockerServer(stack, readyTO)
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			t.Cleanup(h.stop)

			// SPCSO2 is SBO; force direct mode → mismatch reject.
			op := runCtl(t, h.ctlEnv(), commandTimeout,
				"control", "operate",
				"--object", "InteropLD/GGIO1.SPCSO2",
				"--value", "true",
				"--type", "bool",
				"--mode", "direct",
				"--format", "json",
			)
			if op.ExitCode == 0 {
				t.Fatal("mode mismatch: expected non-zero exit")
			}
			var result struct {
				Status string `json:"status"`
			}
			if err := decodeExactJSON(op.Stdout, &result); err != nil {
				logCommandFailure(t, op, h)
				t.Fatalf("decode: %v", err)
			}
			if result.Status != "failed" {
				t.Fatalf("status=%q want failed", result.Status)
			}
		})
	}
}

func TestWrite_NegativeRejectCO(t *testing.T) {
	requireCtlBin(t)
	// No server needed: CO reject is client-side validation.
	res := runCtl(t, map[string]string{
		"IEC61850_HOST":     "127.0.0.1",
		"IEC61850_PORT":     "1",
		"IEC61850_IED_NAME": "InteropIED",
	}, commandTimeout,
		"set", "object",
		"--object", "InteropLD/GGIO1.SPCSO1.Oper.ctlVal",
		"--fc", "CO",
		"--value", "true",
		"--type", "bool",
		"--format", "json",
	)
	if res.ExitCode == 0 {
		t.Fatal("FC=CO write: expected non-zero exit")
	}
	combined := strings.ToLower(string(res.Stderr) + string(res.Stdout))
	if !strings.Contains(combined, "control operate") &&
		!strings.Contains(combined, "fc=co") &&
		!strings.Contains(combined, "fc co") {
		t.Fatalf("expected CO reject guidance; stderr=%s stdout=%s", res.Stderr, res.Stdout)
	}
}
