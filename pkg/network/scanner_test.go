// SPDX-License-Identifier: MIT

package network

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestParseHostSpec_SingleIP(t *testing.T) {
	ips, err := ParseHostSpec("10.0.0.5")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 || ips[0] != "10.0.0.5" {
		t.Fatalf("got %v", ips)
	}
}

func TestParseHostSpec_InvalidIP(t *testing.T) {
	_, err := ParseHostSpec("not-an-ip")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseHostSpec_CIDR(t *testing.T) {
	ips, err := ParseHostSpec("10.0.0.0/30")
	if err != nil {
		t.Fatal(err)
	}
	// /30 has 4 addresses; network+broadcast stripped → 2 hosts
	if len(ips) != 2 {
		t.Fatalf("len=%d want 2: %v", len(ips), ips)
	}
}

func TestParseHostSpec_InvalidCIDR(t *testing.T) {
	_, err := ParseHostSpec("10.0.0.0/99")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseHostSpec_RangeOctet(t *testing.T) {
	ips, err := ParseHostSpec("10.0.0.5-7")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 3 {
		t.Fatalf("len=%d want 3: %v", len(ips), ips)
	}
	if ips[0] != "10.0.0.5" || ips[2] != "10.0.0.7" {
		t.Fatalf("got %v", ips)
	}
}

func TestParseHostSpec_RangeFullIP(t *testing.T) {
	ips, err := ParseHostSpec("10.0.0.1-10.0.0.3")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 3 {
		t.Fatalf("len=%d: %v", len(ips), ips)
	}
}

func TestParseHostSpec_RangeErrors(t *testing.T) {
	cases := []string{
		"10.0.0.1-2-3",
		"bad-5",
		"10.0.0.1-999",
		"10.0.0.1-notip",
	}
	for _, c := range cases {
		if _, err := ParseHostSpec(c); err == nil {
			t.Fatalf("%q: expected error", c)
		}
	}
}

func TestInc(t *testing.T) {
	ip := net.ParseIP("10.0.0.255").To4()
	inc(ip)
	if ip.String() != "10.0.1.0" {
		t.Fatalf("got %s", ip)
	}
}

func TestParseARPOutput_Linux(t *testing.T) {
	linux := `Address                  HWtype  HWaddress           Flags Mask            Iface
10.0.0.5                 ether   aa:bb:cc:dd:ee:ff   C                     eth0
`
	mac := parseARPOutput(linux, "linux")
	if !strings.EqualFold(mac, "aa:bb:cc:dd:ee:ff") {
		t.Fatalf("mac=%q", mac)
	}
}

func TestParseARPOutput_Darwin(t *testing.T) {
	out := `? (10.0.0.5) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]`
	mac := parseARPOutput(out, "darwin")
	if !strings.EqualFold(mac, "aa:bb:cc:dd:ee:ff") {
		t.Fatalf("mac=%q", mac)
	}
}

func TestParseARPOutput_Windows(t *testing.T) {
	out := `  10.0.0.5          aa-bb-cc-dd-ee-ff     dynamic`
	mac := parseARPOutput(out, "windows")
	if !strings.EqualFold(mac, "aa:bb:cc:dd:ee:ff") {
		t.Fatalf("mac=%q", mac)
	}
}

func TestParseARPOutput_Empty(t *testing.T) {
	if parseARPOutput("no mac here", "linux") != "" {
		t.Fatal("expected empty")
	}
}

func TestScanTCP_LocalListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	ok := ScanTCP("127.0.0.1", port, 2*time.Second)
	if !ok {
		t.Fatal("expected reachable")
	}
	<-done

	if ScanTCP("127.0.0.1", port+1, 100*time.Millisecond) {
		t.Fatal("expected unreachable")
	}
}
