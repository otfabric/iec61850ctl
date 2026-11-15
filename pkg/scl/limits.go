// SPDX-License-Identifier: MIT

package scl

import "fmt"

// Bounds of Parse and of the type expansion of Flatten and WriteTree. They
// are the defaults of go-iec61850's scl package, which server start, goose
// and sv use for the same files, so a document one of those commands reads
// is not refused by scl parse or scl convert. SECURITY.md lists them.
//
// maxSCLBytes and maxExpandedNodes are variables so that tests can lower
// them; nothing else changes them.
var (
	// maxSCLBytes is the largest document Parse reads: 1 GiB
	// (go-iec61850 scl.DefaultMaxBytes).
	maxSCLBytes int64 = 1 << 30

	// maxExpandedNodes is the number of nodes one Flatten or WriteTree
	// may expand: 33,554,432 (go-iec61850 scl.DefaultMaxExpandedNodes).
	// A node is one DO, SDO, DA or BDA at one place of the expanded
	// model (a type used at n places counts n times); it counts one more
	// for every pathBytesPerNode bytes of its path.
	maxExpandedNodes = 1 << 25
)

// maxTypeDepth is the number of type references one path of the expansion
// may follow: a DO's DOType is the first, each SDO's DOType and each Struct
// DA's or BDA's DAType below it one more (go-iec61850
// scl.DefaultMaxTypeDepth, 64).
const maxTypeDepth = 64

// pathBytesPerNode is the number of bytes of a path one node pays for, so
// that the expanded paths hold about this much per node at the most.
const pathBytesPerNode = 256

// expansion is the budget of one Flatten or WriteTree: the nodes it may
// still expand and the error that stopped it.
type expansion struct {
	left int
	err  error
}

func newExpansion() *expansion { return &expansion{left: maxExpandedNodes} }

// stopped reports whether a bound was reached; the expansion then produces
// nothing more.
func (e *expansion) stopped() bool { return e.err != nil }

// take charges the node at prefix/path, or at prefix/path.name when name is
// not empty (the path is not built for it). It reports false, and records
// the error, when the budget does not cover the node.
func (e *expansion) take(prefix, path, name string) bool {
	if e.err != nil {
		return false
	}
	n := len(prefix) + 1 + len(path)
	if name != "" {
		n += 1 + len(name)
	}
	if cost := 1 + n/pathBytesPerNode; cost <= e.left {
		e.left -= cost
		return true
	}
	if name != "" {
		path += "." + name
	}
	e.err = fmt.Errorf("scl: type expansion stopped at %s/%s: more than %d nodes", prefix, path, maxExpandedNodes)
	return false
}

// enter reports whether the type at prefix/path may be expanded below the
// types on stack; it records the error when that would follow more than
// maxTypeDepth type references.
func (e *expansion) enter(stack typeStack, prefix, path string) bool {
	if e.err != nil {
		return false
	}
	if len(stack) >= maxTypeDepth {
		e.err = fmt.Errorf("scl: type expansion stopped at %s/%s: type references nested more than %d deep", prefix, path, maxTypeDepth)
		return false
	}
	return true
}
