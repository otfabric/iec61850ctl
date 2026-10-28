// SPDX-License-Identifier: MIT

package scl

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestFlatten_Rich checks every entry produced from testdata/rich.cid, which
// exercises structs, SDOs, SDI values, enums, missing and recursive types.
func TestFlatten_Rich(t *testing.T) {
	t.Parallel()
	flat, err := loadTestdata(t, "rich.cid").Flatten()
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]FlattenEntry, len(flat))
	for _, e := range flat {
		if _, dup := got[e.Path]; dup {
			t.Errorf("duplicate path %s", e.Path)
		}
		got[e.Path] = e
	}

	const modKind = "1(on),5(off)"
	const orCat = "3(remote-control)"
	want := []FlattenEntry{
		// LLN0.Mod: DA leaves with explicit fc, enum type and DOI value.
		{Path: "IED1LD0/LLN0.Mod.stVal", FC: "ST", Value: "1", BType: "INT8", EnumVals: modKind},
		{Path: "IED1LD0/LLN0.Mod.ctlModel", FC: "CF", BType: "INT8", EnumVals: "0(status-only)"},
		// No bType/fc: inferred ST, normalised STRING.
		{Path: "IED1LD0/LLN0.Mod.plain", FC: "ST", BType: "STRING"},
		// Type refers to no known DAType: treated as a leaf.
		{Path: "IED1LD0/LLN0.Mod.unknownStruct", FC: "CF", BType: "Struct"},
		// Struct with fc=CO: BDAs inherit CO unless they set their own fc.
		{Path: "IED1LD0/LLN0.Mod.Oper.ctlVal", FC: "CO", Value: "true", BType: "BOOL"},
		{Path: "IED1LD0/LLN0.Mod.Oper.withFc", FC: "ST", BType: "INT8"},
		{Path: "IED1LD0/LLN0.Mod.Oper.origin.orCat", FC: "CO", Value: "3", BType: "INT8", EnumVals: orCat},
		{Path: "IED1LD0/LLN0.Mod.Oper.origin.orIdent", FC: "CO", BType: "BIT_STRING"},
		// DA children of a DAType.
		{Path: "IED1LD0/LLN0.Mod.Oper.extraDA", FC: "CO", Value: "2", BType: "INT32", EnumVals: modKind},
		{Path: "IED1LD0/LLN0.Mod.Oper.withFcDA", FC: "SP", BType: "UINT16"},
		// Struct DA without fc: inferFC(Struct) = CO.
		{Path: "IED1LD0/LLN0.Mod.origin.orCat", FC: "CO", BType: "INT8", EnumVals: orCat},
		{Path: "IED1LD0/LLN0.Mod.origin.orIdent", FC: "CO", BType: "BIT_STRING"},
		// SDO with a DOI/SDI value (regression: value was lost) and trimmed.
		{Path: "IED1LD0/LLN0.Mod.sub.mag", FC: "MX", Value: "1.5", BType: "FLOAT32"},
		{Path: "IED1LD0/LLN0.Mod.sub.t", FC: "ST", BType: "UTC_TIME"},
		// Recursive DOType/DAType: expanded once, then cut off.
		{Path: "IED1LD0/LOOP1.Rec.x", FC: "ST", BType: "INT8"},
		{Path: "IED1LD0/LOOP1.Rec.l.leaf", FC: "ST", BType: "INT8"},
		{Path: "IED1LD0/Q0XCBR1.Pos.stVal", FC: "ST", Value: "2", BType: "Dbpos"},
		{Path: "IED1LD0/Q0XCBR1.Loc.stVal", FC: "ST", BType: "Dbpos"},
		{Path: "IED1LD0/GGIO1.Pos.stVal", FC: "ST", BType: "Dbpos"},
		{Path: "IED1LD0/GGIO1.Loc.stVal", FC: "ST", BType: "Dbpos"},
		// LD without LN0, and LD whose LN0 is the only LN.
		{Path: "IED1LD1/GGIO2.Pos.stVal", FC: "ST", BType: "Dbpos"},
		{Path: "IED1LD1/GGIO2.Loc.stVal", FC: "ST", BType: "Dbpos"},
		{Path: "IED1LD2/LLN0.Pos.stVal", FC: "ST", BType: "Dbpos"},
		{Path: "IED1LD2/LLN0.Loc.stVal", FC: "ST", BType: "Dbpos"},
	}
	for _, w := range want {
		g, ok := got[w.Path]
		if !ok {
			t.Errorf("missing %s", w.Path)
			continue
		}
		if g != w {
			t.Errorf("%s:\n got %+v\nwant %+v", w.Path, g, w)
		}
	}
	if len(flat) != len(want) {
		t.Errorf("got %d entries, want %d", len(flat), len(want))
		for _, e := range flat {
			t.Logf("  %+v", e)
		}
	}
}

func TestWriteTree_Rich(t *testing.T) {
	t.Parallel()
	doc := loadTestdata(t, "rich.cid")
	tests := []struct {
		detailed bool
		want     []string
		notWant  []string
	}{
		{
			detailed: false,
			want: []string{
				"📦 IED1LD0\n│\n├─ 🔧 LLN0 (Logical Node Zero)\n│  ├─ Mod (Mode)\n│  │  ├─ stVal: 1\n",
				"│  │  ├─ Oper.origin.orCat: 3\n",
				"│  │  └─ sub.t: ?\n",
				"│  │  ├─ sub.mag: 1.5\n",
				"│  ├─ 📊 DataSets (2)\n│  │  ├─ DS1\n│  │  ├─ DS2\n",
				"│  └─ 📡 Reports (2)\n      ├─ brcb01 [Buffered]\n      └─ urcb01 [Unbuffered]\n",
				"├─ 🔧 Q0XCBR1 (Breaker)\n│  ├─ Loc\n│  │  └─ stVal: ?\n│  └─ Pos\n│     └─ stVal: 2\n",
				"├─ 🔧 LOOP1\n│  └─ Rec\n│     ├─ x: ?\n│     └─ l.leaf: ?\n",
				"└─ 🔧 UNKN1\n",
				"📦 IED1LD1\n│\n└─ 🔧 GGIO2\n   ├─ Loc\n   │  └─ stVal: ?\n   └─ Pos\n      └─ stVal: ?\n",
				"📦 IED1LD2\n│\n└─ 🔧 LLN0 (Logical Node Zero)\n",
			},
			notWant: []string{"[type:", "Beh", "gone", "again", "self"},
		},
		{
			detailed: true,
			want: []string{
				"stVal [ST]: 1 [type:INT8] [enum:1(on),5(off)]\n",
				"Oper.ctlVal [CO]: true [type:BOOL]\n",
				"Oper.withFcDA [SP]: ? [type:UINT16]\n",
				"plain [ST]: ? [type:STRING]\n",
			},
		},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		if err := doc.WriteTree(&buf, tt.detailed); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, w := range tt.want {
			if !strings.Contains(out, w) {
				t.Errorf("detailed=%v: missing %q in:\n%s", tt.detailed, w, out)
			}
		}
		for _, nw := range tt.notWant {
			if strings.Contains(out, nw) {
				t.Errorf("detailed=%v: unexpected %q in output", tt.detailed, nw)
			}
		}
		// LNs other than LLN0 are sorted by name.
		if g, l := strings.Index(out, "GGIO1"), strings.Index(out, "LOOP1"); g < 0 || l < g {
			t.Errorf("detailed=%v: LNs not sorted", tt.detailed)
		}
	}
}

// TestRecursiveTypes ensures self- and mutually-recursive type definitions
// terminate (they used to recurse until the goroutine stack overflowed).
func TestRecursiveTypes(t *testing.T) {
	t.Parallel()
	const doc = `<SCL xmlns="http://www.iec.ch/61850/2003/SCL">
<IED name="I"><AccessPoint name="A"><Server><LDevice inst="L">
<LN lnClass="X" inst="1" lnType="T"/></LDevice></Server></AccessPoint></IED>
<DataTypeTemplates>
<LNodeType id="T"><DO name="D" type="DA1"/></LNodeType>
<DOType id="DA1"><SDO name="s" type="DB1"/><DA name="v" bType="Struct" type="SA"/></DOType>
<DOType id="DB1"><SDO name="s" type="DA1"/><SDO name="s2" type="DA1"/><DA name="leaf" bType="INT8"/></DOType>
<DAType id="SA"><BDA name="b" bType="Struct" type="SB"/><BDA name="b2" bType="Struct" type="SB"/></DAType>
<DAType id="SB"><BDA name="a" bType="Struct" type="SA"/><BDA name="end" bType="BOOLEAN"/></DAType>
</DataTypeTemplates></SCL>`
	scl, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	flat, err := scl.Flatten()
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range flat {
		paths = append(paths, e.Path)
	}
	want := []string{
		"IL/X1.D.s.leaf",
		"IL/X1.D.v.b.end",
		"IL/X1.D.v.b2.end",
	}
	if strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Errorf("paths = %v want %v", paths, want)
	}
	var buf bytes.Buffer
	if err := scl.WriteTree(&buf, true); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"v.b.end [CO]", "v.b2.end [CO]", "s.leaf [ST]"} {
		if !strings.Contains(buf.String(), w) {
			t.Errorf("tree missing %q:\n%s", w, buf.String())
		}
	}
}

func TestParse_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, in string }{
		{"empty", ""},
		{"not XML", "hello"},
		{"wrong root", `<Foo xmlns="http://www.iec.ch/61850/2003/SCL"/>`},
		{"wrong namespace", `<SCL xmlns="urn:other"/>`},
		{"no namespace", `<SCL/>`},
		{"truncated", `<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED name="x">`},
		{"bad attribute", `<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED name></SCL>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc, err := Parse(strings.NewReader(tt.in))
			if err == nil || doc != nil {
				t.Fatalf("Parse(%q) = %v, %v; want error", tt.in, doc, err)
			}
		})
	}
}

func TestFlatten_Empty(t *testing.T) {
	t.Parallel()
	doc, err := Parse(strings.NewReader(`<SCL xmlns="http://www.iec.ch/61850/2003/SCL"/>`))
	if err != nil {
		t.Fatal(err)
	}
	flat, err := doc.Flatten()
	if err != nil || len(flat) != 0 {
		t.Fatalf("Flatten = %v, %v", flat, err)
	}
	var buf bytes.Buffer
	if err := doc.WriteTree(&buf, true); err != nil || buf.Len() != 0 {
		t.Fatalf("WriteTree = %q, %v", buf.String(), err)
	}
}

func TestGetDAIValue(t *testing.T) {
	t.Parallel()
	doi := &DOI{
		Name: "Mod",
		DAI:  []DAI{{Name: "stVal", Val: " on "}},
		SDI: []SDI{{
			Name: "Oper",
			DAI:  []DAI{{Name: "ctlVal", Val: "true"}},
			SDI:  []SDI{{Name: "origin", DAI: []DAI{{Name: "orCat", Val: "3"}}}},
		}},
	}
	tests := []struct {
		name string
		doi  *DOI
		path []string
		da   string
		want string
	}{
		{"nil DOI", nil, nil, "stVal", ""},
		{"top level trimmed", doi, nil, "stVal", "on"},
		{"one level", doi, []string{"Oper"}, "ctlVal", "true"},
		{"two levels", doi, []string{"Oper", "origin"}, "orCat", "3"},
		{"missing SDI", doi, []string{"Nope"}, "ctlVal", ""},
		{"missing DAI", doi, []string{"Oper"}, "nope", ""},
	}
	for _, tt := range tests {
		if got := getDAIValue(tt.doi, tt.path, tt.da); got != tt.want {
			t.Errorf("%s: getDAIValue = %q want %q", tt.name, got, tt.want)
		}
	}
}

func TestInferFC(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"Struct": "CO", "INT8": "ST", "": "ST"} {
		if got := inferFC(in); got != want {
			t.Errorf("inferFC(%q) = %q want %q", in, got, want)
		}
	}
}

func TestBuildEnumTypes(t *testing.T) {
	t.Parallel()
	got := buildEnumTypes([]EnumType{
		{ID: "A", EnumVal: []EnumVal{{Ord: " 1 ", Text: " on "}, {Ord: "2", Text: "off"}}},
		{ID: "Empty"},
	})
	if len(got) != 1 || got["A"] != "1(on),2(off)" {
		t.Errorf("buildEnumTypes = %v", got)
	}
}

func TestAppendPathDoesNotAlias(t *testing.T) {
	t.Parallel()
	base := make([]string, 1, 4)
	base[0] = "a"
	x := appendPath(base, "x")
	y := appendPath(base, "y")
	if x[1] != "x" || y[1] != "y" || len(base) != 1 {
		t.Fatalf("aliasing: base=%v x=%v y=%v", base, x, y)
	}
}

// failWriter fails on write number n (0-based).
type failWriter struct{ n, calls int }

var errWrite = errors.New("write failed")

func (w *failWriter) Write(p []byte) (int, error) {
	defer func() { w.calls++ }()
	if w.calls == w.n {
		return 0, errWrite
	}
	return len(p), nil
}

func TestWriteCSV_WriteErrors(t *testing.T) {
	t.Parallel()
	entries := []FlattenEntry{{Path: "LD/LN.DO.DA", FC: "ST", Value: "1"}}
	for _, n := range []int{0, 1} {
		if err := WriteCSV(&failWriter{n: n}, ",", entries); !errors.Is(err, errWrite) {
			t.Errorf("fail on write %d: err = %v", n, err)
		}
	}
	if err := WriteCSV(&failWriter{n: 99}, ",", entries); err != nil {
		t.Errorf("no failure: err = %v", err)
	}
}

func TestWriteCSV_Escaping(t *testing.T) {
	t.Parallel()
	entries := []FlattenEntry{
		{Path: "LD/LN.DO.a.b", FC: "ST", Value: `say "hi", ok`, BType: "STRING", EnumVals: "1(on),2(off)"},
		{Path: "LD/LN.DO.d", FC: "DC", Value: "two\nlines|x", BType: "STRING"},
	}
	tests := []struct {
		sep  string
		want string
	}{
		{",", CSVHeaders + "\n" +
			`LD,LN,DO,a.b,ST,"say ""hi"", ok",STRING,"1(on),2(off)"` + "\n" +
			"LD,LN,DO,d,DC,\"two\nlines|x\",STRING,\n"},
		{"|", strings.ReplaceAll(CSVHeaders, ",", "|") + "\n" +
			`LD|LN|DO|a.b|ST|"say ""hi"", ok"|STRING|1(on),2(off)` + "\n" +
			"LD|LN|DO|d|DC|\"two\nlines|x\"|STRING|\n"},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		if err := WriteCSV(&buf, tt.sep, entries); err != nil {
			t.Fatal(err)
		}
		if buf.String() != tt.want {
			t.Errorf("sep %q:\n got %q\nwant %q", tt.sep, buf.String(), tt.want)
		}
	}
}
