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
	stackDSCSystems   = "dscsystems"
	stackRust         = "iec61850-rust"
	stackSelf         = "self"

	// Reverse stacks (adapter clients → iec61850ctl server), GOOSE and SV stacks
	// are named by prefixing an adapter stack. They are opt-in via
	// IEC61850CTL_E2E_STACKS; they are not part of the default set.
	prefixServer       = "server-"
	prefixServerReader = "server-reader-"
	prefixGoose        = "goose-"
	prefixSV           = "sv-"

	containerInternalPort = 1102

	commandTimeout      = 20 * time.Second
	libReadyTimeout     = 60 * time.Second
	beanReadyTimeout    = 90 * time.Second
	floatCompareEpsilon = 1e-6
)

// adapterStack is one iec61850-interop adapter image.
type adapterStack struct {
	name         string
	imageEnv     string // overrides the image
	readyTimeout time.Duration
	goose        bool // ships *-goose-publisher and *-goose-subscriber
	sv           bool // ships *-sv-publisher and *-sv-subscriber

	// brcbUnreadable: the server answers a read of BRCB TimeOfEntry, and so
	// of the whole BRCB, with object-access-unsupported (iec61850-rust).
	// get report and subscribe report --type BR fail deterministically.
	brcbUnreadable bool
}

// adapterStacks lists every iec61850-interop stack the e2e suite runs against.
var adapterStacks = []adapterStack{
	{name: stackLibIEC61850, imageEnv: "LIBIEC61850_IMAGE", readyTimeout: libReadyTimeout, goose: true, sv: true},
	{name: stackIEC61850Bean, imageEnv: "IEC61850BEAN_IMAGE", readyTimeout: beanReadyTimeout},
	{name: stackDSCSystems, imageEnv: "DSCSYSTEMS_IMAGE", readyTimeout: libReadyTimeout, goose: true, sv: true},
	{name: stackRust, imageEnv: "IEC61850RUST_IMAGE", readyTimeout: libReadyTimeout, goose: true, sv: true, brcbUnreadable: true},
}

func lookupAdapter(name string) (adapterStack, bool) {
	for _, a := range adapterStacks {
		if a.name == name {
			return a, true
		}
	}
	return adapterStack{}, false
}

// clientStacks returns every forward stack (adapter server → iec61850ctl
// client); tests skip the ones not selected.
func clientStacks() []string {
	out := make([]string, len(adapterStacks))
	for i, a := range adapterStacks {
		out[i] = a.name
	}
	return out
}

// brcbUnreadable reports the registered BRCB read limitation of a stack.
func brcbUnreadable(stack string) bool {
	a, _ := lookupAdapter(stack)
	return a.brcbUnreadable
}

// requireBRCBUnreadable asserts the registered limitation: a deterministic
// failure naming object-access-unsupported, never a false success.
func requireBRCBUnreadable(t *testing.T, res commandResult, h *serverHandle) {
	t.Helper()
	if res.TimedOut || res.ExitCode == 0 || !bytes.Contains(res.Stderr, []byte("ObjectAccessUnsupported")) {
		logCommandFailure(t, res, h)
		t.Fatalf("%s BRCB: want a non-zero exit naming ObjectAccessUnsupported (registered limitation)", h.stack)
	}
}

// readyTimeoutFor is the ready-event timeout of a forward stack.
func readyTimeoutFor(stack string) time.Duration {
	if a, ok := lookupAdapter(stack); ok {
		return a.readyTimeout
	}
	return libReadyTimeout
}

// Lock is the checked-in interop.lock.json provenance document.
type Lock struct {
	SourceRepository string `json:"source_repository"`
	SourceCommit     string `json:"source_commit"`
	SourceTag        string `json:"source_tag"`
	FixtureRevision  string `json:"fixture_revision"`
	// GooseFixtureRevision is the iec61850-interop GOOSE fixture (goose.json).
	GooseFixtureRevision string `json:"goose_fixture_revision"`
	// SVFixtureRevision is the iec61850-interop SV fixture (sv.json).
	SVFixtureRevision string `json:"sv_fixture_revision"`
	// SVCBOverMMS records which IED servers expose LLN0$MS$MSVCB02
	// (iec61850-interop capabilities.json svcbOverMms).
	SVCBOverMMS      map[string]bool   `json:"svcb_over_mms"`
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

	// sharedServers holds one long-lived adapter server per selected
	// forward stack, for the read-only tests.
	sharedServers = map[string]*serverHandle{}
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

	if len(selectedStacks) > 0 && ctlBin == "" {
		fmt.Fprintf(os.Stderr, "e2e: IEC61850CTL_BIN is required\n")
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

	stopShared := func() {
		for _, h := range sharedServers {
			h.stop()
		}
	}
	for _, a := range adapterStacks {
		if !selectedStacks[a.name] {
			continue
		}
		h, err := startDockerServer(a.name, a.readyTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "e2e: start %s server: %v\n", a.name, err)
			stopShared()
			os.Exit(1)
		}
		sharedServers[a.name] = h
		fmt.Fprintf(os.Stderr, "[e2e] shared %s ready at %s:%d ied=%s\n", a.name, h.host, h.port, h.iedName)
	}

	code := m.Run()

	stopShared()
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

// knownStacks returns every stack name IEC61850CTL_E2E_STACKS accepts.
func knownStacks() []string {
	out := []string{stackSelf}
	for _, a := range adapterStacks {
		out = append(out, a.name, prefixServer+a.name, prefixServerReader+a.name)
		if a.goose {
			out = append(out, prefixGoose+a.name)
		}
		if a.sv {
			out = append(out, prefixSV+a.name)
		}
	}
	return out
}

func parseStacks(raw string) map[string]bool {
	out := make(map[string]bool)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		// Default: forward client stacks + self. Reverse server-*, goose-* and sv-*
		// stacks are opt-in (the CI matrix selects them one by one).
		for _, name := range clientStacks() {
			out[name] = true
		}
		out[stackSelf] = true
		return out
	}
	known := knownStacks()
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		switch {
		case name == "":
			continue
		case name == "all":
			for _, k := range known {
				out[k] = true
			}
		case containsName(known, name):
			out[name] = true
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
	if _, ok := lookupAdapter(stack); !ok {
		t.Fatalf("no shared docker server for stack %q", stack)
	}
	h := sharedServers[stack]
	if h == nil {
		t.Fatalf("shared server for stack %q was not started", stack)
	}
	return h
}

func externalStacks() []string {
	var stacks []string
	for _, name := range clientStacks() {
		if selectedStacks[name] {
			stacks = append(stacks, name)
		}
	}
	return stacks
}

// resolveImage returns the image of an adapter stack: the stack's image
// environment variable, the lock digest in CI, else the local build tag.
func resolveImage(stack string) string {
	a, ok := lookupAdapter(stack)
	if !ok {
		return ""
	}
	if v := strings.TrimSpace(os.Getenv(a.imageEnv)); v != "" {
		return v
	}
	if os.Getenv("CI") != "" {
		if img := lockFile.Images[a.name]; img != "" {
			return img
		}
	}
	return "iec61850-interop-" + a.name + ":local"
}

func entrypointForStack(stack string) string {
	if _, ok := lookupAdapter(stack); !ok {
		return ""
	}
	return stack + "-ied-server"
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
