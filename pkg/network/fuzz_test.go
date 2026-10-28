// SPDX-License-Identifier: MIT

package network

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// FuzzParseHostSpec checks ParseHostSpec never panics, only fails with
// ErrInvalidInput, bounds the size of every expansion and only returns
// parsable IP addresses.
func FuzzParseHostSpec(f *testing.F) {
	for _, seed := range []string{
		"10.0.0.5", "::1", "", "-", "/", "not_an_ip",
		"10.0.0.0/24", "10.0.0.0/30", "10.0.0.0/31", "10.0.0.9/32", "10.1.0.0/16",
		"0.0.0.0/0", "::/0", "2001:db8::/126", "2001:db8::/64", "10.0.0.0/99",
		"10.0.0.5-25", "10.0.0.5-5", "10.0.0.0-255", "10.0.0.1-10.0.0.3",
		"10.0.0.254-10.0.1.1", "10.0.0.10-5", "10.0.0.1-2-3", "10.0.0.1-999",
		"10.0.0.1--1", "2001:db8::1-5", "::ffff:10.0.0.1-10.0.0.2",
		"255.255.255.255-0.0.0.0", "::1-::3", "::1-1.2.3.4",
		"10.0.0.0-10.0.1.0", // regression: used to return 257 addresses
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, spec string) {
		ips, err := ParseHostSpec(spec)
		if err != nil {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("ParseHostSpec(%q) error %v is not ErrInvalidInput", spec, err)
			}
			if ips != nil {
				t.Fatalf("ParseHostSpec(%q) returned addresses with an error", spec)
			}
			return
		}
		switch {
		case strings.Contains(spec, "/"):
			if len(ips) > 1<<maxCIDRHostBits {
				t.Fatalf("ParseHostSpec(%q) CIDR returned %d addresses", spec, len(ips))
			}
		case strings.Contains(spec, "-"):
			if len(ips) == 0 || len(ips) > 256 {
				t.Fatalf("ParseHostSpec(%q) range returned %d addresses (want 1..256)", spec, len(ips))
			}
		default:
			if len(ips) != 1 || ips[0] != spec {
				t.Fatalf("ParseHostSpec(%q) single = %v", spec, ips)
			}
		}
		for _, ip := range ips {
			if net.ParseIP(ip) == nil {
				t.Fatalf("ParseHostSpec(%q) returned unparsable %q", spec, ip)
			}
		}
	})
}
