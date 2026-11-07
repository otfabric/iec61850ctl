// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// TestPublishSafetySimulationMarker checks the guard rails with the SV
// configuration: the simulation bit is the test marker.
func TestPublishSafetySimulationMarker(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string // error substring; "" = allowed
		code int    // exit code of the refusal
		warn bool
	}{
		{nil, "sv publish needs an explicit --iface", errs.ExitUsage, false},
		{[]string{"--iface", " "}, "needs an explicit --iface", errs.ExitUsage, false},
		{[]string{"--iface", "eth0"}, "refusing to publish SV on eth0 without --yes", errs.ExitSafety, false},
		{[]string{"--iface", "eth0", "--yes"}, "", errs.ExitOK, false},
		{[]string{"--iface", "eth0", "--yes", "--simulation=false"}, "simulation=false without --live", errs.ExitSafety, true},
		{[]string{"--iface", "eth0", "--yes", "--simulation=false", "--live"}, "", errs.ExitOK, true},
	} {
		var iface string
		s := publishSafety{protocol: "SV", iface: &iface, marker: "simulation"}
		c := &cobra.Command{Use: "publish"}
		c.Flags().StringVar(&iface, "iface", "", "")
		s.register(c, "set the simulation bit")
		if err := c.ParseFlags(tc.args); err != nil {
			t.Fatalf("%v: parse: %v", tc.args, err)
		}
		err := s.check(c)
		switch {
		case tc.want == "" && err != nil:
			t.Fatalf("%v: unexpected error %v", tc.args, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Fatalf("%v: got %v, want error containing %q", tc.args, err, tc.want)
		case errs.ExitCode(err) != tc.code:
			t.Fatalf("%v: exit %d, want %d (%v)", tc.args, errs.ExitCode(err), tc.code, err)
		}
		// warnLive runs after a passing check; it only looks at the marker.
		var w bytes.Buffer
		s.warnLive(&w)
		if got := strings.Contains(w.String(), "WARNING: publishing LIVE SV (simulation=false) on eth0"); got != tc.warn {
			t.Fatalf("%v: warning %q, want shown=%v", tc.args, w.String(), tc.warn)
		}
	}
}
