// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/otfabric/go-iec61850/goose"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/gooseops"
	"github.com/otfabric/iec61850ctl/pkg/stack/server"
)

func gooseFixture() string {
	return filepath.Join(repoRoot(), "pkg", "gooseops", "testdata", "interop-goose.cid")
}

// withHub routes every GOOSE transport the CLI opens to a new port on hub.
func withHub(t *testing.T, hub *goose.Hub) {
	t.Helper()
	prev := gooseOpen
	var mu sync.Mutex
	n := byte(0x10)
	gooseOpen = func(iface string, _ goose.TransportOptions) (goose.Transport, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		return hub.NewPort(goose.MAC{0x02, 0, 0, 0, 0, n}), nil
	}
	t.Cleanup(func() { gooseOpen = prev })
}

// startGooseServer starts an MMS server for the GOOSE fixture that
// publishes its GoCBs on a hub port (bare-LD references).
func startGooseServer(t *testing.T, hub *goose.Hub) *server.Running {
	t.Helper()
	port := hub.NewPort(goose.MAC{0x02, 0, 0, 0, 0, 0x01})
	srv, err := server.Start(context.Background(), server.RunConfig{SclPath: gooseFixture(), Host: "127.0.0.1", GOOSE: port})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close(); _ = port.Close() })
	return srv
}

func gooseSrvArgs(srv *server.Running, args ...string) []string {
	_, p, _ := strings.Cut(srv.Addr(), ":")
	return append([]string{"--host", "127.0.0.1", "--port", p}, args...)
}

func TestGooseList(t *testing.T) {
	r := runCLI(t, "goose", "list", "--scl", gooseFixture())
	expectCode(t, r, errs.ExitOK)
	golden(t, "goose_list_text", r.stdout)

	r = runCLI(t, "goose", "list", "--scl", gooseFixture(), "--format", "json")
	expectCode(t, r, errs.ExitOK)
	golden(t, "goose_list_json", r.stdout)

	r = runCLI(t, "goose", "list", "--scl", fixture("interop.icd"))
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "No GOOSE control blocks") {
		t.Fatal(r.stdout)
	}
	expectCode(t, runCLI(t, "goose", "list", "--scl", "nope.cid"), errs.ExitUsage)
	expectCode(t, runCLI(t, "goose", "list", "--scl", gooseFixture(), "--format", "csv"), errs.ExitUsage)
	expectCode(t, runCLI(t, "goose", "list"), errs.ExitUsage)
}

func TestGoosePublishSafety(t *testing.T) {
	hub := goose.NewHub()
	withHub(t, hub)
	base := []string{"goose", "publish", "--scl", gooseFixture(), "--gocb", "gcbBool", "--duration", "10ms"}
	cases := []struct {
		name string
		args []string
		msg  string
	}{
		{"no iface", append(base, "--yes"), "iface"},
		{"empty iface", append(base, "--iface", " ", "--yes"), "explicit --iface"},
		{"no yes", append(base, "--iface", "hub0"), "without --yes"},
		{"live frames without --live", append(base, "--iface", "hub0", "--yes", "--test=false"), "without --live"},
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

// collect subscribes to the gcbBool stream on its own hub port in the
// background and returns the frames it saw once stop is called.
func collect(t *testing.T, hub *goose.Hub) (stop func() []*goose.Frame) {
	t.Helper()
	port := hub.NewPort(goose.MAC{0x02, 0, 0, 0, 0, 0xEE})
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var frames []*goose.Frame
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			raw, err := port.Recv(ctx)
			if err != nil {
				return
			}
			if f, err := goose.Decode(raw); err == nil {
				mu.Lock()
				frames = append(frames, f)
				mu.Unlock()
			}
		}
	}()
	return func() []*goose.Frame {
		cancel()
		<-done
		_ = port.Close()
		mu.Lock()
		defer mu.Unlock()
		return frames
	}
}

func TestGoosePublish(t *testing.T) {
	hub := goose.NewHub()
	withHub(t, hub)
	stop := collect(t, hub)
	r := runCLI(t, "goose", "publish", "--iface", "hub0", "--yes", "--scl", gooseFixture(), "--gocb", "gcbBool",
		"--value", "0=true", "--changes", "2", "--interval", "5ms", "--duration", "30ms", "--src", "02:00:00:00:00:aa",
		"--format", "jsonl")
	expectCode(t, r, errs.ExitOK)
	frames := stop()
	if len(frames) < 3 {
		t.Fatalf("collected %d frames\n%s", len(frames), r.stderr)
	}
	last := frames[len(frames)-1]
	if !last.PDU.Test || last.PDU.StNum != 3 || last.PDU.GocbRef != "InteropIEDInteropLD/LLN0$GO$gcbBool" ||
		last.Src.String() != "02:00:00:00:00:aa" {
		t.Fatalf("frame %+v", last.PDU)
	}
	if b, _ := last.PDU.AllData[0].Bool(); !b {
		t.Fatal("--value override not published")
	}
	var res gooseops.PublishResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(r.stdout)), &res); err != nil || res.StNum != 3 {
		t.Fatalf("summary %q: %v", r.stdout, err)
	}

	// Live frames need both --test=false and --live; references can use the bare LD domain.
	stop = collect(t, hub)
	r = runCLI(t, "goose", "publish", "--iface", "hub0", "--yes", "--live", "--test=false", "--bare-domain",
		"--scl", gooseFixture(), "--gocb", "gcbBool", "--duration", "20ms")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stderr, "LIVE") || !strings.Contains(r.stdout, "test=false") {
		t.Fatalf("stdout %q stderr %q", r.stdout, r.stderr)
	}
	frames = stop()
	if len(frames) == 0 || frames[0].PDU.Test || frames[0].PDU.GocbRef != "InteropLD/LLN0$GO$gcbBool" {
		t.Fatalf("live frames %d: %+v", len(frames), frames)
	}

	for _, args := range [][]string{
		{"--value", "0=maybe"},
		{"--value", "9=true"},
		{"--src", "zz"},
		{"--changes", "-1"},
		{"--format", "xml"},
		{"--gocb", "nope"},
	} {
		a := append([]string{"goose", "publish", "--iface", "hub0", "--yes", "--scl", gooseFixture(), "--gocb", "gcbBool", "--duration", "1ms"}, args...)
		r := runCLI(t, a...)
		want := errs.ExitUsage
		if args[1] == "nope" {
			want = errs.ExitNotFound
		}
		if r.code != want {
			t.Errorf("%v: exit %d want %d (%s)", args, r.code, want, r.stderr)
		}
	}
}

func TestGooseSubscribeAndSniff(t *testing.T) {
	hub := goose.NewHub()
	withHub(t, hub)
	startGooseServer(t, hub)

	r := runCLI(t, "goose", "subscribe", "--iface", "hub0", "--scl", gooseFixture(), "--gocb", "gcbBool",
		"--bare-domain", "--count", "2", "--duration", "10s")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "InteropLD/LLN0$GO$gcbBool") || !strings.Contains(r.stdout, "status=ok") {
		t.Fatalf("stdout:\n%s", r.stdout)
	}

	r = runCLI(t, "goose", "subscribe", "--iface", "hub0", "--scl", gooseFixture(), "--gocb", "InteropLD/gcbAll",
		"--bare-domain", "--only-changes", "--accept-simulation", "--count", "1", "--duration", "10s", "--format", "jsonl")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, `"event":"state"`) || !strings.Contains(r.stdout, `"event":"summary"`) {
		t.Fatalf("jsonl:\n%s", r.stdout)
	}

	// Without --bare-domain the IED-prefixed gocbRef filter matches nothing.
	r = runCLI(t, "goose", "subscribe", "--iface", "hub0", "--scl", gooseFixture(), "--gocb", "gcbBool",
		"--count", "1", "--duration", "300ms")
	expectCode(t, r, errs.ExitTimeout)

	r = runCLI(t, "goose", "sniff", "--iface", "hub0", "--appid", "0x1002", "--dst", "01:0c:cd:01:00:02",
		"--gocb-ref", "InteropLD/LLN0$GO$gcbBool", "--count", "2", "--duration", "10s", "--format", "jsonl")
	expectCode(t, r, errs.ExitOK)
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"app_id":"0x1002"`) {
		t.Fatalf("sniff:\n%s", r.stdout)
	}

	r = runCLI(t, "goose", "sniff", "--iface", "hub0", "--count", "1", "--duration", "10s")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "Frames: 1") {
		t.Fatal(r.stdout)
	}

	for _, args := range [][]string{
		{"goose", "sniff", "--iface", "hub0", "--appid", "zz"},
		{"goose", "sniff", "--iface", "hub0", "--dst", "zz"},
		{"goose", "sniff", "--iface", "hub0", "--count", "-1"},
		{"goose", "sniff", "--iface", "hub0", "--duration", "-1s"},
		{"goose", "sniff", "--iface", "hub0", "--format", "json"},
		{"goose", "sniff", "--iface", ""},
		{"goose", "subscribe", "--iface", "hub0", "--scl", gooseFixture(), "--gocb", "gcbBool", "--count", "-2"},
		{"goose", "subscribe", "--iface", "hub0", "--scl", gooseFixture(), "--gocb", "gcbBool", "--format", "csv"},
		{"goose", "subscribe", "--iface", "hub0", "--scl", "missing.cid", "--gocb", "gcbBool"},
	} {
		if r := runCLI(t, args...); r.code != errs.ExitUsage {
			t.Errorf("%v: exit %d (%s)", args, r.code, r.stderr)
		}
	}
	expectCode(t, runCLI(t, "goose", "subscribe", "--iface", "hub0", "--scl", gooseFixture(), "--gocb", "zz"), errs.ExitNotFound)
}

func TestGooseTransportErrors(t *testing.T) {
	prev := gooseOpen
	t.Cleanup(func() { gooseOpen = prev })
	cases := []struct {
		err  error
		code int
	}{
		{goose.ErrPermission, errs.ExitPermission},
		{goose.ErrInterfaceNotFound, errs.ExitNotFound},
		{goose.ErrUnsupportedPlatform, errs.ExitUnsupported},
	}
	for _, c := range cases {
		gooseOpen = func(string, goose.TransportOptions) (goose.Transport, error) {
			return nil, fmt.Errorf("open: %w", c.err)
		}
		for _, args := range [][]string{
			{"goose", "sniff", "--iface", "eth9"},
			{"goose", "subscribe", "--iface", "eth9", "--scl", gooseFixture(), "--gocb", "gcbBool"},
			{"goose", "publish", "--iface", "eth9", "--yes", "--scl", gooseFixture(), "--gocb", "gcbBool"},
		} {
			r := runCLI(t, args...)
			if r.code != c.code || !strings.Contains(r.stderr, "eth9") {
				t.Errorf("%v with %v: exit %d want %d (%s)", args[:2], c.err, r.code, c.code, r.stderr)
			}
		}
	}
}

func TestGooseGoCB(t *testing.T) {
	hub := goose.NewHub()
	srv := startGooseServer(t, hub)

	r := runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "get", "--ld", "InteropLD", "--gocb", "gcbBool")...)
	expectCode(t, r, errs.ExitOK)
	golden(t, "goose_gocb_get_text", r.stdout)

	r = runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "get", "--ld", "InteropLD", "--gocb", "LLN0.GO.gcbBool", "--format", "json")...)
	expectCode(t, r, errs.ExitOK)
	var v gocbView
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil || !v.GoEna || v.AppID != "0x1002" || v.ConfRev != 1 {
		t.Fatalf("json %q: %v", r.stdout, err)
	}

	// GoID is writable only while disabled.
	r = runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--go-id", "X")...)
	expectCode(t, r, errs.ExitRejected)

	r = runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--disable", "--format", "json")...)
	expectCode(t, r, errs.ExitOK)
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil || v.GoEna {
		t.Fatalf("after disable: %q", r.stdout)
	}
	r = runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--go-id", "renamed", "--enable")...)
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "GoID:       renamed") || !strings.Contains(r.stdout, "GoEna:      true") {
		t.Fatalf("after rename:\n%s", r.stdout)
	}

	expectCode(t, runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool")...), errs.ExitUsage)
	expectCode(t, runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--enable", "--disable")...), errs.ExitUsage)
	expectCode(t, runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "get", "--ld", "InteropLD", "--gocb", "gcbBool", "--format", "yaml")...), errs.ExitUsage)
	expectCode(t, runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--enable", "--format", "yaml")...), errs.ExitUsage)
	if r := runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "get", "--ld", "InteropLD", "--gocb", "nope")...); r.code == errs.ExitOK {
		t.Fatal("unknown GoCB read succeeded")
	}
	expectCode(t, runCLI(t, "--port", "1", "--host", "127.0.0.1", "--connect-timeout", "200ms", "goose", "gocb", "get", "--ld", "X", "--gocb", "g"), errs.ExitConnection)
	expectCode(t, runCLI(t, "--port", "1", "--host", "127.0.0.1", "--connect-timeout", "200ms", "goose", "gocb", "set", "--ld", "X", "--gocb", "g", "--enable"), errs.ExitConnection)

	// A connection without GoCB support.
	withFakeConn(t, emptyConn{})
	expectCode(t, runCLI(t, "--host", "fake", "goose", "gocb", "get", "--ld", "X", "--gocb", "g"), errs.ExitUnsupported)
	expectCode(t, runCLI(t, "--host", "fake", "goose", "gocb", "set", "--ld", "X", "--gocb", "g", "--enable"), errs.ExitUnsupported)
}

func TestGocbItem(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"gcb1": "LLN0$GO$gcb1", "LLN0$GO$g": "LLN0$GO$g", "LLN0.GO.g": "LLN0$GO$g"} {
		if got := gocbItem(in); got != want {
			t.Errorf("gocbItem(%q)=%q", in, got)
		}
	}
}

// A server without a GOOSE publisher accepts the GoEna write but reads back
// false: set --enable must exit 7, not 0.
func TestGooseGoCBEnableNotPublishing(t *testing.T) {
	srv, err := server.Start(context.Background(), server.RunConfig{SclPath: gooseFixture(), Host: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	r := runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--disable")...)
	expectCode(t, r, errs.ExitOK)
	r = runCLI(t, gooseSrvArgs(srv, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--enable")...)
	expectCode(t, r, errs.ExitMismatch)
	if !strings.Contains(r.stdout, "GoEna:      false") || !strings.Contains(r.stderr, "GoEna reads back false") {
		t.Fatalf("stdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
}

func TestVerifyEna(t *testing.T) {
	t.Parallel()
	on, off := true, false
	if verifyEna("GoEna", nil, false) != nil || verifyEna("GoEna", &on, true) != nil || verifyEna("SvEna", &off, false) != nil {
		t.Fatal("matching read-back must pass")
	}
	for _, c := range []struct {
		req  *bool
		back bool
	}{{&on, false}, {&off, true}} {
		if err := verifyEna("SvEna", c.req, c.back); errs.ExitCode(err) != errs.ExitMismatch {
			t.Errorf("verifyEna(%t, %t) = %v", *c.req, c.back, err)
		}
	}
}
