// SPDX-License-Identifier: MIT

package gooseops

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/formatter/value"
)

// FrameView is the JSON representation of a received GOOSE frame.
type FrameView struct {
	Event      string    `json:"event"`
	Received   time.Time `json:"received"`
	Src        string    `json:"src"`
	Dst        string    `json:"dst"`
	AppID      string    `json:"app_id"`
	VLANID     *uint16   `json:"vlan_id,omitempty"`
	VLANPrio   *uint8    `json:"vlan_priority,omitempty"`
	Simulation bool      `json:"simulation"`
	GocbRef    string    `json:"gocb_ref"`
	DatSet     string    `json:"dat_set"`
	GoID       string    `json:"go_id,omitempty"`
	T          time.Time `json:"t"`
	TTLMs      uint32    `json:"ttl_ms"`
	StNum      uint32    `json:"st_num"`
	SqNum      uint32    `json:"sq_num"`
	Test       bool      `json:"test"`
	ConfRev    uint32    `json:"conf_rev"`
	NdsCom     bool      `json:"nds_com"`
	Entries    uint32    `json:"entries"`
	Values     []any     `json:"values"`
}

// ViewFrame renders f, received at rx, as event kind ev.
func ViewFrame(ev string, f *goose.Frame, rx time.Time) FrameView {
	v := FrameView{
		Event: ev, Received: rx.UTC(), Src: f.Src.String(), Dst: f.Dst.String(),
		AppID: FormatAppID(f.AppID), Simulation: f.Simulation,
		GocbRef: f.PDU.GocbRef, DatSet: f.PDU.DatSet, GoID: f.PDU.GoID,
		T: f.PDU.T.UTC(), TTLMs: f.PDU.TimeAllowedToLive, StNum: f.PDU.StNum, SqNum: f.PDU.SqNum,
		Test: f.PDU.Test, ConfRev: f.PDU.ConfRev, NdsCom: f.PDU.NdsCom,
		Entries: f.PDU.NumDatSetEntries, Values: make([]any, len(f.PDU.AllData)),
	}
	if f.VLAN != nil {
		id, prio := f.VLAN.ID, f.VLAN.Priority
		v.VLANID, v.VLANPrio = &id, &prio
	}
	for i, d := range f.PDU.AllData {
		v.Values[i] = ValueJSON(d)
	}
	return v
}

// WriteFrameText writes one human-readable line (plus values) for f.
func WriteFrameText(w io.Writer, ev string, f *goose.Frame, rx time.Time) {
	flags := make([]string, 0, 3)
	if f.PDU.Test {
		flags = append(flags, "test")
	}
	if f.Simulation {
		flags = append(flags, "sim")
	}
	if f.PDU.NdsCom {
		flags = append(flags, "ndsCom")
	}
	vlan := ""
	if f.VLAN != nil {
		vlan = fmt.Sprintf(" vlan=%d/%d", f.VLAN.ID, f.VLAN.Priority)
	}
	fl := ""
	if len(flags) > 0 {
		fl = " [" + strings.Join(flags, ",") + "]"
	}
	_, _ = fmt.Fprintf(w, "%s %-9s %s -> %s appid=%s%s %s st=%d sq=%d ttl=%dms confRev=%d%s\n",
		rx.UTC().Format("15:04:05.000"), ev, f.Src, f.Dst, FormatAppID(f.AppID), vlan,
		f.PDU.GocbRef, f.PDU.StNum, f.PDU.SqNum, f.PDU.TimeAllowedToLive, f.PDU.ConfRev, fl)
	for i, d := range f.PDU.AllData {
		_, _ = fmt.Fprintf(w, "    [%d] %s\n", i, value.FormatMmsValue(d))
	}
}

// writeJSONL writes v as one JSON line.
func writeJSONL(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// ValueJSON converts an MMS value to a JSON-friendly form: booleans,
// numbers, strings, bit/octet strings as hex, times as RFC 3339, arrays and
// structures as lists. Non-finite floats become strings.
func ValueJSON(v *mms.Value) any {
	if v == nil {
		return nil
	}
	switch v.Type() {
	case mms.ValueTypeBoolean:
		b, _ := v.Bool()
		return b
	case mms.ValueTypeInteger:
		i, _ := v.Int64()
		return i
	case mms.ValueTypeUnsigned:
		u, _ := v.Uint64()
		return u
	case mms.ValueTypeFloat:
		f, _ := v.Float64()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Sprint(f)
		}
		return f
	case mms.ValueTypeVisibleString:
		s, _ := v.VisibleString()
		return s
	case mms.ValueTypeMmsString:
		s, _ := v.MmsString()
		return s
	case mms.ValueTypeBitString:
		b, _ := v.BitString()
		n, _ := v.BitStringLength()
		return map[string]any{"bits": hex.EncodeToString(b), "len": n}
	case mms.ValueTypeOctetString:
		b, _ := v.OctetString()
		return hex.EncodeToString(b)
	case mms.ValueTypeUTCTime:
		t, _ := v.UTCTime()
		return t.UTC().Format(time.RFC3339Nano)
	case mms.ValueTypeArray:
		els, _ := v.ArrayElements()
		return listJSON(els)
	case mms.ValueTypeStructure:
		els, _ := v.Structure()
		return listJSON(els)
	default:
		return v.String()
	}
}

func listJSON(els []*mms.Value) []any {
	out := make([]any, len(els))
	for i, e := range els {
		out[i] = ValueJSON(e)
	}
	return out
}
