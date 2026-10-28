// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// publishSafety holds the guard rails shared by the raw-Ethernet publish
// commands (goose publish, and sv publish to come): an explicit interface,
// an explicit confirmation, and frames that carry the protocol's test marker
// unless --live is given.
type publishSafety struct {
	protocol string  // "GOOSE"
	iface    *string // the command's --iface value
	marker   string  // flag that marks frames as test traffic ("test" for GOOSE, "simulation" for SV)
	marked   bool    // value of the marker flag
	live     bool
	yes      bool
}

// register adds the marker flag (default on), --live and --yes to cmd.
func (s *publishSafety) register(cmd *cobra.Command, markerUsage string) {
	f := cmd.Flags()
	f.BoolVar(&s.marked, s.marker, true, fmt.Sprintf("%s; --%s=false requires --live", markerUsage, s.marker))
	f.BoolVar(&s.live, "live", false, fmt.Sprintf("allow %s=false frames that subscribers act on", s.marker))
	f.BoolVar(&s.yes, "yes", false, "confirm that frames may be sent on --iface (required)")
}

// check enforces the guard rails before anything is sent.
func (s *publishSafety) check(cmd *cobra.Command) error {
	iface := strings.TrimSpace(*s.iface)
	if !cmd.Flags().Changed("iface") || iface == "" {
		return errs.Invalidf("%s publish needs an explicit --iface", strings.ToLower(s.protocol))
	}
	if !s.yes {
		return errs.Invalidf("refusing to publish %s on %s without --yes: frames reach every subscriber on that LAN", s.protocol, iface)
	}
	if !s.marked && !s.live {
		return errs.Invalidf("refusing to publish with %s=false without --live: subscribers would act on these frames", s.marker)
	}
	return nil
}

// warnLive prints the live-traffic warning when frames are not marked.
func (s *publishSafety) warnLive(w io.Writer) {
	if !s.marked {
		_, _ = fmt.Fprintf(w, "WARNING: publishing LIVE %s (%s=false) on %s\n", s.protocol, s.marker, *s.iface)
	}
}
