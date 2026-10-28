// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-mms"
)

func newProvider(t *testing.T, files map[string]string) *DirFileProvider {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	p, err := NewDirFileProvider(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Release() })
	return p
}

func TestDirFileProviderList(t *testing.T) {
	t.Parallel()
	p := newProvider(t, map[string]string{"a.cfg": "1", "b.cfg": "22", "c.log": "333"})
	ctx := context.Background()
	tests := []struct {
		req  mms.FileListRequest
		want []string
		more bool
	}{
		{mms.FileListRequest{}, []string{"a.cfg", "b.cfg", "c.log"}, false},
		{mms.FileListRequest{FileSpec: "*"}, []string{"a.cfg", "b.cfg", "c.log"}, false},
		{mms.FileListRequest{FileSpec: "/*.cfg"}, []string{"a.cfg", "b.cfg"}, false},
		{mms.FileListRequest{FileSpec: "*", ContinueAfter: "a.cfg"}, []string{"b.cfg", "c.log"}, false},
		{mms.FileListRequest{FileSpec: "*", MaxEntries: 2}, []string{"a.cfg", "b.cfg"}, true},
		{mms.FileListRequest{FileSpec: "nomatch"}, nil, false},
	}
	for _, tt := range tests {
		res, err := p.List(ctx, tt.req)
		if err != nil {
			t.Fatalf("%+v: %v", tt.req, err)
		}
		var got []string
		for _, e := range res.Entries {
			got = append(got, e.Name)
		}
		if strings.Join(got, ",") != strings.Join(tt.want, ",") || res.MoreFollows != tt.more {
			t.Errorf("%+v: got %v more=%v", tt.req, got, res.MoreFollows)
		}
	}
	if _, err := p.List(ctx, mms.FileListRequest{FileSpec: "["}); err == nil {
		t.Error("bad pattern accepted")
	}
}

func TestDirFileProviderOpenRead(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("x", 10)
	p := newProvider(t, map[string]string{"f.bin": body, "empty": ""})
	ctx := context.Background()

	for _, chunk := range []int{3, 5, 10, 64, 0} {
		h, attrs, err := p.Open(ctx, "/f.bin")
		if err != nil || attrs.Size != 10 || attrs.LastModified.IsZero() {
			t.Fatalf("open: %v %+v", err, attrs)
		}
		var got []byte
		for i := 0; ; i++ {
			data, more, err := p.Read(ctx, h, chunk)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, data...)
			if !more {
				break
			}
			if i > 20 {
				t.Fatal("no EOF")
			}
		}
		if string(got) != body {
			t.Fatalf("chunk %d: got %q", chunk, got)
		}
		if err := p.Close(ctx, h); err != nil {
			t.Fatal(err)
		}
	}

	h, _, err := p.Open(ctx, "empty")
	if err != nil {
		t.Fatal(err)
	}
	if data, more, err := p.Read(ctx, h, 8); err != nil || more || len(data) != 0 {
		t.Fatalf("empty read %q %v %v", data, more, err)
	}
	_ = p.Close(ctx, h)
	if _, _, err := p.Read(ctx, h, 8); err == nil {
		t.Fatal("read after close")
	}
}

func TestDirFileProviderRejects(t *testing.T) {
	t.Parallel()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := newProvider(t, map[string]string{"ok": "1"})
	ctx := context.Background()
	for _, name := range []string{"missing", "sub", "../secret", "..\\..\\secret", "/../../etc/passwd"} {
		if h, _, err := p.Open(ctx, name); err == nil {
			_ = p.Close(ctx, h)
			t.Errorf("Open(%q) succeeded", name)
		}
	}
	if _, _, err := p.Open(ctx, "missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing: %v", err)
	}
	if _, _, err := p.Open(ctx, "sub"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("dir: %v", err)
	}
	if _, _, err := p.Read(ctx, "not a handle", 1); err == nil {
		t.Error("bad handle read")
	}
	if err := p.Close(ctx, 42); err == nil {
		t.Error("bad handle close")
	}
	for _, err := range []error{p.Delete(ctx, "ok"), p.Rename(ctx, "ok", "x"), p.ObtainFile(ctx, "ok", "x")} {
		if !errors.Is(err, mms.ErrFileAccessDenied) {
			t.Errorf("write op: %v", err)
		}
	}
	if _, err := NewDirFileProvider(outside); err == nil {
		t.Error("file as root accepted")
	}
	_ = p.Release()
	if _, err := p.List(ctx, mms.FileListRequest{}); err == nil {
		t.Error("list after release")
	}
}

func TestCleanName(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"": "", "/": "", "a": "a", "/a/../b": "b", "../../x": "x", `a\b`: "a/b"} {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
