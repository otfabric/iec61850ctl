// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// chunkFileConn serves a file of size bytes through DownloadFile in chunks
// of 32 KiB, one Write each, and stops at the first failed Write, as
// go-iec61850 does.
type chunkFileConn struct {
	service.IEC61850Connection
	size int
	sent int
}

func (c *chunkFileConn) DownloadFile(_ context.Context, name string, w io.Writer) (*iec61850.FileEntry, error) {
	chunk := make([]byte, 32<<10)
	for i := range chunk {
		chunk[i] = 'a' + byte(i%26)
	}
	for c.sent < c.size {
		n, err := w.Write(chunk[:min(len(chunk), c.size-c.sent)])
		c.sent += n
		if err != nil {
			return nil, fmt.Errorf("iec61850: download file %q: write: %w", name, err)
		}
	}
	return &iec61850.FileEntry{Name: name, Size: int64(c.size)}, nil
}

func (c *chunkFileConn) Close(context.Context) error { return nil }
func (c *chunkFileConn) Abort(context.Context) error { return nil }

func TestGetFileMaxBytesDefault(t *testing.T) {
	if getFileMaxBytes != 256<<20 {
		t.Fatalf("getFileMaxBytes = %d, SECURITY.md documents 256 MiB", getFileMaxBytes)
	}
}

// TestGetFileMemoryBound: get file holds at most getFileMaxBytes of a file;
// a larger one is stopped after that many bytes, nothing is written to
// stdout or --output, and the command exits 10. The allocation stays a
// small multiple of the bound, whatever the size of the file.
func TestGetFileMemoryBound(t *testing.T) {
	const limit = 1 << 20
	prev := getFileMaxBytes
	getFileMaxBytes = limit
	t.Cleanup(func() { getFileMaxBytes = prev })

	for _, c := range []struct {
		name string
		size int
		want int
	}{
		{"empty", 0, errs.ExitOK},
		{"one byte", 1, errs.ExitOK},
		{"limit-1", limit - 1, errs.ExitOK},
		{"limit", limit, errs.ExitOK},
		{"limit+1", limit + 1, errs.ExitFailure},
		{"64 MiB", 64 << 20, errs.ExitFailure},
	} {
		for _, toFile := range []bool{false, true} {
			name := c.name + "/stdout"
			if toFile {
				name = c.name + "/output"
			}
			t.Run(name, func(t *testing.T) {
				conn := &chunkFileConn{size: c.size}
				withFakeConn(t, conn)
				out := filepath.Join(t.TempDir(), "file.bin")
				args := []string{"--host", "fake", "get", "file", "--name", "file.bin"}
				if toFile {
					args = append(args, "--output", out)
				}
				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				r := runCLI(t, args...)
				runtime.ReadMemStats(&after)

				// The buffer grows to the bound by doubling (2 × limit in
				// all); the test's capture of stdout and its two copies add
				// about 3 × limit, and the race detector more. A file of
				// 64 MiB stays within the same budget.
				if budget := uint64(12 * limit); after.TotalAlloc-before.TotalAlloc > budget {
					t.Fatalf("%d bytes allocated for a file of %d bytes: more than %d", after.TotalAlloc-before.TotalAlloc, c.size, budget)
				}
				expectCodeQuiet(t, r, c.want)
				_, statErr := os.Stat(out)
				if c.want != errs.ExitOK {
					if !strings.Contains(r.stderr, "larger than 1048576 bytes") {
						t.Fatalf("stderr %q does not name the bound", r.stderr)
					}
					if conn.sent > limit {
						t.Fatalf("%d bytes taken from the server, more than the bound %d", conn.sent, limit)
					}
					if r.stdout != "" || statErr == nil {
						t.Fatalf("written although the download stopped: %d bytes to stdout, --output exists=%v", len(r.stdout), statErr == nil)
					}
					return
				}
				got := len(r.stdout)
				if toFile {
					data, err := os.ReadFile(out)
					if err != nil {
						t.Fatal(err)
					}
					got = len(data)
				}
				if got != c.size {
					t.Fatalf("wrote %d bytes, want %d", got, c.size)
				}
			})
		}
	}
}

// expectCodeQuiet is expectCode without the output, which can be large here.
func expectCodeQuiet(t *testing.T, r result, want int) {
	t.Helper()
	if r.code != want {
		t.Fatalf("exit=%d want %d; stderr: %.300s", r.code, want, r.stderr)
	}
}
