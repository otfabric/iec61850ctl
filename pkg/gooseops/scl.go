// SPDX-License-Identifier: MIT

// Package gooseops implements the iec61850ctl GOOSE commands on top of
// github.com/otfabric/go-iec61850/goose: listing control blocks from SCL,
// sniffing and subscribing to GOOSE streams, publishing, and rendering
// frames as text or JSON Lines.
package gooseops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-iec61850/scl"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// LoadControlBlocks parses an SCL file and resolves every GOOSE control
// block. Blocks that cannot be resolved are reported through the returned
// warning (nil when all resolved); a parse failure is an error.
func LoadControlBlocks(path string) (cbs []goose.ControlBlock, warning error, err error) {
	doc, err := scl.ParseFile(path)
	if err != nil {
		return nil, nil, errs.Mark(fmt.Errorf("parse SCL %s: %w", path, err), errs.ErrInvalidInput)
	}
	cbs, err = goose.ControlBlocksFromSCL(doc)
	if err != nil && len(cbs) == 0 && !errors.Is(err, goose.ErrSCL) {
		return nil, nil, err
	}
	return cbs, err, nil
}

// SelectControlBlock finds a control block by name ("gcb1"), LD and name
// ("LD0/gcb1"), IED, LD and name ("IED1/LD0/gcb1"), or its gocbRef in
// either the on-wire form ("IED1LD0/LLN0$GO$gcb1") or the bare-LD form
// ("LD0/LLN0$GO$gcb1"). An ambiguous name is an error.
func SelectControlBlock(cbs []goose.ControlBlock, sel string) (*goose.ControlBlock, error) {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return nil, errs.Invalidf("--gocb is required")
	}
	var found []*goose.ControlBlock
	for i := range cbs {
		cb := &cbs[i]
		if matchesSelector(cb, sel) {
			found = append(found, cb)
		}
	}
	switch len(found) {
	case 0:
		return nil, errs.Newf(errs.ErrNotFound, "GOOSE control block %q not found in SCL", sel)
	case 1:
		return found[0], nil
	}
	names := make([]string, len(found))
	for i, cb := range found {
		names[i] = cb.GocbRef
	}
	return nil, errs.Invalidf("GOOSE control block %q is ambiguous: %s", sel, strings.Join(names, ", "))
}

func matchesSelector(cb *goose.ControlBlock, sel string) bool {
	switch sel {
	case cb.Name,
		cb.LDInst + "/" + cb.Name,
		cb.IEDName + "/" + cb.LDInst + "/" + cb.Name,
		cb.GocbRef,
		BareRef(cb.GocbRef, cb.IEDName):
		return true
	}
	return false
}

// BareRef strips the IED name prefix from the MMS domain of an on-wire
// reference ("IED1LD0/LLN0$GO$gcb1" -> "LD0/LLN0$GO$gcb1"), the form used by
// servers whose MMS domain is the bare LD instance (go-iec61850,
// iec61850ctl server start).
func BareRef(ref, iedName string) string {
	if iedName == "" {
		return ref
	}
	domain, rest, ok := strings.Cut(ref, "/")
	if !ok || !strings.HasPrefix(domain, iedName) || len(domain) == len(iedName) {
		return ref
	}
	return strings.TrimPrefix(domain, iedName) + "/" + rest
}

// ControlBlockView is the list representation of a control block.
type ControlBlockView struct {
	IED       string   `json:"ied"`
	LD        string   `json:"ld"`
	Name      string   `json:"name"`
	GocbRef   string   `json:"gocb_ref"`
	DatSet    string   `json:"dat_set"`
	GoID      string   `json:"go_id"`
	ConfRev   uint32   `json:"conf_rev"`
	Address   bool     `json:"has_address"`
	Dst       string   `json:"dst,omitempty"`
	AppID     string   `json:"app_id,omitempty"`
	VLANID    *uint16  `json:"vlan_id,omitempty"`
	VLANPrio  *uint8   `json:"vlan_priority,omitempty"`
	MinTimeMs int64    `json:"min_time_ms,omitempty"`
	MaxTimeMs int64    `json:"max_time_ms,omitempty"`
	Members   []string `json:"members"`
}

// ViewControlBlock renders cb for listing.
func ViewControlBlock(cb *goose.ControlBlock) ControlBlockView {
	v := ControlBlockView{
		IED: cb.IEDName, LD: cb.LDInst, Name: cb.Name, GocbRef: cb.GocbRef,
		DatSet: cb.DatSet, GoID: cb.GoID, ConfRev: cb.ConfRev, Address: cb.HasAddress,
		Members: make([]string, 0, len(cb.Members)),
	}
	if cb.HasAddress {
		v.Dst = cb.Dst.String()
		v.AppID = FormatAppID(cb.AppID)
		v.MinTimeMs = cb.MinTime.Milliseconds()
		v.MaxTimeMs = cb.MaxTime.Milliseconds()
		if cb.VLAN != nil {
			id, prio := cb.VLAN.ID, cb.VLAN.Priority
			v.VLANID, v.VLANPrio = &id, &prio
		}
	}
	for _, m := range cb.Members {
		v.Members = append(v.Members, m.Ref+"["+m.FC+"]")
	}
	return v
}

// FormatAppID renders an APPID as 0xNNNN.
func FormatAppID(id uint16) string { return fmt.Sprintf("0x%04X", id) }

// ParseAppID accepts hex ("0x1001", "1001") as written in SCL and on the
// wire; a "d:" prefix forces decimal ("d:4097").
func ParseAppID(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	var n uint64
	var err error
	if d, ok := strings.CutPrefix(s, "d:"); ok {
		_, err = fmt.Sscanf(d, "%d", &n)
		if err == nil && fmt.Sprint(n) != d {
			err = errors.New("trailing characters")
		}
	} else {
		h := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
		_, err = fmt.Sscanf(h, "%x", &n)
		if err == nil && !isHex(h) {
			err = errors.New("not hexadecimal")
		}
	}
	if err != nil || n > 0xFFFF {
		return 0, errs.Invalidf("invalid APPID %q (hex 0x0000-0xFFFF, or d:<decimal>)", s)
	}
	return uint16(n), nil
}

func isHex(s string) bool {
	if s == "" || len(s) > 4 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}
