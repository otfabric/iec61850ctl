// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// TestSCLExpansionBoundExit: an SCL file whose types nest 65 type
// references deep is refused by scl parse (tree and flat) and scl convert
// as invalid SCL, exit 3, and the error names the bound; scl convert
// creates no output file.
func TestSCLExpansionBoundExit(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED name="I"><AccessPoint name="A"><Server><LDevice inst="L">`)
	b.WriteString(`<LN0 lnClass="LLN0" inst="" lnType="T"/></LDevice></Server></AccessPoint></IED><DataTypeTemplates>`)
	b.WriteString(`<LNodeType id="T" lnClass="LLN0"><DO name="D" type="d0"/></LNodeType>`)
	const types = 65
	for i := range types {
		fmt.Fprintf(&b, `<DOType id="d%d" cdc="X">`, i)
		if i < types-1 {
			fmt.Fprintf(&b, `<SDO name="s" type="d%d"/>`, i+1)
		} else {
			b.WriteString(`<DA name="v" bType="INT8" fc="ST"/>`)
		}
		b.WriteString(`</DOType>`)
	}
	b.WriteString(`</DataTypeTemplates></SCL>`)
	dir := t.TempDir()
	in := filepath.Join(dir, "deep.icd")
	if err := os.WriteFile(in, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "deep.csv")
	for _, args := range [][]string{
		{"scl", "parse", "--input", in},
		{"scl", "parse", "--input", in, "--flatten"},
		{"scl", "convert", "--input", in, "--output", out},
	} {
		r := runCLI(t, args...)
		expectCode(t, r, errs.ExitInvalidInput)
		if !strings.Contains(r.stderr, "type references nested more than 64 deep") {
			t.Fatalf("%v: stderr %q does not name the bound", args, r.stderr)
		}
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("scl convert wrote an output file for a refused document")
	}
}

// TestSCLInvalidExit: scl parse and scl convert refuse a file that is not
// SCL as invalid SCL, exit 3 (docs/exitcodes.md); scl convert creates no
// output file.
func TestSCLInvalidExit(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "o.csv")
	for _, c := range []struct{ name, doc string }{
		{"not XML", "{"},
		{"truncated", `<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED name="I">`},
	} {
		in := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "_")+".icd")
		if err := os.WriteFile(in, []byte(c.doc), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{
			{"scl", "parse", "--input", in},
			{"scl", "parse", "--input", in, "--flatten"},
			{"scl", "convert", "--input", in, "--output", out},
		} {
			r := runCLI(t, args...)
			if r.code != errs.ExitInvalidInput || !strings.Contains(r.stderr, "parse SCL") {
				t.Errorf("%s: %v: exit=%d want %d (stderr %q)", c.name, args, r.code, errs.ExitInvalidInput, r.stderr)
			}
		}
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("scl convert wrote an output file for invalid SCL")
	}
}
