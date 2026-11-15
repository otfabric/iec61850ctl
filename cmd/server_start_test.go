// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/stack/server"
)

// TestServerStartMaxConnections: server start --max-connections N serves N
// clients at once and closes the connection of client N+1.
func TestServerStartMaxConnections(t *testing.T) {
	const limit = 2
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan result, 1)
	var stdout, stderr lockedBuffer
	go func() {
		cliMu.Lock()
		defer cliMu.Unlock()
		resetFlags(rootCmd)
		code := ExecuteContext(ctx, []string{"server", "start", "--scl", fixture("interop.icd"), "--host", "127.0.0.1", "--port", "0",
			"--ready-json", "--max-connections", strconvItoa(limit)}, &stdout, &stderr)
		done <- result{stdout: stdout.String(), stderr: stderr.String(), code: code}
	}()
	var ev server.ReadyEvent
	for wait := time.Millisecond; ; wait = min(2*wait, 100*time.Millisecond) {
		if line, _, ok := strings.Cut(stdout.String(), "\n"); ok {
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				t.Fatalf("ready line %q: %v", line, err)
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("no ready event; stderr=%s", stderr.String())
		case <-time.After(wait):
		}
	}

	dial := func() (*iec61850.Client, error) {
		dctx, dcancel := context.WithTimeout(ctx, 5*time.Second)
		defer dcancel()
		return iec61850.Dial(dctx, ev.Address, iec61850.DialOptions{})
	}
	for i := range limit {
		c, err := dial()
		if err != nil {
			t.Fatalf("client %d of %d: %v", i+1, limit, err)
		}
		defer func() { _ = c.Close(ctx) }()
	}
	if c, err := dial(); err == nil {
		_ = c.Close(ctx)
		t.Fatalf("client %d associated with --max-connections %d", limit+1, limit)
	}

	cancel()
	select {
	case r := <-done:
		expectCode(t, r, errs.ExitOK)
	case <-time.After(10 * time.Second):
		t.Fatal("server start did not stop on cancel")
	}
}
