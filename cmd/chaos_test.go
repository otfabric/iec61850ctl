// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/stack/server"
)

// fault is what the chaos proxy does to server→client TPKT frames once the
// first `after` frames have passed (COTP CC and the MMS initiate response
// are frames 1 and 2, so after=2 faults the first real request).
type fault int

const (
	faultNone      fault = iota
	faultBlackhole       // stop forwarding; the peer stays connected and silent
	faultReset           // close both sides with RST
	faultDelay           // forward each frame after a fixed delay
)

// chaosProxy is a TCP proxy between the CLI and the shared test server that
// injects faults at TPKT frame granularity.
type chaosProxy struct {
	ln     net.Listener
	target string
	fault  fault
	after  int
	delay  time.Duration

	mu    sync.Mutex
	conns []net.Conn
	wg    sync.WaitGroup
}

func newChaosProxy(t *testing.T, f fault, after int, delay time.Duration) *chaosProxy {
	t.Helper()
	return newChaosProxyTo(t, testServer.addr, f, after, delay)
}

func newChaosProxyTo(t *testing.T, target string, f fault, after int, delay time.Duration) *chaosProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &chaosProxy{ln: ln, target: target, fault: f, after: after, delay: delay}
	p.wg.Add(1)
	go p.accept()
	t.Cleanup(p.close)
	return p
}

func (p *chaosProxy) port() string {
	_, port, _ := net.SplitHostPort(p.ln.Addr().String())
	return port
}

func (p *chaosProxy) track(c net.Conn) {
	p.mu.Lock()
	p.conns = append(p.conns, c)
	p.mu.Unlock()
}

// trip resets every live connection (connection loss mid-session).
func (p *chaosProxy) trip() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		reset(c)
	}
}

func (p *chaosProxy) close() {
	_ = p.ln.Close()
	p.trip()
	p.wg.Wait()
}

func reset(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	_ = c.Close()
}

func (p *chaosProxy) accept() {
	defer p.wg.Done()
	for {
		down, err := p.ln.Accept()
		if err != nil {
			return
		}
		up, err := net.Dial("tcp", p.target)
		if err != nil {
			_ = down.Close()
			continue
		}
		p.track(down)
		p.track(up)
		p.wg.Add(2)
		go func() {
			defer p.wg.Done()
			_, _ = io.Copy(up, down)
			_ = up.Close()
		}()
		go func() {
			defer p.wg.Done()
			p.serverToClient(up, down)
			_ = down.Close()
		}()
	}
}

// serverToClient relays whole TPKT frames, applying the fault.
func (p *chaosProxy) serverToClient(up, down net.Conn) {
	hdr := make([]byte, 4)
	for n := 1; ; n++ {
		if _, err := io.ReadFull(up, hdr); err != nil {
			return
		}
		body := make([]byte, int(binary.BigEndian.Uint16(hdr[2:4]))-4)
		if _, err := io.ReadFull(up, body); err != nil {
			return
		}
		if n > p.after {
			switch p.fault {
			case faultBlackhole:
				continue
			case faultReset:
				reset(down)
				reset(up)
				return
			case faultDelay:
				time.Sleep(p.delay)
			case faultNone:
			}
		}
		if _, err := down.Write(append(hdr, body...)); err != nil {
			return
		}
	}
}

// waitGoroutines fails the test unless the goroutine count drops back to
// the baseline (CLI and proxy goroutines all exited).
func waitGoroutines(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		n := runtime.NumGoroutine()
		if n <= baseline {
			return
		}
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<20)
			t.Fatalf("goroutine leak: %d > baseline %d\n%s", n, baseline, buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestChaos(t *testing.T) {
	timeouts := []string{"--connect-timeout", "500ms", "--timeout", "300ms"}
	cases := []struct {
		name  string
		fault fault
		after int
		delay time.Duration
		args  []string
		want  int
		msg   string
	}{
		{"clean", faultNone, 0, 0, []string{"list", "lds"}, errs.ExitOK, ""},
		{"slow_within_timeout", faultDelay, 2, 50 * time.Millisecond, []string{"list", "lds"}, errs.ExitOK, ""},
		{"slow_peer_times_out", faultDelay, 2, time.Second, []string{"list", "lds"}, errs.ExitTimeout, ""},
		{"blackhole_request", faultBlackhole, 2, 0, []string{"get", "object", "--object", "InteropLD/GGIO1.SPCSO1.stVal", "--fc", "ST"}, errs.ExitTimeout, ""},
		{"blackhole_association", faultBlackhole, 0, 0, []string{"list", "lds"}, errs.ExitTimeout, "dial"},
		{"reset_association", faultReset, 0, 0, []string{"list", "lds"}, errs.ExitConnection, "dial"},
		{"reset_request", faultReset, 2, 0, []string{"list", "lns", "--ld", "InteropLD"}, errs.ExitConnection, ""},
		{"reset_with_retry", faultReset, 0, 0, []string{"--retry", "2", "--retry-max-delay", "10ms", "list", "lds"}, errs.ExitConnection, "gave up after 3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			baseline := runtime.NumGoroutine()
			p := newChaosProxy(t, c.fault, c.after, c.delay)
			args := append([]string{"--host", "127.0.0.1", "--port", p.port()}, timeouts...)
			start := time.Now()
			r := runCLI(t, append(args, c.args...)...)
			expectCode(t, r, c.want)
			if c.want != errs.ExitOK && strings.TrimSpace(r.stderr) == "" {
				t.Fatalf("no error message on stderr")
			}
			if c.msg != "" && !strings.Contains(r.stderr, c.msg) {
				t.Fatalf("stderr %q lacks %q", r.stderr, c.msg)
			}
			if el := time.Since(start); el > 5*time.Second {
				t.Fatalf("took %v; timeouts not honoured", el)
			}
			p.close()
			waitGoroutines(t, baseline)
		})
	}
}

// TestChaosSubscribeConnectionLoss drops the connection under a running
// report subscription: the command must fail with a connection error rather
// than hang or report success. It uses its own server: go-iec61850 v1.0.9
// keeps a URCB enabled for an owner that vanished without disabling it,
// which would starve later subscribers on the shared server.
func TestChaosSubscribeConnectionLoss(t *testing.T) {
	baseline := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv, err := server.Start(ctx, server.RunConfig{SclPath: fixture("interop.icd"), Host: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	p := newChaosProxyTo(t, srv.Addr(), faultNone, 0, 0)
	trip := time.AfterFunc(700*time.Millisecond, p.trip)
	defer trip.Stop()
	start := time.Now()
	r := runCLI(t, "--host", "127.0.0.1", "--port", p.port(), "--timeout", "500ms",
		"subscribe", "report", "--ld", "InteropLD", "--ln", "LLN0", "--report", "urcb01", "--type", "RP", "--duration", "20s", "--keepalive", "200ms")
	if r.code == errs.ExitOK {
		t.Fatalf("subscription survived connection loss\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
	if r.code != errs.ExitConnection {
		t.Fatalf("exit=%d want %d\nstderr:\n%s", r.code, errs.ExitConnection, r.stderr)
	}
	if el := time.Since(start); el > 10*time.Second {
		t.Fatalf("took %v to notice connection loss", el)
	}
	p.close()
	_ = srv.Close()
	cancel()
	waitGoroutines(t, baseline)
}
