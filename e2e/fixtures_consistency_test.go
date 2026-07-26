//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFixtures_SelfServerValuesAgreeWithExpected(t *testing.T) {
	keys := []string{
		"InteropLD/LLN0.Mod.stVal",
		"InteropLD/GGIO1.SPS1.stVal",
		"InteropLD/GGIO1.SPCSO1.stVal",
		"InteropLD/GGIO1.SPCSO1.ctlModel",
		"InteropLD/GGIO1.SPCSO2.stVal",
		"InteropLD/GGIO1.SPCSO2.ctlModel",
		"InteropLD/GGIO1.SPCSO3.stVal",
		"InteropLD/GGIO1.SPCSO3.ctlModel",
		"InteropLD/MMXU1.TotW.mag.f",
	}

	data, err := os.ReadFile(filepath.Join(testDataDirAbs, "self-server-values.json"))
	if err != nil {
		t.Fatalf("read self-server-values: %v", err)
	}
	var seed struct {
		Leaves []struct {
			Ref   string `json:"Ref"`
			Value struct {
				Raw any `json:"Raw"`
			} `json:"Value"`
		} `json:"Leaves"`
	}
	if err := json.Unmarshal(data, &seed); err != nil {
		t.Fatalf("parse self-server-values: %v", err)
	}

	byRef := make(map[string]any, len(seed.Leaves))
	for _, leaf := range seed.Leaves {
		byRef[leaf.Ref] = leaf.Value.Raw
	}

	for _, key := range keys {
		got, ok := byRef[key]
		if !ok {
			t.Fatalf("self-server-values missing leaf %q", key)
		}
		// Re-decode Raw through JSON for number normalisation.
		raw, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal leaf %q: %v", key, err)
		}
		var gotNorm any
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&gotNorm); err != nil {
			t.Fatalf("decode leaf %q: %v", key, err)
		}
		want := expectedValue(t, key)
		if !valuesEqual(gotNorm, want) {
			t.Fatalf("leaf %q: self-server-values %#v != expected-values %#v", key, gotNorm, want)
		}
	}
}

func TestFixtures_LockFileHashes(t *testing.T) {
	if len(lockFile.Files) == 0 {
		t.Fatal("lock files map is empty")
	}
	for name, wantHash := range lockFile.Files {
		path := filepath.Join(testDataDirAbs, name)
		gotHash, err := fileSHA256Hex(path)
		if err != nil {
			t.Fatalf("hash %s: %v", name, err)
		}
		if gotHash != wantHash {
			t.Errorf("%s: got %s, lock wants %s", name, gotHash, wantHash)
		}
	}
}
