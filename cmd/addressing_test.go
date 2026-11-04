// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

// captureInput runs args with a connectFunc that records the connection
// input and refuses to connect.
func captureInput(t *testing.T, args ...string) (client.ConnectionInput, result) {
	t.Helper()
	var got client.ConnectionInput
	prev := connectFunc
	connectFunc = func(_ context.Context, in client.ConnectionInput) (service.IEC61850Connection, error) {
		got = in
		return nil, errors.New("not connecting in this test")
	}
	t.Cleanup(func() { connectFunc = prev })
	r := runCLI(t, append([]string{"--host", "192.0.2.1"}, args...)...)
	return got, r
}

// TestAddressingFlagsDefault: without addressing flags the go-mms
// defaults apply (nil fields), which IEDs such as the REB500 of
// go-iec61850#28 accept.
func TestAddressingFlagsDefault(t *testing.T) {
	in, _ := captureInput(t, "list", "lds")
	if a := in.Addressing; a.LocalTSelector != nil || a.RemoteTSelector != nil || a.RemoteSSelector != nil ||
		a.RemotePSelector != nil || a.RemoteAPTitle != nil || a.RemoteAEQualifier != nil || a.MaxTPDULength != 0 {
		t.Fatalf("Addressing = %+v, want the zero value", a)
	}
}

func TestAddressingFlags(t *testing.T) {
	in, _ := captureInput(t, "--local-tsel", "0001", "--remote-tsel", "00000001", "--ssel", "01", "--psel", "01",
		"--ap-title", "1,3,9999,23", "--ae-qualifier", "23", "--tpdu-size", "2048", "list", "lds")
	a := in.Addressing
	if !bytes.Equal(a.LocalTSelector, []byte{0, 1}) || !bytes.Equal(a.RemoteTSelector, []byte{0, 0, 0, 1}) ||
		!bytes.Equal(a.RemoteSSelector, []byte{1}) || !bytes.Equal(a.RemotePSelector, []byte{1}) {
		t.Fatalf("selectors %+v", a)
	}
	if !a.RemoteAPTitle.Equal(mms.APTitle{1, 3, 9999, 23}) || a.RemoteAEQualifier == nil || *a.RemoteAEQualifier != 23 || a.MaxTPDULength != 2048 {
		t.Fatalf("AP-title %v AE-qualifier %v TPDU %d", a.RemoteAPTitle, a.RemoteAEQualifier, a.MaxTPDULength)
	}

	// An empty value omits the parameter; --ap-title alone keeps qualifier 12.
	in, _ = captureInput(t, "--remote-tsel=", "--ap-title", "1.1.1.999.1", "list", "lds")
	if in.Addressing.RemoteTSelector == nil || len(in.Addressing.RemoteTSelector) != 0 {
		t.Fatalf("--remote-tsel= gave %#v, want a non-nil empty selector", in.Addressing.RemoteTSelector)
	}
	if q := in.Addressing.RemoteAEQualifier; q == nil || *q != 12 {
		t.Fatalf("AE-qualifier %v, want 12", q)
	}
}

func TestAddressingFlagsInvalid(t *testing.T) {
	for _, args := range [][]string{
		{"--remote-tsel", "1"},
		{"--local-tsel", "zz"},
		{"--remote-tsel", "0000000001"},
		{"--ssel", "123"},
		{"--psel", "x"},
		{"--ap-title", "1,a"},
		{"--tpdu-size", "64"},
		{"--tpdu-size", "70000"},
	} {
		in, r := captureInput(t, append(args, "list", "lds")...)
		if r.code != errs.ExitUsage {
			t.Errorf("%v: exit %d, want %d (usage); stderr %q", args, r.code, errs.ExitUsage, r.stderr)
		}
		if in.Host != "" || strings.Contains(r.stderr, "Connecting to") {
			t.Errorf("%v: connected despite invalid addressing", args)
		}
	}
}
