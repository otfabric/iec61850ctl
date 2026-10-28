// SPDX-License-Identifier: MIT

package gooseops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

const fixture = "testdata/interop-goose.cid"

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

func loadCB(t *testing.T, name string) *goose.ControlBlock {
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
	for _, sel := range []string{"gcbBool", "InteropLD/gcbBool", "InteropIED/InteropLD/gcbBool",
		"InteropIEDInteropLD/LLN0$GO$gcbBool", "InteropLD/LLN0$GO$gcbBool"} {
		cb, err := SelectControlBlock(cbs, sel)
		if err != nil || cb.Name != "gcbBool" {
			t.Errorf("select %q: %v", sel, err)
		}
	}
	if _, err := SelectControlBlock(cbs, ""); !errors.Is(err, errs.ErrInvalidInput) {
		t.Errorf("empty: %v", err)
	}
	if _, err := SelectControlBlock(cbs, "nope"); !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	dup := append(slices.Clone(cbs), cbs[0])
	if _, err := SelectControlBlock(dup, cbs[0].Name); !errors.Is(err, errs.ErrInvalidInput) {
		t.Errorf("ambiguous: %v", err)
	}
	if _, _, err := LoadControlBlocks("testdata/missing.cid"); !errors.Is(err, errs.ErrInvalidInput) {
		t.Errorf("missing file: %v", err)
	}

	v := ViewControlBlock(&cbs[1])
	if v.AppID != "0x1002" || v.Dst != "01:0c:cd:01:00:02" || v.VLANID == nil || *v.VLANPrio != 4 ||
		v.MinTimeMs != 10 || v.MaxTimeMs != 1000 || len(v.Members) != 2 || !strings.HasSuffix(v.Members[0], "[ST]") {
		t.Fatalf("view = %+v", v)
	}
	noAddr := cbs[0]
	noAddr.HasAddress = false
	if v := ViewControlBlock(&noAddr); v.Dst != "" || v.AppID != "" {
		t.Fatalf("no-address view = %+v", v)
	}
}

func TestBareRef(t *testing.T) {
	t.Parallel()
	cases := []struct{ ref, ied, want string }{
		{"IED1LD0/LLN0$GO$g", "IED1", "LD0/LLN0$GO$g"},
		{"LD0/LLN0$GO$g", "IED1", "LD0/LLN0$GO$g"},
		{"IED1/LLN0$GO$g", "IED1", "IED1/LLN0$GO$g"},
		{"IED1LD0/LLN0$GO$g", "", "IED1LD0/LLN0$GO$g"},
		{"noslash", "IED1", "noslash"},
	}
	for _, c := range cases {
		if got := BareRef(c.ref, c.ied); got != c.want {
			t.Errorf("BareRef(%q,%q)=%q", c.ref, c.ied, got)
		}
	}
}

func TestParseAppID(t *testing.T) {
	t.Parallel()
	good := map[string]uint16{"0x1001": 0x1001, "1001": 0x1001, "0XFFFF": 0xFFFF, "d:4097": 4097, "0": 0}
	for in, want := range good {
		if got, err := ParseAppID(in); err != nil || got != want {
			t.Errorf("ParseAppID(%q)=%d,%v", in, got, err)
		}
	}
	for _, in := range []string{"", "0x", "zz", "12345", "d:70000", "d:12x", "0x1g"} {
		if _, err := ParseAppID(in); !errors.Is(err, errs.ErrInvalidInput) {
			t.Errorf("ParseAppID(%q) err=%v", in, err)
		}
	}
	if FormatAppID(0x1001) != "0x1001" {
		t.Fatal("FormatAppID")
	}
}

func TestParseValue(t *testing.T) {
	t.Parallel()
	ok := []struct {
		raw string
		ts  mms.TypeSpec
	}{
		{"true", mms.TypeSpec{Type: mms.ValueTypeBoolean}},
		{"-5", mms.TypeSpec{Type: mms.ValueTypeInteger, Size: 8}},
		{"0x10", mms.TypeSpec{Type: mms.ValueTypeInteger}},
		{"7", mms.TypeSpec{Type: mms.ValueTypeUnsigned, Size: 32}},
		{"9", mms.TypeSpec{Type: mms.ValueTypeUnsigned}},
		{"1.5", mms.TypeSpec{Type: mms.ValueTypeFloat, FormatWidth: 32}},
		{"1e300", mms.TypeSpec{Type: mms.ValueTypeFloat, FormatWidth: 64}},
		{"hello", mms.TypeSpec{Type: mms.ValueTypeVisibleString}},
		{"héllo", mms.TypeSpec{Type: mms.ValueTypeMmsString}},
		{"0xdead", mms.TypeSpec{Type: mms.ValueTypeOctetString}},
		{"0000", mms.TypeSpec{Type: mms.ValueTypeBitString, Size: 13}},
		{"c0/2", mms.TypeSpec{Type: mms.ValueTypeBitString}},
		{"now", mms.TypeSpec{Type: mms.ValueTypeUTCTime}},
		{"2026-01-02T03:04:05Z", mms.TypeSpec{Type: mms.ValueTypeUTCTime}},
	}
	for _, c := range ok {
		v, err := ParseValue(c.raw, c.ts)
		if err != nil || v.Type() != c.ts.Type {
			t.Errorf("ParseValue(%q,%v)=%v,%v", c.raw, c.ts.Type, v, err)
		}
	}
	if v, _ := ParseValue("0000", mms.TypeSpec{Type: mms.ValueTypeBitString, Size: 13}); func() int { n, _ := v.BitStringLength(); return n }() != 13 {
		t.Error("bit string size from type")
	}
	bad := []struct {
		raw string
		ts  mms.TypeSpec
	}{
		{"maybe", mms.TypeSpec{Type: mms.ValueTypeBoolean}},
		{"300", mms.TypeSpec{Type: mms.ValueTypeInteger, Size: 8}},
		{"-1", mms.TypeSpec{Type: mms.ValueTypeUnsigned}},
		{"x", mms.TypeSpec{Type: mms.ValueTypeFloat}},
		{"1e39", mms.TypeSpec{Type: mms.ValueTypeFloat}},
		{"zz", mms.TypeSpec{Type: mms.ValueTypeOctetString}},
		{"zz", mms.TypeSpec{Type: mms.ValueTypeBitString}},
		{"ff/9", mms.TypeSpec{Type: mms.ValueTypeBitString}},
		{"yesterday", mms.TypeSpec{Type: mms.ValueTypeUTCTime}},
		{"caf\u00e9", mms.TypeSpec{Type: mms.ValueTypeVisibleString}},
		{"toolong", mms.TypeSpec{Type: mms.ValueTypeVisibleString, Size: 3}},
	}
	for _, c := range bad {
		if _, err := ParseValue(c.raw, c.ts); !errors.Is(err, errs.ErrInvalidInput) {
			t.Errorf("ParseValue(%q,%v) err=%v", c.raw, c.ts.Type, err)
		}
	}
	if _, err := ParseValue("1", mms.TypeSpec{Type: mms.ValueTypeStructure}); !errors.Is(err, errs.ErrUnsupported) {
		t.Errorf("structure: %v", err)
	}
}

func TestApplyOverrides(t *testing.T) {
	t.Parallel()
	cb := loadCB(t, "gcbBool")
	vals := cb.DefaultValues()
	ref := cb.Members[0].Ref
	if err := ApplyOverrides(vals, cb.Members, []string{"0=true", ref + "[ST]=false", ref + "=true"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := vals[0].Bool(); !b {
		t.Fatal("override not applied")
	}
	for _, a := range []string{"noequals", "5=true", "-1=true", "0=maybe"} {
		if err := ApplyOverrides(vals, cb.Members, []string{a}); !errors.Is(err, errs.ErrInvalidInput) {
			t.Errorf("%q: %v", a, err)
		}
	}
	if err := ApplyOverrides(vals, cb.Members, []string{"X/Y.Z=1"}); !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("unknown member: %v", err)
	}
	if err := ApplyOverrides(vals[:1], cb.Members, []string{"1=true"}); !errors.Is(err, errs.ErrInvalidInput) {
		t.Errorf("short values: %v", err)
	}
}

func TestValueJSON(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	v := mms.NewStructure([]*mms.Value{
		mms.NewBoolean(true), mms.NewInteger(-3), mms.NewUnsigned(4), mms.NewFloat(1.5),
		mms.NewFloat(math.Inf(1)), mms.NewVisibleString("a"), mms.NewMmsString("b"),
		mms.NewBitStringWithLength([]byte{0xc0}, 2), mms.NewOctetString([]byte{1, 2}),
		mms.NewUTCTime(ts), mms.NewArray([]*mms.Value{mms.NewBoolean(false)}), mms.NewBinaryTime(0),
	})
	b, err := json.Marshal(ValueJSON(v))
	if err != nil {
		t.Fatal(err)
	}
	want := `[true,-3,4,1.5,"+Inf","a","b",{"bits":"c0","len":2},"0102","2026-01-02T03:04:05Z",[false],`
	if !strings.HasPrefix(string(b), want) {
		t.Fatalf("got %s", b)
	}
	if ValueJSON(nil) != nil {
		t.Fatal("nil")
	}
}

// pair returns a publisher port and a listener port on a fresh hub.
func pair(t *testing.T) (pub, rx *goose.HubPort) {
	t.Helper()
	hub := goose.NewHub()
	pub = hub.NewPort(goose.MAC{0x02, 0, 0, 0, 0, 1})
	rx = hub.NewPort(goose.MAC{0x02, 0, 0, 0, 0, 2})
	t.Cleanup(func() { _ = pub.Close(); _ = rx.Close() })
	return pub, rx
}

func publisherConfig(t *testing.T, name string) goose.PublisherConfig {
	t.Helper()
	pc, err := loadCB(t, name).PublisherConfig()
	if err != nil {
		t.Fatal(err)
	}
	pc.Test = true
	pc.MinTime, pc.MaxTime = 2*time.Millisecond, 20*time.Millisecond
	return pc
}

// startPublisher runs Publish in the background until the test ends.
func startPublisher(t *testing.T, tr goose.Transport, pc goose.PublisherConfig, changes int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Publish(ctx, PublishConfig{Transport: tr, Publisher: pc, Changes: changes, Interval: 5 * time.Millisecond, Output: Output{Out: &syncBuf{}}})
	}()
	t.Cleanup(func() { cancel(); <-done })
}

func TestSniff(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	pc := publisherConfig(t, "gcbBool")
	startPublisher(t, pubPort, pc, 0)

	var out, diag syncBuf
	res, err := Sniff(context.Background(), SniffConfig{
		Transport: rx, Filter: Filter{Dst: pc.Dst, AppIDs: []uint16{pc.AppID}, GocbRef: pc.GocbRef},
		Count: 3, Duration: 5 * time.Second, Output: Output{Out: &out, Err: &diag},
	})
	if err != nil || res.Frames != 3 {
		t.Fatalf("res=%+v err=%v\n%s", res, err, out.String())
	}
	if !strings.Contains(out.String(), "appid=0x1002") || !strings.Contains(out.String(), "[test]") ||
		!strings.Contains(out.String(), "Frames: 3") {
		t.Fatalf("text output:\n%s", out.String())
	}

	// JSONL, filtered out by APPID: no frames before the duration elapses.
	var jl syncBuf
	res, err = Sniff(context.Background(), SniffConfig{
		Transport: rx, Filter: Filter{AppIDs: []uint16{0x9999}}, Count: 1, Duration: 100 * time.Millisecond,
		Output: Output{JSONL: true, Out: &jl},
	})
	if !errors.Is(err, errs.ErrTimeout) || res.Frames != 0 || res.Filtered == 0 {
		t.Fatalf("filtered: res=%+v err=%v", res, err)
	}
	if !strings.Contains(jl.String(), `"event":"summary"`) {
		t.Fatalf("jsonl: %s", jl.String())
	}
}

func TestSniffJSONAndDecodeErrors(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	if err := pubPort.Send([]byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	pc := publisherConfig(t, "gcbAll")
	pc.VLAN = &goose.VLAN{Priority: 4, ID: 5}
	startPublisher(t, pubPort, pc, 0)
	var out, diag syncBuf
	res, err := Sniff(context.Background(), SniffConfig{
		Transport: rx, Count: 1, Duration: 5 * time.Second, Output: Output{JSONL: true, Out: &out, Err: &diag},
		Now: func() time.Time { return time.Unix(0, 0) },
	})
	if err != nil || res.DecodeErrors != 1 || !strings.Contains(diag.String(), "undecodable") {
		t.Fatalf("res=%+v err=%v diag=%s", res, err, diag.String())
	}
	var fv FrameView
	if err := json.Unmarshal([]byte(strings.SplitN(out.String(), "\n", 2)[0]), &fv); err != nil {
		t.Fatal(err)
	}
	if fv.Event != "frame" || fv.AppID != "0x1001" || fv.VLANID == nil || *fv.VLANID != 5 || !fv.Test || len(fv.Values) != 8 {
		t.Fatalf("frame view %+v", fv)
	}
}

func TestSniffErrors(t *testing.T) {
	t.Parallel()
	if _, err := Sniff(context.Background(), SniffConfig{}); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	_, rx := pair(t)
	_ = rx.Close()
	if _, err := Sniff(context.Background(), SniffConfig{Transport: rx, Output: Output{Out: &syncBuf{}}}); !errors.Is(err, goose.ErrClosed) {
		t.Fatalf("closed transport: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, rx2 := pair(t)
	if _, err := Sniff(ctx, SniffConfig{Transport: rx2, Count: 1, Output: Output{Out: &syncBuf{}}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
}

func TestSubscribe(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	cb := loadCB(t, "gcbBool")
	pc := publisherConfig(t, "gcbBool")
	startPublisher(t, pubPort, pc, 2)

	var out syncBuf
	sum, err := Subscribe(context.Background(), SubscribeConfig{
		Transport: rx, Subscription: cb.SubscriptionConfig(), Count: 3, Duration: 5 * time.Second,
		Output: Output{Out: &out},
	})
	if err != nil || sum.Messages != 3 || len(sum.Streams) != 1 || sum.Streams[0].Status != "ok" {
		t.Fatalf("sum=%+v err=%v\n%s", sum, err, out.String())
	}
	if !strings.Contains(out.String(), "state") || !strings.Contains(out.String(), "status=ok") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestSubscribeInvalidAndTimeout(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	cb := loadCB(t, "gcbBool")
	pc := publisherConfig(t, "gcbBool")
	pc.ConfRev = 7 // subscriber expects 1 -> invalid
	startPublisher(t, pubPort, pc, 0)

	var out syncBuf
	sc := cb.SubscriptionConfig()
	sum, err := Subscribe(context.Background(), SubscribeConfig{
		Transport: rx, Subscription: sc, Count: 1, Duration: 300 * time.Millisecond,
		Output: Output{JSONL: true, Out: &out},
	})
	if !errors.Is(err, errs.ErrTimeout) || sum.Messages != 0 {
		t.Fatalf("sum=%+v err=%v", sum, err)
	}
	s := out.String()
	if !strings.Contains(s, `"event":"invalid"`) || !strings.Contains(s, "confRev") || !strings.Contains(s, `"event":"summary"`) {
		t.Fatalf("jsonl:\n%s", s)
	}
	var text syncBuf
	_, _ = Subscribe(context.Background(), SubscribeConfig{
		Transport: rx, Subscription: sc, Duration: 100 * time.Millisecond, Output: Output{Out: &text},
	})
	if !strings.Contains(text.String(), "error=") || !strings.Contains(text.String(), "last error") {
		t.Fatalf("text:\n%s", text.String())
	}
}

func TestSubscribeErrors(t *testing.T) {
	t.Parallel()
	if _, err := Subscribe(context.Background(), SubscribeConfig{}); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	_, rx := pair(t)
	if _, err := Subscribe(context.Background(), SubscribeConfig{Transport: rx, Subscription: goose.SubscriptionConfig{QueueSize: -1}}); err == nil {
		t.Fatal("bad subscription accepted")
	}
	_ = rx.Close()
	if _, err := Subscribe(context.Background(), SubscribeConfig{Transport: rx, Duration: time.Second, Output: Output{Out: &syncBuf{}}}); err == nil {
		t.Fatal("closed transport accepted")
	}
}

func TestPublish(t *testing.T) {
	t.Parallel()
	pubPort, rx := pair(t)
	pc := publisherConfig(t, "gcbBool")
	var out syncBuf
	res, err := Publish(context.Background(), PublishConfig{
		Transport: pubPort, Publisher: pc, Changes: 2, Interval: time.Millisecond, Duration: 30 * time.Millisecond,
		Output: Output{JSONL: true, Out: &out},
	})
	if err != nil || res.StNum != 3 || res.Sent < 3 || !res.Test {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.Contains(out.String(), `"st_num":3`) {
		t.Fatalf("jsonl %s", out.String())
	}
	_ = rx

	// Cancelled by the operator: a normal end.
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	var text syncBuf
	if _, err := Publish(ctx, PublishConfig{Transport: pubPort, Publisher: pc, Output: Output{Out: &text}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "Published") {
		t.Fatal(text.String())
	}
}

func TestPublishErrors(t *testing.T) {
	t.Parallel()
	if _, err := Publish(context.Background(), PublishConfig{}); !errors.Is(err, errs.ErrInvalidInput) {
		t.Fatal(err)
	}
	pubPort, _ := pair(t)
	if _, err := Publish(context.Background(), PublishConfig{Transport: pubPort}); !errors.Is(err, goose.ErrInvalidConfig) {
		t.Fatalf("empty config: %v", err)
	}
	pc := publisherConfig(t, "gcbBool")
	_ = pubPort.Close()
	_, err := Publish(context.Background(), PublishConfig{Transport: pubPort, Publisher: pc, Changes: 1, Duration: time.Second, Output: Output{Out: &syncBuf{}}})
	if err == nil {
		t.Fatal("closed transport accepted")
	}
}
