// SPDX-License-Identifier: MIT

package service

import (
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestFindPathInput_Validate(t *testing.T) {
	t.Parallel()
	if err := (FindPathInput{}).Validate(); err == nil {
		t.Fatal("expected LNPattern error")
	}
	if err := (FindPathInput{LNPattern: ".*"}).Validate(); err == nil {
		t.Fatal("expected DoName error")
	}
	if err := (FindPathInput{LNPattern: ".*", DoName: "DO", DaName: "mag"}).Validate(); err == nil {
		t.Fatal("expected DaName requires IncludeDas")
	}
	if err := (FindPathInput{LNPattern: ".*", DoName: "DO", IncludeDas: true}).Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestFinder_FindPath(t *testing.T) {
	t.Parallel()
	mock := &mockConnection{
		logicalDevices: []string{"LD0"},
		logicalNodes:   []string{"LLN0", "MMXU1"},
		dataObjects:    []string{"DO1", "Hz"},
	}
	finder := NewFinder(mock)

	res, err := finder.Path().MatchingLN("MMXU.*").WithDO("DO1").IncludingAttributes().WithDetails().Find()
	if err != nil {
		t.Fatalf("FindPath: %v", err)
	}
	if len(res.Matches) != 1 || res.Matches[0].LN != "MMXU1" {
		t.Fatalf("matches=%+v", res.Matches)
	}
	if len(res.Matches[0].DataAttributes) == 0 {
		t.Fatal("expected DA map")
	}

	filtered, err := finder.FindPath(FindPathInput{
		LNPattern:  "MMXU.*",
		DoName:     "DO1",
		DaName:     "mag",
		IncludeDas: true,
	})
	if err != nil {
		t.Fatalf("FindPath DA filter: %v", err)
	}
	if len(filtered.Matches) != 1 {
		t.Fatalf("filtered matches=%+v", filtered.Matches)
	}
	if filtered.Matches[0].DataAttributes["mag"] == nil {
		t.Fatalf("mag missing: %+v", filtered.Matches[0].DataAttributes)
	}

	none, err := finder.FindPath(FindPathInput{
		LNPattern:  "MMXU.*",
		DoName:     "DO1",
		DaName:     "doesNotExist",
		IncludeDas: true,
	})
	if err != nil {
		t.Fatalf("missing DA: %v", err)
	}
	if len(none.Matches) != 0 {
		t.Fatalf("expected no matches, got %+v", none.Matches)
	}

	if _, err := finder.FindPath(FindPathInput{LNPattern: "[", DoName: "DO1"}); err == nil {
		t.Fatal("expected bad regex error")
	}
}

func TestFilterDataAttributes(t *testing.T) {
	t.Parallel()
	in := map[string][]domain.DataAttribute{
		"mag": {{Name: "mag"}},
		"q":   {{Name: "q"}},
	}
	got := filterDataAttributes(in, "mag")
	if len(got) != 1 || got["mag"] == nil {
		t.Fatalf("got %+v", got)
	}
	empty := filterDataAttributes(in, "nope")
	if len(empty) != 0 {
		t.Fatalf("expected empty, got %+v", empty)
	}
}

// Regression: find path --include-das printed "TotW.mag.mag.f" because the
// attribute name already carries its parent prefix.
func TestDataAttributePath(t *testing.T) {
	t.Parallel()
	const do = "InteropLD/MMXU1.TotW"
	tests := []struct {
		parent, name, want string
	}{
		{"mag", "mag.f", do + ".mag.f"},
		{"q", "q", do + ".q"},
		{"Oper", "Oper.Check", do + ".Oper.Check"},
		{"mag", "", do + ".mag"},
		{"mag", "f", do + ".mag.f"},
		{"magX", "mag.f", do + ".magX.mag.f"},
		{"", "stVal", do + ".stVal"},
	}
	for _, tt := range tests {
		got := DataAttributePath(do, tt.parent, domain.DataAttribute{Name: tt.name})
		if got != tt.want {
			t.Errorf("DataAttributePath(%q, %q) = %q, want %q", tt.parent, tt.name, got, tt.want)
		}
	}
}

func TestFindPath_DAPathsNotDuplicated(t *testing.T) {
	t.Parallel()
	m := newTreeModel()
	res, err := NewFinder(m).FindPath(FindPathInput{LNPattern: "MMXU.*", DoName: "TotW", DaName: "mag", IncludeDas: true, Detailed: true})
	if err != nil || len(res.Matches) == 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	match := res.Matches[0]
	base := match.LD + "/" + match.LN + "." + match.DO
	var paths []string
	for parent, attrs := range match.DataAttributes {
		for _, a := range attrs {
			paths = append(paths, DataAttributePath(base, parent, a))
		}
	}
	want := "LD0/MMXU1.TotW.mag.f"
	found := false
	for _, p := range paths {
		if strings.Contains(p, "mag.mag") {
			t.Fatalf("duplicated segment in %q", p)
		}
		if p == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("%q not in %v", want, paths)
	}
}
