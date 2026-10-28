// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// TestExitCodes pins the documented exit-code contract (README "Exit codes").
func TestExitCodes(t *testing.T) {
	tmp := t.TempDir()
	badJSON := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(badJSON, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		args   []string
		srv    bool // prefix --host/--port of the test server
		code   int
		stderr string
	}{
		// Usage: rejected by cobra or by command-level validation before any I/O.
		{"unknown command", []string{"frobnicate"}, false, errs.ExitUsage, "unknown command"},
		{"unknown flag", []string{"list", "lds", "--nope"}, false, errs.ExitUsage, "unknown flag"},
		{"bad flag value", []string{"--port", "abc", "list", "lds"}, false, errs.ExitUsage, "invalid argument"},
		{"missing required flag", []string{"get", "object"}, false, errs.ExitUsage, `required flag(s) "object"`},
		{"missing host", []string{"list", "lds"}, false, errs.ExitUsage, "host is required"},
		{"bad log level", []string{"--log-level", "loud", "version"}, false, errs.ExitUsage, "invalid --log-level"},
		{"bad log format", []string{"--log-format", "xml", "version"}, false, errs.ExitUsage, "invalid --log-format"},
		{"negative retry", []string{"--retry", "-1", "version"}, false, errs.ExitUsage, "--retry"},
		{"negative timeout", []string{"--timeout", "-1s", "version"}, false, errs.ExitUsage, "--timeout"},
		{"negative connect timeout", []string{"--connect-timeout", "-1s", "version"}, false, errs.ExitUsage, "--connect-timeout"},
		{"negative retry delay", []string{"--retry-max-delay", "-1s", "version"}, false, errs.ExitUsage, "--retry-max-delay"},
		{"bad fc", []string{"get", "object", "--object", "LD/LN.DO", "--fc", "ZZ"}, true, errs.ExitUsage, "invalid functional constraint"},
		{"bad format", []string{"get", "object", "--object", "LD/LN.DO", "--format", "xml"}, true, errs.ExitUsage, "invalid --format"},
		{"set fc CO", []string{"set", "object", "--object", "a/b.c", "--fc", "CO", "--value", "1", "--type", "int"}, true, errs.ExitUsage, "FC=CO"},
		{"set bad fc", []string{"set", "object", "--object", "a/b.c", "--fc", "ZZ", "--value", "1", "--type", "int"}, true, errs.ExitUsage, "invalid functional constraint"},
		{"set bad type", []string{"set", "object", "--object", "a/b.c", "--fc", "SP", "--value", "1", "--type", "blob"}, true, errs.ExitUsage, "invalid type"},
		{"set bad value", []string{"set", "object", "--object", "a/b.c", "--fc", "SP", "--value", "x", "--type", "int"}, true, errs.ExitUsage, "invalid int"},
		{"set bad format", []string{"set", "object", "--object", "a/b.c", "--fc", "SP", "--value", "1", "--type", "int", "--format", "xml"}, true, errs.ExitUsage, "invalid --format"},
		{"tree bad interval", []string{"tree", "--interval", "soon"}, true, errs.ExitUsage, "--interval"},
		{"tree negative interval", []string{"tree", "--interval", "-1s"}, true, errs.ExitUsage, "--interval"},
		{"reports all+ld", []string{"list", "reports", "--all", "--ld", "X"}, true, errs.ExitUsage, "--all cannot"},
		{"reports no ld", []string{"list", "reports"}, true, errs.ExitUsage, "--ld and --ln"},
		{"subscribe bad duration", []string{"subscribe", "report", "--ld", "a", "--ln", "b", "--report", "c", "--duration", "x"}, true, errs.ExitUsage, "--duration"},
		{"subscribe zero duration", []string{"subscribe", "report", "--ld", "a", "--ln", "b", "--report", "c", "--duration", "0s"}, true, errs.ExitUsage, "positive"},
		{"subscribe negative max", []string{"subscribe", "report", "--ld", "a", "--ln", "b", "--report", "c", "--max-reports", "-1"}, true, errs.ExitUsage, "--max-reports"},
		{"subscribe bad type", []string{"subscribe", "report", "--ld", "a", "--ln", "b", "--report", "c", "--type", "XX"}, true, errs.ExitUsage, "--type"},
		{"subscribe bad entry id", []string{"subscribe", "report", "--ld", "a", "--ln", "b", "--report", "c", "--entry-id", "zz"}, true, errs.ExitUsage, "--entry-id"},
		{"subscribe RP with BR opts", []string{"subscribe", "report", "--ld", "a", "--ln", "b", "--report", "c", "--type", "RP", "--purge-buf"}, true, errs.ExitUsage, "require --type BR"},
		{"subscribe bad format", []string{"subscribe", "report", "--ld", "a", "--ln", "b", "--report", "c", "--format", "xml"}, true, errs.ExitUsage, "invalid --format"},
		{"control bad ctl-num", []string{"control", "operate", "--object", "a/b.c", "--type", "bool", "--value", "true", "--ctl-num", "300"}, true, errs.ExitUsage, "ctl-num"},
		{"control long or-ident", []string{"control", "operate", "--object", "a/b.c", "--type", "bool", "--value", "true", "--or-ident", strings.Repeat("x", 65)}, true, errs.ExitUsage, "or-ident"},
		{"control bad mode", []string{"control", "operate", "--object", "a/b.c", "--type", "bool", "--value", "true", "--mode", "fast"}, true, errs.ExitUsage, "invalid control mode"},
		{"control bad or-cat", []string{"control", "operate", "--object", "a/b.c", "--type", "bool", "--value", "true", "--or-cat", "alien"}, true, errs.ExitUsage, "or-cat"},
		{"control FC in object", []string{"control", "inspect", "--object", "InteropLD/GGIO1.SPCSO1[CO]"}, true, errs.ExitUsage, "must not include FC"},
		{"journal bad from", []string{"get", "journal", "--domain", "InteropLD", "--journal", "j", "--from", "yesterday"}, true, errs.ExitUsage, "--from"},
		{"journal bad to", []string{"get", "journal", "--domain", "InteropLD", "--journal", "j", "--from", "0", "--to", "later"}, true, errs.ExitUsage, "--to"},
		{"discover bad spec", []string{"discover", "--host", "not-an-ip"}, false, errs.ExitUsage, "parse host"},
		{"scl parse bad xml", []string{"scl", "parse", "--input", badJSON}, false, errs.ExitUsage, "parse SCL"},
		{"generate-config bad json", []string{"server", "generate-config", "-i", badJSON, "-o", filepath.Join(tmp, "x.cfg")}, false, errs.ExitUsage, "decode IED JSON"},
		{"find bulk bad json", []string{"find", "bulk", "--mapping", badJSON, "--output", filepath.Join(tmp, "o.json")}, false, errs.ExitUsage, "parse mapping"},
		{"server start bad port", []string{"server", "start", "--scl", fixture("interop.icd"), "--port", "-5"}, false, errs.ExitUsage, "--port"},

		// Not found: local files and server objects.
		{"scl parse missing", []string{"scl", "parse", "--input", "/nonexistent.scd"}, false, errs.ExitNotFound, "no such file"},
		{"scl convert missing", []string{"scl", "convert", "--input", "/nonexistent.scd", "--output", filepath.Join(tmp, "o.csv")}, false, errs.ExitNotFound, "no such file"},
		{"server start missing scl", []string{"server", "start", "--scl", "/nonexistent.icd"}, false, errs.ExitNotFound, "SCL file"},
		{"generate-config missing", []string{"server", "generate-config", "-i", "/nonexistent.json", "-o", filepath.Join(tmp, "x.cfg")}, false, errs.ExitNotFound, "read input"},
		{"find bulk missing", []string{"find", "bulk", "--mapping", "/nonexistent.json", "--output", filepath.Join(tmp, "o.json")}, false, errs.ExitNotFound, "read mapping"},
		{"get object missing", []string{"get", "object", "--object", "InteropLD/GGIO1.Nope.stVal", "--fc", "ST"}, true, errs.ExitNotFound, "ObjectNonExistent"},
		{"get report missing", []string{"get", "report", "--ld", "InteropLD", "--ln", "LLN0", "--report", "nope"}, true, errs.ExitNotFound, "ObjectNonExistent"},
		{"get file missing", []string{"get", "file", "--name", "nope.bin"}, true, errs.ExitRejected, "failed to download file"},

		// Server-side rejections.
		{"set rejected", []string{"set", "object", "--object", "InteropLD/MMXU1.Nope.x", "--fc", "MX", "--value", "1", "--type", "float"}, true, errs.ExitRejected, "write failed"},
		{"control status-only", []string{"control", "operate", "--object", "InteropLD/GGIO1.SPS1", "--type", "bool", "--value", "true"}, true, errs.ExitRejected, ""},
		{"journals unsupported", []string{"list", "journals", "--ld", "InteropLD"}, true, errs.ExitRejected, "list journals"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if tc.srv {
				args = srvArgs(args...)
			}
			r := runCLI(t, args...)
			expectCode(t, r, tc.code)
			if tc.stderr != "" && !strings.Contains(r.stderr, tc.stderr) {
				t.Fatalf("stderr %q lacks %q", r.stderr, tc.stderr)
			}
		})
	}
}

func TestExitConnectionRefused(t *testing.T) {
	start := time.Now()
	r := runCLI(t, "--host", "127.0.0.1", "--port", "1", "--retry", "2", "--retry-max-delay", "20ms", "list", "lds")
	expectCode(t, r, errs.ExitConnection)
	if !strings.Contains(r.stderr, "gave up after 3 attempt(s)") {
		t.Fatalf("stderr %q", r.stderr)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("retry took too long")
	}
}

func TestExitInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := runCLIContext(t, ctx, srvArgs("list", "lds")...)
	expectCode(t, r, errs.ExitInterrupted)
	if !strings.Contains(r.stderr, "interrupted") {
		t.Fatalf("stderr %q", r.stderr)
	}
}

func TestEnvHostPort(t *testing.T) {
	t.Setenv("IEC61850_HOST", testServer.host)
	t.Setenv("IEC61850_PORT", strconvItoa(testServer.port))
	expectCode(t, runCLI(t, "list", "lds"), errs.ExitOK)

	t.Setenv("IEC61850_PORT", "nope")
	r := runCLI(t, "list", "lds")
	expectCode(t, r, errs.ExitUsage)

	// --port wins over the environment.
	r = runCLI(t, "--port", strconvItoa(testServer.port), "list", "lds")
	expectCode(t, r, errs.ExitOK)
}
