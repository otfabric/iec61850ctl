// SPDX-License-Identifier: MIT

package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		level   slog.Level
		enabled bool
		bad     bool
	}{
		{"debug", slog.LevelDebug, true, false},
		{"INFO", slog.LevelInfo, true, false},
		{"", slog.LevelInfo, true, false},
		{"warn", slog.LevelWarn, true, false},
		{" warning ", slog.LevelWarn, true, false},
		{"error", slog.LevelError, true, false},
		{"off", 0, false, false},
		{"none", 0, false, false},
		{"quiet", 0, false, false},
		{"trace", 0, false, true},
	}
	for _, tt := range tests {
		lvl, en, err := ParseLevel(tt.in)
		if tt.bad {
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Errorf("%q: want invalid input, got %v", tt.in, err)
			}
			continue
		}
		if err != nil || lvl != tt.level || en != tt.enabled {
			t.Errorf("%q: got %v %v %v", tt.in, lvl, en, err)
		}
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l, err := New(&buf, "info", "json")
	if err != nil {
		t.Fatal(err)
	}
	l.Debug("hidden")
	l.Info("shown", "k", 1)
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil || rec["msg"] != "shown" {
		t.Fatalf("json log %q: %v", buf.String(), err)
	}

	buf.Reset()
	l, _ = New(&buf, "debug", "TEXT")
	l.Debug("dbg")
	if !strings.Contains(buf.String(), "msg=dbg") {
		t.Fatalf("text log %q", buf.String())
	}

	buf.Reset()
	l, _ = New(&buf, "off", "")
	l.Error("nothing")
	if buf.Len() != 0 {
		t.Fatal("off must discard")
	}
	if l, err := New(nil, "debug", "text"); err != nil || l == nil {
		t.Fatal("nil writer")
	}
	if _, err := New(&buf, "debug", "xml"); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatalf("bad format: %v", err)
	}
	if _, err := New(&buf, "loud", "text"); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatalf("bad level: %v", err)
	}
}
