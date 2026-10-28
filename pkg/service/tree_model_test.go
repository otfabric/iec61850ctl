// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// modelNode describes one node in a scripted device model for Tree tests.
type modelNode struct {
	children []string                      // child names (duplicates allowed to exercise de-duplication)
	typ      mms.ValueType                 // type returned by GetVariableType
	fc       iec61850.FunctionalConstraint // only FC for which GetVariableType succeeds ("" = never)
	value    *mms.Value                    // value returned by Read (nil => read error)
	listErr  bool
	anyFC    bool // GetVariableType succeeds for every FC (DO-level structures)                          // ListChildren fails
}

// modelConn is a mockConnection driven by a scripted node map keyed by
// "LD/LN.path" (FC ignored).
type modelConn struct {
	mockConnection
	nodes map[string]modelNode
	lnErr bool
	doErr bool
	ldErr bool
}

func modelKey(ref iec61850.Ref) string {
	return ref.LD + "/" + ref.LN + "." + strings.Join(ref.Path, ".")
}

func (m *modelConn) ListLogicalDevices(ctx context.Context) ([]iec61850.LogicalDevice, error) {
	if m.ldErr {
		return nil, errors.New("ld boom")
	}
	return m.mockConnection.ListLogicalDevices(ctx)
}

func (m *modelConn) ListLogicalNodes(ctx context.Context, ld string) ([]iec61850.LogicalNode, error) {
	if m.lnErr {
		return nil, errors.New("ln boom")
	}
	return m.mockConnection.ListLogicalNodes(ctx, ld)
}

func (m *modelConn) ListDataObjects(ctx context.Context, ld, ln string) ([]iec61850.DataObject, error) {
	if m.doErr {
		return nil, errors.New("do boom")
	}
	return m.mockConnection.ListDataObjects(ctx, ld, ln)
}

func (m *modelConn) ListChildren(_ context.Context, ref iec61850.Ref) ([]iec61850.BrowseNode, error) {
	n, ok := m.nodes[modelKey(ref)]
	if !ok || n.listErr {
		return nil, errors.New("no children")
	}
	out := make([]iec61850.BrowseNode, 0, len(n.children))
	for _, c := range n.children {
		path := append(append([]string(nil), ref.Path...), c)
		out = append(out, iec61850.BrowseNode{Name: c, Reference: iec61850.Ref{LD: ref.LD, LN: ref.LN, Path: path}})
	}
	return out, nil
}

func (m *modelConn) GetVariableType(_ context.Context, ref iec61850.Ref) (*mms.TypeSpec, error) {
	n, ok := m.nodes[modelKey(ref)]
	if ok && n.anyFC {
		return &mms.TypeSpec{Type: n.typ}, nil
	}
	if !ok || n.fc == "" || ref.FC != n.fc {
		return nil, errors.New("no such type")
	}
	spec := &mms.TypeSpec{Type: n.typ}
	if n.typ == mms.ValueTypeArray {
		spec.Count = 2
	}
	return spec, nil
}

func (m *modelConn) Read(_ context.Context, ref iec61850.Ref) (*iec61850.Value, error) {
	n, ok := m.nodes[modelKey(ref)]
	if !ok || n.value == nil {
		return nil, errors.New("read denied")
	}
	return iec61850.NewValue(n.value), nil
}

// newTreeModel returns a model for LD0/MMXU1.TotW with nested structures,
// a self-referencing child, duplicate children, leaves of varying types,
// an attribute without a resolvable FC, and failing reads.
func newTreeModel() *modelConn {
	return &modelConn{
		mockConnection: mockConnection{
			logicalDevices: []string{"LD0"},
			logicalNodes:   []string{"MMXU1"},
			dataObjects:    []string{"TotW", "TotW"},
		},
		nodes: map[string]modelNode{
			"LD0/MMXU1.TotW": {
				children: []string{"mag", "q", "t", "q", "TotW", "noFC", "denied", "arr", "badList"},
				typ:      mms.ValueTypeStructure, anyFC: true,
			},
			"LD0/MMXU1.TotW.mag": {
				children: []string{"f", "i", "inner", "lost", "bad"},
				typ:      mms.ValueTypeStructure, fc: iec61850.FCMX,
			},
			"LD0/MMXU1.TotW.mag.f":     {typ: mms.ValueTypeFloat, fc: iec61850.FCMX, value: mms.NewFloat(12.5)},
			"LD0/MMXU1.TotW.mag.i":     {typ: mms.ValueTypeInteger, fc: iec61850.FCMX, value: mms.NewInteger(7)},
			"LD0/MMXU1.TotW.mag.inner": {children: []string{"u"}, typ: mms.ValueTypeStructure, fc: iec61850.FCMX},
			"LD0/MMXU1.TotW.mag.inner.u": {
				typ: mms.ValueTypeUnsigned, fc: iec61850.FCMX, value: mms.NewUnsigned(3),
			},
			"LD0/MMXU1.TotW.mag.lost": {},                                             // no FC
			"LD0/MMXU1.TotW.mag.bad":  {typ: mms.ValueTypeBoolean, fc: iec61850.FCMX}, // read fails
			"LD0/MMXU1.TotW.q": {
				typ: mms.ValueTypeBitString, fc: iec61850.FCMX,
				value: mms.NewBitStringWithLength([]byte{0x00, 0x00}, 13),
			},
			"LD0/MMXU1.TotW.t": {
				typ: mms.ValueTypeUTCTime, fc: iec61850.FCMX,
				value: mms.NewUTCTime(time.Unix(1_700_000_000, 0).UTC()),
			},
			"LD0/MMXU1.TotW.noFC":   {value: mms.NewVisibleString("orphan")},
			"LD0/MMXU1.TotW.denied": {typ: mms.ValueTypeVisibleString, fc: iec61850.FCDC},
			"LD0/MMXU1.TotW.arr": {
				children: []string{"0"}, typ: mms.ValueTypeArray, fc: iec61850.FCCF,
			},
			"LD0/MMXU1.TotW.arr.0":   {typ: mms.ValueTypeOctetString, fc: iec61850.FCCF, value: mms.NewOctetString([]byte{1, 2})},
			"LD0/MMXU1.TotW.badList": {typ: mms.ValueTypeStructure, fc: iec61850.FCST, listErr: true},
		},
	}
}

func TestTree_RenderDeviceTree_Model(t *testing.T) {
	t.Parallel()
	m := newTreeModel()
	var buf bytes.Buffer
	tr := NewTree(m).WithCallInterval(0)
	tr.SetCallInterval(time.Nanosecond)
	calls, err := tr.RenderDeviceTree(&buf, "h", 102, "")
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if calls == 0 {
		t.Fatal("expected calls")
	}
	for _, want := range []string{
		"📦 LD0", "📋 MMXU1", "📊 TotW",
		"[MX] mag", "[MX] f → FLOAT", "12.5",
		"[MX] i → INT", "[MX] inner", "[MX] u → ", "= 3",
		"[MX] bad → BOOL = ?",
		"[CF] arr", "OCTET",
		"[?] noFC", "= orphan",
		"[DC] denied",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestTree_RenderDeviceTree_ListErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mut     func(m *modelConn)
		path    string
		wantErr bool
		want    string
	}{
		{name: "ld error", mut: func(m *modelConn) { m.ldErr = true }, wantErr: true},
		{name: "ln error", mut: func(m *modelConn) { m.lnErr = true }, want: "Error listing LNs"},
		{name: "do error", mut: func(m *modelConn) { m.doErr = true }, want: "Error listing DOs"},
		{name: "da error", path: "LD0/MMXU1.Missing", want: "Error listing DAs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newTreeModel()
			if tt.mut != nil {
				tt.mut(m)
			}
			var buf bytes.Buffer
			_, err := NewTree(m).RenderDeviceTree(&buf, "h", 1, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), tt.want) {
				t.Fatalf("missing %q in\n%s", tt.want, buf.String())
			}
		})
	}
}

func TestTree_RenderDeviceTreeFlat_Model(t *testing.T) {
	t.Parallel()
	m := newTreeModel()
	var buf bytes.Buffer
	if _, err := NewTree(m).RenderDeviceTreeFlat(&buf, "h", 1, ""); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"[MX]: 12.5 [type:FLOAT]",
		"[type:UNKNOWN]",
		"orphan",
		"[CF]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("flat output missing %q\n%s", want, out)
		}
	}

	for _, mut := range []func(*modelConn){
		func(m *modelConn) { m.ldErr = true },
		func(m *modelConn) { m.lnErr = true },
		func(m *modelConn) { m.doErr = true },
	} {
		m := newTreeModel()
		mut(m)
		var b bytes.Buffer
		_, err := NewTree(m).RenderDeviceTreeFlat(&b, "h", 1, "")
		if m.ldErr && err == nil {
			t.Fatal("expected LD error")
		}
		if !m.ldErr && (err != nil || b.Len() != 0) {
			t.Fatalf("err=%v out=%q", err, b.String())
		}
	}
}

func TestTree_BuildSerializableModel_Model(t *testing.T) {
	t.Parallel()
	m := newTreeModel()
	ied, err := NewTree(m).BuildSerializableModel("h", 102, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ied.LogicalDevices) != 1 {
		t.Fatalf("ied=%+v", ied)
	}
	dos := ied.LogicalDevices[0].LogicalNodes[0].DataObjects
	if len(dos) != 1 {
		t.Fatalf("expected deduped DOs, got %d", len(dos))
	}
	byName := map[string]domain.DataAttribute{}
	for _, a := range dos[0].Attributes {
		byName[a.Name] = a
	}
	if byName["mag"].Type != domain.TypeStructure || len(byName["mag"].Children) == 0 {
		t.Fatalf("mag = %+v", byName["mag"])
	}
	if byName["noFC"].ValueError == "" || byName["noFC"].Type != domain.TypeUnknown {
		t.Fatalf("noFC = %+v", byName["noFC"])
	}
	if byName["denied"].ValueError == "" {
		t.Fatalf("denied = %+v", byName["denied"])
	}
	if byName["q"].Value == nil || byName["t"].Value == nil {
		t.Fatalf("q=%+v t=%+v", byName["q"], byName["t"])
	}
	if len(ied.Leaves) == 0 {
		t.Fatal("expected leaves")
	}

	for _, mut := range []func(*modelConn){
		func(m *modelConn) { m.ldErr = true },
		func(m *modelConn) { m.lnErr = true },
		func(m *modelConn) { m.doErr = true },
	} {
		m := newTreeModel()
		mut(m)
		ied, err := NewTree(m).BuildSerializableModel("h", 1, "", true, true)
		if m.ldErr {
			if err == nil {
				t.Fatal("expected LD error")
			}
			continue
		}
		if err != nil || ied == nil {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestTree_ResolveVariableType_ExplicitFC(t *testing.T) {
	t.Parallel()
	m := newTreeModel()
	tr := NewTree(m)
	ref := iec61850.Ref{LD: "LD0", LN: "MMXU1", Path: []string{"TotW", "mag", "f"}, FC: iec61850.FCMX}
	fc, spec, err := tr.resolveVariableType(ref)
	if err != nil || fc != iec61850.FCMX || spec.Type != mms.ValueTypeFloat {
		t.Fatalf("fc=%v spec=%v err=%v", fc, spec, err)
	}
	// Wrong explicit FC falls back to probing.
	ref.FC = iec61850.FCST
	fc, _, err = tr.resolveVariableType(ref)
	if err != nil || fc != iec61850.FCMX {
		t.Fatalf("fallback fc=%v err=%v", fc, err)
	}
	ref.Path = []string{"TotW", "noFC"}
	if _, _, err := tr.resolveVariableType(ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v, want ErrNotFound", err)
	}
}

func TestTree_RenderLeafAttribute_ReadError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tr := NewTree(newTreeModel())
	tr.renderLeafAttribute(&buf, iec61850.Ref{LD: "LD0", LN: "MMXU1", Path: []string{"nope"}}, "", "", true)
	if buf.Len() != 0 {
		t.Fatalf("unexpected output %q", buf.String())
	}
	tr.renderLeafAttribute(&buf, iec61850.Ref{LD: "LD0", LN: "MMXU1", Path: []string{"TotW", "noFC"}}, "", "", false)
	if !strings.Contains(buf.String(), "│  └─ = orphan") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestFormatValueDisplay_Bool(t *testing.T) {
	t.Parallel()
	if formatValueDisplay(nil) != "?" {
		t.Fatal("nil")
	}
	if got := formatValueDisplay(iec61850.NewValue(mms.NewBoolean(true))); got != "true" {
		t.Fatalf("got %q", got)
	}
}

// Regression: the flat renderer printed the FC twice ("…stVal[ST][ST]:") and
// UTC_TIME values as a raw Go struct ("{1700000000 0 0}").
func TestTree_RenderDeviceTreeFlat_Format(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if _, err := NewTree(newTreeModel()).RenderDeviceTreeFlat(&buf, "h", 1, ""); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"LD0/MMXU1.TotW.mag.f[MX]: 12.5 [type:FLOAT]\n",
		"LD0/MMXU1.TotW.t[MX]: 2023-11-14 22:13:20.000 UTC",
		"LD0/MMXU1.TotW.noFC: orphan [type:UNKNOWN]\n",
		"LD0/MMXU1.TotW.arr.0[CF]: 0102 [type:OCTET_STRING]\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	for _, bad := range []string{"][", "{1700000000"} {
		if strings.Contains(out, bad) {
			t.Errorf("unexpected %q in\n%s", bad, out)
		}
	}
}

func TestTree_RenderDeviceTree_UTCTimeFormatted(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if _, err := NewTree(newTreeModel()).RenderDeviceTree(&buf, "h", 1, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "UTC_TIME = 2023-11-14 22:13:20.000 UTC") || strings.Contains(buf.String(), "{1700000000") {
		t.Fatalf("UTC_TIME not formatted:\n%s", buf.String())
	}
}
