// SPDX-License-Identifier: MIT

package network

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

func TestParseHostSpec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		spec      string
		wantLen   int
		wantFirst string
		wantLast  string
	}{
		{"single IPv4", "10.0.0.5", 1, "10.0.0.5", "10.0.0.5"},
		{"single IPv6", "::1", 1, "::1", "::1"},
		{"CIDR /30 strips network and broadcast", "10.0.0.0/30", 2, "10.0.0.1", "10.0.0.2"},
		{"CIDR /24", "192.168.1.0/24", 254, "192.168.1.1", "192.168.1.254"},
		{"CIDR host bits ignored", "192.168.1.77/30", 2, "192.168.1.77", "192.168.1.78"},
		{"CIDR /31 keeps both", "10.0.0.0/31", 2, "10.0.0.0", "10.0.0.1"},
		{"CIDR /32", "10.0.0.9/32", 1, "10.0.0.9", "10.0.0.9"},
		{"CIDR /16 is the largest accepted", "10.1.0.0/16", 65534, "10.1.0.1", "10.1.255.254"},
		{"CIDR IPv6 /126", "2001:db8::/126", 2, "2001:db8::1", "2001:db8::2"},
		{"range last octet", "10.0.0.5-7", 3, "10.0.0.5", "10.0.0.7"},
		{"range single address", "10.0.0.5-5", 1, "10.0.0.5", "10.0.0.5"},
		{"range full end IP", "10.0.0.1-10.0.0.3", 3, "10.0.0.1", "10.0.0.3"},
		{"range crosses octet", "10.0.0.254-10.0.1.1", 4, "10.0.0.254", "10.0.1.1"},
		{"range of exactly 256", "10.0.0.0-255", 256, "10.0.0.0", "10.0.0.255"},
		{"range IPv4-mapped start", "::ffff:10.0.0.1-10.0.0.2", 2, "10.0.0.1", "10.0.0.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ips, err := ParseHostSpec(tt.spec)
			if err != nil {
				t.Fatalf("ParseHostSpec(%q): %v", tt.spec, err)
			}
			if len(ips) != tt.wantLen {
				t.Fatalf("len=%d want %d", len(ips), tt.wantLen)
			}
			if ips[0] != tt.wantFirst || ips[len(ips)-1] != tt.wantLast {
				t.Fatalf("first/last = %s/%s want %s/%s", ips[0], ips[len(ips)-1], tt.wantFirst, tt.wantLast)
			}
			for _, ip := range ips {
				if net.ParseIP(ip) == nil {
					t.Fatalf("returned unparsable IP %q", ip)
				}
			}
		})
	}
}

func TestParseHostSpec_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		spec    string
		wantMsg string
	}{
		{"empty", "", "invalid IP address"},
		{"not an IP", "not_an_ip", "invalid IP address"},
		{"bad CIDR prefix", "10.0.0.0/99", "invalid CIDR notation"},
		{"bad CIDR address", "foo/24", "invalid CIDR notation"},
		{"CIDR /15 too large", "10.0.0.0/15", "too large"},
		{"CIDR /0 too large", "0.0.0.0/0", "too large"},
		{"CIDR IPv6 /64 too large", "2001:db8::/64", "too large"},
		{"CIDR IPv6 /0 too large", "::/0", "too large"},
		{"range three parts", "10.0.0.1-2-3", "invalid range format"},
		{"range bad start", "bad-5", "invalid start IP"},
		{"range octet too big", "10.0.0.1-999", "invalid end octet"},
		{"range octet negative", "10.0.0.1--1", "invalid range format"},
		{"range octet empty", "10.0.0.1-", "invalid end octet"},
		{"range octet not number", "10.0.0.1-x", "invalid end octet"},
		{"range bad end IP", "10.0.0.1-10.0.0.x", "invalid end IP"},
		{"range IPv6 with octet", "2001:db8::1-5", "only supports IPv4"},
		{"range reversed", "10.0.0.10-5", "range too large"},
		{"range 257 addresses", "10.0.0.0-10.0.1.0", "range too large"},
		{"range far end", "10.0.0.1-10.9.0.1", "range too large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ips, err := ParseHostSpec(tt.spec)
			if err == nil {
				t.Fatalf("ParseHostSpec(%q) = %d addresses, want error", tt.spec, len(ips))
			}
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Errorf("error %v is not ErrInvalidInput", err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error %q does not contain %q", err, tt.wantMsg)
			}
		})
	}
}

func TestInc(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"10.0.0.1", "10.0.0.2"},
		{"10.0.0.255", "10.0.1.0"},
		{"10.255.255.255", "11.0.0.0"},
		{"2001:db8::ffff", "2001:db8::1:0"},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.in)
		if v4 := ip.To4(); v4 != nil {
			ip = v4
		}
		inc(ip)
		if ip.String() != tt.want {
			t.Errorf("inc(%s) = %s want %s", tt.in, ip, tt.want)
		}
	}
	// Overflow wraps to all zeros.
	ip := net.IP{255, 255, 255, 255}
	inc(ip)
	if !ip.Equal(net.IPv4zero) {
		t.Errorf("inc(255.255.255.255) = %s", ip)
	}
}

func TestParseARPOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, platform, output, want string
	}{
		{"linux table", "linux", "Address  HWtype  HWaddress  Flags Mask  Iface\n10.0.0.5  ether  aa:bb:cc:dd:ee:ff  C  eth0\n", "aa:bb:cc:dd:ee:ff"},
		{"darwin", "darwin", "? (10.0.0.5) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]", "aa:bb:cc:dd:ee:ff"},
		{"darwin short octets via at", "darwin", "? (192.168.0.1) at 90:5c:44:22:a:f4 on en0 ifscope [ethernet]", "90:5c:44:22:a:f4"},
		{"darwin incomplete", "darwin", "? (169.254.169.254) at (incomplete) on en0 [ethernet]", ""},
		{"darwin no entry", "darwin", "10.0.0.5 (10.0.0.5) -- no entry", ""},
		{"blank lines skipped", "linux", "\n\n   \n10.0.0.5 ether 01:02:03:04:05:06 C eth0", "01:02:03:04:05:06"},
		{"at as last field", "linux", "foo at", ""},
		{"windows", "windows", "  10.0.0.5          aa-bb-cc-dd-ee-ff     dynamic", "aa:bb:cc:dd:ee:ff"},
		{"windows header only", "windows", "Interface: 10.0.0.1 --- 0x4\n  Internet Address      Physical Address      Type", ""},
		{"no mac", "linux", "no mac here", ""},
		{"empty", "linux", "", ""},
		{"unknown platform", "plan9", "10.0.0.5 ether aa:bb:cc:dd:ee:ff", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseARPOutput(tt.output, tt.platform); got != tt.want {
				t.Errorf("parseARPOutput = %q want %q", got, tt.want)
			}
		})
	}
}

// closedPort returns a TCP port on 127.0.0.1 that nothing is listening on.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func TestScanTCP(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	open := ln.Addr().(*net.TCPAddr).Port
	closed := closedPort(t)

	tests := []struct {
		name string
		host string
		port int
		want bool
	}{
		{"open port", "127.0.0.1", open, true},
		{"closed port", "127.0.0.1", closed, false},
		{"unresolvable host", "host.invalid", 102, false},
		{"bad port", "127.0.0.1", -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ScanTCP(tt.host, tt.port, 2*time.Second); got != tt.want {
				t.Errorf("ScanTCP(%s, %d) = %v want %v", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

// fakeARP installs a shell script named "arp" as the only entry on PATH. The
// script checks it was called as "arp -n <ip>" and then prints output and
// exits with code.
func fakeARP(t *testing.T, ip, output string, code int) {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\n"+
		"if [ \"$1\" != \"-n\" ] || [ \"$2\" != %q ]; then echo \"bad args: $*\"; exit 2; fi\n"+
		"printf '%%s\\n' %q\nexit %d\n", ip, output, code)
	if err := os.WriteFile(filepath.Join(dir, "arp"), []byte(script), 0o755); err != nil { //nolint:gosec // test helper must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// GetMACAddress shells out to arp; these tests replace it with a fake on PATH,
// so they cannot run in parallel (t.Setenv).
func TestGetMACAddress(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("fake arp script needs /bin/sh and the 'arp -n' form; GOOS=%s", runtime.GOOS)
	}
	tests := []struct {
		name       string
		output     string
		code       int
		wantMAC    string
		wantErr    error // sentinel, or nil for "any error"
		wantErrMsg string
	}{
		{name: "darwin entry", output: "? (10.0.0.5) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]", wantMAC: "aa:bb:cc:dd:ee:ff"},
		{name: "linux entry", output: "10.0.0.5 ether 00:11:22:33:44:55 C eth0", wantMAC: "00:11:22:33:44:55"},
		{name: "no MAC in output", output: "? (10.0.0.5) at (incomplete) on en0", wantErr: errs.ErrNotFound, wantErrMsg: "not found in ARP cache"},
		{name: "arp fails", output: "10.0.0.5 (10.0.0.5) -- no entry", code: 1, wantErrMsg: "failed to execute arp command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeARP(t, "10.0.0.5", tt.output, tt.code)
			mac, err := GetMACAddress("10.0.0.5")
			if tt.wantMAC != "" {
				if err != nil || mac != tt.wantMAC {
					t.Fatalf("GetMACAddress = %q, %v want %q", mac, err, tt.wantMAC)
				}
				return
			}
			if err == nil {
				t.Fatalf("GetMACAddress = %q, want error", mac)
			}
			if mac != "" {
				t.Errorf("mac = %q on error", mac)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error %v is not %v", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("error %q does not contain %q", err, tt.wantErrMsg)
			}
		})
	}

	t.Run("arp missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := GetMACAddress("10.0.0.5"); err == nil || !strings.Contains(err.Error(), "failed to execute arp command") {
			t.Fatalf("err = %v", err)
		}
	})
}
