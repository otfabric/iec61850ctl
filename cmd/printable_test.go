// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

func TestPrintableWriter(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"ASCII", "LD0/LLN0.Mod.stVal = 1", "LD0/LLN0.Mod.stVal = 1"},
		{"newline and tab", "a\tb\nc\n", "a\tb\nc\n"},
		{"ESC", "a\x1b[2Jb", `a\u001b[2Jb`},
		{"CR", "a\rb", `a\u000db`},
		{"BS", "a\bb", `a\u0008b`},
		{"BEL", "\a", `\u0007`},
		{"NUL", "\x00", `\u0000`},
		{"DEL", "\x7f", `\u007f`},
		{"C1 CSI", "a\u009b31mb", `a\u009b31mb`},
		{"C1 NEL and APC", "\u0085\u009f", `\u0085\u009f`},
		{"lone 0x9b", "a\x9bb", `a\x9bb`},
		{"0xff", "\xff", `\xff`},
		{"truncated rune", "\xe2\x94", `\xe2\x94`},
		{"box drawing", "├── │", "├── │"},
		{"Latin-1 letter", "é ü", "é ü"},
		{"U+00A0", "\u00a0", "\u00a0"},
		{"emoji", "\U0001F600", "\U0001F600"},
		{"U+FFFD", "\ufffd", "\ufffd"},
		{"mixed", "ok\x1bok\u009bok\xffok\n", `ok\u001bok\u009bok\xffok` + "\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			n, err := newPrintableWriter(&buf).Write([]byte(c.in))
			if err != nil || n != len(c.in) {
				t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(c.in))
			}
			if buf.String() != c.want {
				t.Fatalf("wrote %q, want %q", buf.String(), c.want)
			}
			if firstUnprintable(buf.Bytes()) != buf.Len() {
				t.Fatalf("output %q still holds a control", buf.String())
			}
		})
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

func TestPrintableWriterError(t *testing.T) {
	for _, in := range []string{"plain", "with \x1b"} {
		if n, err := newPrintableWriter(failWriter{}).Write([]byte(in)); !errors.Is(err, io.ErrShortWrite) || n != 0 {
			t.Fatalf("%q: Write = %d, %v; want 0, %v", in, n, err, io.ErrShortWrite)
		}
	}
	if newPrintableWriter(nil) != nil {
		t.Fatal("newPrintableWriter(nil) is not nil")
	}
	var buf bytes.Buffer
	if rawWriter(newPrintableWriter(&buf)) != io.Writer(&buf) || rawWriter(&buf) != io.Writer(&buf) {
		t.Fatal("rawWriter does not return the writer below")
	}
}

// TestCompletionScriptsUnchanged: the completion scripts hold no byte that
// the writer of stdout escapes, so they are written as cobra generates them.
// (Cobra's completion command keeps the writer of the first run of the
// process, so the scripts are generated here rather than through runCLI.)
func TestCompletionScriptsUnchanged(t *testing.T) {
	cliMu.Lock()
	defer cliMu.Unlock()
	gens := map[string]func(io.Writer) error{
		"bash":                func(w io.Writer) error { return rootCmd.GenBashCompletionV2(w, true) },
		"bash, no desc":       func(w io.Writer) error { return rootCmd.GenBashCompletionV2(w, false) },
		"zsh":                 rootCmd.GenZshCompletion,
		"zsh, no desc":        rootCmd.GenZshCompletionNoDesc,
		"fish":                func(w io.Writer) error { return rootCmd.GenFishCompletion(w, true) },
		"fish, no desc":       func(w io.Writer) error { return rootCmd.GenFishCompletion(w, false) },
		"powershell":          rootCmd.GenPowerShellCompletionWithDesc,
		"powershell, no desc": rootCmd.GenPowerShellCompletion,
	}
	for name, gen := range gens {
		var raw, filtered bytes.Buffer
		if err := gen(&raw); err != nil {
			t.Fatal(err)
		}
		if err := gen(newPrintableWriter(&filtered)); err != nil {
			t.Fatal(err)
		}
		if i := firstUnprintable(raw.Bytes()); i != raw.Len() {
			t.Errorf("%s: byte %#x at %d of the script is escaped on stdout", name, raw.Bytes()[i], i)
		}
		if raw.Len() == 0 || filtered.String() != raw.String() {
			t.Errorf("%s: the script on stdout (%d bytes) differs from the generated one (%d bytes)", name, filtered.Len(), raw.Len())
		}
	}
}

// textConn is a device whose names and error messages hold terminal
// controls and bytes that are not UTF-8.
type textConn struct {
	emptyConn
	names []string
	err   error
}

func (c *textConn) ListLogicalDevices(context.Context) ([]iec61850.LogicalDevice, error) {
	if c.err != nil {
		return nil, c.err
	}
	lds := make([]iec61850.LogicalDevice, len(c.names))
	for i, n := range c.names {
		lds[i] = iec61850.LogicalDevice{Name: n}
	}
	return lds, nil
}

// expectPrintable fails when out holds a byte that a terminal reads as a
// control (anything firstUnprintable stops at).
func expectPrintable(t *testing.T, what, out string) {
	t.Helper()
	if i := firstUnprintable([]byte(out)); i != len(out) {
		t.Fatalf("%s holds byte %#x at %d: %q", what, out[i], i, out)
	}
}

// TestDeviceTextEscaped: names and messages from a device reach stdout and
// stderr with their controls escaped; JSON output stays valid and decodes
// to the names the device sent.
func TestDeviceTextEscaped(t *testing.T) {
	names := []string{"LD\x1b[2J\x1b]0;t\x07", "LD\rX\bY", "LD\u009b31m\x7f"}
	withFakeConn(t, &textConn{names: append(names, "LD\xffZ")})

	r := runCLI(t, "--host", "fake", "list", "lds")
	expectCode(t, r, errs.ExitOK)
	expectPrintable(t, "text stdout", r.stdout)
	for _, want := range []string{`LD\u001b[2J\u001b]0;t\u0007`, `LD\u000dX\u0008Y`, `LD\u009b31m\u007f`, `LD\xffZ`} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("text stdout lacks %q:\n%s", want, r.stdout)
		}
	}

	for _, format := range []string{"csv", "table", "yaml"} {
		r = runCLI(t, "--host", "fake", "list", "lds", "--format", format)
		expectCode(t, r, errs.ExitOK)
		expectPrintable(t, format+" stdout", r.stdout)
	}

	withFakeConn(t, &textConn{names: names})
	r = runCLI(t, "--host", "fake", "list", "lds", "--format", "json")
	expectCode(t, r, errs.ExitOK)
	expectPrintable(t, "JSON stdout", r.stdout)
	var got any
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("JSON stdout does not decode: %v\n%s", err, r.stdout)
	}
	for _, n := range names {
		if !strings.Contains(jsonStrings(got), "|"+n+"|") {
			t.Fatalf("JSON stdout does not decode to %q:\n%s", n, r.stdout)
		}
	}

	withFakeConn(t, &textConn{err: errors.New("device says \x1b]0;title\x07\u009b2J")})
	r = runCLI(t, "--host", "fake", "list", "lds")
	if r.code == errs.ExitOK {
		t.Fatal("list lds succeeded with a failing device")
	}
	expectPrintable(t, "stderr", r.stderr)
	if !strings.Contains(r.stderr, `device says \u001b]0;title\u0007\u009b2J`) {
		t.Fatalf("stderr lacks the escaped message:\n%s", r.stderr)
	}
}

// jsonStrings returns every string in a decoded JSON value, each between
// bars.
func jsonStrings(v any) string {
	var b strings.Builder
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			b.WriteString("|" + x + "|")
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(v)
	return b.String()
}

// bytesFileConn serves one file holding data.
type bytesFileConn struct {
	service.IEC61850Connection
	data []byte
}

func (c *bytesFileConn) DownloadFile(_ context.Context, name string, w io.Writer) (*iec61850.FileEntry, error) {
	if _, err := w.Write(c.data); err != nil {
		return nil, err
	}
	return &iec61850.FileEntry{Name: name, Size: int64(len(c.data))}, nil
}
func (c *bytesFileConn) Close(context.Context) error { return nil }
func (c *bytesFileConn) Abort(context.Context) error { return nil }

// TestGetFileBytesUnchanged: get file writes the file's bytes to stdout as
// they are, controls and all.
func TestGetFileBytesUnchanged(t *testing.T) {
	data := make([]byte, 0, 512)
	for range 2 {
		for b := range 256 {
			data = append(data, byte(b))
		}
	}
	withFakeConn(t, &bytesFileConn{data: data})
	r := runCLI(t, "--host", "fake", "get", "file", "--name", "f.bin")
	expectCode(t, r, errs.ExitOK)
	if r.stdout != string(data) {
		t.Fatalf("stdout is %d bytes and differs from the file of %d bytes", len(r.stdout), len(data))
	}
}

// TestGooseSniffTextEscaped: a GOOSE string value holding terminal
// controls is written escaped by goose sniff, in text and in JSON lines.
func TestGooseSniffTextEscaped(t *testing.T) {
	const s = "on\x1b[2J\x1b]0;t\x07\u009b31m\x7f"
	frame, err := goose.Encode(&goose.Frame{
		Dst: goose.MAC{0x01, 0x0c, 0xcd, 0x01, 0x00, 0x09}, Src: goose.MAC{0x02, 0, 0, 0, 0, 0x09}, AppID: 0x1009,
		PDU: goose.PDU{
			GocbRef: "LD/LLN0$GO$gcb", TimeAllowedToLive: 2000, DatSet: "LD/LLN0$ds", GoID: "id",
			T: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), StNum: 1, ConfRev: 1,
			AllData: []*mms.Value{mms.NewMmsString(s)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	hub := goose.NewHub()
	sender := hub.NewPort(goose.MAC{0x02, 0, 0, 0, 0, 0x09})
	t.Cleanup(func() { _ = sender.Close() })
	prev := gooseOpen
	gooseOpen = func(string, goose.TransportOptions) (goose.Transport, error) {
		// The frame is sent once the sniffer's port is on the hub.
		p := hub.NewPort(goose.MAC{0x02, 0, 0, 0, 0, 0x10})
		return p, sender.Send(frame)
	}
	t.Cleanup(func() { gooseOpen = prev })

	r := runCLI(t, "goose", "sniff", "--iface", "hub0", "--count", "1", "--duration", "10s")
	expectCode(t, r, errs.ExitOK)
	expectPrintable(t, "text stdout", r.stdout)
	if want := `on\u001b[2J\u001b]0;t\u0007\u009b31m\u007f`; !strings.Contains(r.stdout, want) {
		t.Fatalf("text stdout lacks %q:\n%s", want, r.stdout)
	}

	r = runCLI(t, "goose", "sniff", "--iface", "hub0", "--count", "1", "--duration", "10s", "--format", "jsonl")
	expectCode(t, r, errs.ExitOK)
	expectPrintable(t, "JSON lines", r.stdout)
	line, _, _ := strings.Cut(r.stdout, "\n")
	var got any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("JSON line does not decode: %v\n%s", err, line)
	}
	if !strings.Contains(jsonStrings(got), s) {
		t.Fatalf("JSON line does not decode to the value %q:\n%s", s, line)
	}
}
