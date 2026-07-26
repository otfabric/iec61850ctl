// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestFindPathInput_Validate(t *testing.T) {
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
