// SPDX-License-Identifier: MIT

package domain

import (
	"testing"
)

func TestDataAttribute_IsLeaf(t *testing.T) {
	tests := []struct {
		name string
		da   DataAttribute
		want bool
	}{
		{"float leaf", DataAttribute{Name: "f", Type: TypeFloat, Value: NewValue(1.0, TypeFloat)}, true},
		{"struct type no children", DataAttribute{Name: "mag", Type: TypeStructure}, false},
		{"array type no children", DataAttribute{Name: "arr", Type: TypeArray}, false},
		{"has children", DataAttribute{Name: "mag", Type: TypeFloat, Children: []DataAttribute{{Name: "f"}}}, false},
		{"unknown leaf-ish", DataAttribute{Name: "x", Type: TypeBoolean}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.da.IsLeaf(); got != tt.want {
				t.Fatalf("IsLeaf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataAttribute_Walk_Leaves_LeafCount(t *testing.T) {
	tree := DataAttribute{
		Name: "AnIn1",
		Type: TypeStructure,
		Children: []DataAttribute{
			{
				Name: "mag",
				Type: TypeStructure,
				Children: []DataAttribute{
					{Name: "f", Type: TypeFloat, Value: NewValue(50.0, TypeFloat)},
					{Name: "i", Type: TypeInteger, Value: NewValue(int64(1), TypeInteger)},
				},
			},
			{Name: "q", Type: TypeBitString, Value: NewValue([]byte{0}, TypeBitString)},
			{
				Name:     "emptyStruct",
				Type:     TypeStructure,
				Children: nil,
			},
		},
	}

	var visited []string
	var depths []int
	tree.Walk(func(da *DataAttribute, depth int) {
		visited = append(visited, da.Name)
		depths = append(depths, depth)
	})

	wantOrder := []string{"AnIn1", "mag", "f", "i", "q", "emptyStruct"}
	if len(visited) != len(wantOrder) {
		t.Fatalf("Walk visited %v, want %v", visited, wantOrder)
	}
	for i := range wantOrder {
		if visited[i] != wantOrder[i] {
			t.Fatalf("Walk[%d] = %q, want %q", i, visited[i], wantOrder[i])
		}
	}
	if depths[0] != 0 || depths[2] != 2 {
		t.Fatalf("Walk depths = %v", depths)
	}

	leaves := tree.Leaves()
	if len(leaves) != 3 {
		t.Fatalf("Leaves() len = %d, want 3 (%v)", len(leaves), leafNames(leaves))
	}
	wantLeaves := []string{"f", "i", "q"}
	for i, name := range wantLeaves {
		if leaves[i].Name != name {
			t.Fatalf("Leaves()[%d] = %q, want %q", i, leaves[i].Name, name)
		}
	}

	if got := tree.LeafCount(); got != 3 {
		t.Fatalf("LeafCount() = %d, want 3", got)
	}

	leafOnly := DataAttribute{Name: "stVal", Type: TypeBoolean}
	if got := leafOnly.LeafCount(); got != 1 {
		t.Fatalf("single leaf LeafCount() = %d, want 1", got)
	}
	if got := leafOnly.Leaves(); len(got) != 1 || got[0].Name != "stVal" {
		t.Fatalf("single Leaves() = %v", leafNames(got))
	}
}

func leafNames(leaves []*DataAttribute) []string {
	out := make([]string, len(leaves))
	for i, l := range leaves {
		out[i] = l.Name
	}
	return out
}
