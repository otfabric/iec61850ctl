// SPDX-License-Identifier: MIT

package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

func TestServeGracefulShutdown(t *testing.T) {
	t.Parallel()
	conn := &errMockConn{}
	s := NewServerWithApp("127.0.0.1:0", app.New(conn))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get("http://" + ln.Addr().String() + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return after ctx cancel")
	}
	if n := conn.closed.Load(); n != 1 {
		t.Fatalf("Close called %d times, want 1", n)
	}
	if n := conn.aborted.Load(); n != 0 {
		t.Fatalf("Abort called %d times, want 0", n)
	}
}

func TestServeListenerError(t *testing.T) {
	t.Parallel()
	conn := &errMockConn{closeErr: errors.New("close failed")}
	s := NewServerWithApp("127.0.0.1:0", app.New(conn))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close() // Serve fails immediately on a closed listener.

	if err := s.Serve(context.Background(), ln); err == nil {
		t.Fatal("Serve on closed listener returned nil")
	}
	if conn.closed.Load() != 1 {
		t.Fatalf("Close called %d times, want 1", conn.closed.Load())
	}
	// A failing Close falls back to Abort.
	if conn.aborted.Load() != 1 {
		t.Fatalf("Abort called %d times, want 1", conn.aborted.Load())
	}
}

func TestStartListenError(t *testing.T) {
	t.Parallel()
	conn := &errMockConn{}
	s := NewServerWithApp("127.0.0.1:-1", app.New(conn))
	err := s.Start()
	if err == nil {
		t.Fatal("Start with invalid address returned nil")
	}
	if !errors.Is(err, errs.ErrConnectionFailed) {
		t.Fatalf("err=%v, want ErrConnectionFailed", err)
	}
}

func TestClose(t *testing.T) {
	t.Parallel()
	conn := &errMockConn{}
	s := NewServerWithApp(":0", app.New(conn))
	s.Close()
	if conn.closed.Load() != 1 {
		t.Fatalf("Close called %d times, want 1", conn.closed.Load())
	}
	// Nil app / nil connection must not panic.
	(&Server{}).Close()
	NewServerWithApp(":0", app.New(nil)).Close()
}

func TestNewServerConnectionRefused(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	s, err := NewServer(ctx, Config{
		ListenAddr: "127.0.0.1:0",
		Connection: client.ConnectionInput{Host: "127.0.0.1", Port: 1, ConnectTimeout: 2 * time.Second},
	})
	if err == nil {
		s.Close()
		t.Fatal("NewServer against refused port returned nil error")
	}
	if s != nil {
		t.Fatal("NewServer returned non-nil server on error")
	}
	if !errors.Is(err, errs.ErrConnectionFailed) {
		t.Fatalf("err=%v, want errors.Is ErrConnectionFailed", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("NewServer took %v, want fail-fast", elapsed)
	}
}
