// SPDX-License-Identifier: MIT

package svops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/otfabric/go-iec61850/sv"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

const fixture = "testdata/interop-sv.cid"

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func loadCB(t *testing.T, name string) *sv.ControlBlock {
	t.Helper()
	cbs, warn, err := LoadControlBlocks(fixture)
	if err != nil || warn != nil {
		t.Fatalf("load: %v / %v", err, warn)
	}
	cb, err := SelectControlBlock(cbs, name)
	if err != nil {
		t.Fatal(err)
	}
	return cb
}

func TestLoadAndSelect(t *testing.T) {
	t.Parallel()
	cbs, warn, err := LoadControlBlocks(fixture)
	if err != nil || warn != nil || len(cbs) != 2 {
		t.Fatalf("cbs=%d warn=%v err=%v", len(cbs), warn, err)
	}
	for _, sel := range []string{"MSVCB01", "InteropLD/MSVCB01", "InteropIED/InteropLD/MSVCB01",
		"InteropIEDInteropLD/LLN0$MS$MSVCB01", "InteropLD/LLN0$MS$MSVCB01"} {
		cb, err := SelectControlBlock(cbs, sel)
		if err != nil || cb.Name != "MSVCB01" {
			t.Errorf("%s: %v %v", sel, cb, err)
		}
	}
	if _, err := SelectControlBlock(cbs, ""); !errors.Is(err, errs.ErrInvalidInput) {
		t.Errorf("empty: %v", err)
	}
	if _, err := SelectControlBlock(cbs, "nope"); !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	dup := append(append([]sv.ControlBlock(nil), cbs...), cbs[0])
	dup[2].SvCBRef = "other"
	if _, err := SelectControlBlock(dup, "MSVCB01"); !errors.Is(err, errs.ErrInvalidInput) || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("ambiguous: %v", err)
	}
	if _, _, err := LoadControlBlocks("testdata/missing.cid"); !errors.Is(err, errs.ErrInvalidInput) {
		t.Errorf("missing file: %v", err)
	}
}

func TestViewControlBlock(t *testing.T) {
	t.Parallel()
	le := ViewControlBlock(loadCB(t, "MSVCB01"))
	if le.Profile != "9-2LE" || le.SvID != "InteropSv92LE" || le.AppID != "0x4000" || le.Dst != "01:0c:cd:04:00:00" ||
		le.SmpRate != 80 || le.SmpMod != "SmpPerPeriod" || le.SampleOctets != 64 || strings.Join(le.OptFlds, ",") != "sample-synchronized" || le.VLANID == nil {
		t.Fatalf("%+v", le)
	}
	gen := ViewControlBlock(loadCB(t, "MSVCB02"))
	if gen.Profile != "generic" || gen.NoASDU != 4 || gen.SmpMod != "SmpPerSec" || len(gen.Members) != 5 ||
		strings.Join(gen.OptFlds, ",") != "refresh-time,sample-synchronized,sample-rate,data-set" {
		t.Fatalf("%+v", gen)
	}
	b, err := json.Marshal(gen)
	if err != nil || !strings.Contains(string(b), `"svcb_ref":"InteropIEDInteropLD/LLN0$MS$MSVCB02"`) {
		t.Fatalf("%s %v", b, err)
	}
}

func TestOptFldsAndSmpMod(t *testing.T) {
	t.Parallel()
	o, err := ParseOptFlds("refresh-time, sample-rate,data-set,sample-synchronized,security")
	if err != nil || !o.RefreshTime || !o.SampleRate || !o.DataSet || !o.SampleSynchronized || !o.Security {
		t.Fatalf("%+v %v", o, err)
	}
	if got := strings.Join(OptFldsNames(o), ","); got != "refresh-time,sample-synchronized,sample-rate,data-set,security" {
		t.Fatal(got)
	}
	if o, err := ParseOptFlds("none"); err != nil || o != (sv.OptFlds{}) {
		t.Fatalf("%+v %v", o, err)
	}
	if _, err := ParseOptFlds("bogus"); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	for in, want := range map[string]sv.SmpMod{
		"SmpPerPeriod": sv.SmpPerPeriod, "samples-per-second": sv.SmpPerSec, "2": sv.SecPerSmp, "smppersec": sv.SmpPerSec,
	} {
		if got, err := ParseSmpMod(in); err != nil || got != want {
			t.Errorf("%s: %v %v", in, got, err)
		}
	}
	if _, err := ParseSmpMod("hourly"); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
}

func TestParseKindValue(t *testing.T) {
	t.Parallel()
	ok := []struct {
		raw  string
		kind sv.Kind
		want string
	}{
		{"true", sv.KindBoolean, "true"},
		{"-5", sv.KindInt8, "-5"},
		{"0x7fffffff", sv.KindInt32, "2147483647"},
		{"3", sv.KindEnum, "3"},
		{"16777215", sv.KindUint24, "16777215"},
		{"1.5", sv.KindFloat32, "1.5"},
		{"on", sv.KindDbpos, "2"},
		{"3", sv.KindDbpos, "3"},
	}
	for _, c := range ok {
		v, err := ParseKindValue(c.raw, c.kind)
		if err != nil {
			t.Errorf("%s/%s: %v", c.raw, c.kind, err)
			continue
		}
		if got := strings.TrimSpace(v.String()); !strings.Contains(got, c.want) {
			t.Errorf("%s/%s: %s", c.raw, c.kind, got)
		}
	}
	for _, c := range []struct {
		raw  string
		kind sv.Kind
	}{
		{"maybe", sv.KindBoolean}, {"200", sv.KindInt8}, {"-1", sv.KindUint16}, {"16777216", sv.KindUint24},
		{"1e39", sv.KindFloat32}, {"x", sv.KindFloat64}, {"up", sv.KindDbpos}, {"wat", sv.KindQuality},
		{"yesterday", sv.KindTimestamp},
	} {
		if _, err := ParseKindValue(c.raw, c.kind); !errors.Is(err, errs.ErrInvalidInput) {
			t.Errorf("%s/%s accepted: %v", c.raw, c.kind, err)
		}
	}
	ts, err := ParseKindValue("2026-01-02T03:04:05Z", sv.KindTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	if tm, _ := ts.UTCTime(); tm.Year() != 2026 {
		t.Fatal(tm)
	}
	if _, err := ParseKindValue("now", sv.KindTimestamp); err != nil {
		t.Fatal(err)
	}
}

func TestParseQuality(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]sv.Quality{
		"good": sv.QualityGood, "invalid+test": sv.QualityInvalid | sv.QualityTest, "0x0802": sv.QualityInvalid | sv.QualityTest,
		"0000000000010": sv.QualityTest, "0100000000000": sv.QualityInvalid, "questionable": sv.QualityQuestionable,
	} {
		if got, err := ParseQuality(in); err != nil || got != want {
			t.Errorf("%s: %#x %v", in, got, err)
		}
	}
	if _, err := ParseQuality("good+bogus"); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
}

func TestOverridesAndLeafNames(t *testing.T) {
	t.Parallel()
	cb := loadCB(t, "MSVCB01")
	vals := ZeroValues(cb.Layout)
	if err := ApplyOverrides(cb, vals, []string{"IA=1000", "qVB=invalid", "0=7"}); err != nil {
		t.Fatal(err)
	}
	if i, _ := vals[0].Int64(); i != 7 {
		t.Fatal(vals[0])
	}
	if q, _ := sv.QualityFromValue(vals[11]); q != sv.QualityInvalid {
		t.Fatalf("qVB %#x", q)
	}
	names := LeafNames(cb)
	if len(names) != 16 || !contains(names[0], "IA") || !contains(names[15], "qVN") {
		t.Fatalf("%v", names)
	}
	for _, bad := range []string{"IA", "XX=1", "99=1", "IA=abc"} {
		if err := ApplyOverrides(cb, vals, []string{bad}); !errors.Is(err, errs.ErrInvalidInput) {
			t.Errorf("%s: %v", bad, err)
		}
	}

	gen := loadCB(t, "MSVCB02")
	gv := ZeroValues(gen.Layout)
	ref := gen.Members[0].Ref
	if err := ApplyOverrides(gen, gv, []string{ref + "=12.5", "1=test"}); err != nil {
		t.Fatal(err)
	}
	if f, _ := gv[0].Float64(); f != 12.5 {
		t.Fatal(gv[0])
	}
	if _, err := gen.Layout.Encode(nil, gv); err != nil {
		t.Fatal(err)
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestWaveform(t *testing.T) {
	t.Parallel()
	w, err := ParseWaveform("vb=325,60,-120", 50)
	if err != nil || w.Channel != 5 || w.Amplitude != 325 || w.Frequency != 60 || w.Phase != -120 {
		t.Fatalf("%+v %v", w, err)
	}
	if w, err := ParseWaveform("IA=100", 50); err != nil || w.Frequency != 50 || w.Phase != 0 {
		t.Fatalf("%+v %v", w, err)
	}
	for _, bad := range []string{"IA", "XX=1", "IA=a", "IA=1,2,3,4", "IA=1,-5", "IA=NaN"} {
		if _, err := ParseWaveform(bad, 50); !errors.Is(err, errs.ErrInvalidInput) {
			t.Errorf("%s: %v", bad, err)
		}
	}

	cb := loadCB(t, "MSVCB01")
	base := ZeroValues(cb.Layout)
	base[15] = sv.QualityTest.Value()
	src, err := WaveformSource(base, []Waveform{{Channel: 0, Amplitude: 100, Frequency: 50}, {Channel: 4, Amplitude: 230, Frequency: 50, Phase: 90}}, 4000)
	if err != nil {
		t.Fatal(err)
	}
	var le sv.LESample
	// A quarter period in (sample 20 at 4000/s, 50 Hz): IA peaks, VA (90°) crosses zero.
	if err := sv.DecodeLE(src(sv.SampleInfo{Index: 20}, nil), &le); err != nil {
		t.Fatal(err)
	}
	if math.Abs(sv.Amps(le.Current[0].Value)-100) > 0.01 || math.Abs(sv.Volts(le.Voltage[0].Value)) > 0.1 || le.Voltage[3].Q != sv.QualityTest {
		t.Fatalf("%+v", le)
	}
	if _, err := WaveformSource(base[:3], nil, 4000); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	if _, err := WaveformSource(base, nil, 0); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	bad := ZeroValues(cb.Layout)
	bad[1] = mms.NewVisibleString("q")
	if _, err := WaveformSource(bad, nil, 4000); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
}

func pair(t *testing.T) (pub, rx *sv.HubPort) {
	t.Helper()
	hub := sv.NewHub()
	pub = hub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, 1})
	rx = hub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, 2})
	t.Cleanup(func() { _ = pub.Close(); _ = rx.Close() })
	return pub, rx
}

func publisherConfig(t *testing.T, name string) sv.PublisherConfig {
	t.Helper()
	pc, err := loadCB(t, name).PublisherConfig()
	if err != nil {
		t.Fatal(err)
	}
	pc.Simulation = true
	return pc
}

// startPublisher runs Publish in the background until the test ends.
func startPublisher(t *testing.T, tr sv.Transport, pc sv.PublisherConfig) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Publish(ctx, PublishConfig{Transport: tr, Publisher: pc, Output: Output{Out: &syncBuf{}}})
	}()
	t.Cleanup(func() { cancel(); <-done })
}

func TestSniffLE(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	cb := loadCB(t, "MSVCB01")
	pc := publisherConfig(t, "MSVCB01")
	src, err := WaveformSource(ZeroValues(cb.Layout), []Waveform{{Channel: 0, Amplitude: 10, Frequency: 50}}, 4000)
	if err != nil {
		t.Fatal(err)
	}
	pc.Source = src
	startPublisher(t, pubPort, pc)

	var out syncBuf
	res, err := Sniff(context.Background(), SniffConfig{
		Transport: rx, Filter: Filter{SvID: "InteropSv92LE", AppIDs: []uint16{0x4000}, Dst: cb.Dst},
		Layout: cb.Layout, SampleEvery: 2, Count: 6, Duration: 5 * time.Second, Output: Output{Out: &out},
	})
	if err != nil || res.ASDUs != 6 || res.Printed != 3 || res.Frames != 6 {
		t.Fatalf("res=%+v err=%v\n%s", res, err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "svID=InteropSv92LE") || !strings.Contains(s, "I [A]:") || !strings.Contains(s, "[sim]") ||
		!strings.Contains(s, "ASDUs: 6 in 6 frame(s), printed 3") {
		t.Fatalf("output:\n%s", s)
	}
}

func TestSniffGenericJSON(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	pc := publisherConfig(t, "MSVCB02")
	startPublisher(t, pubPort, pc)

	var out syncBuf
	res, err := Sniff(context.Background(), SniffConfig{
		Transport: rx, Count: 8, Duration: 5 * time.Second, Output: Output{JSONL: true, Out: &out},
	})
	if err != nil || res.ASDUs != 8 || res.Frames != 2 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var first ASDUView
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.SvID != "InteropSvGeneric" || first.NoASDU != 4 || first.Sample == "" || first.Values != nil ||
		first.DatSet == "" || first.SmpMod != "SmpPerSec" || first.SmpRate == nil || first.RefrTm == nil {
		t.Fatalf("%s", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], `"event":"summary"`) {
		t.Fatal(lines[len(lines)-1])
	}
}

func TestSniffFilterTimeoutAndErrors(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	startPublisher(t, pubPort, publisherConfig(t, "MSVCB01"))
	_ = pubPort.Send([]byte{0x01, 0x0c, 0xcd, 0x04, 0, 0, 2, 0, 0, 0, 0, 1, 0x88, 0xba, 0, 1})

	var out, diag syncBuf
	res, err := Sniff(context.Background(), SniffConfig{
		Transport: rx, Filter: Filter{SvID: "other"}, Count: 1, Duration: 200 * time.Millisecond,
		Output: Output{Out: &out, Err: &diag},
	})
	if !errors.Is(err, errs.ErrTimeout) || res.ASDUs != 0 || res.Filtered == 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if res.DecodeErrors != 1 || !strings.Contains(diag.String(), "undecodable") {
		t.Fatalf("res=%+v diag=%s", res, diag.String())
	}
	if res, err := Sniff(context.Background(), SniffConfig{Transport: rx, Filter: Filter{AppIDs: []uint16{1}}, Duration: 50 * time.Millisecond, Output: Output{Out: &out}}); err != nil || res.Filtered == 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, err := Sniff(context.Background(), SniffConfig{}); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	_ = rx.Close()
	if _, err := Sniff(context.Background(), SniffConfig{Transport: rx, Duration: time.Second, Output: Output{Out: &out}}); err == nil {
		t.Fatal("closed transport accepted")
	}
}

func TestSubscribe(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	cb := loadCB(t, "MSVCB01")
	startPublisher(t, pubPort, publisherConfig(t, "MSVCB01"))

	var out syncBuf
	sum, err := Subscribe(context.Background(), SubscribeConfig{
		Transport: rx, Subscription: cb.SubscriptionConfig(), AcceptSimulation: true,
		SampleEvery: 1000, StatsInterval: 100 * time.Millisecond, Count: 2000, Duration: 5 * time.Second,
		Output: Output{Out: &out},
	})
	if err != nil || len(sum.Streams) != 1 || sum.Streams[0].Status != "ok" || sum.ASDUs < 2000 || !sum.Streams[0].Simulation {
		t.Fatalf("sum=%+v err=%v\n%s", sum, err, out.String())
	}
	s := out.String()
	for _, want := range []string{"start", "I [A]:", "stats:", "summary:", "SVID", "InteropSv92LE", "(sim)"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q:\n%s", want, s)
		}
	}
}

func TestSubscribeJSONAndTimeout(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	cb := loadCB(t, "MSVCB02")
	pc := publisherConfig(t, "MSVCB02")
	pc.Simulation = false
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Publish(ctx, PublishConfig{Transport: pubPort, Publisher: pc, Duration: 150 * time.Millisecond, Output: Output{Out: &syncBuf{}}})
	}()
	t.Cleanup(func() { stop(); <-done })

	var out syncBuf
	sc := cb.SubscriptionConfig()
	sc.Timeout = 50 * time.Millisecond
	sum, err := Subscribe(context.Background(), SubscribeConfig{
		Transport: rx, Subscription: sc, SampleEvery: 1, Count: 1_000_000, Duration: 400 * time.Millisecond,
		Output: Output{JSONL: true, Out: &out},
	})
	if !errors.Is(err, errs.ErrTimeout) || len(sum.Streams) != 1 || sum.Streams[0].Status != "timeout" {
		t.Fatalf("sum=%+v err=%v", sum, err)
	}
	s := out.String()
	for _, want := range []string{`"event":"start"`, `"event":"timeout"`, `"event":"asdu"`, `"values":[`, `"event":"summary"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s:\n%.2000s", want, s)
		}
	}
}

func TestSubscribeInvalid(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	cb := loadCB(t, "MSVCB01")
	pc := publisherConfig(t, "MSVCB01")
	pc.Simulation = false
	pc.ConfRev = 9 // subscriber expects 1
	startPublisher(t, pubPort, pc)

	var out syncBuf
	sum, err := Subscribe(context.Background(), SubscribeConfig{
		Transport: rx, Subscription: cb.SubscriptionConfig(), Count: 1, Duration: 200 * time.Millisecond,
		Output: Output{Out: &out},
	})
	if !errors.Is(err, errs.ErrTimeout) || len(sum.Streams) != 1 || sum.Streams[0].Invalid == 0 || sum.Streams[0].Status != "invalid" {
		t.Fatalf("sum=%+v err=%v", sum, err)
	}
	if !strings.Contains(out.String(), "invalid") || !strings.Contains(out.String(), "error=") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestSubscribeErrors(t *testing.T) {
	t.Parallel()
	if _, err := Subscribe(context.Background(), SubscribeConfig{}); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	_, rx := pair(t)
	if _, err := Subscribe(context.Background(), SubscribeConfig{Transport: rx, Subscription: sv.SubscriptionConfig{SmpCntWrap: -1}}); err == nil {
		t.Fatal("bad subscription accepted")
	}
	_ = rx.Close()
	if _, err := Subscribe(context.Background(), SubscribeConfig{Transport: rx, Duration: time.Second, Output: Output{Out: &syncBuf{}}}); err == nil {
		t.Fatal("closed transport accepted")
	}
}

func TestPublish(t *testing.T) {
	t.Parallel()
	pubPort, _ := pair(t)
	pc := publisherConfig(t, "MSVCB02")
	var out syncBuf
	res, err := Publish(context.Background(), PublishConfig{Transport: pubPort, Publisher: pc, Duration: 100 * time.Millisecond, Output: Output{Out: &out}})
	if err != nil || res.Rate != 1000 || res.NoASDU != 4 || res.Samples == 0 || res.Frames == 0 || res.SvID != "InteropSvGeneric" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.Contains(out.String(), "Published InteropSvGeneric (appid 0x4001 -> 01:0c:cd:04:00:01, simulation=true, 1000 samples/s, 4 ASDU/frame)") {
		t.Fatal(out.String())
	}
	var js syncBuf
	if _, err := Publish(context.Background(), PublishConfig{Transport: pubPort, Publisher: pc, Duration: 20 * time.Millisecond, Output: Output{JSONL: true, Out: &js}}); err != nil ||
		!strings.Contains(js.String(), `"event":"summary"`) {
		t.Fatalf("%v %s", err, js.String())
	}
}

func TestPublishErrors(t *testing.T) {
	t.Parallel()
	if _, err := Publish(context.Background(), PublishConfig{}); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	pubPort, _ := pair(t)
	if _, err := Publish(context.Background(), PublishConfig{Transport: pubPort, Publisher: sv.PublisherConfig{}}); err == nil {
		t.Fatal("empty config accepted")
	}
	pc := publisherConfig(t, "MSVCB01")
	_ = pubPort.Close()
	if _, err := Publish(context.Background(), PublishConfig{Transport: pubPort, Publisher: pc, Duration: time.Second, Output: Output{Out: &syncBuf{}}}); err == nil {
		t.Fatal("closed transport accepted")
	}
}

func TestEventView(t *testing.T) {
	t.Parallel()
	key := sv.StreamKey{Src: sv.MAC{2, 0, 0, 0, 0, 1}, AppID: 0x4000, SvID: "x"}
	var out bytes.Buffer
	for _, ev := range []sv.Event{
		{Kind: sv.EventGap, Stream: key, SmpCnt: 10, Lost: 3},
		{Kind: sv.EventSynch, Stream: key, SmpCnt: 11, SmpSynch: 2},
		{Kind: sv.EventConfRev, Stream: key, SmpCnt: 12, ConfRev: 4},
		{Kind: sv.EventTimeout, Stream: key},
		{Kind: sv.EventInvalid, Stream: key, Err: errors.New("bad")},
	} {
		WriteEventText(&out, ViewEvent(ev))
	}
	s := out.String()
	for _, want := range []string{"lost=3", "smpSynch=2", "confRev=4", "error=bad", "0x4000"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q:\n%s", want, s)
		}
	}
	if v := ViewEvent(sv.Event{Kind: sv.EventTimeout, Stream: key}); v.SmpCnt != nil {
		t.Fatal("timeout carries smpCnt")
	}
	var empty bytes.Buffer
	WriteStatsTable(&empty, StatsView{Event: "stats"})
	if !strings.Contains(empty.String(), "(no stream)") {
		t.Fatal(empty.String())
	}
}
