// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/sv"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

func repoPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	return filepath.Join(append([]string{filepath.Dir(file), "..", "..", ".."}, parts...)...)
}

func interopICD(t *testing.T) string { return repoPath(t, "e2e", "testdata", "interop.icd") }

func TestStartServesAndCloses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Start(context.Background(), RunConfig{
		SclPath:    interopICD(t),
		ValuesPath: repoPath(t, "e2e", "testdata", "self-server-values.json"),
		FilesDir:   dir,
		Host:       "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.IEDName() != "InteropIED" {
		t.Fatalf("ied %q", r.IEDName())
	}
	if _, port, _ := net.SplitHostPort(r.Addr()); port == "0" || port == "" {
		t.Fatalf("ephemeral port not resolved: %s", r.Addr())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := iec61850.Dial(ctx, r.Addr(), iec61850.DialOptions{})
	if err != nil {
		t.Fatal(err)
	}
	files, err := c.ListFiles(ctx, "*")
	if err != nil || len(files) != 1 || files[0].Name != "a.txt" {
		t.Fatalf("files=%v err=%v", files, err)
	}
	var buf bytes.Buffer
	if _, err := c.DownloadFile(ctx, "a.txt", &buf); err != nil || buf.String() != "hello" {
		t.Fatalf("download %q err=%v", buf.String(), err)
	}
	_ = c.Close(ctx)

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil { // idempotent
		t.Fatal(err)
	}
	select {
	case <-r.Done():
	default:
		t.Fatal("Done not closed")
	}
}

func TestStartContextCancelStops(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	r, err := Start(ctx, RunConfig{SclPath: interopICD(t), Host: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-r.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop on ctx cancel")
	}
	_ = r.Close()
}

func TestStartErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  RunConfig
		code int
	}{
		{"bad port", RunConfig{SclPath: interopICD(t), Port: -1}, errs.ExitUsage},
		{"port too large", RunConfig{SclPath: interopICD(t), Port: 70000}, errs.ExitUsage},
		{"missing scl", RunConfig{SclPath: "/nonexistent.icd"}, errs.ExitNotFound},
		{"unknown ied", RunConfig{SclPath: interopICD(t), IEDName: "Nope"}, errs.ExitNotFound},
		{"missing values", RunConfig{SclPath: interopICD(t), Host: "127.0.0.1", ValuesPath: "/nonexistent.json"}, errs.ExitNotFound},
		{"missing files dir", RunConfig{SclPath: interopICD(t), Host: "127.0.0.1", FilesDir: "/nonexistent-dir"}, errs.ExitNotFound},
		{"address not local", RunConfig{SclPath: interopICD(t), Host: "192.0.2.1"}, errs.ExitConnection},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, err := Start(context.Background(), tt.cfg)
			if err == nil {
				_ = r.Close()
				t.Fatal("expected error")
			}
			if got := errs.ExitCode(err); got != tt.code {
				t.Fatalf("exit=%d want %d (err=%v)", got, tt.code, err)
			}
		})
	}
}

func TestStartBadValuesJSON(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "v.json")
	if err := os.WriteFile(p, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(context.Background(), RunConfig{SclPath: interopICD(t), Host: "127.0.0.1", ValuesPath: p}); err == nil {
		t.Fatal("expected JSON error")
	}
}

// syncBuffer is a goroutine-safe bytes.Buffer.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestRunContextReadyJSON(t *testing.T) {
	t.Parallel()
	var stdout, stderr syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunContext(ctx, RunConfig{SclPath: interopICD(t), Host: "127.0.0.1", ReadyJSON: true,
			FixtureID: "fx", Version: "v1", Stdout: &stdout, Stderr: &stderr})
	}()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(stdout.String(), "\n") {
		if time.Now().After(deadline) {
			t.Fatal("no ready event")
		}
		time.Sleep(5 * time.Millisecond)
	}
	var ev ReadyEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Event != "ready" || ev.Fixture != "fx" || ev.IEDName != "InteropIED" || strings.HasSuffix(ev.Address, ":0") {
		t.Fatalf("event %+v", ev)
	}
	if !strings.Contains(stderr.String(), "listening on "+ev.Address) {
		t.Fatalf("stderr %q", stderr.String())
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunContext did not return")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("closed pipe") }

func TestRunContextErrors(t *testing.T) {
	t.Parallel()
	if err := RunContext(context.Background(), RunConfig{SclPath: "/nonexistent.icd"}); err == nil {
		t.Fatal("expected start error")
	}
	err := RunContext(context.Background(), RunConfig{SclPath: interopICD(t), Host: "127.0.0.1", ReadyJSON: true,
		Stdout: failWriter{}, Stderr: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "write ready event") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveIEDNamePrefersServerIED(t *testing.T) {
	t.Parallel()
	// The fixture lists a client-only IED (no Server access point) first.
	r, err := Start(context.Background(), RunConfig{SclPath: filepath.Join("testdata", "client-first.icd"), Host: "127.0.0.1"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = r.Close() }()
	if r.IEDName() != "OtherIED" {
		t.Fatalf("picked %q", r.IEDName())
	}
}

func TestStartWithSV(t *testing.T) {
	t.Parallel()
	hub := sv.NewHub()
	port := hub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, 1})
	t.Cleanup(func() { _ = port.Close() })
	r, err := Start(context.Background(), RunConfig{
		SclPath: repoPath(t, "e2e", "testdata", "interop-sv.cid"), Host: "127.0.0.1",
		SV: port, SVStartDisabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := iec61850.Dial(ctx, r.Addr(), iec61850.DialOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	cb, err := c.GetMSVCB(ctx, "InteropLD", "LLN0$MS$MSVCB01")
	if err != nil {
		t.Fatal(err)
	}
	if cb.SvEna || cb.MsvID != "InteropSv92LE" {
		t.Fatalf("MSVCB01 %+v, want disabled InteropSv92LE", cb)
	}
}
