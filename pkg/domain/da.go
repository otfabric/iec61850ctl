// SPDX-License-Identifier: MIT

package domain

// DataAttribute represents an IEC 61850 data attribute (DA) in a recursive tree.
// A DA is either a leaf (has Value, no Children) or a structure/array (has Children).
type DataAttribute struct {
	Name string
	Ref  string
	FC   FunctionalConstraint

	Type      MmsDataType
	ArraySize int

	Value      *Value
	ValueError string

	Timestamp *Timestamp
	Quality   *Quality

	Children []DataAttribute
}

// IsLeaf returns true when the attribute has no children and is not STRUCT or ARRAY.
func (da *DataAttribute) IsLeaf() bool {
	return len(da.Children) == 0 && da.Type != TypeStructure && da.Type != TypeArray
}

// Walk performs a depth-first traversal of the attribute tree.
func (da *DataAttribute) Walk(fn func(da *DataAttribute, depth int)) {
	da.walk(fn, 0)
}

func (da *DataAttribute) walk(fn func(da *DataAttribute, depth int), depth int) {
	fn(da, depth)
	for i := range da.Children {
		da.Children[i].walk(fn, depth+1)
	}
}

// Leaves collects all leaf nodes from the attribute tree.
func (da *DataAttribute) Leaves() []*DataAttribute {
	var leaves []*DataAttribute
	da.Walk(func(a *DataAttribute, _ int) {
		if a.IsLeaf() {
			leaves = append(leaves, a)
		}
	})
	return leaves
}

// LeafCount returns the number of leaf nodes in the tree.
func (da *DataAttribute) LeafCount() int {
	count := 0
	da.Walk(func(a *DataAttribute, _ int) {
		if a.IsLeaf() {
			count++
		}
	})
	return count
}
