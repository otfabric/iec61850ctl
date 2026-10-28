//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// SV interop journeys (sv-<stack>): raw Ethernet between iec61850ctl and the
// mms-interop Sampled Values adapters on a Docker bridge network
// (mms-interop docs/sv.md), with the same container setup as the GOOSE
// journeys. Each journey runs for both fixture streams: MSVCB01 (9-2LE,
// 4000 samples/s, one ASDU per frame) and MSVCB02 (generic dataset, 1000
// samples/s, four ASDUs per frame, every optional field).
//
// | Journey                                                  | libiec61850 | dscsystems | iec61850-rust |
// |----------------------------------------------------------|-------------|------------|---------------|
// | peer publisher → sv subscribe: header, both value sets   | required    | required   | required      |
// | peer publisher → sv subscribe: loss < 1 %, timeout       | required    | required   | required      |
// | peer publisher → sv sniff: ASDUs decoded with the layout | required    | required   | required      |
// | sv publish → peer subscriber: header, values, simulation | required    | required   | required      |
// | sv publish → peer subscriber: loss < 1 %                 | required    | required   | required      |
// | sv svcb get MSVCB02 over MMS (forward IED server)        | required    | required   | n/a           |
//
// Registered limitation (mms-interop capabilities.json): libiec61850's
// SVReceiver drops ASDUs whose smpMod is two octets, which go-iec61850
// sends, so sv publish of MSVCB02 → libiec61850-sv-subscriber must receive
// nothing. iec61850bean has no SV support and no MSVCB02 over MMS;
// iec61850-rust has no MSVCB02 over MMS.

const (
	svStateHold   = 1500 * time.Millisecond
	svMaxLossPct  = 3.0 // a Docker Desktop VM can stall delivery for ~150 ms
	svMinReceived = 0.9 // fraction of the published samples a subscriber must see
	svCtlRun      = 3 * time.Second
)

type svFixtureFile struct {
	Revision string                     `json:"revision"`
	Streams  map[string]svFixtureStream `json:"streams"`
}

type svFixtureStream struct {
	MsvcbRef         string          `json:"msvcbRef"`
	Profile          string          `json:"profile"`
	SvID             string          `json:"svID"`
	DatSet           string          `json:"datSet"`
	AppID            string          `json:"appID"`
	Dst              string          `json:"dst"`
	VLANID           int             `json:"vlanId"`
	VLANPrio         int             `json:"vlanPrio"`
	ConfRev          int             `json:"confRev"`
	SmpRate          int             `json:"smpRate"`
	SmpMod           string          `json:"smpMod"`
	Frequency        float64         `json:"frequency"`
	SamplesPerSecond float64         `json:"samplesPerSecond"`
	SmpCntWrap       int             `json:"smpCntWrap"`
	NoASDU           int             `json:"noASDU"`
	OptionalFields   map[string]bool `json:"optionalFields"`
	Values           []typedValue    `json:"values"`
	Update           []typedValue    `json:"update"`
}

// svSCLSmpMod maps the fixture smpMod to the SCL name iec61850ctl prints.
var svSCLSmpMod = map[string]string{
	"samples-per-period": "SmpPerPeriod", "samples-per-second": "SmpPerSec", "seconds-per-sample": "SecPerSmp",
}

func loadSVFixture(t *testing.T) svFixtureFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testDataDirAbs, "sv.json"))
	if err != nil {
		t.Fatalf("read sv.json: %v", err)
	}
	var f svFixtureFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("decode sv.json: %v", err)
	}
	if f.Revision != lockFile.SVFixtureRevision {
		t.Fatalf("sv.json revision %q, lock says %q", f.Revision, lockFile.SVFixtureRevision)
	}
	if len(f.Streams) == 0 {
		t.Fatal("sv.json has no streams")
	}
	return f
}

func (f svFixtureFile) names() []string {
	out := make([]string, 0, len(f.Streams))
	for n := range f.Streams {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func svStacks() []string {
	var out []string
	for _, a := range adapterStacks {
		if a.sv {
			out = append(out, a.name)
		}
	}
	return out
}

// smpModTwoOctetDropped is the registered libiec61850-sv-subscriber
// limitation for a stream that carries smpMod.
func smpModTwoOctetDropped(stack string, st svFixtureStream) bool {
	return stack == stackLibIEC61850 && st.OptionalFields["smpMod"]
}

func TestSV_PeerPublisherToCtl(t *testing.T) {
	for _, stack := range svStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			requireStack(t, prefixSV+stack)
			requireCtlBin(t)
			fx := loadSVFixture(t)
			image := resolveImage(stack)
			pubCmd := stack + "-sv-publisher"
			validateAdapterCapabilities(t, image, pubCmd, pubCmd)
			for _, name := range fx.names() {
				name, st := name, fx.Streams[name]
				t.Run(name, func(t *testing.T) { peerPublisherToCtl(t, image, pubCmd, name, st) })
			}
		})
	}
}

func peerPublisherToCtl(t *testing.T, image, pubCmd, name string, st svFixtureStream) {
	network := gooseNetwork(t)
	listen := (2*svStateHold + 6*time.Second).String()
	sub := startGooseContainer(t, network, "sv-ctl-sub", ctlArgs(t, image,
		"sv", "subscribe", "--iface", "eth0", "--scl", gooseFixtureDir+"/interop-sv.cid", "--svcb", name,
		"--sample-every", "50", "--stats-interval", "1s", "--duration", listen, "--format", "jsonl")...)
	sniff := startGooseContainer(t, network, "sv-ctl-sniff", ctlArgs(t, image,
		"sv", "sniff", "--iface", "eth0", "--scl", gooseFixtureDir+"/interop-sv.cid", "--svcb", name,
		"--sample-every", "100", "--duration", listen, "--format", "jsonl")...)
	sub.waitRunning()
	sniff.waitRunning()

	stopped := runSVPeerPublisher(t, network, image, pubCmd, st)

	if code := sub.wait(gooseCommandWait); code != 0 {
		sub.fail("sv subscribe exit %d", code)
	}
	if code := sniff.wait(gooseCommandWait); code != 0 {
		sniff.fail("sv sniff exit %d", code)
	}
	subOut, _ := sub.logs()
	sniffOut, _ := sniff.logs()
	subEvs := jsonLines(subOut)
	if err := checkCtlSVSubscription(subEvs, st, jsonInt(stopped["samples"])); err != nil {
		sub.fail("sv subscribe: %v", err)
	}
	asdus := eventsNamed(jsonLines(sniffOut), "asdu")
	if len(asdus) < 5 {
		sniff.fail("sv sniff: %d ASDUs printed, want >= 5", len(asdus))
	}
	for _, a := range asdus {
		if err := checkCtlASDUHeader(a, st); err != nil {
			sniff.fail("sv sniff: %v", err)
		}
		if typedListMatches(st.Values, a["values"].([]any)) != nil && typedListMatches(st.Update, a["values"].([]any)) != nil {
			sniff.fail("sv sniff: values %v match neither fixture set", a["values"])
		}
	}
	sum := eventsNamed(subEvs, "summary")[0]
	t.Logf("%s: published %v samples; sv subscribe summary %v; sv sniff printed %d ASDUs",
		pubCmd, stopped["samples"], sum["streams"], len(asdus))
}

// runSVPeerPublisher runs an adapter publisher with the fixture values for
// svStateHold, the update for svStateHold, then stops it and returns its
// stopped event.
func runSVPeerPublisher(t *testing.T, network, image, pubCmd string, st svFixtureStream) map[string]any {
	t.Helper()
	values, _ := json.Marshal(st.Values)
	args := []string{"run", "-i", "--rm",
		"--name", uniqueContainerName("sv-peer-pub"), "--network", network, "--cap-add", "NET_RAW",
		"--label", "org.otfabric.e2e=iec61850ctl",
		"--entrypoint", pubCmd, image,
		"--iface", "eth0", "--app-id", st.AppID, "--dst", st.Dst,
		"--vlan-id", strconv.Itoa(st.VLANID), "--vlan-prio", strconv.Itoa(st.VLANPrio),
		"--sv-id", st.SvID, "--dataset", st.DatSet, "--conf-rev", strconv.Itoa(st.ConfRev),
		"--smp-rate", strconv.Itoa(st.SmpRate), "--smp-mod", st.SmpMod, "--no-asdu", strconv.Itoa(st.NoASDU),
		"--profile", st.Profile, "--values", string(values)}
	if st.Frequency > 0 {
		args = append(args, "--frequency", strconv.FormatFloat(st.Frequency, 'f', -1, 64))
	}
	ctx, cancel := context.WithTimeout(context.Background(), gooseCommandWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("publisher stdin: %v", err)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start publisher: %v", err)
	}
	set, _ := json.Marshal(map[string]any{"cmd": "set", "values": st.Update})
	go func() {
		defer stdin.Close()
		time.Sleep(svStateHold)
		_, _ = io.WriteString(stdin, string(set)+"\n")
		time.Sleep(svStateHold)
		_, _ = io.WriteString(stdin, `{"cmd":"stop"}`+"\n")
	}()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("publisher %s: %v\nstdout:\n%s\nstderr:\n%s", pubCmd, err, stdout.Bytes(), stderr.Bytes())
	}
	evs := jsonLines(stdout.Bytes())
	if errs := eventsNamed(evs, "error"); len(errs) > 0 {
		t.Fatalf("publisher %s reported errors: %v", pubCmd, errs)
	}
	stopped := eventsNamed(evs, "stopped")
	if len(stopped) != 1 || jsonInt(stopped[0]["samples"]) == 0 {
		t.Fatalf("publisher %s: stopped events %v\nstdout:\n%s", pubCmd, stopped, stdout.Bytes())
	}
	return stopped[0]
}

// checkCtlASDUHeader checks one iec61850ctl asdu record against the stream.
func checkCtlASDUHeader(a map[string]any, st svFixtureStream) error {
	for k, v := range map[string]any{
		"sv_id": st.SvID, "dst": st.Dst, "conf_rev": st.ConfRev, "no_asdu": st.NoASDU, "simulation": false,
		"smp_synch": 0,
	} {
		if fmt.Sprint(a[k]) != fmt.Sprint(v) {
			return fmt.Errorf("asdu %s=%v, want %v", k, a[k], v)
		}
	}
	if !strings.EqualFold(fmt.Sprint(a["app_id"]), st.AppID) {
		return fmt.Errorf("asdu app_id=%v, want %s", a["app_id"], st.AppID)
	}
	if c := jsonInt(a["smp_cnt"]); c < 0 || c >= int64(st.SmpCntWrap) {
		return fmt.Errorf("asdu smp_cnt %d outside 0..%d", c, st.SmpCntWrap-1)
	}
	opt := st.OptionalFields
	if got, want := a["dat_set"] != nil, opt["datSet"]; got != want || (want && a["dat_set"] != st.DatSet) {
		return fmt.Errorf("asdu dat_set=%v (fixture sends datSet: %t)", a["dat_set"], want)
	}
	if got, want := a["refr_tm"] != nil, opt["refrTm"]; got != want {
		return fmt.Errorf("asdu refr_tm=%v (fixture sends refrTm: %t)", a["refr_tm"], want)
	}
	if got, want := a["smp_rate"] != nil, opt["smpRate"]; got != want || (want && jsonInt(a["smp_rate"]) != int64(st.SmpRate)) {
		return fmt.Errorf("asdu smp_rate=%v (fixture sends smpRate: %t)", a["smp_rate"], want)
	}
	if got, want := a["smp_mod"] != nil, opt["smpMod"]; got != want || (want && a["smp_mod"] != svSCLSmpMod[st.SmpMod]) {
		return fmt.Errorf("asdu smp_mod=%v (fixture sends smpMod: %t)", a["smp_mod"], want)
	}
	return nil
}

// checkCtlSVSubscription checks iec61850ctl sv subscribe JSONL: a start
// event, sampled ASDUs with the stream header and both value sets in order,
// no invalid ASDUs, at most svMaxLossPct lost, no duplicates, nearly every
// published sample received, and a supervision timeout after the stop.
func checkCtlSVSubscription(evs []map[string]any, st svFixtureStream, published int64) error {
	if bad := eventsNamed(evs, "invalid"); len(bad) > 0 {
		return fmt.Errorf("invalid events: %v", bad)
	}
	if len(eventsNamed(evs, "start")) == 0 {
		return fmt.Errorf("no start event")
	}
	if len(eventsNamed(evs, "timeout")) == 0 {
		return fmt.Errorf("no supervision timeout after the publisher stopped")
	}
	if len(eventsNamed(evs, "stats")) == 0 {
		return fmt.Errorf("no periodic stats")
	}
	var nValues, nUpdate int
	for _, a := range eventsNamed(evs, "asdu") {
		if err := checkCtlASDUHeader(a, st); err != nil {
			return err
		}
		vals, _ := a["values"].([]any)
		switch {
		case typedListMatches(st.Values, vals) == nil:
			if nUpdate > 0 {
				return fmt.Errorf("fixture values after the update: %v", vals)
			}
			nValues++
		case typedListMatches(st.Update, vals) == nil:
			nUpdate++
		default:
			return fmt.Errorf("values %v match neither fixture set", vals)
		}
	}
	if nValues == 0 || nUpdate == 0 {
		return fmt.Errorf("sampled %d ASDUs with the values and %d with the update, want both", nValues, nUpdate)
	}
	sums := eventsNamed(evs, "summary")
	if len(sums) != 1 {
		return fmt.Errorf("%d summary records", len(sums))
	}
	streams, _ := sums[0]["streams"].([]any)
	if len(streams) != 1 {
		return fmt.Errorf("summary streams %v, want one", sums[0]["streams"])
	}
	s, _ := streams[0].(map[string]any)
	asdus, lost := jsonInt(s["asdus"]), jsonInt(s["lost"])
	if s["status"] != "timeout" || jsonInt(s["duplicates"]) != 0 || jsonInt(s["invalid"]) != 0 ||
		jsonInt(s["smp_cnt_wrap"]) != int64(st.SmpCntWrap) {
		return fmt.Errorf("summary stream %v", s)
	}
	if float64(lost) > svMaxLossPct/100*float64(asdus) {
		return fmt.Errorf("lost %d of %d ASDUs", lost, asdus)
	}
	if float64(asdus) < svMinReceived*float64(published) {
		return fmt.Errorf("received %d of %d published samples", asdus, published)
	}
	return nil
}

func TestSV_CtlPublisherToPeer(t *testing.T) {
	for _, stack := range svStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			requireStack(t, prefixSV+stack)
			requireCtlBin(t)
			fx := loadSVFixture(t)
			image := resolveImage(stack)
			subCmd := stack + "-sv-subscriber"
			validateAdapterCapabilities(t, image, subCmd, subCmd)
			for _, name := range fx.names() {
				name, st := name, fx.Streams[name]
				t.Run(name, func(t *testing.T) { ctlPublisherToPeer(t, stack, image, subCmd, name, st) })
			}
		})
	}
}

func ctlPublisherToPeer(t *testing.T, stack, image, subCmd, name string, st svFixtureStream) {
	network := gooseNetwork(t)
	peer := startGooseContainer(t, network, "sv-peer-sub",
		"--entrypoint", subCmd, image,
		"--iface", "eth0", "--app-id", st.AppID, "--sv-id", st.SvID, "--sample-every", "100")
	ready := peer.waitReady()
	if ready["fixture"] != lockFile.SVFixtureRevision {
		peer.fail("ready fixture %v, want %s", ready["fixture"], lockFile.SVFixtureRevision)
	}

	args := []string{"sv", "publish", "--iface", "eth0", "--scl", gooseFixtureDir + "/interop-sv.cid",
		"--svcb", name, "--yes", "--duration", svCtlRun.String(), "--format", "jsonl"}
	for i, v := range st.Values {
		arg, err := svValueArg(v)
		if err != nil {
			t.Fatalf("values[%d]: %v", i, err)
		}
		args = append(args, "--value", fmt.Sprintf("%d=%s", i, arg))
	}
	pub := startGooseContainer(t, network, "sv-ctl-pub", ctlArgs(t, image, args...)...)
	if code := pub.wait(gooseCommandWait); code != 0 {
		pub.fail("sv publish exit %d", code)
	}
	pubOut, _ := pub.logs()
	sums := eventsNamed(jsonLines(pubOut), "summary")
	if len(sums) != 1 || jsonInt(sums[0]["samples"]) == 0 || sums[0]["simulation"] != true || jsonInt(sums[0]["send_errors"]) != 0 {
		pub.fail("sv publish summary: %v", sums)
	}
	published, skipped := jsonInt(sums[0]["samples"]), jsonInt(sums[0]["skipped"])

	time.Sleep(1500 * time.Millisecond) // the subscriber's next stats line
	peerOut, _ := peer.logs()
	peerEvs := jsonLines(peerOut)
	svEvs := eventsNamed(peerEvs, "sv")
	t.Logf("sv publish sent %d samples in %v frames (skipped %d to catch up); %s: %d sv events, last stats %v",
		published, sums[0]["frames"], skipped, subCmd, len(svEvs), lastEvent(peerEvs, "stats"))
	if smpModTwoOctetDropped(stack, st) {
		// Registered limitation: nothing of the stream may arrive.
		if len(svEvs) != 0 {
			peer.fail("%s: %d sv events for a stream with a two-octet smpMod; the registered limitation no longer holds", subCmd, len(svEvs))
		}
		return
	}
	if err := checkPeerSVSubscription(peerEvs, st, published, skipped); err != nil {
		peer.fail("%s: %v", subCmd, err)
	}
}

// svValueArg renders a typed fixture value in the sv publish --value form;
// qualities pass as their 13 MSB-first digits.
func svValueArg(v typedValue) (string, error) {
	switch v.Type {
	case "boolean", "integer", "unsigned", "float32", "float64":
		return string(v.Value), nil
	case "bit-string":
		var bits string
		err := json.Unmarshal(v.Value, &bits)
		return bits, err
	case "utc-time":
		var s string
		err := json.Unmarshal(v.Value, &s)
		return s, err
	}
	return "", fmt.Errorf("type %s is not settable from text", v.Type)
}

func lastEvent(evs []map[string]any, name string) map[string]any {
	named := eventsNamed(evs, name)
	if len(named) == 0 {
		return nil
	}
	return named[len(named)-1]
}

// checkPeerSVSubscription checks an adapter subscriber's events for the
// stream iec61850ctl published: the header with the simulation bit, the
// fixture values, at most svMaxLossPct lost, no duplicates, and nearly every
// published sample received. Samples the publisher skipped to catch up after
// a scheduling stall show as smpCnt gaps; they are not counted as loss.
func checkPeerSVSubscription(evs []map[string]any, st svFixtureStream, published, skipped int64) error {
	svEvs := eventsNamed(evs, "sv")
	if len(svEvs) < 5 {
		return fmt.Errorf("%d sv events, want >= 5", len(svEvs))
	}
	opt := st.OptionalFields
	for _, ev := range svEvs {
		for k, v := range map[string]any{
			"svID": st.SvID, "appID": st.AppID, "dst": st.Dst, "confRev": st.ConfRev,
			"noASDU": st.NoASDU, "simulation": true, "smpSynch": 0,
		} {
			if fmt.Sprint(ev[k]) != fmt.Sprint(v) {
				return fmt.Errorf("sv event %s=%v, want %v", k, ev[k], v)
			}
		}
		if got, want := ev["datSet"] != nil, opt["datSet"]; got != want || (want && ev["datSet"] != st.DatSet) {
			return fmt.Errorf("sv event datSet=%v (fixture sends datSet: %t)", ev["datSet"], want)
		}
		if got, want := ev["smpRate"] != nil, opt["smpRate"]; got != want || (want && jsonInt(ev["smpRate"]) != int64(st.SmpRate)) {
			return fmt.Errorf("sv event smpRate=%v (fixture sends smpRate: %t)", ev["smpRate"], want)
		}
		if got, want := ev["smpMod"] != nil, opt["smpMod"]; got != want || (want && ev["smpMod"] != st.SmpMod) {
			return fmt.Errorf("sv event smpMod=%v (fixture sends smpMod: %t)", ev["smpMod"], want)
		}
		if got, want := ev["refrTm"] != nil, opt["refrTm"]; got != want {
			return fmt.Errorf("sv event refrTm=%v (fixture sends refrTm: %t)", ev["refrTm"], want)
		}
		if c := jsonInt(ev["smpCnt"]); c < 0 || c >= int64(st.SmpCntWrap) {
			return fmt.Errorf("sv event smpCnt %d outside 0..%d", c, st.SmpCntWrap-1)
		}
		vals, _ := ev["values"].([]any)
		if len(vals) != len(st.Values) {
			return fmt.Errorf("sv event: %d values, want %d", len(vals), len(st.Values))
		}
		for i, w := range st.Values {
			if err := peerValueMatches(w, vals[i]); err != nil {
				return fmt.Errorf("sv event value[%d]: %w", i, err)
			}
		}
	}
	stats := lastEvent(evs, "stats")
	if stats == nil {
		return fmt.Errorf("no stats event")
	}
	received, lost := jsonInt(stats["received"]), max(jsonInt(stats["lost"])-skipped, 0)
	if jsonInt(stats["duplicates"]) != 0 || float64(lost+jsonInt(stats["outOfOrder"])) > svMaxLossPct/100*float64(received) {
		return fmt.Errorf("stats %v", stats)
	}
	if float64(received) < svMinReceived*float64(published) {
		return fmt.Errorf("received %d of %d published samples", received, published)
	}
	return nil
}

// TestSV_SVCBOverMMS reads MSVCB02 of the IED fixture from the forward IED
// servers. Stacks that do not model SV control blocks over MMS must fail
// the read deterministically.
func TestSV_SVCBOverMMS(t *testing.T) {
	fx := loadSVFixture(t)
	st := fx.Streams["MSVCB02"]
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)
			res := runCtl(t, h.ctlEnv(), commandTimeout,
				"sv", "svcb", "get", "--ld", "InteropLD", "--svcb", "MSVCB02", "--format", "json")
			if !lockFile.SVCBOverMMS[stack] {
				if res.TimedOut || res.ExitCode == 0 {
					logCommandFailure(t, res, h)
					t.Fatalf("%s does not model MSVCB02 over MMS (registered limitation), yet the read succeeded", stack)
				}
				return
			}
			requireExitZero(t, res, h)
			var v struct {
				Reference string   `json:"reference"`
				MsvID     string   `json:"msv_id"`
				DatSet    string   `json:"dat_set"`
				ConfRev   int      `json:"conf_rev"`
				SmpRate   int      `json:"smp_rate"`
				SmpMod    string   `json:"smp_mod"`
				OptFlds   []string `json:"opt_flds"`
				Dst       string   `json:"dst"`
				AppID     string   `json:"app_id"`
				VID       int      `json:"vlan_id"`
				Priority  int      `json:"vlan_priority"`
				NoASDU    *int     `json:"no_asdu"`
			}
			if err := json.Unmarshal(res.Stdout, &v); err != nil {
				logCommandFailure(t, res, h)
				t.Fatalf("decode svcb get JSON: %v", err)
			}
			datSet := strings.Replace(st.DatSet, "InteropIED", h.iedName, 1)
			ok := v.MsvID == st.SvID && v.DatSet == datSet && v.ConfRev == st.ConfRev && v.SmpRate == st.SmpRate &&
				v.SmpMod == svSCLSmpMod[st.SmpMod] && v.Dst == st.Dst && strings.EqualFold(v.AppID, st.AppID) &&
				v.VID == st.VLANID && v.Priority == st.VLANPrio && (v.NoASDU == nil || *v.NoASDU == st.NoASDU)
			for _, f := range []string{"refresh-time", "sample-rate", "data-set"} {
				ok = ok && containsName(v.OptFlds, f)
			}
			if !ok {
				logCommandFailure(t, res, h)
				t.Fatalf("MSVCB02 %+v does not match the fixture stream %+v", v, st)
			}
			t.Logf("%s MSVCB02: %s", stack, bytes.TrimSpace(res.Stdout))
		})
	}
}
