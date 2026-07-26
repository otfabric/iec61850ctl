//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"testing"
)

// Phase 4 forward write journeys use isolated servers and ING SetInt1.setVal[SP].

func TestWrite_ClientSetInt1(t *testing.T) {
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
				t.Fatalf("start isolated %s: %v", stack, err)
			}
			t.Cleanup(h.stop)

			set := runCtl(t, h.ctlEnv(), commandTimeout,
				"set", "object",
				"--object", "InteropLD/GGIO1.SetInt1.setVal",
				"--fc", "SP",
				"--value", "5",
				"--type", "int",
				"--verify",
				"--format", "json",
			)
			requireExitZero(t, set, h)

			var out struct {
				WriteOK      bool `json:"write_ok"`
				Verification *struct {
					Matched bool `json:"matched"`
					Value   any  `json:"value"`
				} `json:"verification"`
			}
			if err := decodeExactJSON(set.Stdout, &out); err != nil {
				logCommandFailure(t, set, h)
				t.Fatalf("decode set object: %v", err)
			}
			if !out.WriteOK || out.Verification == nil || !out.Verification.Matched {
				logCommandFailure(t, set, h)
				t.Fatalf("unexpected set result: %+v", out)
			}

			get := runCtl(t, h.ctlEnv(), commandTimeout,
				"get", "object",
				"--object", "InteropLD/GGIO1.SetInt1.setVal",
				"--fc", "SP",
				"--format", "json",
			)
			requireExitZero(t, get, h)
			var st struct {
				Value any `json:"value"`
			}
			if err := decodeExactJSON(get.Stdout, &st); err != nil {
				t.Fatalf("decode get: %v", err)
			}
			n, ok := asInt64(st.Value)
			if !ok || n != 5 {
				t.Fatalf("value=%#v want 5", st.Value)
			}
		})
	}
}
