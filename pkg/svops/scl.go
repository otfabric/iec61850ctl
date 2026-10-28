// SPDX-License-Identifier: MIT

// Package svops implements the iec61850ctl Sampled Values commands on top
// of github.com/otfabric/go-iec61850/sv: listing control blocks from SCL,
// sniffing and subscribing to SV streams, publishing (including 9-2LE
// waveform injection), and rendering ASDUs and stream statistics as text
// or JSON Lines.
package svops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/otfabric/go-iec61850/scl"
	"github.com/otfabric/go-iec61850/sv"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/gooseops"
)

// LoadControlBlocks parses an SCL file and resolves every
// SampledValueControl block. Blocks that cannot be resolved are reported
// through the returned warning (nil when all resolved); a parse failure is
// an error.
func LoadControlBlocks(path string) (cbs []sv.ControlBlock, warning error, err error) {
	doc, err := scl.ParseFile(path)
	if err != nil {
		return nil, nil, errs.Mark(fmt.Errorf("parse SCL %s: %w", path, err), errs.ErrInvalidInput)
	}
	cbs, err = sv.ControlBlocksFromSCL(doc)
	if err != nil && len(cbs) == 0 && !errors.Is(err, sv.ErrSCL) {
		return nil, nil, err
	}
	return cbs, err, nil
}

// SelectControlBlock finds a control block by name ("MSVCB01"), LD and
// name ("LD0/MSVCB01"), IED, LD and name ("IED1/LD0/MSVCB01"), or its
// reference in either the on-wire form ("IED1LD0/LLN0$MS$MSVCB01") or the
// bare-LD form ("LD0/LLN0$MS$MSVCB01"). An ambiguous name is an error.
func SelectControlBlock(cbs []sv.ControlBlock, sel string) (*sv.ControlBlock, error) {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return nil, errs.Invalidf("--svcb is required")
	}
	var found []*sv.ControlBlock
	for i := range cbs {
		cb := &cbs[i]
		switch sel {
		case cb.Name, cb.LDInst + "/" + cb.Name, cb.IEDName + "/" + cb.LDInst + "/" + cb.Name,
			cb.SvCBRef, gooseops.BareRef(cb.SvCBRef, cb.IEDName):
			found = append(found, cb)
		}
	}
	switch len(found) {
	case 0:
		return nil, errs.Newf(errs.ErrNotFound, "SV control block %q not found in SCL", sel)
	case 1:
		return found[0], nil
	}
	names := make([]string, len(found))
	for i, cb := range found {
		names[i] = cb.SvCBRef
	}
	return nil, errs.Invalidf("SV control block %q is ambiguous: %s", sel, strings.Join(names, ", "))
}

// IsLE reports whether layout is the UCA 9-2LE dataset (four currents and
// four voltages, each INT32 + Quality).
func IsLE(layout sv.Layout) bool {
	return layout.String() == sv.LELayout.String()
}

// LEChannels are the 9-2LE channel names in dataset order.
var LEChannels = [8]string{"IA", "IB", "IC", "IN", "VA", "VB", "VC", "VN"}

// ControlBlockView is the list representation of a control block.
type ControlBlockView struct {
	IED          string   `json:"ied"`
	LD           string   `json:"ld"`
	Name         string   `json:"name"`
	SvCBRef      string   `json:"svcb_ref"`
	SvID         string   `json:"sv_id"`
	DatSet       string   `json:"dat_set"`
	ConfRev      uint32   `json:"conf_rev"`
	SmpRate      uint16   `json:"smp_rate"`
	SmpMod       string   `json:"smp_mod"`
	NoASDU       int      `json:"no_asdu"`
	OptFlds      []string `json:"opt_flds"`
	Address      bool     `json:"has_address"`
	Dst          string   `json:"dst,omitempty"`
	AppID        string   `json:"app_id,omitempty"`
	VLANID       *uint16  `json:"vlan_id,omitempty"`
	VLANPrio     *uint8   `json:"vlan_priority,omitempty"`
	Profile      string   `json:"profile"`
	Layout       string   `json:"layout"`
	SampleOctets int      `json:"sample_octets"`
	Members      []string `json:"members"`
}

// ViewControlBlock renders cb for listing.
func ViewControlBlock(cb *sv.ControlBlock) ControlBlockView {
	v := ControlBlockView{
		IED: cb.IEDName, LD: cb.LDInst, Name: cb.Name, SvCBRef: cb.SvCBRef, SvID: cb.SvID,
		DatSet: cb.DatSet, ConfRev: cb.ConfRev, SmpRate: cb.SmpRate, SmpMod: cb.SmpMod.String(),
		NoASDU: cb.NoASDU, OptFlds: OptFldsNames(cb.Opts), Address: cb.HasAddress,
		Profile: "generic", Layout: cb.Layout.String(), SampleOctets: cb.Layout.Size(),
		Members: make([]string, 0, len(cb.Members)),
	}
	if IsLE(cb.Layout) {
		v.Profile = "9-2LE"
	}
	if cb.HasAddress {
		v.Dst = cb.Dst.String()
		v.AppID = gooseops.FormatAppID(cb.AppID)
		if cb.VLAN != nil {
			id, prio := cb.VLAN.ID, cb.VLAN.Priority
			v.VLANID, v.VLANPrio = &id, &prio
		}
	}
	for _, m := range cb.Members {
		kinds := make([]string, len(m.Kinds))
		for i, k := range m.Kinds {
			kinds[i] = k.String()
		}
		v.Members = append(v.Members, fmt.Sprintf("%s[%s] %s", m.Ref, m.FC, strings.Join(kinds, ",")))
	}
	return v
}

// optFldNames maps the OptFlds names used on the command line.
var optFldNames = []struct {
	name string
	get  func(*sv.OptFlds) *bool
}{
	{"refresh-time", func(o *sv.OptFlds) *bool { return &o.RefreshTime }},
	{"sample-synchronized", func(o *sv.OptFlds) *bool { return &o.SampleSynchronized }},
	{"sample-rate", func(o *sv.OptFlds) *bool { return &o.SampleRate }},
	{"data-set", func(o *sv.OptFlds) *bool { return &o.DataSet }},
	{"security", func(o *sv.OptFlds) *bool { return &o.Security }},
}

// OptFldsNames lists the set optional fields.
func OptFldsNames(o sv.OptFlds) []string {
	out := []string{}
	for _, f := range optFldNames {
		if *f.get(&o) {
			out = append(out, f.name)
		}
	}
	return out
}

// ParseOptFlds parses a comma-separated list of optional field names
// (refresh-time, sample-synchronized, sample-rate, data-set, security);
// "none" or an empty string clears all.
func ParseOptFlds(s string) (sv.OptFlds, error) {
	var o sv.OptFlds
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(part)
		if name == "" || name == "none" {
			continue
		}
		ok := false
		for _, f := range optFldNames {
			if f.name == name {
				*f.get(&o), ok = true, true
			}
		}
		if !ok {
			return o, errs.Invalidf("unknown OptFlds field %q (refresh-time, sample-synchronized, sample-rate, data-set, security)", name)
		}
	}
	return o, nil
}

// ParseSmpMod accepts the SCL names (SmpPerPeriod, SmpPerSec, SecPerSmp),
// the iec61850-interop names (samples-per-period, samples-per-second,
// seconds-per-sample) and 0-2.
func ParseSmpMod(s string) (sv.SmpMod, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "smpperperiod", "samples-per-period", "0":
		return sv.SmpPerPeriod, nil
	case "smppersec", "samples-per-second", "1":
		return sv.SmpPerSec, nil
	case "secpersmp", "seconds-per-sample", "2":
		return sv.SecPerSmp, nil
	}
	return 0, errs.Invalidf("invalid smpMod %q (SmpPerPeriod, SmpPerSec, SecPerSmp)", s)
}

// layoutKinds are the sample encodings ParseLayout accepts.
var layoutKinds = []sv.Kind{
	sv.KindBoolean, sv.KindInt8, sv.KindInt16, sv.KindInt32, sv.KindInt64, sv.KindUint8, sv.KindUint16,
	sv.KindUint24, sv.KindUint32, sv.KindFloat32, sv.KindFloat64, sv.KindEnum, sv.KindDbpos,
	sv.KindQuality, sv.KindTimestamp,
}

// ParseLayout parses a sample layout: "9-2LE" for the UCA 9-2LE dataset or
// a comma-separated list of leaf encodings as printed by sv list
// ("FLOAT32,Quality,INT32U").
func ParseLayout(s string) (sv.Layout, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "9-2LE") || strings.EqualFold(s, "LE") {
		return append(sv.Layout(nil), sv.LELayout...), nil
	}
	var names []string
	for _, k := range layoutKinds {
		names = append(names, k.String())
	}
	var out sv.Layout
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		found := false
		for _, k := range layoutKinds {
			if strings.EqualFold(part, k.String()) {
				out, found = append(out, k), true
				break
			}
		}
		if !found {
			return nil, errs.Invalidf("--layout: unknown encoding %q (9-2LE or a list of %s)", part, strings.Join(names, ", "))
		}
	}
	return out, nil
}
