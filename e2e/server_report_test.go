//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"testing"
)

// Reverse report journeys: mms-interop reporter adapters → iec61850ctl server.
// Operation names match go-iec61850 interop Test*Server_URCB_DataChange.
func TestReverse_ServerReport(t *testing.T) {
	for _, tc := range []struct {
		stack      string
		entrypoint string
	}{
		{stackServerLib, "libiec61850-ied-reporter"},
		{stackServerBean, "iec61850bean-ied-reporter"},
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

			srv := startCtlServer(t)
			// Fixture SPS1.stVal is false; reporter toggles to !initial.
			res := runAdapterJSONL(t, tc.entrypoint, image, srv.hostPort,
				"--sps1-initial", "0",
			)
			if res.ExitErr != nil {
				logAdapterFailure(t, res, srv)
				t.Fatalf("reporter exited with error: %v", res.ExitErr)
			}
			requireOps(t, res.Results,
				"get-rcb",
				"enable-rcb",
				"write",
				"receive-report",
				"disable-rcb",
				"conclude",
			)
		})
	}
}
