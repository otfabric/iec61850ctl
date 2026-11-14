// SPDX-License-Identifier: MIT

package svops

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
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

// TestSubscribeUndecodableAndCancel runs in a synctest bubble: the
// frame is cancelled only once the subscriber has handled it and blocks
// for the next one, however slowly its goroutine starts (iec61850ctl#41).
func TestSubscribeUndecodableAndCancel(t *testing.T) {
	t.Parallel()
	sc := loadCB(t, "MSVCB01").SubscriptionConfig()
	synctest.Test(t, func(t *testing.T) {
		pubPort, rx := pair(t)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			_ = pubPort.Send([]byte{0x01, 0x0c, 0xcd, 0x04, 0, 0, 2, 0, 0, 0, 0, 1, 0x88, 0xba, 0, 1})
			synctest.Wait() // the subscriber has read the frame and waits for more
			cancel()
		}()
		var out, diag syncBuf
		sum, err := Subscribe(ctx, SubscribeConfig{
			Transport: rx, Subscription: sc, Count: 1,
			Output: Output{Out: &out, Err: &diag},
		})
		if !errors.Is(err, context.Canceled) || sum.ASDUs != 0 {
			t.Fatalf("sum=%+v err=%v", sum, err)
		}
		if !strings.Contains(diag.String(), "undecodable") {
			t.Fatalf("diag=%s", diag.String())
		}
	})
}

// slowStop is a transport whose Recv, once its context has ended, returns
// only after release is closed: a receive goroutine that is slow to stop.
type slowStop struct {
	*sv.HubPort
	release chan struct{}
}

func (s *slowStop) Recv(ctx context.Context) ([]byte, error) {
	f, err := s.HubPort.Recv(ctx)
	if err != nil && ctx.Err() != nil {
		<-s.release
	}
	return f, err
}

// TestSubscribeWaitsForReceiver checks that Subscribe returns only after
// the receive goroutine it started has stopped, so that nothing it
// reports (OnDecodeError) is written after the return.
func TestSubscribeWaitsForReceiver(t *testing.T) {
	t.Parallel()
	sc := loadCB(t, "MSVCB01").SubscriptionConfig()
	synctest.Test(t, func(t *testing.T) {
		_, rx := pair(t)
		tr := &slowStop{HubPort: rx, release: make(chan struct{})}
		release := sync.OnceFunc(func() { close(tr.release) })
		defer release()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := Subscribe(ctx, SubscribeConfig{
				Transport: tr, Subscription: sc, Count: 1,
				Output: Output{Out: &syncBuf{}, Err: &syncBuf{}},
			})
			done <- err
		}()
		synctest.Wait()
		cancel()
		synctest.Wait() // the receive goroutine is in Recv, waiting for release
		select {
		case err := <-done:
			t.Fatalf("Subscribe returned (%v) while its receive goroutine was running", err)
		default:
		}
		release()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	})
}
