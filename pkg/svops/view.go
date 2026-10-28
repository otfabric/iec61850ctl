// SPDX-License-Identifier: MIT

package svops

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/otfabric/go-iec61850/sv"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/formatter/value"
	"github.com/otfabric/iec61850ctl/pkg/gooseops"
)

// Output selects the rendering of streamed results.
type Output struct {
	JSONL bool      // one JSON object per line instead of text
	Out   io.Writer // results
	Err   io.Writer // diagnostics
}

func (o Output) emit(text func(io.Writer), obj any) {
	if o.JSONL {
		b, err := json.Marshal(obj)
		if err == nil {
			_, _ = o.Out.Write(append(b, '\n'))
		}
		return
	}
	text(o.Out)
}

func (o Output) warnf(format string, args ...any) {
	if o.Err != nil {
		_, _ = fmt.Fprintf(o.Err, "warning: "+format+"\n", args...)
	}
}

// ASDUView is the JSON representation of one received ASDU.
type ASDUView struct {
	Event      string     `json:"event"`
	Received   time.Time  `json:"received"`
	Src        string     `json:"src"`
	Dst        string     `json:"dst"`
	AppID      string     `json:"app_id"`
	VLANID     *uint16    `json:"vlan_id,omitempty"`
	VLANPrio   *uint8     `json:"vlan_priority,omitempty"`
	Simulation bool       `json:"simulation"`
	NoASDU     int        `json:"no_asdu"`
	Index      int        `json:"asdu_index"`
	SvID       string     `json:"sv_id"`
	DatSet     string     `json:"dat_set,omitempty"`
	SmpCnt     uint16     `json:"smp_cnt"`
	ConfRev    uint32     `json:"conf_rev"`
	RefrTm     *time.Time `json:"refr_tm,omitempty"`
	SmpSynch   uint8      `json:"smp_synch"`
	SmpRate    *uint16    `json:"smp_rate,omitempty"`
	SmpMod     string     `json:"smp_mod,omitempty"`
	Lost       int        `json:"lost,omitempty"`
	Values     []any      `json:"values,omitempty"`
	Sample     string     `json:"sample,omitempty"` // hex, when no layout is known
	LE         *LEView    `json:"le,omitempty"`

	vals []*mms.Value // decoded values, for text rendering
}

// LEView is a 9-2LE sample in physical units.
type LEView struct {
	Amps  [4]float64 `json:"amps"`  // IA, IB, IC, IN
	Volts [4]float64 `json:"volts"` // VA, VB, VC, VN
	Good  bool       `json:"good"`  // every channel has good quality
}

// ViewASDU renders ASDU a, at position i of frame f, received at rx. With a
// layout the sample is decoded into values (and, for 9-2LE, physical
// units); without one it is shown as hex. f.PDU.ASDUs may be empty (the
// subscriber's owned copies drop it); the count comes from f.PDU.NoASDU.
func ViewASDU(ev string, f *sv.Frame, a *sv.ASDU, i int, layout sv.Layout, lost int, rx time.Time) ASDUView {
	n := f.PDU.NoASDU
	if n == 0 {
		n = len(f.PDU.ASDUs)
	}
	v := ASDUView{
		Event: ev, Received: rx.UTC(), Src: f.Src.String(), Dst: f.Dst.String(),
		AppID: gooseops.FormatAppID(f.AppID), Simulation: f.Simulation, NoASDU: n, Index: i,
		SvID: a.SvID, DatSet: a.DatSet, SmpCnt: a.SmpCnt, ConfRev: a.ConfRev, SmpSynch: a.SmpSynch, Lost: lost,
	}
	if f.VLAN != nil {
		id, prio := f.VLAN.ID, f.VLAN.Priority
		v.VLANID, v.VLANPrio = &id, &prio
	}
	if a.HasRefrTm {
		t := a.RefrTm.UTC()
		v.RefrTm = &t
	}
	if a.HasSmpRate {
		r := a.SmpRate
		v.SmpRate = &r
	}
	if a.HasSmpMod {
		v.SmpMod = a.SmpMod.String()
	}
	vals, err := decodeSample(layout, a.Sample)
	if err != nil {
		v.Sample = hex.EncodeToString(a.Sample)
		return v
	}
	v.vals = vals
	v.Values = make([]any, len(vals))
	for j, x := range vals {
		v.Values[j] = gooseops.ValueJSON(x)
	}
	if IsLE(layout) {
		var le sv.LESample
		if sv.DecodeLE(a.Sample, &le) == nil {
			lv := &LEView{Good: true}
			for c := 0; c < 4; c++ {
				lv.Amps[c] = sv.Amps(le.Current[c].Value)
				lv.Volts[c] = sv.Volts(le.Voltage[c].Value)
				lv.Good = lv.Good && le.Current[c].Q.IsGood() && le.Voltage[c].Q.IsGood()
			}
			v.LE = lv
		}
	}
	return v
}

func decodeSample(layout sv.Layout, sample []byte) ([]*mms.Value, error) {
	if len(layout) == 0 {
		return nil, fmt.Errorf("no layout")
	}
	return layout.Decode(sample)
}

// WriteASDUText writes one human-readable line (plus decoded values) for v.
func WriteASDUText(w io.Writer, v ASDUView) {
	var flags []string
	if v.Simulation {
		flags = append(flags, "sim")
	}
	if v.Lost > 0 {
		flags = append(flags, fmt.Sprintf("lost=%d", v.Lost))
	}
	fl := ""
	if len(flags) > 0 {
		fl = " [" + strings.Join(flags, ",") + "]"
	}
	vlan := ""
	if v.VLANID != nil {
		vlan = fmt.Sprintf(" vlan=%d/%d", *v.VLANID, *v.VLANPrio)
	}
	_, _ = fmt.Fprintf(w, "%s %-5s %s -> %s appid=%s%s svID=%s smpCnt=%d confRev=%d synch=%d asdu=%d/%d%s\n",
		v.Received.Format("15:04:05.000"), v.Event, v.Src, v.Dst, v.AppID, vlan, v.SvID, v.SmpCnt, v.ConfRev,
		v.SmpSynch, v.Index+1, v.NoASDU, fl)
	switch {
	case v.LE != nil:
		_, _ = fmt.Fprintf(w, "    I [A]: %9.3f %9.3f %9.3f %9.3f   V [V]: %9.2f %9.2f %9.2f %9.2f  quality=%s\n",
			v.LE.Amps[0], v.LE.Amps[1], v.LE.Amps[2], v.LE.Amps[3],
			v.LE.Volts[0], v.LE.Volts[1], v.LE.Volts[2], v.LE.Volts[3], goodText(v.LE.Good))
	case v.vals != nil:
		parts := make([]string, len(v.vals))
		for i, x := range v.vals {
			parts[i] = fmt.Sprintf("[%d] %s", i, value.FormatMmsValue(x))
		}
		_, _ = fmt.Fprintf(w, "    %s\n", strings.Join(parts, "  "))
	default:
		_, _ = fmt.Fprintf(w, "    sample %s\n", v.Sample)
	}
}

func goodText(good bool) string {
	if good {
		return "good"
	}
	return "not good"
}

// StreamView is the JSON form of one stream's statistics.
type StreamView struct {
	Src        string  `json:"src"`
	AppID      string  `json:"app_id"`
	SvID       string  `json:"sv_id"`
	Status     string  `json:"status"`
	Frames     uint64  `json:"frames"`
	ASDUs      uint64  `json:"asdus"`
	Rate       float64 `json:"rate"`
	Lost       uint64  `json:"lost"`
	Duplicates uint64  `json:"duplicates"`
	OutOfOrder uint64  `json:"out_of_order"`
	Invalid    uint64  `json:"invalid"`
	Timeouts   uint64  `json:"timeouts"`
	QueueDrops uint64  `json:"queue_drops"`
	LastSmpCnt uint16  `json:"last_smp_cnt"`
	SmpCntWrap int     `json:"smp_cnt_wrap"`
	SmpSynch   uint8   `json:"smp_synch"`
	ConfRev    uint32  `json:"conf_rev"`
	Simulation bool    `json:"simulation"`
}

// ViewStats renders one stream's statistics.
func ViewStats(s sv.Stats) StreamView {
	return StreamView{
		Src: s.Stream.Src.String(), AppID: gooseops.FormatAppID(s.Stream.AppID), SvID: s.Stream.SvID,
		Status: s.Status.String(), Frames: s.Frames, ASDUs: s.ASDUs, Rate: roundRate(s.Rate), Lost: s.Lost,
		Duplicates: s.Duplicates, OutOfOrder: s.OutOfOrder, Invalid: s.Invalid, Timeouts: s.Timeouts,
		QueueDrops: s.QueueDrops, LastSmpCnt: s.LastSmpCnt, SmpCntWrap: s.SmpCntWrap, SmpSynch: s.SmpSynch,
		ConfRev: s.ConfRev, Simulation: s.Simulation,
	}
}

func roundRate(r float64) float64 { return float64(int64(r*10+0.5)) / 10 }

// StatsView is a periodic or final statistics record.
type StatsView struct {
	Event   string       `json:"event"` // "stats" or "summary"
	Time    time.Time    `json:"time"`
	ASDUs   uint64       `json:"asdus"` // accepted ASDUs, all streams
	Streams []StreamView `json:"streams"`
}

// WriteStatsTable writes the statistics as a table.
func WriteStatsTable(w io.Writer, s StatsView) {
	_, _ = fmt.Fprintf(w, "%s %s: %d ASDU(s)\n", s.Time.Format("15:04:05"), s.Event, s.ASDUs)
	if len(s.Streams) == 0 {
		_, _ = fmt.Fprintln(w, "  (no stream)")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  SVID\tAPPID\tSRC\tSTATUS\tRATE/s\tASDUS\tLOST\tDUP\tOOO\tINVALID\tTIMEOUTS\tSMPCNT\tSYNCH\tCONFREV")
	for _, v := range s.Streams {
		sim := ""
		if v.Simulation {
			sim = " (sim)"
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s%s\t%.1f\t%d\t%d\t%d\t%d\t%d\t%d\t%d/%d\t%d\t%d\n",
			v.SvID, v.AppID, v.Src, v.Status, sim, v.Rate, v.ASDUs, v.Lost, v.Duplicates, v.OutOfOrder,
			v.Invalid, v.Timeouts, v.LastSmpCnt, v.SmpCntWrap, v.SmpSynch, v.ConfRev)
	}
	_ = tw.Flush()
}

// EventView is the JSON form of a subscription event.
type EventView struct {
	Event    string    `json:"event"`
	Time     time.Time `json:"time"`
	Src      string    `json:"src"`
	AppID    string    `json:"app_id"`
	SvID     string    `json:"sv_id"`
	SmpCnt   *uint16   `json:"smp_cnt,omitempty"`
	Lost     int       `json:"lost,omitempty"`
	SmpSynch uint8     `json:"smp_synch"`
	ConfRev  uint32    `json:"conf_rev"`
	Error    string    `json:"error,omitempty"`
}

// ViewEvent renders a subscription event.
func ViewEvent(ev sv.Event) EventView {
	v := EventView{
		Event: ev.Kind.String(), Time: ev.Time.UTC(), Src: ev.Stream.Src.String(),
		AppID: gooseops.FormatAppID(ev.Stream.AppID), SvID: ev.Stream.SvID, Lost: ev.Lost,
		SmpSynch: ev.SmpSynch, ConfRev: ev.ConfRev,
	}
	if ev.Kind != sv.EventTimeout {
		c := ev.SmpCnt
		v.SmpCnt = &c
	}
	if ev.Err != nil {
		v.Error = ev.Err.Error()
	}
	return v
}

// WriteEventText writes one line for a subscription event.
func WriteEventText(w io.Writer, v EventView) {
	line := fmt.Sprintf("%s %-11s %s %s %s", v.Time.Format("15:04:05.000"), v.Event, v.SvID, v.AppID, v.Src)
	if v.SmpCnt != nil {
		line += fmt.Sprintf(" smpCnt=%d", *v.SmpCnt)
	}
	if v.Lost > 0 {
		line += fmt.Sprintf(" lost=%d", v.Lost)
	}
	switch v.Event {
	case "synch":
		line += fmt.Sprintf(" smpSynch=%d", v.SmpSynch)
	case "confrev":
		line += fmt.Sprintf(" confRev=%d", v.ConfRev)
	}
	if v.Error != "" {
		line += " error=" + v.Error
	}
	_, _ = fmt.Fprintln(w, line)
}
