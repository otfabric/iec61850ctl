//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	stackLibIEC61850  = "libiec61850"
	stackIEC61850Bean = "iec61850bean"
	stackSelf         = "self"
	// Reverse stacks (adapter clients → iec61850ctl server) are opt-in via
	// IEC61850CTL_E2E_STACKS; they are not part of the default set.
	stackServerLib  = "server-libiec61850"
	stackServerBean = "server-iec61850bean"
	// Reverse general-reader stacks (Phase 3B).
	stackServerReaderLib  = "server-reader-libiec61850"
	stackServerReaderBean = "server-reader-iec61850bean"

	defaultLibIECImage    = "mms-interop-libiec61850:local"
	defaultBeanImage      = "mms-interop-iec61850bean:local"
	containerInternalPort = 1102

	commandTimeout      = 20 * time.Second
	libReadyTimeout     = 60 * time.Second
	beanReadyTimeout    = 90 * time.Second
	floatCompareEpsilon = 1e-6
)

// Lock is the checked-in interop.lock.json provenance document.
type Lock struct {
	SourceRepository string            `json:"source_repository"`
	SourceCommit     string            `json:"source_commit"`
	SourceTag        string            `json:"source_tag"`
	FixtureRevision  string            `json:"fixture_revision"`
	Files            map[string]string `json:"files"`
	Images           map[string]string `json:"images"`
	RequiredCommands map[string]bool   `json:"required_commands"`
}

type readyEvent struct {
	Event   string `json:"event"`
	Address string `json:"address"`
	Fixture string `json:"fixture"`
	Adapter string `json:"adapter"`
	Version string `json:"version"`
	IEDName string `json:"ied_name"`
}

type commandResult struct {
	Args     []string
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
	TimedOut bool
}

type expectedValues struct {
	Values map[string]json.RawMessage `json:"values"`
}

// serverHandle is a running external (Docker) IED server.
type serverHandle struct {
	stack     string
	container string
	image     string
	host      string
	port      int
	iedName   string
	ready     readyEvent
	stdoutBuf bytes.Buffer
	stderrBuf bytes.Buffer
	mu        sync.Mutex
	cmd       *exec.Cmd
	cmdCancel context.CancelFunc
	waitDone  chan struct{}
	waitErr   error
}

var (
	testDataDirAbs string
	lockFile       Lock
	expectedVals   expectedValues
	selectedStacks map[string]bool
	ctlBin         string

	sharedLib  *serverHandle
	sharedBean *serverHandle
)

func TestMain(m *testing.M) {
	var err error
	testDataDirAbs, err = resolveTestDataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: resolve testdata: %v\n", err)
		os.Exit(1)
	}

	lockFile, err = loadLock(filepath.Join(testDataDirAbs, "interop.lock.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: load lock: %v\n", err)
		os.Exit(1)
	}

	selectedStacks = parseStacks(os.Getenv("IEC61850CTL_E2E_STACKS"))
	ctlBin = strings.TrimSpace(os.Getenv("IEC61850CTL_BIN"))

	needsBin := selectedStacks[stackLibIEC61850] || selectedStacks[stackIEC61850Bean] ||
		selectedStacks[stackSelf] || selectedStacks[stackServerLib] || selectedStacks[stackServerBean] ||
		selectedStacks[stackServerReaderLib] || selectedStacks[stackServerReaderBean]
	if needsBin && ctlBin == "" {
		fmt.Fprintf(os.Stderr, "e2e: IEC61850CTL_BIN is required when stacks include libiec61850, iec61850bean, self, or server-*\n")
		os.Exit(1)
	}
	if ctlBin != "" {
		if st, err := os.Stat(ctlBin); err != nil || st.IsDir() {
			fmt.Fprintf(os.Stderr, "e2e: IEC61850CTL_BIN %q is not a usable binary: %v\n", ctlBin, err)
			os.Exit(1)
		}
	}

	expectedVals, err = loadExpectedValues(filepath.Join(testDataDirAbs, "expected-values.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: load expected-values: %v\n", err)
		os.Exit(1)
	}

	if selectedStacks[stackLibIEC61850] {
		sharedLib, err = startDockerServer(stackLibIEC61850, libReadyTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "e2e: start libiec61850 server: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[e2e] shared libiec61850 ready at %s:%d ied=%s\n", sharedLib.host, sharedLib.port, sharedLib.iedName)
	}

	if selectedStacks[stackIEC61850Bean] {
		sharedBean, err = startDockerServer(stackIEC61850Bean, beanReadyTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "e2e: start iec61850bean server: %v\n", err)
			if sharedLib != nil {
				sharedLib.stop()
			}
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[e2e] shared iec61850bean ready at %s:%d ied=%s\n", sharedBean.host, sharedBean.port, sharedBean.iedName)
	}

	code := m.Run()

	if sharedBean != nil {
		sharedBean.stop()
	}
	if sharedLib != nil {
		sharedLib.stop()
	}
	os.Exit(code)
}

func resolveTestDataDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(file), "testdata")
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return "", fmt.Errorf("testdata dir %q: %w", abs, err)
	}
	return abs, nil
}

func loadLock(path string) (Lock, error) {
	var l Lock
	data, err := os.ReadFile(path)
	if err != nil {
		return l, err
	}
	if err := json.Unmarshal(data, &l); err != nil {
		return l, err
	}
	if l.FixtureRevision == "" {
		return l, errors.New("fixture_revision is empty")
	}
	if l.SourceCommit == "" {
		return l, errors.New("source_commit is empty")
	}
	return l, nil
}

func loadExpectedValues(path string) (expectedValues, error) {
	var ev expectedValues
	data, err := os.ReadFile(path)
	if err != nil {
		return ev, err
	}
	if err := json.Unmarshal(data, &ev); err != nil {
		return ev, err
	}
	if len(ev.Values) == 0 {
		return ev, errors.New("expected-values.json has empty values map")
	}
	return ev, nil
}

func parseStacks(raw string) map[string]bool {
	out := make(map[string]bool)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		// Default: forward client stacks + self. Reverse server-* stacks are
		// opt-in (CI matrix sets IEC61850CTL_E2E_STACKS=server-libiec61850, etc.).
		out[stackLibIEC61850] = true
		out[stackIEC61850Bean] = true
		out[stackSelf] = true
		return out
	}
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		switch name {
		case "all":
			out[stackLibIEC61850] = true
			out[stackIEC61850Bean] = true
			out[stackSelf] = true
			out[stackServerLib] = true
			out[stackServerBean] = true
			out[stackServerReaderLib] = true
			out[stackServerReaderBean] = true
		case stackLibIEC61850, stackIEC61850Bean, stackSelf,
			stackServerLib, stackServerBean, stackServerReaderLib, stackServerReaderBean:
			out[name] = true
		case "":
			continue
		default:
			fmt.Fprintf(os.Stderr, "e2e: ignoring unknown stack %q in IEC61850CTL_E2E_STACKS\n", name)
		}
	}
	return out
}

func requireStack(t *testing.T, name string) {
	t.Helper()
	if !selectedStacks[name] {
		t.Skipf("stack %q not selected (IEC61850CTL_E2E_STACKS)", name)
	}
}

func requireCtlBin(t *testing.T) {
	t.Helper()
	if ctlBin == "" {
		t.Skip("IEC61850CTL_BIN unset")
	}
}

func sharedServer(t *testing.T, stack string) *serverHandle {
	t.Helper()
	requireStack(t, stack)
	requireCtlBin(t)
	var h *serverHandle
	switch stack {
	case stackLibIEC61850:
		h = sharedLib
	case stackIEC61850Bean:
		h = sharedBean
	default:
		t.Fatalf("no shared docker server for stack %q", stack)
	}
	if h == nil {
		t.Fatalf("shared server for stack %q was not started", stack)
	}
	return h
}

func externalStacks() []string {
	var stacks []string
	if selectedStacks[stackLibIEC61850] {
		stacks = append(stacks, stackLibIEC61850)
	}
	if selectedStacks[stackIEC61850Bean] {
		stacks = append(stacks, stackIEC61850Bean)
	}
	return stacks
}

func resolveImage(stack string) string {
	switch stack {
	case stackLibIEC61850:
		if v := strings.TrimSpace(os.Getenv("LIBIEC61850_IMAGE")); v != "" {
			return v
		}
		if os.Getenv("CI") != "" {
			if img := lockFile.Images[stackLibIEC61850]; img != "" {
				return img
			}
		}
		return defaultLibIECImage
	case stackIEC61850Bean:
		if v := strings.TrimSpace(os.Getenv("IEC61850BEAN_IMAGE")); v != "" {
			return v
		}
		if os.Getenv("CI") != "" {
			if img := lockFile.Images[stackIEC61850Bean]; img != "" {
				return img
			}
		}
		return defaultBeanImage
	default:
		return ""
	}
}

func entrypointForStack(stack string) string {
	switch stack {
	case stackLibIEC61850:
		return "libiec61850-ied-server"
	case stackIEC61850Bean:
		return "iec61850bean-ied-server"
	default:
		return ""
	}
}

func freeHostPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func uniqueContainerName(stack string) string {
	name := fmt.Sprintf("iec61850ctl-e2e-%s-%d-%d", stack, os.Getpid(), time.Now().UnixNano())
	return strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(name)
}

func startDockerServer(stack string, readyTimeout time.Duration) (*serverHandle, error) {
	image := resolveImage(stack)
	entrypoint := entrypointForStack(stack)
	if image == "" || entrypoint == "" {
		return nil, fmt.Errorf("unsupported stack %q", stack)
	}
	hostPort, err := freeHostPort()
	if err != nil {
		return nil, fmt.Errorf("allocate host port: %w", err)
	}
	container := uniqueContainerName(stack)

	cmdCtx, cmdCancel := context.WithCancel(context.Background())
	args := []string{
		"run",
		"--name", container,
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", hostPort, containerInternalPort),
		"--label", "org.otfabric.e2e=iec61850ctl",
		"--label", "org.otfabric.e2e.stack=" + stack,
		"--label", fmt.Sprintf("org.otfabric.e2e.pid=%d", os.Getpid()),
		"--entrypoint", entrypoint,
		image,
		"--port", strconv.Itoa(containerInternalPort),
	}
	cmd := exec.CommandContext(cmdCtx, "docker", args...)

	h := &serverHandle{
		stack:     stack,
		container: container,
		image:     image,
		host:      "127.0.0.1",
		port:      hostPort,
		cmd:       cmd,
		cmdCancel: cmdCancel,
		waitDone:  make(chan struct{}),
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cmdCancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cmdCancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cmdCancel()
		return nil, fmt.Errorf("docker run: %w", err)
	}

	go func() {
		sc := bufio.NewScanner(stderr)
		// Allow long Java lines.
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
			// Keep draining for diagnostics.
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
		readyErr <- errors.New("stdout closed before ready event")
	}()

	go func() {
		h.waitErr = cmd.Wait()
		close(h.waitDone)
	}()

	startCtx, startCancel := context.WithTimeout(context.Background(), readyTimeout)
	defer startCancel()

	select {
	case ev := <-readyCh:
		if err := validateReadyEvent(ev, stack); err != nil {
			h.stop()
			return nil, err
		}
		h.ready = ev
		h.iedName = ev.IEDName
		// Brief association warm-up (no testing.T available in TestMain).
		if err := waitForCtlReadyMain(h.ctlEnv(), 5*time.Second); err != nil {
			h.stop()
			return nil, fmt.Errorf("post-ready warm-up: %w", err)
		}
		return h, nil
	case err := <-readyErr:
		h.stop()
		return nil, fmt.Errorf("readiness: %w", err)
	case <-h.waitDone:
		h.stop()
		return nil, fmt.Errorf("container exited before ready: %v\nstdout:\n%s\nstderr:\n%s",
			h.waitErr, h.stdoutSnapshot(), h.stderrSnapshot())
	case <-startCtx.Done():
		h.stop()
		return nil, fmt.Errorf("timed out waiting for %s readiness after %s\nstdout:\n%s\nstderr:\n%s",
			stack, readyTimeout, h.stdoutSnapshot(), h.stderrSnapshot())
	}
}

func validateReadyEvent(ev readyEvent, stack string) error {
	if ev.Adapter != stack {
		return fmt.Errorf("ready adapter: got %q, want %q", ev.Adapter, stack)
	}
	if ev.Fixture != lockFile.FixtureRevision {
		return fmt.Errorf("ready fixture: got %q, want %q (from lock)", ev.Fixture, lockFile.FixtureRevision)
	}
	if strings.TrimSpace(ev.IEDName) == "" {
		return errors.New("ready ied_name is empty")
	}
	if strings.TrimSpace(ev.Version) == "" {
		return errors.New("ready version is empty")
	}
	if os.Getenv("CI") != "" && ev.Version == "dev" {
		return fmt.Errorf("ready version is %q in CI; pin a released image digest", ev.Version)
	}
	return nil
}

func (h *serverHandle) stdoutSnapshot() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stdoutBuf.String()
}

func (h *serverHandle) stderrSnapshot() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stderrBuf.String()
}

func (h *serverHandle) stop() {
	if h == nil {
		return
	}
	if h.container != "" {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = exec.CommandContext(stopCtx, "docker", "stop", "-t", "2", h.container).Run()
		cancel()
		rmCtx, rmCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = exec.CommandContext(rmCtx, "docker", "rm", "-f", h.container).Run()
		rmCancel()
	}
	if h.cmdCancel != nil {
		h.cmdCancel()
	}
	if h.waitDone != nil {
		select {
		case <-h.waitDone:
		case <-time.After(5 * time.Second):
		}
	}
}

func (h *serverHandle) ctlEnv() map[string]string {
	return map[string]string{
		"IEC61850_HOST":     h.host,
		"IEC61850_PORT":     strconv.Itoa(h.port),
		"IEC61850_IED_NAME": h.iedName,
	}
}

// waitForCtlReady retries a cheap list command until the server accepts associations.
// Listener bind and active serving can still race after the ready event.
func waitForCtlReady(t *testing.T, env map[string]string, timeout time.Duration) {
	t.Helper()
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var last commandResult
	for time.Now().Before(deadline) {
		last = runCtl(t, env, 3*time.Second, "list", "lds", "--format", "json")
		if last.ExitCode == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	logCommandFailure(t, last, nil)
	t.Fatalf("server not accepting associations within %s", timeout)
}

func logServerDiagnostics(t *testing.T, h *serverHandle) {
	t.Helper()
	if h == nil {
		return
	}
	t.Logf("adapter stack=%s container=%s image=%s host=%s port=%d",
		h.stack, h.container, h.image, h.host, h.port)
	t.Logf("readiness: %+v", h.ready)
	t.Logf("lock: source_commit=%s fixture_revision=%s", lockFile.SourceCommit, lockFile.FixtureRevision)
	t.Logf("adapter stdout:\n%s", h.stdoutSnapshot())
	t.Logf("adapter stderr:\n%s", h.stderrSnapshot())
	if h.container != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "docker", "inspect", h.container).CombinedOutput()
		if err != nil {
			t.Logf("docker inspect: %v (%s)", err, bytes.TrimSpace(out))
		} else {
			t.Logf("docker inspect:\n%s", out)
		}
	}
}

func logCommandFailure(t *testing.T, res commandResult, h *serverHandle) {
	t.Helper()
	t.Logf("argv: %v", res.Args)
	t.Logf("exit=%d timed_out=%v duration=%s", res.ExitCode, res.TimedOut, res.Duration)
	t.Logf("stdout:\n%s", res.Stdout)
	t.Logf("stderr:\n%s", res.Stderr)
	logServerDiagnostics(t, h)
}

func mergeEnv(extra map[string]string) []string {
	base := os.Environ()
	if len(extra) == 0 {
		return base
	}
	deny := make(map[string]struct{}, len(extra))
	for k := range extra {
		deny[k] = struct{}{}
	}
	out := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		if _, drop := deny[key]; drop {
			continue
		}
		out = append(out, kv)
	}
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}

func runCtl(t *testing.T, env map[string]string, timeout time.Duration, args ...string) commandResult {
	t.Helper()
	requireCtlBin(t)
	return runCtlBin(ctlBin, env, timeout, args...)
}

func runCtlBin(bin string, env map[string]string, timeout time.Duration, args ...string) commandResult {
	if timeout <= 0 {
		timeout = commandTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = mergeEnv(env)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	res := commandResult{
		Args:     append([]string{bin}, args...),
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Duration: time.Since(start),
	}
	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitCode = -1
		return res
	}
	if err == nil {
		res.ExitCode = 0
		return res
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		res.ExitCode = ee.ExitCode()
		return res
	}
	res.ExitCode = -1
	return res
}

func waitForCtlReadyMain(env map[string]string, timeout time.Duration) error {
	if ctlBin == "" {
		return errors.New("IEC61850CTL_BIN unset")
	}
	deadline := time.Now().Add(timeout)
	var last commandResult
	for time.Now().Before(deadline) {
		last = runCtlBin(ctlBin, env, 3*time.Second, "list", "lds", "--format", "json")
		if last.ExitCode == 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("association warm-up failed (exit=%d): stderr=%s", last.ExitCode, bytes.TrimSpace(last.Stderr))
}

func decodeExactJSON(data []byte, dest any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return errors.New("stdout is empty; expected one JSON value")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("stdout contains more than one JSON value")
		}
		return fmt.Errorf("trailing data after JSON value: %w", err)
	}
	return nil
}

func requireExitZero(t *testing.T, res commandResult, h *serverHandle) {
	t.Helper()
	if res.TimedOut || res.ExitCode != 0 {
		logCommandFailure(t, res, h)
		if res.TimedOut {
			t.Fatalf("command timed out after %s", res.Duration)
		}
		t.Fatalf("exit code %d, want 0", res.ExitCode)
	}
}

func requireExitNonZero(t *testing.T, res commandResult) {
	t.Helper()
	if res.TimedOut {
		t.Fatalf("command timed out after %s; stdout=%q stderr=%q", res.Duration, res.Stdout, res.Stderr)
	}
	if res.ExitCode == 0 {
		t.Fatalf("exit code 0, want non-zero; stdout=%q stderr=%q", res.Stdout, res.Stderr)
	}
}

func expectedValue(t *testing.T, key string) any {
	t.Helper()
	raw, ok := expectedVals.Values[key]
	if !ok {
		t.Fatalf("expected-values missing key %q", key)
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode expected value %q: %v", key, err)
	}
	return v
}

func valuesEqual(got, want any) bool {
	if gf, ok := asFloat64(got); ok {
		if wf, ok := asFloat64(want); ok {
			return floatClose(gf, wf)
		}
	}
	gb, ok1 := got.(bool)
	wb, ok2 := want.(bool)
	if ok1 && ok2 {
		return gb == wb
	}
	gs, ok1 := got.(string)
	ws, ok2 := want.(string)
	if ok1 && ok2 {
		return gs == ws
	}
	return fmt.Sprint(got) == fmt.Sprint(want)
}

func asFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case float32:
		return int64(x), true
	case int:
		return int64(x), true
	case int64:
		return x, true
	case uint64:
		return int64(x), true
	case json.Number:
		i, err := x.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func floatClose(a, b float64) bool {
	if a == b {
		return true
	}
	diff := math.Abs(a - b)
	if diff <= floatCompareEpsilon {
		return true
	}
	scale := math.Max(math.Abs(a), math.Abs(b))
	return diff <= floatCompareEpsilon*scale
}

func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}
