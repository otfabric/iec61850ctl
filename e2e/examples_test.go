//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestExamplesManifestPresent ensures the declarative example source exists and parses.
// Journey assertions for these cases live in browse/read/dataset/report tests;
// this guards documentation drift of the shared case file.
func TestExamplesManifestPresent(t *testing.T) {
	path := filepath.Join(testDataDirAbs, "examples.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Title string `json:"title"`
		Cases []struct {
			Title string   `json:"title"`
			Argv  []string `json:"argv"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Title == "" || len(doc.Cases) == 0 {
		t.Fatal("examples.json missing title or cases")
	}
	for _, c := range doc.Cases {
		if c.Title == "" || len(c.Argv) == 0 {
			t.Fatalf("invalid case: %+v", c)
		}
	}
}
