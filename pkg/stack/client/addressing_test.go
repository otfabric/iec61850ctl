// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"context"
	"errors"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
)

// dialOpts returns the DialOptions newConnection passes to the dialer.
func dialOpts(t *testing.T, a Addressing) iec61850.DialOptions {
	t.Helper()
	var got iec61850.DialOptions
	stop := errors.New("stop")
	_, _ = newConnection(context.Background(), ConnectionInput{Host: "h", Port: 102, Addressing: a},
		func(_ context.Context, _ string, opts iec61850.DialOptions) (*iec61850.Client, error) {
			got = opts
			return nil, stop
		})
	return got
}

func TestAddressingApply(t *testing.T) {
	t.Parallel()
	// The zero value leaves every field nil or zero: go-mms applies its
	// defaults (T-selector 0001, TPDU 1024, AP-title 1.1.1.999.1, ...).
	d := dialOpts(t, Addressing{})
	if d.MMS.Transport.LocalTSelector != nil || d.MMS.Transport.RemoteTSelector != nil ||
		d.MMS.ISO.RemoteSSelector != nil || d.MMS.ISO.RemotePSelector != nil ||
		d.MMS.ISO.RemoteAPTitle != nil || d.MMS.ISO.RemoteAEQualifier != 0 || d.MaxTPDULength != 0 {
		t.Fatalf("zero Addressing changed DialOptions: %+v", d)
	}

	q := 23
	d = dialOpts(t, Addressing{
		LocalTSelector: []byte{0, 1}, RemoteTSelector: []byte{0, 0, 0, 1},
		RemoteSSelector: []byte{1}, RemotePSelector: []byte{}, // empty: omit
		RemoteAPTitle: mms.APTitle{1, 3, 9999, 23}, RemoteAEQualifier: &q, MaxTPDULength: 2048,
	})
	if !bytes.Equal(d.MMS.Transport.LocalTSelector, []byte{0, 1}) || !bytes.Equal(d.MMS.Transport.RemoteTSelector, []byte{0, 0, 0, 1}) ||
		!bytes.Equal(d.MMS.ISO.RemoteSSelector, []byte{1}) || d.MMS.ISO.RemotePSelector == nil || len(d.MMS.ISO.RemotePSelector) != 0 {
		t.Fatalf("selectors %+v", d.MMS)
	}
	if !d.MMS.ISO.RemoteAPTitle.Equal(mms.APTitle{1, 3, 9999, 23}) || d.MMS.ISO.RemoteAEQualifier != 23 || d.MaxTPDULength != 2048 {
		t.Fatalf("AP-title %v AE-qualifier %d TPDU %d", d.MMS.ISO.RemoteAPTitle, d.MMS.ISO.RemoteAEQualifier, d.MaxTPDULength)
	}
}
