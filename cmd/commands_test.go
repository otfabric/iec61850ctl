// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
	"github.com/otfabric/iec61850ctl/pkg/stack/server"
)

func TestVersion(t *testing.T) {
	SetBuildMeta("v9.9.9", "v9.9.9", "abc123", "2026-01-01T00:00:00Z")
	t.Cleanup(func() { SetBuildMeta("", "", "", "") })
	r := runCLI(t, "version")
	expectCode(t, r, errs.ExitOK)
	for _, want := range []string{"iec61850ctl version: v9.9.9", "tag:         v9.9.9", "commit:      abc123", "build date:", "go version:", "platform:"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("version output lacks %q:\n%s", want, r.stdout)
		}
	}
	SetBuildMeta("", "", "", "")
	if r := runCLI(t, "version"); !strings.Contains(r.stdout, "version: dev") || strings.Contains(r.stdout, "tag:") {
		t.Errorf("dev version output:\n%s", r.stdout)
	}
}

func TestSetObject(t *testing.T) {
	obj := "InteropLD/GGIO1.SetInt1.setVal"
	r := runSrv(t, "set", "object", "--object", obj, "--fc", "SP", "--value", "7", "--type", "int", "--verify", "--format", "json")
	expectCode(t, r, errs.ExitOK)
	golden(t, "set_object_json", r.stdout)
	r = runSrv(t, "set", "object", "--object", obj, "--fc", "SP", "--value", "5", "--type", "int", "--verify")
	expectCode(t, r, errs.ExitOK)
	golden(t, "set_object_text", r.stdout)
	r = runSrv(t, "get", "object", "--object", obj, "--fc", "SP", "--format", "json")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, `"value": 5`) {
		t.Fatalf("read-back: %s", r.stdout)
	}
	// Write failure prints the result and exits 6.
	r = runSrv(t, "set", "object", "--object", "InteropLD/MMXU1.Nope.x", "--fc", "MX", "--value", "1", "--type", "float", "--format", "json")
	expectCode(t, r, errs.ExitRejected)
	if !strings.Contains(r.stdout, `"write_ok": false`) {
		t.Fatalf("json failure body: %s", r.stdout)
	}
}

func TestControlOperate(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"control_operate_direct", []string{"--object", "InteropLD/GGIO1.SPCSO1", "--type", "bool", "--value", "true", "--confirm-ref", "InteropLD/GGIO1.SPCSO1.stVal[ST]"}},
		{"control_operate_sbo_json", []string{"--object", "InteropLD/GGIO1.SPCSO2", "--type", "bool", "--value", "true", "--mode", "sbo", "--confirm-ref", "InteropLD/GGIO1.SPCSO2.stVal[ST]", "--format", "json"}},
		{"control_operate_sbow", []string{"--object", "InteropLD/GGIO1.SPCSO3", "--type", "bool", "--value", "true", "--ctl-num", "9", "--or-ident", "tester", "--check-interlock", "--check-synchro"}},
		{"control_operate_test_bit", []string{"--object", "InteropLD/GGIO1.SPCSO1", "--type", "bool", "--value", "false", "--test", "--confirm-ref", "InteropLD/GGIO1.SPCSO1.stVal[ST]"}},
		{"control_operate_no_confirm", []string{"--object", "InteropLD/GGIO1.SPCSO1", "--type", "bool", "--value", "false", "--no-confirm", "--confirm-ref", "InteropLD/GGIO1.SPCSO1.stVal[ST]"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := runSrv(t, append([]string{"control", "operate"}, tc.args...)...)
			expectCode(t, r, errs.ExitOK)
			golden(t, tc.name, r.stdout)
		})
	}
	// Mode incompatible with ctlModel is a usage error.
	r := runSrv(t, "control", "operate", "--object", "InteropLD/GGIO1.SPCSO1", "--type", "bool", "--value", "true", "--mode", "sbo")
	expectCode(t, r, errs.ExitUsage)
	// A confirmation mismatch is a rejection.
	r = runSrv(t, "control", "operate", "--object", "InteropLD/GGIO1.SPCSO1", "--type", "bool", "--value", "true", "--confirm-ref", "InteropLD/GGIO1.SPS1.stVal[ST]", "--format", "json")
	expectCode(t, r, errs.ExitMismatch)
}

func TestSubscribeReport(t *testing.T) {
	base := []string{"subscribe", "report", "--ld", "InteropLD", "--ln", "LLN0", "--interrogation", "--max-reports", "1", "--duration", "10s"}
	t.Run("urcb text", func(t *testing.T) {
		r := runSrv(t, append(base, "--report", "urcb01", "--type", "RP", "--show-values")...)
		expectCode(t, r, errs.ExitOK)
		for _, want := range []string{"[Report #1]", "Values:", "Total reports: 1", "Connection closed."} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
			}
		}
		if !strings.Contains(r.stdout+r.stderr, "Subscribing to: InteropLD/LLN0.RP.urcb01") {
			t.Errorf("no subscribe banner:\n%s\n%s", r.stdout, r.stderr)
		}
	})
	t.Run("brcb jsonl sync", func(t *testing.T) {
		r := runSrv(t, append(base, "--report", "brcb01", "--type", "BR", "--purge-buf", "--entry-id", "0000000000000000", "--sync", "--format", "jsonl")...)
		expectCode(t, r, errs.ExitOK)
		var events []string
		sc := bufio.NewScanner(strings.NewReader(r.stdout))
		for sc.Scan() {
			var ev map[string]any
			if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
				t.Fatalf("stdout line is not JSON: %q", sc.Text())
			}
			events = append(events, ev["event"].(string))
		}
		if len(events) < 2 || events[len(events)-1] != "summary" || !strings.Contains(strings.Join(events, ","), "report") {
			t.Fatalf("events %v\n%s", events, r.stdout)
		}
	})
	t.Run("duration elapses without max", func(t *testing.T) {
		r := runSrv(t, "subscribe", "report", "--ld", "InteropLD", "--ln", "LLN0", "--report", "urcb01", "--type", "RP", "--duration", "200ms")
		expectCode(t, r, errs.ExitOK)
	})
	t.Run("missing rcb", func(t *testing.T) {
		r := runSrv(t, "subscribe", "report", "--ld", "InteropLD", "--ln", "LLN0", "--report", "nope", "--type", "RP", "--duration", "1s")
		if r.code == errs.ExitOK {
			t.Fatal("subscribing to a missing RCB must fail")
		}
	})
	t.Run("ctrl-c", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(300*time.Millisecond, cancel)
		r := runCLIContext(t, ctx, srvArgs("subscribe", "report", "--ld", "InteropLD", "--ln", "LLN0", "--report", "urcb01", "--type", "RP")...)
		expectCode(t, r, errs.ExitOK)
		if !strings.Contains(r.stderr+r.stdout, "Shutting down gracefully") {
			t.Fatalf("no graceful shutdown:\n%s\n%s", r.stdout, r.stderr)
		}
	})
}

func TestFiles(t *testing.T) {
	r := runSrv(t, "list", "files", "--detailed")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "COMTRADE.cfg") || !strings.Contains(r.stdout, "Size: 17 bytes") {
		t.Fatalf("list files detailed:\n%s", r.stdout)
	}
	r = runSrv(t, "get", "file", "--name", "COMTRADE.cfg")
	expectCode(t, r, errs.ExitOK)
	if r.stdout != "station,rec,1999\n" {
		t.Fatalf("stdout download %q", r.stdout)
	}
	r = runSrv(t, "get", "file", "--name", "COMTRADE.cfg", "--detailed")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stderr, "Checksum (SHA256):") {
		t.Fatalf("detailed metadata goes to stderr: %q", r.stderr)
	}
	out := filepath.Join(t.TempDir(), "events.log")
	for _, detailed := range []bool{false, true} {
		args := []string{"get", "file", "--name", "events.log", "--output", out}
		if detailed {
			args = append(args, "--detailed")
		}
		r = runSrv(t, args...)
		expectCode(t, r, errs.ExitOK)
		if !strings.Contains(r.stdout, "Downloaded events.log ->") {
			t.Fatalf("stdout %q", r.stdout)
		}
		data, err := os.ReadFile(out)
		if err != nil || len(data) != 1100 {
			t.Fatalf("downloaded %d bytes, err=%v", len(data), err)
		}
	}
	r = runSrv(t, "get", "file", "--name", "events.log", "--output", filepath.Join(t.TempDir(), "missing-dir", "x"))
	if r.code == errs.ExitOK {
		t.Fatal("writing into a missing directory must fail")
	}
	r = runSrv(t, "list", "files", "--path", "nomatch*")
	expectCode(t, r, errs.ExitOK)
}

func TestTreeVariants(t *testing.T) {
	r := runSrv(t, "tree", "--path", "InteropLD/MMXU1", "--flatten", "--interval", "1ms")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "InteropLD/MMXU1.TotW.mag.f") {
		t.Fatalf("flat tree:\n%s", r.stdout)
	}
	r = runSrv(t, "tree")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "GGIO1") || !strings.Contains(r.stdout, "Total IEC61850 calls made") {
		t.Fatalf("full tree:\n%s", r.stdout)
	}

	r = runSrv(t, "tree", "--serialize", "--include", "all")
	expectCode(t, r, errs.ExitOK)
	var model map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &model); err != nil {
		t.Fatalf("serialize output is not JSON: %v", err)
	}

	out := filepath.Join(t.TempDir(), "ied.json")
	r = runSrv(t, "tree", "--serialize", "--include", "data_sets,report_control_blocks", "--output", out)
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stderr, "Wrote serialized IED to") {
		t.Fatalf("stderr %q", r.stderr)
	}

	// Round trip: generate a libiec61850 .cfg from the serialized IED.
	cfg := filepath.Join(t.TempDir(), "nested", "ied.cfg")
	r = runCLI(t, "server", "generate-config", "-i", out, "-o", cfg, "--name", "IED")
	expectCode(t, r, errs.ExitOK)
	if b, err := os.ReadFile(cfg); err != nil || !strings.Contains(string(b), "MODEL(IED)") {
		t.Fatalf("cfg: %v\n%.200s", err, b)
	}

	r = runSrv(t, "tree", "--serialize", "--output", filepath.Join(t.TempDir(), "no", "such", "dir.json"))
	if r.code == errs.ExitOK {
		t.Fatal("serialize into missing dir must fail")
	}
}

func TestFindBulk(t *testing.T) {
	dir := t.TempDir()
	mapping := filepath.Join(dir, "mapping.json")
	results := filepath.Join(dir, "results.json")
	write := func(body string) {
		if err := os.WriteFile(mapping, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"iec61850Mappings":[{"controlledPropertyId":"power","baseLn":"MMXU","DoDaPath":"TotW.mag"},{"controlledPropertyId":"x","baseLn":"ZZZZ","DoDaPath":"A.b"}]}`)
	r := runSrv(t, "find", "bulk", "--mapping", mapping, "--output", results)
	expectCode(t, r, errs.ExitOK)
	b, err := os.ReadFile(results)
	if err != nil || !strings.Contains(string(b), "InteropLD/MMXU1.TotW.mag") {
		t.Fatalf("results: %v\n%s", err, b)
	}
	write(`{"iec61850Mappings":[]}`)
	r = runCLI(t, "find", "bulk", "--mapping", mapping, "--output", results)
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "calls made: 0") {
		t.Fatalf("empty mapping: %s", r.stdout)
	}
	r = runCLI(t, "find", "bulk", "--mapping", mapping, "--output", filepath.Join(dir, "missing", "r.json"))
	if r.code == errs.ExitOK {
		t.Fatal("unwritable output must fail")
	}
	// The example mapping in docs/examples parses.
	r = runSrv(t, "find", "bulk", "--mapping", filepath.Join(repoRoot(), "docs", "examples", "mapping.json"), "--output", results)
	expectCode(t, r, errs.ExitOK)
}

func TestFindPathNoMatch(t *testing.T) {
	r := runSrv(t, "find", "path", "--ln", "XCBR.*", "--path", "Pos")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "No matches found") {
		t.Fatal(r.stdout)
	}
	r = runSrv(t, "find", "path", "--ln", "GGIO.*", "--path", "SPCSO1.stVal")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "InteropLD/GGIO1.SPCSO1.stVal") {
		t.Fatal(r.stdout)
	}
	r = runSrv(t, "find", "path", "--ln", "GGIO.*", "--path", "SPS1", "--include-das")
	expectCode(t, r, errs.ExitOK)
}

func TestSCLCommands(t *testing.T) {
	icd := fixture("interop.icd")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"scl_parse_tree", []string{"scl", "parse", "--input", icd}},
		{"scl_parse_tree_detailed", []string{"scl", "parse", "--input", icd, "--detailed"}},
		{"scl_parse_flat", []string{"scl", "parse", "--input", icd, "--flatten"}},
		{"scl_parse_flat_detailed", []string{"scl", "parse", "--input", icd, "--flatten", "--detailed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := runCLI(t, tc.args...)
			expectCode(t, r, errs.ExitOK)
			golden(t, tc.name, r.stdout)
		})
	}
	out := filepath.Join(t.TempDir(), "model.csv")
	r := runCLI(t, "scl", "convert", "--input", icd, "--output", out, "--separator", "|")
	expectCode(t, r, errs.ExitOK)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "scl_convert_pipe", string(b))
	r = runCLI(t, "scl", "convert", "--input", icd, "--output", filepath.Join(t.TempDir(), "x", "y.csv"))
	if r.code == errs.ExitOK {
		t.Fatal("convert into a missing dir must fail")
	}
}

// fakeJournalConn serves journals without a real device (the in-process
// server does not record journal entries for control writes).
type fakeJournalConn struct {
	service.IEC61850Connection
	entries []iec61850.JournalEntry
	more    bool
}

func (f *fakeJournalConn) ListJournals(context.Context, string) ([]string, error) {
	return []string{"LLN0$EventLog"}, nil
}

func (f *fakeJournalConn) ReadJournal(context.Context, string, string, time.Time, time.Time) (*iec61850.JournalReadResult, error) {
	return &iec61850.JournalReadResult{Entries: f.entries, MoreFollows: f.more}, nil
}

func (f *fakeJournalConn) ReadJournalAfter(context.Context, string, string, time.Time, []byte) (*iec61850.JournalReadResult, error) {
	return &iec61850.JournalReadResult{Entries: f.entries}, nil
}

func (f *fakeJournalConn) Close(context.Context) error { return nil }
func (f *fakeJournalConn) Abort(context.Context) error { return nil }

func withFakeConn(t *testing.T, conn service.IEC61850Connection) {
	t.Helper()
	prev := connectFunc
	connectFunc = func(context.Context, client.ConnectionInput) (service.IEC61850Connection, error) { return conn, nil }
	t.Cleanup(func() { connectFunc = prev })
}

func TestJournals(t *testing.T) {
	// Real server: the interop model has no journals.
	r := runSrv(t, "get", "journal", "--domain", "InteropLD", "--journal", "EventLog", "--from", "0")
	if r.code == errs.ExitOK {
		t.Fatal("reading a journal from a model without logs must fail")
	}

	// Real server with a journal-enabled model.
	jsrv, err := server.Start(context.Background(), server.RunConfig{SclPath: filepath.Join("testdata", "journal.icd"), Host: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = jsrv.Close() })
	h, p, _ := net.SplitHostPort(jsrv.Addr())
	r = runCLI(t, "--host", h, "--port", p, "list", "journals", "--domain", "InteropLD")
	expectCode(t, r, errs.ExitOK)
	golden(t, "list_journals", r.stdout)
	r = runCLI(t, "--host", h, "--port", p, "get", "journal", "--domain", "InteropLD", "--journal", "lcb01", "--from", "0")
	expectCode(t, r, errs.ExitOK)
	golden(t, "get_journal_empty", r.stdout)
	r = runCLI(t, "--host", h, "--port", p, "list", "journals", "--ld", "InteropLD", "--domain", "InteropLD")
	expectCode(t, r, errs.ExitUsage)

	// Fake device: entry rendering, pagination hint.
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	withFakeConn(t, &fakeJournalConn{more: true, entries: []iec61850.JournalEntry{{
		EntryID:        []byte{0, 1, 2, 3},
		OccurrenceTime: ts,
		Variables:      []iec61850.JournalVariable{{Tag: "InteropLD/GGIO1$ST$SPCSO1$stVal"}},
	}}})
	r = runCLI(t, "--host", "fake", "list", "journals", "--ld", "LD")
	expectCode(t, r, errs.ExitOK)
	golden(t, "list_journals_fake", r.stdout)
	r = runCLI(t, "--host", "fake", "get", "journal", "--domain", "LD", "--journal", "EventLog", "--from", "2026-01-01T00:00:00Z", "--to", "2026-02-01T00:00:00Z")
	expectCode(t, r, errs.ExitOK)
	golden(t, "get_journal_fake", r.stdout)
	r = runCLI(t, "--host", "fake", "get", "journal", "--domain", "LD", "--journal", "EventLog", "--from", "1767225600000")
	expectCode(t, r, errs.ExitOK)
}

func TestListLDsEmptyAndDetailedErrors(t *testing.T) {
	withFakeConn(t, &emptyConn{})
	for _, args := range [][]string{
		{"list", "lds"}, {"list", "lds", "--format", "json"}, {"list", "lds", "--detailed"},
		{"list", "dos", "--ld", "X", "--ln", "Y"}, {"list", "dos", "--ld", "X", "--ln", "Y", "--format", "json"},
		{"list", "dss", "--ld", "X", "--ln", "Y"}, {"list", "dss", "--ld", "X", "--ln", "Y", "--format", "json"},
		{"list", "reports", "--ld", "X", "--ln", "Y"}, {"list", "reports", "--all"},
		{"list", "files"}, {"list", "journals", "--ld", "X"},
	} {
		r := runCLI(t, append([]string{"--host", "fake"}, args...)...)
		expectCode(t, r, errs.ExitOK)
		golden(t, "empty_"+strings.ReplaceAll(strings.Join(args, "_"), "-", ""), r.stdout)
	}
	// A data object that exists under no FC is "not found".
	r := runCLI(t, "--host", "fake", "list", "das", "--ld", "X", "--ln", "Y", "--do", "Z")
	expectCode(t, r, errs.ExitNotFound)
	// An LD with no logical nodes that the directory does not list is "not
	// found" (servers that answer GetNameList for unknown domains with []).
	for _, format := range []string{"text", "json"} {
		r = runCLI(t, "--host", "fake", "list", "lns", "--ld", "X", "--format", format)
		expectCode(t, r, errs.ExitNotFound)
	}
}

// emptyConn is a device with an empty model.
type emptyConn struct{ service.IEC61850Connection }

func (emptyConn) ListLogicalDevices(context.Context) ([]iec61850.LogicalDevice, error) {
	return nil, nil
}
func (emptyConn) ListLogicalNodes(context.Context, string) ([]iec61850.LogicalNode, error) {
	return nil, nil
}
func (emptyConn) ListDataObjects(context.Context, string, string) ([]iec61850.DataObject, error) {
	return nil, nil
}
func (emptyConn) ListChildren(context.Context, iec61850.Ref) ([]iec61850.BrowseNode, error) {
	return nil, nil
}
func (emptyConn) ListDataSets(context.Context, string) ([]string, error) { return nil, nil }
func (emptyConn) ListReports(context.Context, string) ([]string, error)  { return nil, nil }
func (emptyConn) ListFiles(context.Context, string) ([]iec61850.FileEntry, error) {
	return nil, nil
}
func (emptyConn) ListJournals(context.Context, string) ([]string, error) { return nil, nil }
func (emptyConn) GetVariableType(context.Context, iec61850.Ref) (*mms.TypeSpec, error) {
	return nil, iec61850.ErrNotFound
}
func (emptyConn) Read(context.Context, iec61850.Ref) (*iec61850.Value, error) {
	return nil, iec61850.ErrNotFound
}
func (emptyConn) Close(context.Context) error { return nil }
func (emptyConn) Abort(context.Context) error { return nil }

func TestServerStartCommand(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan result, 1)
	var stdout, stderr lockedBuffer
	go func() {
		cliMu.Lock()
		defer cliMu.Unlock()
		resetFlags(rootCmd)
		code := ExecuteContext(ctx, []string{"--log-level", "info", "server", "start", "--scl", fixture("interop.icd"), "--host", "127.0.0.1", "--port", "0",
			"--ready-json", "--fixture-id", "fx1", "--files-dir", dir}, &stdout, &stderr)
		done <- result{stdout: stdout.String(), stderr: stderr.String(), code: code}
	}()
	deadline := time.Now().Add(15 * time.Second)
	var ev server.ReadyEvent
	for {
		if line, _, ok := strings.Cut(stdout.String(), "\n"); ok {
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				t.Fatalf("ready line %q: %v", line, err)
			}
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("no ready event; stderr=%s", stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ev.Fixture != "fx1" || ev.IEDName != "InteropIED" {
		t.Fatalf("ready %+v", ev)
	}
	// The server answers MMS on the advertised address.
	c, err := iec61850.Dial(ctx, ev.Address, iec61850.DialOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close(ctx)
	cancel()
	select {
	case r := <-done:
		expectCode(t, r, errs.ExitOK)
		if !strings.Contains(r.stderr, "listening on") {
			t.Fatalf("stderr %q", r.stderr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server start did not stop on cancel")
	}
}

func TestHTTPCommand(t *testing.T) {
	// Reserve a free port for the HTTP listener.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan result, 1)
	var stdout, stderr lockedBuffer
	go func() {
		cliMu.Lock()
		defer cliMu.Unlock()
		resetFlags(rootCmd)
		code := ExecuteContext(ctx, []string{"http", "--listen", addr, "--iec-host", testServer.host, "--iec-port", strconv.Itoa(testServer.port)}, &stdout, &stderr)
		done <- result{stdout: stdout.String(), stderr: stderr.String(), code: code}
	}()
	var body []byte
	deadline := time.Now().Add(15 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + "/api/logical-devices/names")
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("http server not up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(string(body), "InteropLD") {
		t.Fatalf("body %s", body)
	}
	cancel()
	select {
	case r := <-done:
		expectCode(t, r, errs.ExitOK)
		if !strings.Contains(r.stdout, "HTTP server listening on "+addr) {
			t.Fatalf("stdout %q", r.stdout)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("http did not stop on cancel")
	}

	// Device unreachable → connection error before listening.
	r := runCLI(t, "http", "--listen", "127.0.0.1:0", "--iec-host", "127.0.0.1", "--iec-port", "1")
	expectCode(t, r, errs.ExitConnection)
	// Listen address in use → connection error.
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()
	r = runCLI(t, "http", "--listen", busy.Addr().String(), "--iec-host", testServer.host, "--iec-port", strconv.Itoa(testServer.port))
	expectCode(t, r, errs.ExitConnection)
}

func TestDiscover(t *testing.T) {
	r := runCLI(t, "discover", "--host", "127.0.0.1", "--port", strconv.Itoa(testServer.port))
	expectCode(t, r, errs.ExitOK)
	golden(t, "discover_one", r.stdout)
	// A range where nothing listens on the port.
	r = runCLI(t, "scan", "--host", "127.0.0.1-3", "--port", "1")
	expectCode(t, r, errs.ExitOK)
	golden(t, "discover_none", r.stdout)
	// A TCP listener that is not an MMS server.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	r = runCLI(t, "discover", "--host", "127.0.0.1", "--port", port, "--resolve-mac")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stdout, "TCP Open but IEC 61850 Failed") {
		t.Fatalf("stdout:\n%s", r.stdout)
	}
}

func TestStructuredLogging(t *testing.T) {
	r := runSrv(t, "--log-level", "debug", "--log-format", "json", "list", "lds", "--format", "json")
	expectCode(t, r, errs.ExitOK)
	var lds []map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &lds); err != nil {
		t.Fatalf("logs leaked into stdout: %v\n%s", err, r.stdout)
	}
	sawLog := false
	for _, line := range strings.Split(r.stderr, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("non-JSON log line %q", line)
		}
		sawLog = true
	}
	if !sawLog {
		t.Fatalf("no JSON log records on stderr:\n%s", r.stderr)
	}
	r = runSrv(t, "--debug", "list", "lds")
	expectCode(t, r, errs.ExitOK)
	if !strings.Contains(r.stderr, "level=DEBUG") && !strings.Contains(r.stderr, "level=INFO") {
		t.Fatalf("--debug produced no text logs:\n%s", r.stderr)
	}
	r = runSrv(t, "--log-level", "off", "--debug", "list", "lds")
	expectCode(t, r, errs.ExitOK)
	if strings.Contains(r.stderr, "level=") {
		t.Fatalf("--log-level=off must win over --debug:\n%s", r.stderr)
	}
}

// TestExamplesContract runs every examples.json case marked for the "self"
// stack in-process and checks the exit code and documented JSON fields.
func TestExamplesContract(t *testing.T) {
	b, err := os.ReadFile(fixture("examples.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Title        string         `json:"title"`
			Stacks       []string       `json:"stacks"`
			Argv         []string       `json:"argv"`
			ExpectedExit int            `json:"expected_exit"`
			Fields       map[string]any `json:"expect_json_fields"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, c := range doc.Cases {
		self := false
		for _, s := range c.Stacks {
			self = self || s == "self"
		}
		if !self {
			continue
		}
		ran++
		t.Run(c.Title, func(t *testing.T) {
			r := runSrv(t, c.Argv...)
			expectCode(t, r, c.ExpectedExit)
			for k, v := range c.Fields {
				switch k {
				case "contains_name":
					if !strings.Contains(r.stdout, `"`+v.(string)+`"`) {
						t.Errorf("missing name %v", v)
					}
				case "contains_names":
					for _, n := range v.([]any) {
						if !strings.Contains(r.stdout, `"`+n.(string)+`"`) {
							t.Errorf("missing name %v", n)
						}
					}
				default:
					var obj map[string]any
					if err := json.Unmarshal([]byte(r.stdout), &obj); err == nil {
						if got, ok := obj[k]; ok && got != v {
							t.Errorf("%s = %v, want %v", k, got, v)
						}
					}
				}
			}
		})
	}
	if ran == 0 {
		t.Fatal("no self cases in examples.json")
	}
}
