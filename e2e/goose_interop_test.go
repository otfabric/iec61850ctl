//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// GOOSE interop journeys (goose-<stack>): raw Ethernet between iec61850ctl
// and the iec61850-interop GOOSE adapters on a Docker bridge network
// (iec61850-interop docs/goose.md). GOOSE has no IP, so iec61850ctl runs as a
// static Linux binary inside a container on the same bridge, with NET_RAW;
// this works on Linux CI and on Docker Desktop alike.
//
// | Journey                                               | libiec61850 | dscsystems | iec61850-rust |
// |-------------------------------------------------------|-------------|------------|---------------|
// | peer publisher → goose subscribe: both states, values | required    | required   | required      |
// | peer publisher → goose subscribe: supervision timeout | required    | required   | required      |
// | peer publisher → goose sniff: frames decoded          | required    | required   | required      |
// | goose publish → peer subscriber: fields, test=true    | required    | required   | required      |
// | goose publish → peer subscriber: values, sqNum order  | required    | required   | required      |
// | goose publish → peer subscriber: supervision timeout  | required    | required   | required      |

const (
	gooseCB          = "gcbAll"
	gooseMinFrames   = 5 // per state over 1.5 s with MinTime 10 ms (8 expected)
	gooseStateHold   = 1500 * time.Millisecond
	gooseCtlMount    = "/opt/iec61850ctl"
	gooseFixtureDir  = "/fixtures"
	gooseReadyWait   = 30 * time.Second
	gooseCommandWait = 60 * time.Second
)

type gooseFixture struct {
	Revision      string                    `json:"revision"`
	ControlBlocks map[string]gooseFixtureCB `json:"controlBlocks"`
}

type gooseFixtureCB struct {
	GocbRef  string       `json:"gocbRef"`
	GoID     string       `json:"goID"`
	DatSet   string       `json:"datSet"`
	AppID    string       `json:"appID"`
	Dst      string       `json:"dst"`
	VLANID   int          `json:"vlanId"`
	VLANPrio int          `json:"vlanPrio"`
	ConfRev  int          `json:"confRev"`
	MinTime  int          `json:"minTime"`
	MaxTime  int          `json:"maxTime"`
	Values   []typedValue `json:"values"`
	Update   []typedValue `json:"update"`
}

// typedValue is the iec61850-interop typed value JSON: {"type":T,"value":V}.
type typedValue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func loadGooseFixture(t *testing.T) gooseFixtureCB {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testDataDirAbs, "goose.json"))
	if err != nil {
		t.Fatalf("read goose.json: %v", err)
	}
	var f gooseFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("decode goose.json: %v", err)
	}
	if f.Revision != lockFile.GooseFixtureRevision {
		t.Fatalf("goose.json revision %q, lock says %q", f.Revision, lockFile.GooseFixtureRevision)
	}
	cb, ok := f.ControlBlocks[gooseCB]
	if !ok {
		t.Fatalf("goose.json has no control block %q", gooseCB)
	}
	return cb
}

func gooseStacks() []string {
	var out []string
	for _, a := range adapterStacks {
		if a.goose {
			out = append(out, a.name)
		}
	}
	return out
}

var (
	linuxBinOnce sync.Once
	linuxBinDir  string
	linuxBinErr  error
)

// linuxCtlDir returns a directory holding a static Linux iec61850ctl for
// the Docker daemon's architecture: IEC61850CTL_LINUX_BIN when set, else
// built from this module once per run.
func linuxCtlDir(t *testing.T) string {
	t.Helper()
	linuxBinOnce.Do(func() {
		if bin := strings.TrimSpace(os.Getenv("IEC61850CTL_LINUX_BIN")); bin != "" {
			abs, err := filepath.Abs(bin)
			if err != nil {
				linuxBinErr = err
				return
			}
			if filepath.Base(abs) != "iec61850ctl" {
				linuxBinErr = fmt.Errorf("IEC61850CTL_LINUX_BIN must be named iec61850ctl, got %s", abs)
				return
			}
			linuxBinDir = filepath.Dir(abs)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Arch}}").Output()
		if err != nil {
			linuxBinErr = fmt.Errorf("docker server arch: %w", err)
			return
		}
		arch := strings.TrimSpace(string(out))
		dir, err := os.MkdirTemp("", "iec61850ctl-linux-")
		if err != nil {
			linuxBinErr = err
			return
		}
		build := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(dir, "iec61850ctl"), ".")
		build.Dir = filepath.Dir(filepath.Dir(testDataDirAbs))
		build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0", "GOWORK=off")
		if b, err := build.CombinedOutput(); err != nil {
			linuxBinErr = fmt.Errorf("go build linux/%s: %w\n%s", arch, err, b)
			return
		}
		linuxBinDir = dir
	})
	if linuxBinErr != nil {
		t.Fatalf("linux iec61850ctl: %v", linuxBinErr)
	}
	return linuxBinDir
}

// gooseNetwork creates a Docker bridge network for one journey.
func gooseNetwork(t *testing.T) string {
	t.Helper()
	suffix := strconv.FormatInt(time.Now().UnixNano()%1_000_000_000, 36)
	name := fmt.Sprintf("iec61850ctl-goose-%d-%s", os.Getpid(), suffix)
	bridge := "ictlg" + suffix
	if len(bridge) > 15 {
		bridge = bridge[:15]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", "network", "create",
		"-o", "com.docker.network.bridge.name="+bridge,
		"--label", "org.otfabric.e2e=iec61850ctl", name).CombinedOutput(); err != nil {
		t.Fatalf("docker network create: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "docker", "network", "rm", name).Run()
	})
	return name
}

// gooseContainer is a detached container on the GOOSE network.
type gooseContainer struct {
	t    *testing.T
	name string
}

func startGooseContainer(t *testing.T, network, role string, args ...string) *gooseContainer {
	t.Helper()
	name := uniqueContainerName("goose-" + role)
	full := append([]string{"run", "-d", "--name", name, "--network", network,
		"--cap-add", "NET_RAW", "--label", "org.otfabric.e2e=iec61850ctl"}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", full...).CombinedOutput(); err != nil {
		t.Fatalf("docker run %s: %v\n%s", role, err, out)
	}
	c := &gooseContainer{t: t, name: name}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
	})
	return c
}

// ctlArgs runs iec61850ctl from the mounted Linux binary inside image. The
// binary has no file capabilities, so it runs as root with NET_RAW.
func ctlArgs(t *testing.T, image string, cmd ...string) []string {
	return append([]string{
		"--user", "0",
		"-v", linuxCtlDir(t) + ":" + gooseCtlMount + ":ro",
		"-v", testDataDirAbs + ":" + gooseFixtureDir + ":ro",
		"--entrypoint", gooseCtlMount + "/iec61850ctl",
		image,
	}, cmd...)
}

func (c *gooseContainer) logs() (stdout, stderr []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var o, e bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", "logs", c.name)
	cmd.Stdout, cmd.Stderr = &o, &e
	_ = cmd.Run()
	return o.Bytes(), e.Bytes()
}

// wait blocks until the container exits and returns its exit code.
func (c *gooseContainer) wait(timeout time.Duration) int {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "wait", c.name).Output()
	if err != nil {
		c.fail("docker wait: %v", err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		c.fail("docker wait output %q: %v", out, err)
	}
	return code
}

// waitRunning blocks until the container runs, then lets the bridge port
// come up (docs/goose.md: the first sends after a port joins can stall).
func (c *gooseContainer) waitRunning() {
	c.t.Helper()
	deadline := time.Now().Add(gooseReadyWait)
	for time.Now().Before(deadline) {
		out, _ := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", c.name).Output()
		if strings.TrimSpace(string(out)) == "true" {
			time.Sleep(time.Second)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	c.fail("container not running within %s", gooseReadyWait)
}

// waitReady blocks until an adapter container logged its ready event.
func (c *gooseContainer) waitReady() map[string]any {
	c.t.Helper()
	deadline := time.Now().Add(gooseReadyWait)
	for time.Now().Before(deadline) {
		stdout, _ := c.logs()
		for _, ev := range jsonLines(stdout) {
			if ev["event"] == "ready" {
				time.Sleep(time.Second) // bridge port warm-up, as above
				return ev
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	c.fail("no ready event within %s", gooseReadyWait)
	return nil
}

func (c *gooseContainer) fail(format string, args ...any) {
	c.t.Helper()
	stdout, stderr := c.logs()
	c.t.Logf("container %s stdout:\n%s", c.name, stdout)
	c.t.Logf("container %s stderr:\n%s", c.name, stderr)
	c.t.Fatalf(format, args...)
}

func jsonLines(data []byte) []map[string]any {
	var out []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.UseNumber()
		var ev map[string]any
		if dec.Decode(&ev) == nil {
			out = append(out, ev)
		}
	}
	return out
}

func eventsNamed(evs []map[string]any, names ...string) []map[string]any {
	var out []map[string]any
	for _, ev := range evs {
		for _, n := range names {
			if ev["event"] == n {
				out = append(out, ev)
				break
			}
		}
	}
	return out
}

func jsonInt(v any) int64 {
	n, _ := asInt64(v)
	return n
}

func TestGOOSE_PeerPublisherToCtl(t *testing.T) {
	for _, stack := range gooseStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			requireStack(t, prefixGoose+stack)
			requireCtlBin(t)
			cb := loadGooseFixture(t)
			image := resolveImage(stack)
			pubCmd := stack + "-goose-publisher"
			validateAdapterCapabilities(t, image, pubCmd, pubCmd)
			network := gooseNetwork(t)

			// Both receivers outlive the publisher by more than 2*MaxTime,
			// so the subscription sees its supervision timeout.
			listen := (2*gooseStateHold + 2*time.Duration(cb.MaxTime)*time.Millisecond + 4*time.Second).String()
			sub := startGooseContainer(t, network, "ctl-sub", ctlArgs(t, image,
				"goose", "subscribe", "--iface", "eth0",
				"--scl", gooseFixtureDir+"/interop-goose.cid", "--gocb", gooseCB,
				"--duration", listen, "--format", "jsonl")...)
			sniff := startGooseContainer(t, network, "ctl-sniff", ctlArgs(t, image,
				"goose", "sniff", "--iface", "eth0", "--appid", cb.AppID,
				"--duration", listen, "--format", "jsonl")...)
			sub.waitRunning()
			sniff.waitRunning()

			pub := runPeerPublisher(t, network, image, pubCmd, cb)

			if code := sub.wait(gooseCommandWait); code != 0 {
				sub.fail("goose subscribe exit %d", code)
			}
			if code := sniff.wait(gooseCommandWait); code != 0 {
				sniff.fail("goose sniff exit %d", code)
			}
			subOut, _ := sub.logs()
			sniffOut, _ := sniff.logs()
			if err := checkCtlSubscription(jsonLines(subOut), cb); err != nil {
				t.Logf("publisher stdout:\n%s", pub)
				sub.fail("goose subscribe: %v", err)
			}
			frames := eventsNamed(jsonLines(sniffOut), "frame")
			if len(frames) < 2*gooseMinFrames {
				sniff.fail("goose sniff: %d frames, want >= %d", len(frames), 2*gooseMinFrames)
			}
			subEvs := jsonLines(subOut)
			t.Logf("goose subscribe: %d messages (%d state), %d timeout; goose sniff: %d frames",
				len(eventsNamed(subEvs, "state", "message")), len(eventsNamed(subEvs, "state")),
				len(eventsNamed(subEvs, "timeout")), len(frames))
			for _, f := range frames {
				if f["gocb_ref"] != cb.GocbRef || !strings.EqualFold(fmt.Sprint(f["app_id"]), cb.AppID) {
					sniff.fail("goose sniff: unexpected frame %v", f)
				}
			}
		})
	}
}

// runPeerPublisher runs an adapter publisher: the fixture values (stNum 1)
// for gooseStateHold, the update (stNum 2) for gooseStateHold, then stop.
func runPeerPublisher(t *testing.T, network, image, pubCmd string, cb gooseFixtureCB) []byte {
	t.Helper()
	values, _ := json.Marshal(cb.Values)
	ctx, cancel := context.WithTimeout(context.Background(), gooseCommandWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "-i", "--rm",
		"--name", uniqueContainerName("goose-peer-pub"), "--network", network, "--cap-add", "NET_RAW",
		"--label", "org.otfabric.e2e=iec61850ctl",
		"--entrypoint", pubCmd, image,
		"--iface", "eth0", "--gocb-ref", cb.GocbRef, "--go-id", cb.GoID, "--dataset", cb.DatSet,
		"--app-id", cb.AppID, "--dst", cb.Dst,
		"--vlan-id", strconv.Itoa(cb.VLANID), "--vlan-prio", strconv.Itoa(cb.VLANPrio),
		"--conf-rev", strconv.Itoa(cb.ConfRev),
		"--min-time", strconv.Itoa(cb.MinTime), "--max-time", strconv.Itoa(cb.MaxTime),
		"--values", string(values))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("publisher stdin: %v", err)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start publisher: %v", err)
	}
	set, _ := json.Marshal(map[string]any{"cmd": "set", "values": cb.Update})
	go func() {
		defer stdin.Close()
		time.Sleep(gooseStateHold)
		_, _ = io.WriteString(stdin, string(set)+"\n")
		time.Sleep(gooseStateHold)
		_, _ = io.WriteString(stdin, `{"cmd":"stop"}`+"\n")
	}()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("publisher %s: %v\nstdout:\n%s\nstderr:\n%s", pubCmd, err, stdout.Bytes(), stderr.Bytes())
	}
	if errs := eventsNamed(jsonLines(stdout.Bytes()), "error"); len(errs) > 0 {
		t.Fatalf("publisher %s reported errors: %v", pubCmd, errs)
	}
	return stdout.Bytes()
}

// checkCtlSubscription checks iec61850ctl goose subscribe JSONL against the
// fixture: frames of both states with the fixture values and control block
// fields, no invalid frames, and a supervision timeout after the stop.
func checkCtlSubscription(evs []map[string]any, cb gooseFixtureCB) error {
	if bad := eventsNamed(evs, "invalid"); len(bad) > 0 {
		return fmt.Errorf("invalid events: %v", bad)
	}
	frames := eventsNamed(evs, "state", "message")
	for state, want := range map[int64][]typedValue{1: cb.Values, 2: cb.Update} {
		n := 0
		var lastSq int64 = -1
		for _, f := range frames {
			if jsonInt(f["st_num"]) != state {
				continue
			}
			n++
			if sq := jsonInt(f["sq_num"]); sq <= lastSq {
				return fmt.Errorf("stNum %d: sqNum %d after %d", state, sq, lastSq)
			}
			lastSq = jsonInt(f["sq_num"])
			for k, v := range map[string]any{
				"gocb_ref": cb.GocbRef, "dat_set": cb.DatSet, "go_id": cb.GoID,
				"dst": cb.Dst, "test": false, "simulation": false, "nds_com": false,
			} {
				if fmt.Sprint(f[k]) != fmt.Sprint(v) {
					return fmt.Errorf("stNum %d frame %s=%v, want %v", state, k, f[k], v)
				}
			}
			if !strings.EqualFold(fmt.Sprint(f["app_id"]), cb.AppID) || jsonInt(f["conf_rev"]) != int64(cb.ConfRev) {
				return fmt.Errorf("stNum %d frame app_id=%v conf_rev=%v", state, f["app_id"], f["conf_rev"])
			}
			got, _ := f["values"].([]any)
			if err := typedListMatches(want, got); err != nil {
				return fmt.Errorf("stNum %d values: %w", state, err)
			}
		}
		// The first frames of state 1 can fall before the bridge port is
		// up; state 2 starts with the receiver long established.
		if n < gooseMinFrames-2 || (state == 2 && n < gooseMinFrames) {
			return fmt.Errorf("%d frames with stNum %d", n, state)
		}
	}
	if len(eventsNamed(evs, "state")) == 0 {
		return fmt.Errorf("no state event")
	}
	if len(eventsNamed(evs, "timeout")) == 0 {
		return fmt.Errorf("no supervision timeout after the publisher stopped")
	}
	return nil
}

func TestGOOSE_CtlPublisherToPeer(t *testing.T) {
	for _, stack := range gooseStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			requireStack(t, prefixGoose+stack)
			requireCtlBin(t)
			cb := loadGooseFixture(t)
			image := resolveImage(stack)
			subCmd := stack + "-goose-subscriber"
			validateAdapterCapabilities(t, image, subCmd, subCmd)
			network := gooseNetwork(t)

			peer := startGooseContainer(t, network, "peer-sub",
				"--entrypoint", subCmd, image,
				"--iface", "eth0", "--app-id", cb.AppID, "--gocb-ref", cb.GocbRef, "--dst", cb.Dst)
			ready := peer.waitReady()
			if ready["fixture"] != lockFile.GooseFixtureRevision {
				peer.fail("ready fixture %v, want %s", ready["fixture"], lockFile.GooseFixtureRevision)
			}

			// The update vector, minus the structure (not settable from
			// text), as iec61850ctl --value arguments.
			args := []string{"goose", "publish", "--iface", "eth0",
				"--scl", gooseFixtureDir + "/interop-goose.cid", "--gocb", gooseCB, "--yes",
				"--changes", "1", "--interval", gooseStateHold.String(), "--duration", gooseStateHold.String(),
				"--format", "jsonl"}
			settable := len(cb.Update) - 1
			for i, v := range cb.Update[:settable] {
				arg, err := ctlValueArg(v)
				if err != nil {
					t.Fatalf("update[%d]: %v", i, err)
				}
				args = append(args, "--value", fmt.Sprintf("%d=%s", i, arg))
			}
			pub := startGooseContainer(t, network, "ctl-pub", ctlArgs(t, image, args...)...)
			if code := pub.wait(gooseCommandWait); code != 0 {
				pub.fail("goose publish exit %d", code)
			}
			pubOut, _ := pub.logs()
			sums := eventsNamed(jsonLines(pubOut), "summary")
			if len(sums) != 1 || jsonInt(sums[0]["st_num"]) != 2 || sums[0]["test"] != true {
				pub.fail("goose publish summary: %v", sums)
			}

			// TAL of the last frame is at most 2*MaxTime.
			time.Sleep(2*time.Duration(cb.MaxTime)*time.Millisecond + time.Second)
			peerOut, _ := peer.logs()
			peerEvs := jsonLines(peerOut)
			t.Logf("goose publish sent %v frames; %s: %d goose, %d supervision events",
				sums[0]["sent"], subCmd, len(eventsNamed(peerEvs, "goose")), len(eventsNamed(peerEvs, "supervision")))
			if err := checkPeerSubscription(jsonLines(peerOut), cb, settable); err != nil {
				peer.fail("%s: %v", subCmd, err)
			}
		})
	}
}

// ctlValueArg renders a typed fixture value in the iec61850ctl --value form.
func ctlValueArg(v typedValue) (string, error) {
	switch v.Type {
	case "boolean", "integer", "unsigned", "float32", "float64":
		return string(v.Value), nil
	case "bit-string":
		var bits string
		if err := json.Unmarshal(v.Value, &bits); err != nil {
			return "", err
		}
		return hex.EncodeToString(packBits(bits)) + "/" + strconv.Itoa(len(bits)), nil
	case "utc-time":
		var s string
		err := json.Unmarshal(v.Value, &s)
		return s, err
	default:
		return "", fmt.Errorf("type %s is not settable from text", v.Type)
	}
}

// checkPeerSubscription checks an adapter subscriber's events for the frames
// iec61850ctl published: both states, the control block fields with
// test=true, the first settable values, increasing sqNum and a supervision
// timeout after the publisher stopped.
func checkPeerSubscription(evs []map[string]any, cb gooseFixtureCB, settable int) error {
	frames := eventsNamed(evs, "goose")
	for _, state := range []int64{1, 2} {
		n := 0
		var lastSq int64 = -1
		for _, f := range frames {
			if jsonInt(f["stNum"]) != state {
				continue
			}
			n++
			sq := jsonInt(f["sqNum"])
			if (n == 1 && sq != 0) || sq <= lastSq {
				return fmt.Errorf("stNum %d: sqNum %d after %d", state, sq, lastSq)
			}
			lastSq = sq
			for k, v := range map[string]any{
				"gocbRef": cb.GocbRef, "datSet": cb.DatSet, "goID": cb.GoID,
				"appID": cb.AppID, "dst": cb.Dst, "confRev": cb.ConfRev,
				"test": true, "simulation": false, "ndsCom": false,
				"numDatSetEntries": len(cb.Update),
			} {
				if fmt.Sprint(f[k]) != fmt.Sprint(v) {
					return fmt.Errorf("stNum %d frame %s=%v, want %v", state, k, f[k], v)
				}
			}
			got, _ := f["values"].([]any)
			if len(got) != len(cb.Update) {
				return fmt.Errorf("stNum %d: %d values, want %d", state, len(got), len(cb.Update))
			}
			for i := 0; i < settable; i++ {
				if err := peerValueMatches(cb.Update[i], got[i]); err != nil {
					return fmt.Errorf("stNum %d value[%d]: %w", state, i, err)
				}
			}
			last, _ := got[settable].(map[string]any)
			if last["type"] != "structure" {
				return fmt.Errorf("stNum %d value[%d] type %v, want structure", state, settable, last["type"])
			}
		}
		if n < gooseMinFrames {
			return fmt.Errorf("%d frames with stNum %d, want >= %d", n, state, gooseMinFrames)
		}
	}
	timeout := false
	for _, ev := range eventsNamed(evs, "supervision") {
		if ev["gocbRef"] == cb.GocbRef && ev["state"] == "timeout" {
			timeout = true
		}
	}
	if !timeout {
		return fmt.Errorf("no supervision timeout after the publisher stopped")
	}
	return nil
}

// peerValueMatches compares an adapter's typed value with the fixture.
func peerValueMatches(want typedValue, got any) error {
	m, ok := got.(map[string]any)
	if !ok || m["type"] != want.Type {
		return fmt.Errorf("got %v, want type %s", got, want.Type)
	}
	b, _ := json.Marshal(m["value"])
	return typedValueMatches(want, func(dst any) error { return json.Unmarshal(b, dst) }, b)
}

// typedListMatches compares iec61850ctl JSON values with typed fixture values.
func typedListMatches(want []typedValue, got []any) error {
	if len(got) != len(want) {
		return fmt.Errorf("%d values, want %d", len(got), len(want))
	}
	for i, w := range want {
		if err := ctlValueMatches(w, got[i]); err != nil {
			return fmt.Errorf("value[%d]: %w", i, err)
		}
	}
	return nil
}

// ctlValueMatches compares one iec61850ctl JSON value (pkg/gooseops
// ValueJSON) with a typed fixture value.
func ctlValueMatches(want typedValue, got any) error {
	switch want.Type {
	case "bit-string":
		var bits string
		_ = json.Unmarshal(want.Value, &bits)
		m, _ := got.(map[string]any)
		if m == nil || m["bits"] != hex.EncodeToString(packBits(bits)) || jsonInt(m["len"]) != int64(len(bits)) {
			return fmt.Errorf("bit-string %v, want %s", got, bits)
		}
		return nil
	case "structure", "array":
		var inner []typedValue
		if err := json.Unmarshal(want.Value, &inner); err != nil {
			return err
		}
		list, _ := got.([]any)
		return typedListMatches(inner, list)
	}
	b, _ := json.Marshal(got)
	return typedValueMatches(want, func(dst any) error { return json.Unmarshal(b, dst) }, b)
}

// typedValueMatches compares a scalar typed fixture value with a decoded
// JSON value; times match to the millisecond, floats approximately.
func typedValueMatches(want typedValue, decode func(any) error, raw []byte) error {
	switch want.Type {
	case "float32", "float64":
		var w, g float64
		if json.Unmarshal(want.Value, &w) != nil || decode(&g) != nil || !floatClose(w, g) {
			return fmt.Errorf("%s %s, want %s", want.Type, raw, want.Value)
		}
	case "utc-time", "binary-time":
		var ws, gs string
		if json.Unmarshal(want.Value, &ws) != nil || decode(&gs) != nil {
			return fmt.Errorf("%s %s, want %s", want.Type, raw, want.Value)
		}
		w, err1 := time.Parse(time.RFC3339Nano, ws)
		g, err2 := time.Parse(time.RFC3339Nano, gs)
		if err1 != nil || err2 != nil || w.Sub(g).Abs() > time.Millisecond {
			return fmt.Errorf("%s %s, want %s", want.Type, raw, want.Value)
		}
	case "structure", "array":
		var inner []typedValue
		var got []any
		if json.Unmarshal(want.Value, &inner) != nil || decode(&got) != nil || len(inner) != len(got) {
			return fmt.Errorf("%s %s, want %s", want.Type, raw, want.Value)
		}
		for i := range inner {
			if err := peerValueMatches(inner[i], got[i]); err != nil {
				return err
			}
		}
	default:
		if !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(want.Value)) {
			return fmt.Errorf("%s %s, want %s", want.Type, raw, want.Value)
		}
	}
	return nil
}

// packBits packs MSB-first binary digits into octets.
func packBits(bits string) []byte {
	out := make([]byte, (len(bits)+7)/8)
	for i, c := range bits {
		if c == '1' {
			out[i/8] |= 0x80 >> (i % 8)
		}
	}
	return out
}
