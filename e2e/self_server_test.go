//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestSelfSmoke(t *testing.T) {
	requireStack(t, stackSelf)
	requireCtlBin(t)

	port, err := freeHostPort()
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}

	scl := filepath.Join(testDataDirAbs, "interop.icd")
	values := filepath.Join(testDataDirAbs, "self-server-values.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, ctlBin,
		"server", "start",
		"--scl", scl,
		"--values", values,
		"--ied-name", "InteropIED",
		"--fixture-id", lockFile.FixtureRevision,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--ready-json",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start self server: %v", err)
	}

	stopped := make(chan error, 1)
	go func() { stopped <- cmd.Wait() }()

	readyCh := make(chan readyEvent, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			var ev readyEvent
			if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.Event == "ready" {
				readyCh <- ev
				return
			}
		}
	}()

	var ready readyEvent
	select {
	case ready = <-readyCh:
	case err := <-stopped:
		t.Fatalf("self server exited before ready: %v", err)
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Signal(syscall.SIGTERM)
		t.Fatal("timed out waiting for self-server ready event")
	}

	if ready.Adapter != "iec61850ctl" {
		t.Fatalf("ready adapter: got %q, want iec61850ctl", ready.Adapter)
	}
	if ready.Fixture != lockFile.FixtureRevision {
		t.Fatalf("ready fixture: got %q, want %q", ready.Fixture, lockFile.FixtureRevision)
	}
	if ready.IEDName == "" {
		t.Fatal("ready ied_name is empty")
	}

	// The iec61850ctl / go-iec61850 server registers MMS domains as LD names
	// without an IED prefix, so the client must not set IEC61850_IED_NAME.
	env := map[string]string{
		"IEC61850_HOST":     "127.0.0.1",
		"IEC61850_PORT":     strconv.Itoa(port),
		"IEC61850_IED_NAME": "",
	}
	waitForCtlReady(t, env, 5*time.Second)

	t.Cleanup(func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-stopped:
			return
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-stopped
		}
	})

	t.Run("SelfSmoke_ListLDs", func(t *testing.T) {
		res := runCtl(t, env, commandTimeout, "list", "lds", "--format", "json")
		requireExitZero(t, res, nil)
		var devices []struct {
			Name string `json:"name"`
		}
		if err := decodeExactJSON(res.Stdout, &devices); err != nil {
			logCommandFailure(t, res, nil)
			t.Fatalf("decode: %v", err)
		}
		names := make([]string, len(devices))
		for i, d := range devices {
			names[i] = d.Name
		}
		if !containsName(names, "InteropLD") {
			logCommandFailure(t, res, nil)
			t.Fatalf("InteropLD missing: %v", names)
		}
	})

	t.Run("SelfSmoke_ListLNs", func(t *testing.T) {
		res := runCtl(t, env, commandTimeout,
			"list", "lns", "--ld", "InteropLD", "--format", "json")
		requireExitZero(t, res, nil)
		var nodes []struct {
			Name string `json:"name"`
		}
		if err := decodeExactJSON(res.Stdout, &nodes); err != nil {
			logCommandFailure(t, res, nil)
			t.Fatalf("decode: %v", err)
		}
		names := make([]string, len(nodes))
		for i, n := range nodes {
			names[i] = n.Name
		}
		for _, want := range []string{"LLN0", "GGIO1", "MMXU1"} {
			if !containsName(names, want) {
				logCommandFailure(t, res, nil)
				t.Fatalf("missing LN %q in %v", want, names)
			}
		}
	})

	t.Run("SelfSmoke_ReadST", func(t *testing.T) {
		const key = "InteropLD/LLN0.Mod.stVal"
		res := runCtl(t, env, commandTimeout,
			"get", "object", "--object", key, "--fc", "ST", "--format", "json")
		requireExitZero(t, res, nil)
		var out struct {
			Value any `json:"value"`
		}
		if err := decodeExactJSON(res.Stdout, &out); err != nil {
			logCommandFailure(t, res, nil)
			t.Fatalf("decode: %v", err)
		}
		want := expectedValue(t, key)
		if !valuesEqual(out.Value, want) {
			logCommandFailure(t, res, nil)
			t.Fatalf("value: got %#v, want %#v", out.Value, want)
		}
	})

	t.Run("SelfSmoke_ReadMX", func(t *testing.T) {
		const key = "InteropLD/MMXU1.TotW.mag.f"
		res := runCtl(t, env, commandTimeout,
			"get", "object", "--object", key, "--fc", "MX", "--format", "json")
		requireExitZero(t, res, nil)
		var out struct {
			Value any `json:"value"`
		}
		if err := decodeExactJSON(res.Stdout, &out); err != nil {
			logCommandFailure(t, res, nil)
			t.Fatalf("decode: %v", err)
		}
		want := expectedValue(t, key)
		if !valuesEqual(out.Value, want) {
			logCommandFailure(t, res, nil)
			t.Fatalf("value: got %#v, want %#v", out.Value, want)
		}
	})
}
