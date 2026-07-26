// SPDX-License-Identifier: MIT

package service

import (
	"testing"
)

func TestParseDoDaPath(t *testing.T) {
	do, da := parseDoDaPath("Hz")
	if do != "Hz" || da != "" {
		t.Fatalf("got %q %q", do, da)
	}
	do, da = parseDoDaPath("Hz.mag")
	if do != "Hz" || da != "mag" {
		t.Fatalf("got %q %q", do, da)
	}
	do, da = parseDoDaPath("AnIn1.mag.f")
	if do != "AnIn1" || da != "mag.f" {
		t.Fatalf("got %q %q", do, da)
	}
}

func TestFinder_BulkFind(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD0"},
		logicalNodes:   []string{"LLN0", "FMMXU1"},
		dataObjects:    []string{"DO1", "Hz"},
	}
	finder := NewFinder(mock)

	empty, err := finder.BulkFind(nil)
	if err != nil || empty.CallCount != 0 || empty.Entries != nil {
		t.Fatalf("empty: %+v err=%v", empty, err)
	}

	res, err := finder.BulkFind([]BulkMappingEntry{
		{ControlledPropertyId: "p1", BaseLn: "MMXU", DoDaPath: "DO1"},
		{ControlledPropertyId: "p2", BaseLn: "MMXU", DoDaPath: "DO1.mag"},
		{ControlledPropertyId: "p3", BaseLn: "MMXU", DoDaPath: "Hz"},
		{ControlledPropertyId: "p4", BaseLn: "LLN0", DoDaPath: "missing"},
	})
	if err != nil {
		t.Fatalf("BulkFind: %v", err)
	}
	if len(res.Entries) != 4 {
		t.Fatalf("entries=%+v", res.Entries)
	}
	if res.CallCount == 0 {
		t.Fatal("expected calls")
	}

	byID := map[string][]string{}
	for _, e := range res.Entries {
		byID[e.ControlledPropertyId] = e.Paths
	}
	if len(byID["p1"]) == 0 {
		t.Fatalf("p1 paths empty: %+v", byID)
	}
	foundMag := false
	for _, p := range byID["p2"] {
		if p == "LD0/FMMXU1.DO1.mag" {
			foundMag = true
		}
	}
	if !foundMag {
		t.Fatalf("expected DO1.mag path, got %+v", byID["p2"])
	}
	if len(byID["p4"]) != 0 {
		t.Fatalf("missing DO should be empty, got %v", byID["p4"])
	}
}
