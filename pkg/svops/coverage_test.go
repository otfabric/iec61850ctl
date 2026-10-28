// SPDX-License-Identifier: MIT

package svops

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-iec61850/sv"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

func TestParseLayout(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"9-2LE", " le "} {
		l, err := ParseLayout(in)
		if err != nil || len(l) != len(sv.LELayout) {
			t.Fatalf("ParseLayout(%q) = %v, %v", in, l, err)
		}
	}
	l, err := ParseLayout("FLOAT32, quality,int32")
	if err != nil || len(l) != 3 || l[0] != sv.KindFloat32 || l[1] != sv.KindQuality || l[2] != sv.KindInt32 {
		t.Fatalf("generic layout = %v, %v", l, err)
	}
	if _, err := ParseLayout("FLOAT32,nope"); errs.ExitCode(err) != errs.ExitUsage || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("unknown encoding: %v", err)
	}
}

func TestLoadControlBlocksBadSCL(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "bad.cid")
	if err := os.WriteFile(p, []byte("<SCL"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadControlBlocks(p); errs.ExitCode(err) != errs.ExitUsage {
		t.Fatalf("bad SCL: %v", err)
	}
}

func TestZeroValuesAndInvalidKind(t *testing.T) {
	t.Parallel()
	vals := ZeroValues(sv.Layout{sv.KindBoolean, sv.KindTimestamp, sv.KindUint16, sv.KindFloat64, sv.KindQuality, sv.KindInt32})
	if len(vals) != 6 || vals[0].String() != "false" || !strings.HasPrefix(vals[1].String(), "1970-01-01") {
		t.Fatalf("zero values %v", vals)
	}
	if _, err := ParseKindValue("1", sv.KindInvalid); errs.ExitCode(err) != errs.ExitUsage {
		t.Fatalf("invalid kind: %v", err)
	}
}

func TestViewASDUAndText(t *testing.T) {
	t.Parallel()
	rx := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	a := sv.ASDU{SvID: "x", SmpCnt: 7, ConfRev: 1, Sample: []byte{0, 0, 0, 5}}
	f := &sv.Frame{Src: sv.MAC{2, 0, 0, 0, 0, 1}, Dst: sv.MAC{1, 0x0c, 0xcd, 4, 0, 1}, AppID: 0x4000, Simulation: true}
	f.PDU.ASDUs = []sv.ASDU{a, a}

	hex := ViewASDU("asdu", f, &a, 0, nil, 2, rx)
	if hex.NoASDU != 2 || hex.Sample == "" {
		t.Fatalf("hex view %+v", hex)
	}
	var out bytes.Buffer
	WriteASDUText(&out, hex)
	for _, want := range []string{"[sim,lost=2]", "sample 00000005", "asdu=1/2"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	WriteASDUText(&out, ViewASDU("asdu", f, &a, 1, sv.Layout{sv.KindInt32}, 0, rx))
	if !strings.Contains(out.String(), "[0] 5") {
		t.Fatalf("decoded view:\n%s", out.String())
	}

	out.Reset()
	WriteASDUText(&out, ASDUView{LE: &LEView{}})
	if !strings.Contains(out.String(), "quality=not good") {
		t.Fatalf("LE view:\n%s", out.String())
	}
}

func TestFilterAndCountErr(t *testing.T) {
	t.Parallel()
	f := Filter{Dst: sv.MAC{1, 0x0c, 0xcd, 4, 0, 1}}
	if f.matchFrame(&sv.Frame{Dst: sv.MAC{1, 0x0c, 0xcd, 4, 0, 2}}) {
		t.Fatal("dst filter matched another MAC")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := countErr(ctx, 5, 1, "ASDUs"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled count: %v", err)
	}
}

func TestSubscribeUndecodableAndCancel(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = pubPort.Send([]byte{0x01, 0x0c, 0xcd, 0x04, 0, 0, 2, 0, 0, 0, 0, 1, 0x88, 0xba, 0, 1})
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	var out, diag syncBuf
	sum, err := Subscribe(ctx, SubscribeConfig{
		Transport: rx, Subscription: loadCB(t, "MSVCB01").SubscriptionConfig(), Count: 1,
		Output: Output{Out: &out, Err: &diag},
	})
	if !errors.Is(err, context.Canceled) || sum.ASDUs != 0 {
		t.Fatalf("sum=%+v err=%v", sum, err)
	}
	if !strings.Contains(diag.String(), "undecodable") {
		t.Fatalf("diag=%s", diag.String())
	}
}
