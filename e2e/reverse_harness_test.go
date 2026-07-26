//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	adapterControllerTimeoutLib  = 60 * time.Second
	adapterControllerTimeoutBean = 90 * time.Second
	ctlServerReadyTimeout        = 30 * time.Second
)

// ctlServerHandle is a running iec61850ctl server used as the reverse IED.
type ctlServerHandle struct {
	hostPort  int
	ready     readyEvent
	stdoutBuf bytes.Buffer
	stderrBuf bytes.Buffer
	mu        sync.Mutex
	cmd       *exec.Cmd
	waitDone  chan error
}

func (h *ctlServerHandle) stdoutSnapshot() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stdoutBuf.String()
}

func (h *ctlServerHandle) stderrSnapshot() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stderrBuf.String()
}

// startCtlServer starts the built CLI MMS server on 0.0.0.0:<freePort> with
// the interop ICD/values fixture and waits for a ready JSON event on stdout.
func startCtlServer(t *testing.T) *ctlServerHandle {
	t.Helper()
	requireCtlBin(t)

	port, err := freeHostPort()
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}

	scl := filepath.Join(testDataDirAbs, "interop.icd")
	values := filepath.Join(testDataDirAbs, "self-server-values.json")

	cmd := exec.Command(ctlBin,
		"server", "start",
		"--scl", scl,
		"--values", values,
		"--ied-name", "InteropIED",
		"--fixture-id", lockFile.FixtureRevision,
		"--host", "0.0.0.0",
		"--port", strconv.Itoa(port),
		"--ready-json",
	)

	h := &ctlServerHandle{
		hostPort: port,
		cmd:      cmd,
		waitDone: make(chan error, 1),
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start ctl server: %v", err)
	}

	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			h.mu.Lock()
			h.stderrBuf.Write(sc.Bytes())
			h.stderrBuf.WriteByte('\n')
			h.mu.Unlock()
		}
	}()

	readyCh := make(chan readyEvent, 1)
	readyErr := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			h.mu.Lock()
			h.stdoutBuf.Write(line)
			h.stdoutBuf.WriteByte('\n')
			h.mu.Unlock()

			var ev readyEvent
			if json.Unmarshal(line, &ev) != nil || ev.Event != "ready" {
				continue
			}
			readyCh <- ev
			for sc.Scan() {
				h.mu.Lock()
				h.stdoutBuf.Write(sc.Bytes())
				h.stdoutBuf.WriteByte('\n')
				h.mu.Unlock()
			}
			return
		}
		if err := sc.Err(); err != nil {
			readyErr <- err
			return
		}
		readyErr <- fmt.Errorf("stdout closed before ready event")
	}()

	go func() {
		h.waitDone <- cmd.Wait()
	}()

	select {
	case ev := <-readyCh:
		if ev.Adapter != "iec61850ctl" {
			h.stop()
			t.Fatalf("ready adapter: got %q, want iec61850ctl", ev.Adapter)
		}
		if ev.Fixture != lockFile.FixtureRevision {
			h.stop()
			t.Fatalf("ready fixture: got %q, want %q", ev.Fixture, lockFile.FixtureRevision)
		}
		if strings.TrimSpace(ev.IEDName) == "" {
			h.stop()
			t.Fatal("ready ied_name is empty")
		}
		h.ready = ev
	case err := <-readyErr:
		h.stop()
		t.Fatalf("ctl server readiness: %v\nstdout:\n%s\nstderr:\n%s",
			err, h.stdoutSnapshot(), h.stderrSnapshot())
	case err := <-h.waitDone:
		h.stop()
		t.Fatalf("ctl server exited before ready: %v\nstdout:\n%s\nstderr:\n%s",
			err, h.stdoutSnapshot(), h.stderrSnapshot())
	case <-time.After(ctlServerReadyTimeout):
		h.stop()
		t.Fatalf("timed out waiting for ctl server ready after %s\nstdout:\n%s\nstderr:\n%s",
			ctlServerReadyTimeout, h.stdoutSnapshot(), h.stderrSnapshot())
	}

	t.Cleanup(func() { h.stop() })
	return h
}

func (h *ctlServerHandle) stop() {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return
	}
	_ = h.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-h.waitDone:
		return
	case <-time.After(5 * time.Second):
		_ = h.cmd.Process.Kill()
		<-h.waitDone
	}
}

// adapterOpResult is one JSONL operation result from an mms-interop adapter.
type adapterOpResult struct {
	Operation string `json:"operation"`
	OK        bool   `json:"ok"`
}

type adapterRunResult struct {
	Results []adapterOpResult
	Stdout  []byte
	Stderr  []byte
	Args    []string
	ExitErr error
}

func parseAdapterOpLine(line []byte) (adapterOpResult, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return adapterOpResult{}, false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return adapterOpResult{}, false
	}
	var op string
	if v, ok := raw["operation"]; ok {
		_ = json.Unmarshal(v, &op)
	}
	if op == "" {
		return adapterOpResult{}, false
	}
	var okVal bool
	if v, ok := raw["ok"]; ok {
		_ = json.Unmarshal(v, &okVal)
	}
	return adapterOpResult{Operation: op, OK: okVal}, true
}

func resolveImageForReverseStack(stack string) string {
	switch stack {
	case stackServerLib:
		return resolveImage(stackLibIEC61850)
	case stackServerBean:
		return resolveImage(stackIEC61850Bean)
	default:
		return ""
	}
}

// runAdapterJSONL runs an adapter entrypoint against host.docker.internal:hostPort
// and collects all JSONL operation results until the process exits.
func runAdapterJSONL(t *testing.T, entrypoint, image string, hostPort int, extraArgs ...string) adapterRunResult {
	t.Helper()

	timeout := adapterControllerTimeoutLib
	if strings.Contains(entrypoint, "iec61850bean") {
		timeout = adapterControllerTimeoutBean
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{
		"run", "--rm",
		"--add-host=host.docker.internal:host-gateway",
		"--entrypoint", entrypoint,
		image,
		"--host", "host.docker.internal",
		"--port", strconv.Itoa(hostPort),
	}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := adapterRunResult{
		Stdout:  stdout.Bytes(),
		Stderr:  stderr.Bytes(),
		Args:    append([]string{"docker"}, args...),
		ExitErr: err,
	}

	sc := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if op, ok := parseAdapterOpLine(sc.Bytes()); ok {
			res.Results = append(res.Results, op)
		}
	}
	return res
}

type adapterCapabilities struct {
	Event           string          `json:"event"`
	FixtureRevision string          `json:"fixtureRevision"`
	Commands        json.RawMessage `json:"commands"`
}

func commandEnabled(commands json.RawMessage, required string) bool {
	if len(commands) == 0 {
		return false
	}
	var asMap map[string]bool
	if err := json.Unmarshal(commands, &asMap); err == nil && asMap != nil {
		return asMap[required]
	}
	var asList []string
	if err := json.Unmarshal(commands, &asList); err == nil {
		for _, c := range asList {
			if c == required {
				return true
			}
		}
	}
	return false
}

// validateAdapterCapabilities fails the test when the image entrypoint does not
// advertise the required command or fixture revision.
func validateAdapterCapabilities(t *testing.T, image, entrypoint, requiredCommand string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--entrypoint", entrypoint,
		image,
		"--capabilities",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("capabilities: docker run failed: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.Bytes(), stderr.Bytes())
	}

	trimmed := bytes.TrimSpace(stdout.Bytes())
	if len(trimmed) == 0 {
		t.Fatal("capabilities: empty stdout")
	}
	// Accept a single JSON object or the first JSONL line.
	line := trimmed
	if idx := bytes.IndexByte(trimmed, '\n'); idx >= 0 {
		line = bytes.TrimSpace(trimmed[:idx])
	}

	var caps adapterCapabilities
	if err := json.Unmarshal(line, &caps); err != nil {
		t.Fatalf("capabilities: decode JSON: %v\nstdout:\n%s", err, trimmed)
	}
	if caps.FixtureRevision != lockFile.FixtureRevision {
		t.Fatalf("capabilities fixtureRevision: got %q, want %q (from lock)",
			caps.FixtureRevision, lockFile.FixtureRevision)
	}
	if !commandEnabled(caps.Commands, requiredCommand) {
		t.Fatalf("capabilities: required command %q missing or false; commands=%s",
			requiredCommand, string(caps.Commands))
	}
}

func requireOps(t *testing.T, results []adapterOpResult, ops ...string) {
	t.Helper()
	byOp := make(map[string]adapterOpResult, len(results))
	for _, r := range results {
		if _, exists := byOp[r.Operation]; !exists {
			byOp[r.Operation] = r
		}
	}
	var missing []string
	var failed []string
	for _, op := range ops {
		r, ok := byOp[op]
		if !ok {
			missing = append(missing, op)
			continue
		}
		if !r.OK {
			failed = append(failed, op)
		}
	}
	if len(missing) == 0 && len(failed) == 0 {
		return
	}
	var b strings.Builder
	b.WriteString("adapter operations:")
	for _, r := range results {
		fmt.Fprintf(&b, "\n  op=%q ok=%v", r.Operation, r.OK)
	}
	if len(missing) > 0 {
		t.Fatalf("missing operations %v\n%s", missing, b.String())
	}
	t.Fatalf("operations not ok %v\n%s", failed, b.String())
}

func logAdapterFailure(t *testing.T, res adapterRunResult, srv *ctlServerHandle) {
	t.Helper()
	t.Logf("argv: %v", res.Args)
	t.Logf("adapter exit: %v", res.ExitErr)
	t.Logf("adapter stdout:\n%s", res.Stdout)
	t.Logf("adapter stderr:\n%s", res.Stderr)
	if srv != nil {
		t.Logf("ctl server ready: %+v", srv.ready)
		t.Logf("ctl server stdout:\n%s", srv.stdoutSnapshot())
		t.Logf("ctl server stderr:\n%s", srv.stderrSnapshot())
	}
}

func requireReverseStack(t *testing.T, name string) {
	t.Helper()
	if !selectedStacks[name] {
		t.Skipf("stack %q not selected (IEC61850CTL_E2E_STACKS)", name)
	}
}
