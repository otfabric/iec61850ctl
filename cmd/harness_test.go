// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/otfabric/iec61850ctl/pkg/stack/server"
)

var update = flag.Bool("update", false, "rewrite cmd/testdata/golden files")

// testServer is the in-process MMS server shared by the cmd tests (interop
// fixture, seeded values, a files directory). Started in TestMain.
var testServer struct {
	addr     string
	host     string
	port     int
	filesDir string
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..")
}

func fixture(parts ...string) string {
	return filepath.Join(append([]string{repoRoot(), "e2e", "testdata"}, parts...)...)
}

func TestMain(m *testing.M) {
	flag.Parse()
	for _, k := range []string{"IEC61850_HOST", "IEC61850_PORT", "IEC61850_IED_NAME"} {
		_ = os.Unsetenv(k)
	}
	dir, err := os.MkdirTemp("", "iec61850ctl-files-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.WriteFile(filepath.Join(dir, "COMTRADE.cfg"), []byte("station,rec,1999\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "events.log"), []byte(strings.Repeat("event line\n", 100)), 0o600)

	ctx, cancel := context.WithCancel(context.Background())
	srv, err := server.Start(ctx, server.RunConfig{
		SclPath:    fixture("interop.icd"),
		ValuesPath: fixture("self-server-values.json"),
		FilesDir:   dir,
		Host:       "127.0.0.1",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "start test server:", err)
		os.Exit(1)
	}
	h, p, _ := net.SplitHostPort(srv.Addr())
	testServer.addr = srv.Addr()
	testServer.host = h
	testServer.port, _ = strconv.Atoi(p)
	testServer.filesDir = dir

	code := m.Run()
	cancel()
	_ = srv.Close()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// cliMu serialises CLI invocations: commands share package-level flag state.
var cliMu sync.Mutex

type result struct {
	stdout, stderr string
	code           int
}

// resetFlags restores every flag in the command tree to its default and
// clears Changed, so each in-process invocation starts from a clean slate.
func resetFlags(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}

// runCLI executes the CLI in-process with a timeout and returns its output.
func runCLI(t *testing.T, args ...string) result {
	t.Helper()
	return runCLIContext(t, context.Background(), args...)
}

func runCLIContext(t *testing.T, parent context.Context, args ...string) result {
	t.Helper()
	cliMu.Lock()
	defer cliMu.Unlock()
	resetFlags(rootCmd)
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	var stdout, stderr lockedBuffer
	code := ExecuteContext(ctx, args, &stdout, &stderr)
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// srvArgs prefixes args with --host/--port of the shared test server.
func srvArgs(args ...string) []string {
	return append([]string{"--host", testServer.host, "--port", strconv.Itoa(testServer.port)}, args...)
}

// runSrv runs a command against the shared test server.
func runSrv(t *testing.T, args ...string) result {
	t.Helper()
	return runCLI(t, srvArgs(args...)...)
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

var (
	reAddr     = regexp.MustCompile(`127\.0\.0\.1:\d+`)
	reDuration = regexp.MustCompile(`\b\d+(\.\d+)?(ns|µs|us|ms|s|m|h)\b`)
	reRFC3339  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)
	reTmp      = regexp.MustCompile(`(/private)?/(var|tmp)/[^\s"]*`)
	rePortWord = regexp.MustCompile(`\bport \d+`)
)

// normalize replaces run-specific values (addresses, durations, timestamps,
// temp paths) so outputs can be compared with golden files.
func normalize(s string) string {
	s = reAddr.ReplaceAllString(s, "127.0.0.1:<PORT>")
	s = reRFC3339.ReplaceAllString(s, "<TIME>")
	s = reDuration.ReplaceAllString(s, "<DUR>")
	s = reTmp.ReplaceAllString(s, "<TMP>")
	s = rePortWord.ReplaceAllString(s, "port <PORT>")
	return s
}

// golden compares got (normalised) with testdata/golden/<name>.golden.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".golden")
	got = normalize(got)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden %s missing (run go test ./cmd -run %s -update): %v", path, t.Name(), err)
	}
	if string(want) != got {
		t.Errorf("golden %s mismatch\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}

func expectCode(t *testing.T, r result, want int) {
	t.Helper()
	if r.code != want {
		t.Fatalf("exit=%d want %d\nstdout:\n%s\nstderr:\n%s", r.code, want, r.stdout, r.stderr)
	}
}

func strconvItoa(i int) string { return strconv.Itoa(i) }
