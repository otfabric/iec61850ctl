// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSetLogger(t *testing.T) {
	prev := Logger()
	t.Cleanup(func() { SetLogger(prev) })

	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	debugf("hello %s", "world")
	if !strings.Contains(buf.String(), "hello world") || !strings.Contains(buf.String(), "component=service") {
		t.Fatalf("debug line missing: %q", buf.String())
	}

	buf.Reset()
	SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	debugf("filtered")
	if buf.Len() != 0 {
		t.Fatalf("debug should be filtered at info: %q", buf.String())
	}

	SetLogger(nil)
	if Logger() != nil {
		t.Fatal("expected nil logger")
	}
	debugf("silent") // disabled path
}
