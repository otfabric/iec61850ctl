// SPDX-License-Identifier: MIT

package scl

import (
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// recursiveCID has self- and mutually-recursive DOType/DAType definitions.
const recursiveCID = `<SCL xmlns="http://www.iec.ch/61850/2003/SCL">
<IED name="I"><AccessPoint name="A"><Server><LDevice inst="L">
<LN0 lnClass="LLN0" lnType="T"><DOI name="D"><SDI name="s"><DAI name="leaf"><Val>1</Val></DAI></SDI></DOI></LN0>
<LN lnClass="X" inst="1" lnType="T"/></LDevice></Server></AccessPoint></IED>
<DataTypeTemplates>
<LNodeType id="T"><DO name="D" type="DA1"/></LNodeType>
<DOType id="DA1"><SDO name="s" type="DB1"/><SDO name="me" type="DA1"/><DA name="v" bType="Struct" type="SA"/></DOType>
<DOType id="DB1"><SDO name="s" type="DA1"/><DA name="leaf" bType="INT8"/></DOType>
<DAType id="SA"><BDA name="b" bType="Struct" type="SB"/><BDA name="self" bType="Struct" type="SA"/></DAType>
<DAType id="SB"><BDA name="a" bType="Struct" type="SA"/><DA name="end" bType="BOOLEAN" type="E"/></DAType>
<EnumType id="E"><EnumVal ord="0">x</EnumVal></EnumType>
</DataTypeTemplates></SCL>`

// FuzzParse feeds arbitrary bytes through every public SCL entry point:
// Parse, Flatten, WriteTree (both modes), WriteCSV, FormatEntry and CSVRow.
// None may panic or hang; errors from Parse are fine.
func FuzzParse(f *testing.F) {
	f.Add([]byte(minimalCID))
	f.Add([]byte(recursiveCID))
	if b, err := os.ReadFile(filepath.Join("testdata", "rich.cid")); err == nil {
		f.Add(b)
	}
	f.Add([]byte(""))
	f.Add([]byte("<SCL/>"))
	f.Add([]byte(`<SCL xmlns="http://www.iec.ch/61850/2003/SCL"/>`))
	f.Add([]byte(`<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED name="x">`))
	f.Add([]byte(`<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED><AccessPoint><Server><LDevice><LN0/><LN/></LDevice></Server></AccessPoint></IED></SCL>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Parse(bytes.NewReader(data))
		if err != nil {
			if doc != nil {
				t.Fatal("Parse returned a document with an error")
			}
			return
		}
		flat, err := doc.Flatten()
		if err != nil {
			t.Fatalf("Flatten: %v", err)
		}
		for _, e := range flat {
			_ = FormatEntry(e, false)
			_ = FormatEntry(e, true)
			_, _, _, _, _, _, _, _ = CSVRow(e)
		}
		if err := WriteCSV(io.Discard, ",", flat); err != nil {
			t.Fatalf("WriteCSV: %v", err)
		}
		for _, detailed := range []bool{false, true} {
			if err := doc.WriteTree(io.Discard, detailed); err != nil {
				t.Fatalf("WriteTree: %v", err)
			}
		}
	})
}

// FuzzWriteCSV checks that one entry written by WriteCSV reads back through
// encoding/csv as exactly the eight columns CSVRow reports, for each
// supported single-character separator.
func FuzzWriteCSV(f *testing.F) {
	f.Add("IED1LD0/LLN0.Mod.stVal", "ST", "1", "INT8", "1(on),5(off)", ",")
	f.Add("LD/LN.DO.a.b", "CO", `say "hi", ok`, "STRING", "", "|")
	f.Add("LD/LN", "", "two\nlines\r\nhere", "", "", ";")
	f.Add("ONLY", "DC", "", "", "", "\t")
	f.Add("a/b/c.d", "SP", "x|y,z;w\tv", "", "", ",")
	f.Add("", "", "\"", "", "", "|")

	f.Fuzz(func(t *testing.T, path, fc, value, bType, enum, sep string) {
		e := FlattenEntry{Path: path, FC: fc, Value: value, BType: bType, EnumVals: enum}
		var buf bytes.Buffer
		if err := WriteCSV(&buf, sep, []FlattenEntry{e}); err != nil {
			t.Fatalf("WriteCSV: %v", err)
		}
		// Only separators encoding/csv can read back are round-tripped.
		if sep != "," && sep != "|" && sep != ";" && sep != "\t" {
			return
		}
		// Restrict the round trip to valid UTF-8; encoding/csv also normalises
		// \r\n to \n inside quoted fields, which the comparison accounts for.
		for _, s := range []string{path, fc, value, bType, enum} {
			if !utf8.ValidString(s) {
				return
			}
		}
		r := csv.NewReader(&buf)
		r.Comma, _ = utf8.DecodeRuneInString(sep)
		r.FieldsPerRecord = 8
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("CSV for %+v (sep %q) does not parse: %v\n%q", e, sep, err, buf.String())
		}
		if len(records) != 2 {
			t.Fatalf("records=%d want 2: %q", len(records), buf.String())
		}
		ld, ln, do, da, gfc, val, bt, en := CSVRow(e)
		want := []string{ld, ln, do, da, gfc, val, bt, en}
		for i := range want {
			w := strings.ReplaceAll(want[i], "\r\n", "\n")
			if records[1][i] != w {
				t.Fatalf("column %d = %q want %q (entry %+v, sep %q)", i, records[1][i], w, e, sep)
			}
		}
	})
}
