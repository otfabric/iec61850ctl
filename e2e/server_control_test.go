//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"testing"
)

// Reverse control journeys: mms-interop controller adapters → iec61850ctl server.
// Operation names match go-iec61850 interop Test*Server_Control_* expectations.
func TestReverse_ServerControl(t *testing.T) {
	for _, tc := range []struct {
		stack      string
		entrypoint string
	}{
		{stackServerLib, "libiec61850-ied-controller"},
		{stackServerBean, "iec61850bean-ied-controller"},
	} {
		tc := tc
		t.Run(tc.stack, func(t *testing.T) {
			requireReverseStack(t, tc.stack)
			requireCtlBin(t)

			image := resolveImageForReverseStack(tc.stack)
			if image == "" {
				t.Fatalf("no image for stack %q", tc.stack)
			}
			validateAdapterCapabilities(t, image, tc.entrypoint, tc.entrypoint)

			journeys := []struct {
				name string
				do   string
				ops  []string
			}{
				{
					name: "SPCSO1_Direct",
					do:   "SPCSO1",
					ops:  []string{"read-ctlmodel", "operate", "read-stval"},
				},
				{
					name: "SPCSO2_SBO",
					do:   "SPCSO2",
					ops:  []string{"read-ctlmodel", "select", "operate", "read-stval"},
				},
				{
					name: "SPCSO3_SBOw",
					do:   "SPCSO3",
					ops:  []string{"read-ctlmodel", "select-with-value", "operate", "read-stval"},
				},
			}

			for _, j := range journeys {
				j := j
				t.Run(j.name, func(t *testing.T) {
					srv := startCtlServer(t)
					res := runAdapterJSONL(t, tc.entrypoint, image, srv.hostPort,
						"--ctlval", "1",
						"--do", j.do,
					)
					if res.ExitErr != nil {
						logAdapterFailure(t, res, srv)
						t.Fatalf("controller exited with error: %v", res.ExitErr)
					}
					requireOps(t, res.Results, j.ops...)
				})
			}
		})
	}
}
