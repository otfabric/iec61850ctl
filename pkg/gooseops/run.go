// SPDX-License-Identifier: MIT

package gooseops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// Output selects the rendering of streamed results.
type Output struct {
	JSONL bool      // one JSON object per line instead of text
	Out   io.Writer // results
	Err   io.Writer // diagnostics
}

func (o Output) emit(text func(io.Writer), obj any) {
	if o.JSONL {
		_ = writeJSONL(o.Out, obj)
		return
	}
	text(o.Out)
}

// Filter selects frames by destination, APPID and gocbRef; zero fields match all.
type Filter struct {
	Dst     goose.MAC
	AppIDs  []uint16
	GocbRef string
}

func (f Filter) match(fr *goose.Frame) bool {
	if !f.Dst.IsZero() && fr.Dst != f.Dst {
		return false
	}
	if len(f.AppIDs) > 0 && !slices.Contains(f.AppIDs, fr.AppID) {
		return false
	}
	return f.GocbRef == "" || fr.PDU.GocbRef == f.GocbRef
}

// SniffConfig configures Sniff.
type SniffConfig struct {
	Transport goose.Transport
	Filter    Filter
	Count     int           // stop after this many matching frames (0 = unlimited)
	Duration  time.Duration // stop after this long (0 = until ctx is done)
	Output    Output
	Now       func() time.Time // test seam; nil uses time.Now
}

// SniffResult summarises a sniff run.
type SniffResult struct {
	Event        string `json:"event"`
	Frames       int    `json:"frames"`
	Filtered     int    `json:"filtered"`
	DecodeErrors int    `json:"decode_errors"`
}

// Sniff decodes GOOSE frames from the transport and prints those that match
// the filter. It returns an error matching errs.ErrTimeout when Count was
// set and the duration elapsed first.
func Sniff(ctx context.Context, cfg SniffConfig) (SniffResult, error) {
	res := SniffResult{Event: "summary"}
	if cfg.Transport == nil {
		return res, errs.Invalidf("sniff: no transport")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	ctx, cancel := withDuration(ctx, cfg.Duration)
	defer cancel()
	for cfg.Count == 0 || res.Frames < cfg.Count {
		raw, err := cfg.Transport.Recv(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return res, fmt.Errorf("goose receive: %w", err)
		}
		rx := now()
		fr, err := goose.DecodeWithLimits(raw, goose.DefaultLimits)
		if err != nil {
			res.DecodeErrors++
			if cfg.Output.Err != nil {
				_, _ = fmt.Fprintf(cfg.Output.Err, "warning: undecodable frame (%d octets): %v\n", len(raw), err)
			}
			continue
		}
		if !cfg.Filter.match(fr) {
			res.Filtered++
			continue
		}
		res.Frames++
		cfg.Output.emit(func(w io.Writer) { WriteFrameText(w, "frame", fr, rx) }, ViewFrame("frame", fr, rx))
	}
	cfg.Output.emit(func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "Frames: %d (filtered %d, undecodable %d)\n", res.Frames, res.Filtered, res.DecodeErrors)
	}, res)
	return res, countErr(ctx, cfg.Count, res.Frames, "GOOSE frame(s)")
}

// withDuration bounds ctx by d when positive.
func withDuration(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d > 0 {
		return context.WithTimeout(ctx, d)
	}
	return context.WithCancel(ctx)
}

// countErr reports a run that ended before the expected number of items:
// a timeout when the duration elapsed, the cancellation otherwise.
func countErr(ctx context.Context, want, got int, what string) error {
	if want == 0 || got >= want {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errs.Newf(errs.ErrTimeout, "received %d of %d %s before --duration elapsed", got, want, what)
	}
	return ctx.Err()
}

// SubscribeConfig configures Subscribe.
type SubscribeConfig struct {
	Transport    goose.Transport
	Subscription goose.SubscriptionConfig
	Count        int           // stop after this many valid messages (0 = unlimited)
	Duration     time.Duration // stop after this long (0 = until ctx is done)
	Output       Output
}

// EventView is the JSON form of a non-message subscriber event.
type EventView struct {
	Event    string           `json:"event"`
	Received time.Time        `json:"received"`
	Stream   string           `json:"stream"`
	Status   string           `json:"status"`
	Error    string           `json:"error,omitempty"`
	State    *SupervisionView `json:"state,omitempty"`
}

// SupervisionView is the JSON form of one stream's supervision state.
type SupervisionView struct {
	Src           string `json:"src"`
	AppID         string `json:"app_id"`
	GocbRef       string `json:"gocb_ref"`
	Status        string `json:"status"`
	St            bool   `json:"st"`
	SimSt         bool   `json:"sim_st"`
	NdsCom        bool   `json:"nds_com"`
	LastStNum     uint32 `json:"last_st_num"`
	LastSqNum     uint32 `json:"last_sq_num"`
	ConfRev       uint32 `json:"conf_rev"`
	RxConfRev     uint32 `json:"rx_conf_rev"`
	LastErr       string `json:"last_error,omitempty"`
	Messages      uint64 `json:"messages"`
	StateChanges  uint64 `json:"state_changes"`
	Duplicates    uint64 `json:"duplicates"`
	OutOfOrder    uint64 `json:"out_of_order"`
	StNumGaps     uint64 `json:"st_num_gaps"`
	SqNumGaps     uint64 `json:"sq_num_gaps"`
	Restarts      uint64 `json:"restarts"`
	Timeouts      uint64 `json:"timeouts"`
	Invalid       uint64 `json:"invalid"`
	DroppedEvents uint64 `json:"dropped_events"`
}

// ViewSupervision renders s.
func ViewSupervision(s goose.Supervision) SupervisionView {
	v := SupervisionView{
		Src: s.Stream.Src.String(), AppID: FormatAppID(s.Stream.AppID), GocbRef: s.Stream.GocbRef,
		Status: s.Status.String(), St: s.St, SimSt: s.SimSt, NdsCom: s.NdsCom,
		LastStNum: s.LastStNum, LastSqNum: s.LastSqNum, ConfRev: s.ConfRevNum, RxConfRev: s.RxConfRevNum,
		Messages: s.Messages, StateChanges: s.StateChanges, Duplicates: s.Duplicates, OutOfOrder: s.OutOfOrder,
		StNumGaps: s.StNumGaps, SqNumGaps: s.SqNumGaps, Restarts: s.Restarts, Timeouts: s.Timeouts,
		Invalid: s.Invalid, DroppedEvents: s.DroppedEvents,
	}
	if s.LastErr != nil {
		v.LastErr = s.LastErr.Error()
	}
	return v
}

// SubscribeSummary is the final JSON Lines record of Subscribe.
type SubscribeSummary struct {
	Event    string            `json:"event"`
	Messages int               `json:"messages"`
	Streams  []SupervisionView `json:"streams"`
}

// Subscribe receives GOOSE through a supervised subscription and prints
// every event until Count valid messages arrived, the duration elapsed or
// ctx is done. It returns the supervision summary; an error matching
// errs.ErrTimeout means Count was not reached in time.
func Subscribe(ctx context.Context, cfg SubscribeConfig) (SubscribeSummary, error) {
	sum := SubscribeSummary{Event: "summary"}
	if cfg.Transport == nil {
		return sum, errs.Invalidf("subscribe: no transport")
	}
	subscriber, err := goose.NewSubscriber(cfg.Transport, goose.SubscriberConfig{
		OnDecodeError: func(raw []byte, err error) {
			if cfg.Output.Err != nil {
				_, _ = fmt.Fprintf(cfg.Output.Err, "warning: undecodable frame (%d octets): %v\n", len(raw), err)
			}
		},
	})
	if err != nil {
		return sum, err
	}
	defer func() { _ = subscriber.Close() }()
	sc := cfg.Subscription
	sc.Handler = nil
	sub, err := subscriber.Subscribe(sc)
	if err != nil {
		return sum, err
	}
	defer func() { _ = sub.Close() }()

	ctx, cancel := withDuration(ctx, cfg.Duration)
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- subscriber.Run(ctx) }()

loop:
	for cfg.Count == 0 || sum.Messages < cfg.Count {
		select {
		case <-ctx.Done():
			break loop
		case err := <-runErr:
			if ctx.Err() != nil {
				break loop
			}
			if err == nil {
				err = goose.ErrClosed
			}
			return sum, fmt.Errorf("goose receive: %w", err)
		case ev := <-sub.Events():
			if ev.Kind == goose.EventMessage {
				sum.Messages++
			}
			emitEvent(cfg.Output, ev)
		}
	}
	for _, s := range sub.Supervision() {
		sum.Streams = append(sum.Streams, ViewSupervision(s))
	}
	cfg.Output.emit(func(w io.Writer) { writeSummaryText(w, sum) }, sum)
	return sum, countErr(ctx, cfg.Count, sum.Messages, "GOOSE message(s)")
}

func emitEvent(o Output, ev goose.Event) {
	name := ev.Kind.String()
	if ev.Kind == goose.EventMessage && ev.Frame != nil {
		if ev.NewState {
			name = "state"
		}
		o.emit(func(w io.Writer) { WriteFrameText(w, name, ev.Frame, ev.Received) }, ViewFrame(name, ev.Frame, ev.Received))
		return
	}
	st := ViewSupervision(ev.State)
	v := EventView{Event: name, Received: ev.Received.UTC(), Stream: streamName(ev.State.Stream), Status: st.Status, State: &st}
	if ev.Err != nil {
		v.Error = ev.Err.Error()
	}
	o.emit(func(w io.Writer) {
		line := fmt.Sprintf("%s %-9s %s status=%s", ev.Received.UTC().Format("15:04:05.000"), name, v.Stream, v.Status)
		if v.Error != "" {
			line += " error=" + v.Error
		}
		_, _ = fmt.Fprintln(w, line)
	}, v)
}

func streamName(k goose.StreamKey) string {
	return fmt.Sprintf("%s %s %s", k.Src, FormatAppID(k.AppID), k.GocbRef)
}

func writeSummaryText(w io.Writer, sum SubscribeSummary) {
	_, _ = fmt.Fprintf(w, "\nMessages: %d\n", sum.Messages)
	for _, s := range sum.Streams {
		_, _ = fmt.Fprintf(w, "Stream %s %s %s: status=%s st=%d sq=%d messages=%d states=%d timeouts=%d invalid=%d gaps=%d/%d dup=%d ooo=%d restarts=%d\n",
			s.Src, s.AppID, s.GocbRef, s.Status, s.LastStNum, s.LastSqNum, s.Messages, s.StateChanges,
			s.Timeouts, s.Invalid, s.StNumGaps, s.SqNumGaps, s.Duplicates, s.OutOfOrder, s.Restarts)
		if s.LastErr != "" {
			_, _ = fmt.Fprintf(w, "  last error: %s\n", s.LastErr)
		}
	}
}

// PublishConfig configures Publish.
type PublishConfig struct {
	Transport goose.Transport
	Publisher goose.PublisherConfig
	// Changes publishes this many additional state changes (same values,
	// new stNum) at Interval after the initial state.
	Changes  int
	Interval time.Duration
	// Duration keeps retransmitting this long after the last change
	// (0 = until ctx is done).
	Duration time.Duration
	Output   Output
}

// PublishResult summarises a publish run.
type PublishResult struct {
	Event   string `json:"event"`
	GocbRef string `json:"gocb_ref"`
	AppID   string `json:"app_id"`
	Dst     string `json:"dst"`
	Test    bool   `json:"test"`
	StNum   uint32 `json:"st_num"`
	SqNum   uint32 `json:"sq_num"`
	Sent    uint64 `json:"sent"`
}

// Publish runs a GOOSE publisher: the initial state, Changes further state
// changes, then retransmissions until Duration elapses or ctx is done.
func Publish(ctx context.Context, cfg PublishConfig) (PublishResult, error) {
	pc := cfg.Publisher
	res := PublishResult{Event: "summary", GocbRef: pc.GocbRef, AppID: FormatAppID(pc.AppID), Dst: pc.Dst.String(), Test: pc.Test}
	if cfg.Transport == nil {
		return res, errs.Invalidf("publish: no transport")
	}
	pub, err := goose.NewPublisher(cfg.Transport, pc)
	if err != nil {
		return res, err
	}
	defer func() { _ = pub.Close() }()

	// Send the initial state here rather than leaving it to Run: a state
	// change published before the Run goroutine starts would otherwise
	// replace it, and stNum 1 would never carry the configured values.
	if pc.Values != nil {
		if err := pub.Publish(cloneValues(pc.Values)); err != nil {
			return res, fmt.Errorf("goose publish: %w", err)
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- pub.Run(runCtx) }()

	// wait blocks for d (forever when negative), returning early when ctx
	// is done or the publisher stops on its own (transport failure).
	wait := func(d time.Duration) error {
		var timeout <-chan time.Time
		if d >= 0 {
			t := time.NewTimer(d)
			defer t.Stop()
			timeout = t.C
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-runErr:
			if err == nil {
				err = goose.ErrClosed
			}
			return fmt.Errorf("goose publisher: %w", err)
		case <-timeout:
			return nil
		}
	}

	var stopErr error
	for i := 0; i < cfg.Changes && stopErr == nil; i++ {
		if stopErr = wait(cfg.Interval); stopErr != nil {
			break
		}
		if err := pub.Publish(cloneValues(pc.Values)); err != nil {
			stopErr = fmt.Errorf("goose publish: %w", err)
		}
	}
	if stopErr == nil {
		d := cfg.Duration
		if d <= 0 {
			d = -1
		}
		stopErr = wait(d)
	}
	cancel()
	st := pub.State()
	res.StNum, res.SqNum, res.Sent = st.StNum, st.SqNum, st.Sent
	cfg.Output.emit(func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "Published %s (appid %s -> %s, test=%t): stNum=%d sqNum=%d frames=%d\n",
			res.GocbRef, res.AppID, res.Dst, res.Test, res.StNum, res.SqNum, res.Sent)
	}, res)
	if errors.Is(stopErr, context.Canceled) || errors.Is(stopErr, context.DeadlineExceeded) {
		stopErr = nil // operator stop is the normal end of a publish run
	}
	return res, stopErr
}

func cloneValues(vs []*mms.Value) []*mms.Value {
	out := make([]*mms.Value, len(vs))
	for i, v := range vs {
		out[i] = v.Clone()
	}
	return out
}
