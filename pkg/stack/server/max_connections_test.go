// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
)

// TestMaxConnections: the server serves MaxConnections connections at once
// (5 when it is zero or less); one more is closed before its association and
// counted as refused, and a connection is served again once one has closed.
func TestMaxConnections(t *testing.T) {
	for _, c := range []struct {
		name string
		max  int
		want int
	}{
		{"1", 1, 1},
		{"2", 2, 2},
		{"default", 0, 5},
		{"negative", -1, 5},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			r, err := Start(ctx, RunConfig{SclPath: interopICD(t), Host: "127.0.0.1", MaxConnections: c.max})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.Close() }()

			dial := func() (*iec61850.Client, error) {
				dctx, dcancel := context.WithTimeout(ctx, 5*time.Second)
				defer dcancel()
				return iec61850.Dial(dctx, r.Addr(), iec61850.DialOptions{})
			}
			clients := make([]*iec61850.Client, 0, c.want)
			defer func() {
				for _, cl := range clients {
					_ = cl.Close(ctx)
				}
			}()
			for i := range c.want {
				cl, err := dial()
				if err != nil {
					t.Fatalf("client %d of %d: %v", i+1, c.want, err)
				}
				clients = append(clients, cl)
			}
			if cl, err := dial(); err == nil {
				_ = cl.Close(ctx)
				t.Fatalf("client %d associated, more than %d", c.want+1, c.want)
			}
			// The server counts the refusal after it has closed the
			// connection.
			waitFor(ctx, t, "ConnectionsRefused = 1", func() bool { return r.srv.Stats().ConnectionsRefused == 1 })
			// The served clients still work.
			for i, cl := range clients {
				if _, err := cl.ListLogicalDevices(ctx); err != nil {
					t.Fatalf("client %d: %v", i+1, err)
				}
			}

			// Once a client has closed, a new one is served.
			_ = clients[0].Close(ctx)
			clients = clients[1:]
			waitFor(ctx, t, "the closed connection no longer served", func() bool { return r.srv.Stats().Connections < c.want })
			cl, err := dial()
			if err != nil {
				t.Fatalf("client after a close: %v", err)
			}
			clients = append(clients, cl)
		})
	}
}

// waitFor polls cond with a growing pause until it holds or ctx is done.
func waitFor(ctx context.Context, t *testing.T, what string, cond func() bool) {
	t.Helper()
	for wait := time.Millisecond; !cond(); wait = min(2*wait, 100*time.Millisecond) {
		select {
		case <-ctx.Done():
			t.Fatalf("not reached: %s", what)
		case <-time.After(wait):
		}
	}
}
