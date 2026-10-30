// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/otfabric/go-iec61850/sv"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/stack/server"
	"github.com/otfabric/iec61850ctl/pkg/svops"
)

func svFixture() string {
	return filepath.Join(repoRoot(), "pkg", "svops", "testdata", "interop-sv.cid")
}

// withSVHub routes every SV transport the CLI opens to a new port on hub.
func withSVHub(t *testing.T, hub *sv.Hub) {
	t.Helper()
	prev := svOpen
	var mu sync.Mutex
	n := byte(0x10)
	svOpen = func(iface string, _ sv.TransportOptions) (sv.Transport, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		return hub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, n}), nil
	}
	t.Cleanup(func() { svOpen = prev })
}

// startSVServer starts an MMS server for the SV fixture that publishes its
// MSVCBs on a hub port.
func startSVServer(t *testing.T, hub *sv.Hub) *server.Running {
	t.Helper()
	port := hub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, 0x01})
	srv, err := server.Start(context.Background(), server.RunConfig{SclPath: svFixture(), Host: "127.0.0.1", SV: port})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close(); _ = port.Close() })
	return srv
}

func TestSVList(t *testing.T) {
	r := runCLI(t, "sv", "list", "--scl", svFixture())
	expectCode(t, r, errs.ExitOK)
	golden(t, "sv_list_text", r.stdout)

	r = runCLI(t, "sv", "list", "--scl", svFixture(), "--format", "json")
	expectCode(t, r, errs.ExitOK)
	golden(t, "sv_list_json", r.stdout)

	r = runCLI(t, "sv", "list", "--scl", gooseFixture())
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "No SV control blocks") {
		t.Fatal(r.stdout)
	}
	expectCode(t, runCLI(t, "sv", "list", "--scl", "nope.cid"), errs.ExitUsage)
	expectCode(t, runCLI(t, "sv", "list", "--scl", svFixture(), "--format", "csv"), errs.ExitUsage)
	expectCode(t, runCLI(t, "sv", "list"), errs.ExitUsage)
}

func TestSVPublishSafety(t *testing.T) {
	hub := sv.NewHub()
	withSVHub(t, hub)
	base := []string{"sv", "publish", "--scl", svFixture(), "--svcb", "MSVCB01", "--duration", "10ms"}
	cases := []struct {
		name string
		args []string
		msg  string
	}{
		{"no iface", append(base, "--yes"), "iface"},
		{"empty iface", append(base, "--iface", " ", "--yes"), "explicit --iface"},
		{"no yes", append(base, "--iface", "hub0"), "without --yes"},
		{"live frames without --live", append(base, "--iface", "hub0", "--yes", "--simulation=false"), "without --live"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := runCLI(t, c.args...)
			expectCode(t, r, errs.ExitUsage)
			if !strings.Contains(r.stderr, c.msg) {
				t.Fatalf("stderr %q lacks %q", r.stderr, c.msg)
			}
		})
	}
	if s := hub.Stats(); s.Sent != 0 {
		t.Fatalf("a refused publish sent %d frames", s.Sent)
	}
}

// collectSV records the SV frames on a hub port until stop is called.
func collectSV(t *testing.T, hub *sv.Hub) (stop func() []*sv.Frame) {
	t.Helper()
	port := hub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, 0xEE})
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var frames []*sv.Frame
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			raw, err := port.Recv(ctx)
			if err != nil {
				return
			}
			if f, err := sv.Decode(raw); err == nil {
				mu.Lock()
				frames = append(frames, f)
				mu.Unlock()
			}
		}
	}()
	return func() []*sv.Frame {
		cancel()
		<-done
		_ = port.Close()
		mu.Lock()
		defer mu.Unlock()
		return frames
	}
}

func TestSVPublishWaveform(t *testing.T) {
	hub := sv.NewHub()
	withSVHub(t, hub)
	stop := collectSV(t, hub)
	r := runCLI(t, "sv", "publish", "--iface", "hub0", "--yes", "--scl", svFixture(), "--svcb", "MSVCB01",
		"--waveform", "IA=100", "--waveform", "VA=230,50,90", "--value", "qVN=test", "--value", "IN=-5",
		"--smp-synch", "2", "--src", "02:00:00:00:00:aa", "--duration", "60ms", "--format", "jsonl")
	expectCode(t, r, errs.ExitOK)
	frames := stop()
	if len(frames) < 40 {
		t.Fatalf("collected %d frames\n%s", len(frames), r.stderr)
	}
	var res svops.PublishResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(r.stdout)), &res); err != nil || res.Rate != 4000 || res.Samples == 0 || !res.Simulation {
		t.Fatalf("summary %q: %v", r.stdout, err)
	}
	peak := 0.0
	for _, f := range frames {
		a := f.PDU.ASDUs[0]
		if !f.Simulation || a.SvID != "InteropSv92LE" || a.SmpSynch != 2 || f.Src.String() != "02:00:00:00:00:aa" {
			t.Fatalf("frame %+v asdu %+v", f, a)
		}
		var le sv.LESample
		if err := sv.DecodeLE(a.Sample, &le); err != nil {
			t.Fatal(err)
		}
		// Sample n is at n/4000 s: IA = 100 sin(2π·50·n/4000), VA has a +90° phase.
		want := 100 * math.Sin(2*math.Pi*50*float64(a.SmpCnt)/4000)
		if got := sv.Amps(le.Current[0].Value); math.Abs(got-want) > 0.01 {
			t.Fatalf("smpCnt %d: IA %v want %v", a.SmpCnt, got, want)
		}
		if le.Voltage[3].Q != sv.QualityTest || sv.Amps(le.Current[3].Value) != -0.005 {
			t.Fatalf("static channels %+v", le)
		}
		peak = math.Max(peak, sv.Volts(le.Voltage[0].Value))
	}
	if peak < 229 || peak > 230.01 {
		t.Fatalf("VA peak %v", peak)
	}

	for _, args := range [][]string{
		{"--waveform", "IA"},
		{"--waveform", "XX=1"},
		{"--value", "IA=abc"},
		{"--value", "99=1"},
		{"--src", "zz"},
		{"--duration", "-1s"},
		{"--frequency", "0"},
		{"--format", "xml"},
		{"--svcb", "nope"},
	} {
		a := append([]string{"sv", "publish", "--iface", "hub0", "--yes", "--scl", svFixture(), "--svcb", "MSVCB01", "--duration", "1ms"}, args...)
		r := runCLI(t, a...)
		want := errs.ExitUsage
		if args[1] == "nope" {
			want = errs.ExitNotFound
		}
		if r.code != want {
			t.Errorf("%v: exit %d want %d (%s)", args, r.code, want, r.stderr)
		}
	}
	// --waveform needs the 9-2LE layout.
	r = runCLI(t, "sv", "publish", "--iface", "hub0", "--yes", "--scl", svFixture(), "--svcb", "MSVCB02", "--waveform", "IA=1", "--duration", "1ms")
	expectCode(t, r, errs.ExitUsage)
	if !strings.Contains(r.stderr, "9-2LE") {
		t.Fatal(r.stderr)
	}
}

func TestSVPublishLiveGeneric(t *testing.T) {
	hub := sv.NewHub()
	withSVHub(t, hub)
	stop := collectSV(t, hub)
	r := runCLI(t, "sv", "publish", "--iface", "hub0", "--yes", "--live", "--simulation=false", "--scl", svFixture(),
		"--svcb", "MSVCB02", "--value", "InteropLD/MMXU1.TotW.mag.f=12.5", "--value", "1=invalid", "--duration", "30ms")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stderr, "LIVE") || !strings.Contains(r.stdout, "simulation=false") || !strings.Contains(r.stdout, "4 ASDU/frame") {
		t.Fatalf("stdout %q stderr %q", r.stdout, r.stderr)
	}
	frames := stop()
	if len(frames) == 0 || frames[0].Simulation || len(frames[0].PDU.ASDUs) != 4 {
		t.Fatalf("frames %d: %+v", len(frames), frames)
	}
	cb := loadSVCB(t, "MSVCB02")
	vals, err := cb.Layout.Decode(frames[0].PDU.ASDUs[0].Sample)
	if err != nil {
		t.Fatal(err)
	}
	if f, _ := vals[0].Float64(); f != 12.5 {
		t.Fatal(vals[0])
	}
	if q, _ := sv.QualityFromValue(vals[1]); q != sv.QualityInvalid {
		t.Fatalf("quality %#x", q)
	}
}

func loadSVCB(t *testing.T, name string) *sv.ControlBlock {
	t.Helper()
	cbs, _, err := svops.LoadControlBlocks(svFixture())
	if err != nil {
		t.Fatal(err)
	}
	cb, err := svops.SelectControlBlock(cbs, name)
	if err != nil {
		t.Fatal(err)
	}
	return cb
}

func TestSVSubscribeAndSniff(t *testing.T) {
	hub := sv.NewHub()
	withSVHub(t, hub)
	startSVServer(t, hub)

	r := runCLI(t, "sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "MSVCB01",
		"--sample-every", "100", "--stats-interval", "50ms", "--count", "400", "--duration", "10s")
	expectCode(t, r, errs.ExitOK)
	for _, want := range []string{"start", "I [A]:", "stats:", "summary:", "InteropSv92LE", "0x4000"} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("missing %q:\n%s", want, r.stdout)
		}
	}

	// The server sends datSet with the bare LD as domain.
	r = runCLI(t, "sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "InteropLD/MSVCB02",
		"--count", "1", "--duration", "300ms", "--format", "jsonl")
	expectCode(t, r, errs.ExitTimeout)
	if n := strings.Count(r.stdout, `"event":"invalid"`); n != 1 || !strings.Contains(r.stdout, "datSet") {
		t.Fatalf("%d invalid events:\n%.3000s", n, r.stdout)
	}
	r = runCLI(t, "sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "InteropLD/MSVCB02", "--bare-domain",
		"--sample-every", "1", "--stats-interval", "0", "--count", "8", "--duration", "10s", "--format", "jsonl")
	expectCode(t, r, errs.ExitOK)
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if !strings.Contains(r.stdout, `"event":"asdu"`) || !strings.Contains(lines[len(lines)-1], `"event":"summary"`) ||
		strings.Contains(r.stdout, `"event":"stats"`) {
		t.Fatalf("jsonl:\n%s", r.stdout)
	}

	// The server publishes without the simulation bit: --accept-simulation is harmless.
	r = runCLI(t, "sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "MSVCB01", "--accept-simulation",
		"--timeout", "50ms", "--count", "1", "--duration", "10s")
	expectCode(t, r, errs.ExitOK)

	r = runCLI(t, "sv", "sniff", "--iface", "hub0", "--appid", "0x4001", "--dst", "01:0c:cd:04:00:01",
		"--sv-id", "InteropSvGeneric", "--layout", "FLOAT32,Quality,INT32U,Quality,INT32", "--count", "4", "--duration", "10s", "--format", "jsonl")
	expectCode(t, r, errs.ExitOK)
	lines = strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 5 || !strings.Contains(lines[0], `"app_id":"0x4001"`) || !strings.Contains(lines[0], `"values":[`) {
		t.Fatalf("sniff:\n%s", r.stdout)
	}

	r = runCLI(t, "sv", "sniff", "--iface", "hub0", "--scl", svFixture(), "--svcb", "MSVCB01", "--count", "2", "--duration", "10s")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "I [A]:") || !strings.Contains(r.stdout, "ASDUs: 2 in 2 frame(s)") {
		t.Fatal(r.stdout)
	}
	r = runCLI(t, "sv", "sniff", "--iface", "hub0", "--sample-every", "3", "--count", "6", "--duration", "10s")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "sample ") || !strings.Contains(r.stdout, "printed 2") {
		t.Fatal(r.stdout)
	}

	for _, args := range [][]string{
		{"sv", "sniff", "--iface", "hub0", "--appid", "zz"},
		{"sv", "sniff", "--iface", "hub0", "--dst", "zz"},
		{"sv", "sniff", "--iface", "hub0", "--layout", "INT128"},
		{"sv", "sniff", "--iface", "hub0", "--svcb", "MSVCB01"},
		{"sv", "sniff", "--iface", "hub0", "--scl", "missing.cid", "--svcb", "MSVCB01"},
		{"sv", "sniff", "--iface", "hub0", "--count", "-1"},
		{"sv", "sniff", "--iface", "hub0", "--sample-every", "-1"},
		{"sv", "sniff", "--iface", "hub0", "--format", "json"},
		{"sv", "sniff", "--iface", ""},
		{"sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "MSVCB01", "--duration", "-1s"},
		{"sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "MSVCB01", "--stats-interval", "-1s"},
		{"sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "MSVCB01", "--timeout", "-1s"},
		{"sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "MSVCB01", "--format", "csv"},
		{"sv", "subscribe", "--iface", "hub0", "--scl", "missing.cid", "--svcb", "MSVCB01"},
	} {
		if r := runCLI(t, args...); r.code != errs.ExitUsage {
			t.Errorf("%v: exit %d (%s)", args, r.code, r.stderr)
		}
	}
	expectCode(t, runCLI(t, "sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "zz"), errs.ExitNotFound)
	expectCode(t, runCLI(t, "sv", "sniff", "--iface", "hub0", "--scl", svFixture(), "--svcb", "zz"), errs.ExitNotFound)
}

func TestSVSubscribeTimeout(t *testing.T) {
	hub := sv.NewHub()
	withSVHub(t, hub)
	// Nothing publishes: --count is not reached within --duration.
	r := runCLI(t, "sv", "subscribe", "--iface", "hub0", "--scl", svFixture(), "--svcb", "MSVCB01", "--count", "1", "--duration", "100ms")
	expectCode(t, r, errs.ExitTimeout)
	if !strings.Contains(r.stdout, "(no stream)") {
		t.Fatal(r.stdout)
	}
}

func TestSVTransportErrors(t *testing.T) {
	prev := svOpen
	t.Cleanup(func() { svOpen = prev })
	cases := []struct {
		err  error
		code int
	}{
		{sv.ErrPermission, errs.ExitPermission},
		{sv.ErrInterfaceNotFound, errs.ExitNotFound},
		{sv.ErrUnsupportedPlatform, errs.ExitUnsupported},
	}
	for _, c := range cases {
		svOpen = func(string, sv.TransportOptions) (sv.Transport, error) {
			return nil, fmt.Errorf("open: %w", c.err)
		}
		for _, args := range [][]string{
			{"sv", "sniff", "--iface", "eth9"},
			{"sv", "subscribe", "--iface", "eth9", "--scl", svFixture(), "--svcb", "MSVCB01"},
			{"sv", "publish", "--iface", "eth9", "--yes", "--scl", svFixture(), "--svcb", "MSVCB01"},
		} {
			r := runCLI(t, args...)
			if r.code != c.code || !strings.Contains(r.stderr, "eth9") {
				t.Errorf("%v with %v: exit %d want %d (%s)", args[:2], c.err, r.code, c.code, r.stderr)
			}
		}
	}
}

func TestSVSvcb(t *testing.T) {
	hub := sv.NewHub()
	srv := startSVServer(t, hub)
	args := func(a ...string) []string { return gooseSrvArgs(srv, append([]string{"sv", "svcb"}, a...)...) }

	r := runCLI(t, args("get", "--ld", "InteropLD", "--svcb", "MSVCB01")...)
	expectCode(t, r, errs.ExitOK)
	golden(t, "sv_svcb_get_text", r.stdout)

	r = runCLI(t, args("get", "--ld", "InteropLD", "--svcb", "LLN0.MS.MSVCB02", "--format", "json")...)
	expectCode(t, r, errs.ExitOK)
	var v svcbView
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil || !v.SvEna || v.AppID != "0x4001" || v.SmpRate != 1000 || v.SmpMod != "SmpPerSec" {
		t.Fatalf("json %q: %v", r.stdout, err)
	}

	// Attributes other than SvEna are writable only while disabled.
	expectCode(t, runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--msv-id", "X")...), errs.ExitRejected)

	r = runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--disable", "--format", "json")...)
	expectCode(t, r, errs.ExitOK)
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil || v.SvEna {
		t.Fatalf("after disable: %q", r.stdout)
	}
	r = runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--msv-id", "renamed", "--opt-flds", "refresh-time,sample-rate",
		"--smp-rate", "80", "--smp-mod", "SmpPerPeriod", "--enable")...)
	expectCode(t, r, errs.ExitOK)
	for _, want := range []string{"MsvID:      renamed", "SvEna:      true", "OptFlds:    refresh-time,sample-rate", "SmpRate:    80 SmpPerPeriod"} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("after rename, missing %q:\n%s", want, r.stdout)
		}
	}
	// SvEna is written last: disable in its own call before changing attributes.
	expectCode(t, runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--disable")...), errs.ExitOK)
	r = runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--opt-flds", "none", "--dat-set", "InteropLD/LLN0$PhsMeas1")...)
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "OptFlds:    none") {
		t.Fatal(r.stdout)
	}

	expectCode(t, runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01")...), errs.ExitUsage)
	expectCode(t, runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--enable", "--disable")...), errs.ExitUsage)
	expectCode(t, runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--opt-flds", "bogus")...), errs.ExitUsage)
	expectCode(t, runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--smp-mod", "hourly")...), errs.ExitUsage)
	expectCode(t, runCLI(t, args("get", "--ld", "InteropLD", "--svcb", "MSVCB01", "--format", "yaml")...), errs.ExitUsage)
	expectCode(t, runCLI(t, args("set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--enable", "--format", "yaml")...), errs.ExitUsage)
	if r := runCLI(t, args("get", "--ld", "InteropLD", "--svcb", "nope")...); r.code == errs.ExitOK {
		t.Fatal("unknown MSVCB read succeeded")
	}
	expectCode(t, runCLI(t, "--port", "1", "--host", "127.0.0.1", "--connect-timeout", "200ms", "sv", "svcb", "get", "--ld", "X", "--svcb", "s"), errs.ExitConnection)
	expectCode(t, runCLI(t, "--port", "1", "--host", "127.0.0.1", "--connect-timeout", "200ms", "sv", "svcb", "set", "--ld", "X", "--svcb", "s", "--enable"), errs.ExitConnection)

	// A connection without MSVCB support.
	withFakeConn(t, emptyConn{})
	expectCode(t, runCLI(t, "--host", "fake", "sv", "svcb", "get", "--ld", "X", "--svcb", "s"), errs.ExitUnsupported)
	expectCode(t, runCLI(t, "--host", "fake", "sv", "svcb", "set", "--ld", "X", "--svcb", "s", "--enable"), errs.ExitUnsupported)
}

func TestSvcbItem(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"MSVCB01": "LLN0$MS$MSVCB01", "LLN0$MS$m": "LLN0$MS$m", "LLN0.MS.m": "LLN0$MS$m"} {
		if got := svcbItem(in); got != want {
			t.Errorf("svcbItem(%q)=%q", in, got)
		}
	}
}

// A server without an SV publisher accepts the SvEna write but reads back
// false: set --enable must exit 7, not 0.
func TestSVSvcbEnableNotPublishing(t *testing.T) {
	srv, err := server.Start(context.Background(), server.RunConfig{SclPath: svFixture(), Host: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	r := runCLI(t, gooseSrvArgs(srv, "sv", "svcb", "set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--disable")...)
	expectCode(t, r, errs.ExitOK)
	r = runCLI(t, gooseSrvArgs(srv, "sv", "svcb", "set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--enable", "--format", "json")...)
	expectCode(t, r, errs.ExitMismatch)
	if !strings.Contains(r.stdout, `"sv_ena": false`) || !strings.Contains(r.stderr, "SvEna reads back false") {
		t.Fatalf("stdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
}
