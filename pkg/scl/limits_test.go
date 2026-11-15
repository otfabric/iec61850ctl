// SPDX-License-Identifier: MIT

package scl

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
)

// typeChain returns an SCL document whose one data object expands through
// a chain of distinct types, each used fan times by the one above it.
//
// With sdo false the DO's DOType has one Struct DA of type t0, and DAType
// ti has fan BDAs of type ti+1; the last has fan BOOLEAN BDAs. A path
// follows depth+1 type references and the expansion has 2+fan+fan²+…+fan^depth
// nodes, fan^depth of them leaves.
//
// With sdo true DOType di has fan SDOs of type di+1, and the last has one
// INT8 DA when leaf is set and nothing otherwise. A path follows depth
// type references.
func typeChain(depth, fan int, sdo, leaf bool) []byte {
	var b strings.Builder
	b.WriteString(`<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED name="I"><AccessPoint name="A"><Server><LDevice inst="L">`)
	b.WriteString(`<LN0 lnClass="LLN0" inst="" lnType="T"/></LDevice></Server></AccessPoint></IED><DataTypeTemplates>`)
	if sdo {
		b.WriteString(`<LNodeType id="T" lnClass="LLN0"><DO name="D" type="d0"/></LNodeType>`)
		for i := range depth {
			fmt.Fprintf(&b, `<DOType id="d%d" cdc="X">`, i)
			for j := range fan {
				if i < depth-1 {
					fmt.Fprintf(&b, `<SDO name="s%d" type="d%d"/>`, j, i+1)
				}
			}
			if i == depth-1 && leaf {
				b.WriteString(`<DA name="v" bType="INT8" fc="ST"/>`)
			}
			b.WriteString(`</DOType>`)
		}
	} else {
		b.WriteString(`<LNodeType id="T" lnClass="LLN0"><DO name="D" type="DT"/></LNodeType>`)
		b.WriteString(`<DOType id="DT" cdc="X"><DA name="a" bType="Struct" type="t0" fc="ST"/></DOType>`)
		for i := range depth {
			fmt.Fprintf(&b, `<DAType id="t%d">`, i)
			for j := range fan {
				if i == depth-1 {
					fmt.Fprintf(&b, `<BDA name="b%d" bType="BOOLEAN"/>`, j)
				} else {
					fmt.Fprintf(&b, `<BDA name="b%d" bType="Struct" type="t%d"/>`, j, i+1)
				}
			}
			b.WriteString(`</DAType>`)
		}
	}
	b.WriteString(`</DataTypeTemplates></SCL>`)
	return []byte(b.String())
}

func parseChain(t *testing.T, doc []byte) *SCL {
	t.Helper()
	s, err := Parse(bytes.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// setMaxExpandedNodes lowers the node budget for one test.
func setMaxExpandedNodes(t *testing.T, n int) {
	t.Helper()
	prev := maxExpandedNodes
	maxExpandedNodes = n
	t.Cleanup(func() { maxExpandedNodes = prev })
}

// expand runs Flatten and WriteTree (both modes) and returns the entries of
// Flatten, the bytes allocated by all three, and their errors.
func expand(s *SCL) (entries int, alloc uint64, flatErr, treeErr error) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	entries, flatErr = func() (int, error) {
		flat, err := s.Flatten()
		return len(flat), err
	}()
	for _, detailed := range []bool{false, true} {
		if err := s.WriteTree(io.Discard, detailed); err != nil {
			treeErr = err
		}
	}
	runtime.ReadMemStats(&after)
	return entries, after.TotalAlloc - before.TotalAlloc, flatErr, treeErr
}

func TestLimitDefaults(t *testing.T) {
	if maxSCLBytes != 1<<30 || maxExpandedNodes != 1<<25 || maxTypeDepth != 64 {
		t.Fatalf("maxSCLBytes %d, maxExpandedNodes %d, maxTypeDepth %d: SECURITY.md documents 1 GiB, 33554432 and 64",
			maxSCLBytes, maxExpandedNodes, maxTypeDepth)
	}
}

// TestTypeDepthLimit: a path may follow maxTypeDepth type references; one
// more stops Flatten and WriteTree with an error that says so, through DA
// and BDA types as through SDO types.
func TestTypeDepthLimit(t *testing.T) {
	for _, c := range []struct {
		name string
		doc  []byte
		ok   bool
	}{
		{"DAType chain, 1 reference", typeChain(0, 1, false, false), true},
		{"DAType chain, limit-1", typeChain(maxTypeDepth-2, 1, false, false), true},
		{"DAType chain, limit", typeChain(maxTypeDepth-1, 1, false, false), true},
		{"DAType chain, limit+1", typeChain(maxTypeDepth, 1, false, false), false},
		{"DAType chain, 10000", typeChain(10000, 1, false, false), false},
		{"SDO chain, limit", typeChain(maxTypeDepth, 1, true, true), true},
		{"SDO chain, limit+1", typeChain(maxTypeDepth+1, 1, true, true), false},
		{"SDO chain, 10000", typeChain(10000, 1, true, true), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			entries, _, flatErr, treeErr := expand(parseChain(t, c.doc))
			if c.ok {
				if flatErr != nil || treeErr != nil {
					t.Fatalf("Flatten: %v; WriteTree: %v", flatErr, treeErr)
				}
				if entries == 0 && !strings.Contains(c.name, "1 reference") {
					t.Fatal("no entries")
				}
				return
			}
			want := fmt.Sprintf("type references nested more than %d deep", maxTypeDepth)
			for _, err := range []error{flatErr, treeErr} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("err %v, want %q (Flatten: %v; WriteTree: %v)", err, want, flatErr, treeErr)
				}
			}
			if entries != 0 {
				t.Fatalf("Flatten returned %d entries with its error", entries)
			}
		})
	}
}

// TestExpansionBudget: an expansion of exactly maxExpandedNodes nodes is
// produced, one node more stops Flatten and WriteTree with an error, also
// when the nodes are SDOs that produce no entry at all.
func TestExpansionBudget(t *testing.T) {
	const depth = 10
	// DA chain of fan 2: the DO, the DA a, then 2+4+…+2^depth BDAs.
	daNodes := 2 + (1<<(depth+1) - 2)
	// SDO chain of fan 2 without a leaf: the DO, then 2+4+…+2^(depth-1) SDOs.
	sdoNodes := 1 + (1<<depth - 2)
	for _, c := range []struct {
		name   string
		doc    []byte
		budget int
		ok     bool
	}{
		{"DA chain, budget = nodes", typeChain(depth, 2, false, false), daNodes, true},
		{"DA chain, budget = nodes-1", typeChain(depth, 2, false, false), daNodes - 1, false},
		{"DA chain, budget 1", typeChain(depth, 2, false, false), 1, false},
		{"SDO chain without entries, budget = nodes", typeChain(depth, 2, true, false), sdoNodes, true},
		{"SDO chain without entries, budget = nodes-1", typeChain(depth, 2, true, false), sdoNodes - 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			setMaxExpandedNodes(t, c.budget)
			entries, _, flatErr, treeErr := expand(parseChain(t, c.doc))
			if c.ok {
				if flatErr != nil || treeErr != nil {
					t.Fatalf("Flatten: %v; WriteTree: %v", flatErr, treeErr)
				}
				return
			}
			want := fmt.Sprintf("more than %d nodes", c.budget)
			for _, err := range []error{flatErr, treeErr} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("err %v, want %q (Flatten: %v; WriteTree: %v)", err, want, flatErr, treeErr)
				}
			}
			if entries != 0 {
				t.Fatalf("Flatten returned %d entries with its error", entries)
			}
		})
	}
}

// TestExpansionBudgetAllocation: an expansion that would produce 2^17
// entries from a document of 3 KiB stops at the node budget, and Flatten
// and WriteTree together allocate a small multiple of that budget. A long
// name costs one node per 256 bytes of path, so long paths do not grow the
// allocation beyond it either.
func TestExpansionBudgetAllocation(t *testing.T) {
	const budget = 4096
	setMaxExpandedNodes(t, budget)
	long := bytes.ReplaceAll(typeChain(17, 2, false, false), []byte(`name="b0"`), []byte(`name="`+strings.Repeat("n", 4096)+`"`))
	for _, c := range []struct {
		name string
		doc  []byte
	}{
		{"short names", typeChain(17, 2, false, false)},
		{"4 KiB names", long},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, alloc, flatErr, treeErr := expand(parseChain(t, c.doc))
			// About 1 KiB per node: the paths and entries of the nodes
			// expanded until the stop, three times.
			if limit := uint64(budget * 1024 * 3); alloc > limit {
				t.Fatalf("%d bytes allocated for a budget of %d nodes: more than %d", alloc, budget, limit)
			}
			if flatErr == nil || treeErr == nil {
				t.Fatalf("Flatten: %v; WriteTree: %v; want both stopped", flatErr, treeErr)
			}
		})
	}
}

// endless is an SCL document whose IED name never ends.
type endless struct{ head []byte }

func (e *endless) Read(p []byte) (int, error) {
	n := copy(p, e.head)
	e.head = e.head[n:]
	for i := n; i < len(p); i++ {
		p[i] = 'x'
	}
	return len(p), nil
}

// sized returns an SCL document of exactly size bytes whose root element
// ends at its last byte.
func sized(t *testing.T, size int) []byte {
	t.Helper()
	head, tail := `<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED name="`, `"/></SCL>`
	if size < len(head)+len(tail) {
		t.Fatalf("size %d too small", size)
	}
	return []byte(head + strings.Repeat("x", size-len(head)-len(tail)) + tail)
}

// TestParseSizeLimit: Parse reads a document of up to maxSCLBytes and
// refuses a longer one after reading maxSCLBytes+1 bytes of it, whatever
// its length; the allocation stays a small multiple of the bound.
func TestParseSizeLimit(t *testing.T) {
	const limit = 1 << 20
	prev := maxSCLBytes
	maxSCLBytes = limit
	t.Cleanup(func() { maxSCLBytes = prev })

	for _, c := range []struct {
		name string
		r    func() io.Reader
		ok   bool
	}{
		{"limit-1", func() io.Reader { return bytes.NewReader(sized(t, limit-1)) }, true},
		{"limit", func() io.Reader { return bytes.NewReader(sized(t, limit)) }, true},
		{"limit+1", func() io.Reader { return bytes.NewReader(sized(t, limit+1)) }, false},
		{"64 MiB", func() io.Reader {
			return io.LimitReader(&endless{head: []byte(`<SCL xmlns="http://www.iec.ch/61850/2003/SCL"><IED name="`)}, 64<<20)
		}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := c.r()
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			doc, err := Parse(r)
			runtime.ReadMemStats(&after)
			if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 8*limit {
				t.Fatalf("%d bytes allocated: more than %d", alloc, 8*limit)
			}
			if c.ok {
				if err != nil || doc == nil || len(doc.IED) != 1 {
					t.Fatalf("doc %v, err %v", doc, err)
				}
				return
			}
			if want := fmt.Sprintf("document larger than %d bytes", limit); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err %v, want %q", err, want)
			}
			if doc != nil {
				t.Fatal("document returned with the error")
			}
		})
	}
}
