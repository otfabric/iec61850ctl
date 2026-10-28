// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"errors"
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
		warn bool
	}{
		{nil, "sv publish needs an explicit --iface", false},
		{[]string{"--iface", " "}, "needs an explicit --iface", false},
		{[]string{"--iface", "eth0"}, "refusing to publish SV on eth0 without --yes", false},
		{[]string{"--iface", "eth0", "--yes"}, "", false},
		{[]string{"--iface", "eth0", "--yes", "--simulation=false"}, "simulation=false without --live", true},
		{[]string{"--iface", "eth0", "--yes", "--simulation=false", "--live"}, "", true},
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
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want) || !errors.Is(err, errs.ErrInvalidInput)):
			t.Fatalf("%v: got %v, want usage error containing %q", tc.args, err, tc.want)
		}
		// warnLive runs after a passing check; it only looks at the marker.
		var w bytes.Buffer
		s.warnLive(&w)
		if got := strings.Contains(w.String(), "WARNING: publishing LIVE SV (simulation=false) on eth0"); got != tc.warn {
			t.Fatalf("%v: warning %q, want shown=%v", tc.args, w.String(), tc.warn)
		}
	}
}
