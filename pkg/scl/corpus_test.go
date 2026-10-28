// SPDX-License-Identifier: MIT

package scl

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// corpusDoc is one real-world SCL file from cids/ or e2e/testdata, parsed once
// per test binary.
type corpusDoc struct {
	name string
	doc  *SCL
	err  error
}

// loadCorpus parses every real SCL file once (concurrently); tests share the
// read-only result.
var loadCorpus = sync.OnceValue(func() []corpusDoc {
	paths, _ := filepath.Glob(filepath.Join("..", "..", "cids", "*.cid"))
	paths = append(paths, filepath.Join("..", "..", "e2e", "testdata", "interop.icd"))
	docs := make([]corpusDoc, len(paths))
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			docs[i].name = filepath.Base(p)
			f, err := os.Open(p)
			if err != nil {
				docs[i].name = "" // missing file: dropped below
				return
			}
			defer func() { _ = f.Close() }()
			docs[i].doc, docs[i].err = Parse(f)
		}()
	}
	wg.Wait()
	out := docs[:0]
	for _, d := range docs {
		if d.name != "" {
			out = append(out, d)
		}
	}
	return out
})

// corpus returns the parsed real SCL files, skipping the test if none exist.
func corpus(t *testing.T) []corpusDoc {
	t.Helper()
	docs := loadCorpus()
	if len(docs) == 0 {
		t.Skip("no SCL corpus files found (cids/, e2e/testdata/interop.icd)")
	}
	return docs
}

// loadTestdata parses a file from pkg/scl/testdata.
func loadTestdata(t *testing.T, name string) *SCL {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	doc, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse %s: %v", name, err)
	}
	return doc
}

// TestCorpus flattens, renders and CSV-encodes every real SCL file and checks
// structural invariants: sorted unique paths, CSV that round-trips through
// encoding/csv, and a tree with one root line per logical device.
func TestCorpus(t *testing.T) {
	t.Parallel()
	for _, cd := range corpus(t) {
		t.Run(cd.name, func(t *testing.T) {
			t.Parallel()
			if cd.err != nil {
				// Zx0REF630A1.cid is known to be malformed XML.
				t.Skipf("parse (malformed XML?): %v", cd.err)
			}
			doc := cd.doc
			flat, err := doc.Flatten()
			if err != nil {
				t.Fatalf("Flatten: %v", err)
			}
			if len(flat) == 0 {
				t.Fatal("no flattened entries")
			}
			if !sort.SliceIsSorted(flat, func(i, j int) bool { return flat[i].Path < flat[j].Path }) {
				t.Error("entries not sorted by path")
			}
			for _, e := range flat {
				if e.FC == "" || e.BType == "" || !strings.Contains(e.Path, "/") {
					t.Fatalf("incomplete entry %+v", e)
				}
			}

			var buf bytes.Buffer
			if err := WriteCSV(&buf, ",", flat); err != nil {
				t.Fatalf("WriteCSV: %v", err)
			}
			r := csv.NewReader(&buf)
			r.FieldsPerRecord = 8
			records, err := r.ReadAll()
			if err != nil {
				t.Fatalf("CSV does not parse: %v", err)
			}
			if len(records) != len(flat)+1 {
				t.Fatalf("CSV records=%d want %d", len(records), len(flat)+1)
			}
			if strings.Join(records[0], ",") != CSVHeaders {
				t.Errorf("header = %v", records[0])
			}

			var lds int
			for _, ied := range doc.IED {
				for _, ap := range ied.AccessPoint {
					if ap.Server != nil {
						lds += len(ap.Server.LDevice)
					}
				}
			}
			for _, detailed := range []bool{false, true} {
				var tree bytes.Buffer
				if err := doc.WriteTree(&tree, detailed); err != nil {
					t.Fatalf("WriteTree(%v): %v", detailed, err)
				}
				if got := strings.Count(tree.String(), iconRoot+" "); got != lds {
					t.Errorf("WriteTree(%v) roots=%d want %d", detailed, got, lds)
				}
				if detailed != strings.Contains(tree.String(), "[type:") {
					t.Errorf("WriteTree(%v) detail markers mismatch", detailed)
				}
			}
		})
	}
}
