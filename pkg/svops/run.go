// SPDX-License-Identifier: MIT

package svops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/otfabric/go-iec61850/sv"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/gooseops"
)

// Filter selects ASDUs by destination, APPID and svID; zero fields match all.
type Filter struct {
	Dst    sv.MAC
	AppIDs []uint16
	SvID   string
}

func (f Filter) matchFrame(fr *sv.Frame) bool {
	if !f.Dst.IsZero() && fr.Dst != f.Dst {
		return false
	}
	return len(f.AppIDs) == 0 || slices.Contains(f.AppIDs, fr.AppID)
}

// SniffConfig configures Sniff.
type SniffConfig struct {
	Transport sv.Transport
	Filter    Filter
	// Layout decodes the samples into values; nil shows them as hex.
	Layout sv.Layout
	// SampleEvery prints every Nth matching ASDU (0 or 1 = all).
	SampleEvery int
	Count       int           // stop after this many matching ASDUs (0 = unlimited)
	Duration    time.Duration // stop after this long (0 = until ctx is done)
	Output      Output
	Now         func() time.Time // test seam; nil uses time.Now
}

// SniffResult summarises a sniff run.
type SniffResult struct {
	Event        string `json:"event"`
	Frames       int    `json:"frames"` // frames with at least one matching ASDU
	ASDUs        int    `json:"asdus"`  // matching ASDUs
	Printed      int    `json:"printed"`
	Filtered     int    `json:"filtered"` // frames without a matching ASDU
	DecodeErrors int    `json:"decode_errors"`
}

// Sniff decodes SV frames from the transport and prints the ASDUs that
// match the filter, without supervision. It returns an error matching
// errs.ErrTimeout when Count was set and the duration elapsed first.
func Sniff(ctx context.Context, cfg SniffConfig) (SniffResult, error) {
	res := SniffResult{Event: "summary"}
	if cfg.Transport == nil {
		return res, errs.Invalidf("sniff: no transport")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	every := max(cfg.SampleEvery, 1)
	ctx, cancel := withDuration(ctx, cfg.Duration)
	defer cancel()
	var fr sv.Frame
recv:
	for cfg.Count == 0 || res.ASDUs < cfg.Count {
		raw, err := cfg.Transport.Recv(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return res, fmt.Errorf("sv receive: %w", err)
		}
		rx := now()
		if err := sv.DecodeInto(&fr, raw, sv.DefaultLimits); err != nil {
			res.DecodeErrors++
			cfg.Output.warnf("undecodable frame (%d octets): %v", len(raw), err)
			continue
		}
		if !cfg.Filter.matchFrame(&fr) {
			res.Filtered++
			continue
		}
		matched := false
		for i := range fr.PDU.ASDUs {
			if cfg.Filter.SvID != "" && fr.PDU.ASDUs[i].SvID != cfg.Filter.SvID {
				continue
			}
			matched = true
			res.ASDUs++
			if (res.ASDUs-1)%every == 0 {
				res.Printed++
				v := ViewASDU("asdu", &fr, &fr.PDU.ASDUs[i], i, cfg.Layout, 0, rx)
				cfg.Output.emit(func(w io.Writer) { WriteASDUText(w, v) }, v)
			}
			if cfg.Count > 0 && res.ASDUs >= cfg.Count {
				res.Frames++
				break recv
			}
		}
		if matched {
			res.Frames++
		} else {
			res.Filtered++
		}
	}
	cfg.Output.emit(func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "ASDUs: %d in %d frame(s), printed %d (filtered %d frame(s), undecodable %d)\n",
			res.ASDUs, res.Frames, res.Printed, res.Filtered, res.DecodeErrors)
	}, res)
	return res, countErr(ctx, cfg.Count, res.ASDUs, "SV ASDU(s)")
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

// sampleQueue is the subscription's sample channel capacity: about half a
// second of a 9-2LE stream.
const sampleQueue = 2048

// SubscribeConfig configures Subscribe.
type SubscribeConfig struct {
	Transport    sv.Transport
	Subscription sv.SubscriptionConfig
	// AcceptSimulation accepts frames with the simulation bit set.
	AcceptSimulation bool
	// SampleEvery prints every Nth accepted ASDU (0 = none).
	SampleEvery int
	// StatsInterval prints the stream statistics this often (0 = only at
	// the end).
	StatsInterval time.Duration
	Count         int           // stop after this many accepted ASDUs (0 = unlimited)
	Duration      time.Duration // stop after this long (0 = until ctx is done)
	Output        Output
	Now           func() time.Time // test seam; nil uses time.Now
}

// Subscribe receives SV through a supervised subscription. It prints every
// supervision event (start, gap, timeout, ...), every SampleEvery-th ASDU
// and the stream statistics each StatsInterval, until Count ASDUs were
// accepted, the duration elapsed or ctx is done, then prints and returns
// the final statistics. An error matching errs.ErrTimeout means Count was
// not reached in time.
func Subscribe(ctx context.Context, cfg SubscribeConfig) (StatsView, error) {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	sum := StatsView{Event: "summary", Streams: []StreamView{}}
	if cfg.Transport == nil {
		return sum, errs.Invalidf("subscribe: no transport")
	}
	subscriber, err := sv.NewSubscriber(cfg.Transport, sv.SubscriberConfig{
		AcceptSimulation: cfg.AcceptSimulation,
		OnDecodeError: func(raw []byte, err error) {
			cfg.Output.warnf("undecodable frame (%d octets): %v", len(raw), err)
		},
	})
	if err != nil {
		return sum, err
	}
	defer func() { _ = subscriber.Close() }()
	sc := cfg.Subscription
	sc.OnASDU = nil
	sc.SampleQueue = sampleQueue
	sub, err := subscriber.Subscribe(sc)
	if err != nil {
		return sum, err
	}
	defer func() { _ = sub.Close() }()

	ctx, cancel := withDuration(ctx, cfg.Duration)
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- subscriber.Run(ctx) }()

	var tick <-chan time.Time
	if cfg.StatsInterval > 0 {
		t := time.NewTicker(cfg.StatsInterval)
		defer t.Stop()
		tick = t.C
	}
	stats := func(event string) StatsView {
		s := StatsView{Event: event, Time: now().UTC(), Streams: []StreamView{}}
		for _, st := range sub.Stats() {
			s.ASDUs += st.ASDUs
			s.Streams = append(s.Streams, ViewStats(st))
		}
		return s
	}
	accepted := 0
	// A stream that keeps failing validation reports every ASDU; print an
	// invalid event only when its error changes (Stats counts them all).
	lastInvalid := map[sv.StreamKey]string{}
	samples, events := sub.Samples(), sub.Events()
loop:
	for cfg.Count == 0 || accepted < cfg.Count {
		select {
		case <-ctx.Done():
			break loop
		case err := <-runErr:
			if ctx.Err() != nil {
				break loop
			}
			if err == nil {
				err = sv.ErrClosed
			}
			return sum, fmt.Errorf("sv receive: %w", err)
		case s, ok := <-samples:
			if !ok {
				break loop
			}
			accepted++
			if cfg.SampleEvery > 0 && (accepted-1)%cfg.SampleEvery == 0 {
				v := ViewASDU("asdu", s.Frame, s.ASDU, s.Index, sc.Layout, s.Lost, s.Received)
				cfg.Output.emit(func(w io.Writer) { WriteASDUText(w, v) }, v)
			}
		case ev, ok := <-events:
			if !ok {
				break loop
			}
			v := ViewEvent(ev)
			if ev.Kind == sv.EventInvalid {
				if lastInvalid[ev.Stream] == v.Error {
					continue
				}
				lastInvalid[ev.Stream] = v.Error
			} else {
				delete(lastInvalid, ev.Stream)
			}
			cfg.Output.emit(func(w io.Writer) { WriteEventText(w, v) }, v)
		case <-tick:
			s := stats("stats")
			cfg.Output.emit(func(w io.Writer) { WriteStatsTable(w, s) }, s)
		}
	}
	sum = stats("summary")
	cfg.Output.emit(func(w io.Writer) { WriteStatsTable(w, sum) }, sum)
	return sum, countErr(ctx, cfg.Count, accepted, "SV ASDU(s)")
}

// PublishConfig configures Publish.
type PublishConfig struct {
	Transport sv.Transport
	Publisher sv.PublisherConfig
	// Duration stops the publisher after this long (0 = until ctx is done).
	Duration time.Duration
	Output   Output
}

// PublishResult summarises a publish run.
type PublishResult struct {
	Event      string  `json:"event"`
	SvID       string  `json:"sv_id"`
	AppID      string  `json:"app_id"`
	Dst        string  `json:"dst"`
	Simulation bool    `json:"simulation"`
	Rate       float64 `json:"rate"` // configured samples per second
	NoASDU     int     `json:"no_asdu"`
	Samples    uint64  `json:"samples"`
	Frames     uint64  `json:"frames"`
	Skipped    uint64  `json:"skipped"`
	Late       uint64  `json:"late"`
	SendErrors uint64  `json:"send_errors"`
	NextSmpCnt uint16  `json:"next_smp_cnt"`
}

// Publish runs an SV publisher until Duration elapses or ctx is done and
// returns its counters.
func Publish(ctx context.Context, cfg PublishConfig) (PublishResult, error) {
	pc := cfg.Publisher
	res := PublishResult{Event: "summary", SvID: pc.SvID, AppID: gooseops.FormatAppID(pc.AppID), Dst: pc.Dst.String(),
		Simulation: pc.Simulation, NoASDU: max(pc.NoASDU, 1)}
	if cfg.Transport == nil {
		return res, errs.Invalidf("publish: no transport")
	}
	pub, err := sv.NewPublisher(cfg.Transport, pc)
	if err != nil {
		return res, err
	}
	defer func() { _ = pub.Close() }()
	res.Rate = pub.State().Rate

	ctx, cancel := withDuration(ctx, cfg.Duration)
	defer cancel()
	stopErr := pub.Run(ctx)
	if stopErr == nil && ctx.Err() == nil {
		stopErr = sv.ErrClosed
	}
	if stopErr != nil && ctx.Err() == nil {
		stopErr = fmt.Errorf("sv publisher: %w", stopErr)
	} else {
		stopErr = nil // operator stop or --duration is the normal end of a publish run
	}
	st := pub.State()
	res.Samples, res.Frames, res.Skipped, res.Late, res.SendErrors = st.Samples, st.Frames, st.Skipped, st.Late, st.SendErrors
	res.NextSmpCnt = st.NextSmpCnt
	cfg.Output.emit(func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "Published %s (appid %s -> %s, simulation=%t, %g samples/s, %d ASDU/frame): samples=%d frames=%d skipped=%d late=%d send-errors=%d\n",
			res.SvID, res.AppID, res.Dst, res.Simulation, res.Rate, res.NoASDU, res.Samples, res.Frames,
			res.Skipped, res.Late, res.SendErrors)
	}, res)
	return res, stopErr
}
