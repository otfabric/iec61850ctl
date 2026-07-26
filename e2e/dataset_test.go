//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"strings"
	"testing"
)

func TestDataset_ListAndGet(t *testing.T) {
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)

			listRes := runCtl(t, h.ctlEnv(), commandTimeout,
				"list", "dss",
				"--ld", "InteropLD",
				"--ln", "LLN0",
				"--format", "json",
			)
			requireExitZero(t, listRes, h)

			var listed []struct {
				Name string `json:"name"`
			}
			if err := decodeExactJSON(listRes.Stdout, &listed); err != nil {
				logCommandFailure(t, listRes, h)
				t.Fatalf("decode list dss JSON: %v", err)
			}
			names := make([]string, len(listed))
			for i, e := range listed {
				names[i] = e.Name
			}
			if !containsName(names, "dsInterop") {
				logCommandFailure(t, listRes, h)
				t.Fatalf("dsInterop not in list dss: %v", names)
			}

			getRes := runCtl(t, h.ctlEnv(), commandTimeout,
				"get", "ds",
				"--ld", "InteropLD",
				"--ln", "LLN0",
				"--name", "dsInterop",
				"--format", "json",
			)
			requireExitZero(t, getRes, h)

			var ds struct {
				Name        string `json:"name"`
				MemberCount int    `json:"member_count"`
				Members     []struct {
					Index int    `json:"index"`
					Ref   string `json:"ref"`
					FC    string `json:"fc"`
				} `json:"members"`
			}
			if err := decodeExactJSON(getRes.Stdout, &ds); err != nil {
				logCommandFailure(t, getRes, h)
				t.Fatalf("decode get ds JSON: %v", err)
			}
			if ds.MemberCount != 2 || len(ds.Members) != 2 {
				logCommandFailure(t, getRes, h)
				t.Fatalf("want 2 members, got member_count=%d len=%d", ds.MemberCount, len(ds.Members))
			}
			// ICD order: GGIO1.SPS1.stVal[ST], LLN0.Mod.stVal[ST]
			if !strings.Contains(ds.Members[0].Ref, "GGIO1.SPS1.stVal") {
				logCommandFailure(t, getRes, h)
				t.Fatalf("member[0] ref want ...GGIO1.SPS1.stVal..., got %q", ds.Members[0].Ref)
			}
			if !strings.Contains(ds.Members[1].Ref, "LLN0.Mod.stVal") {
				logCommandFailure(t, getRes, h)
				t.Fatalf("member[1] ref want ...LLN0.Mod.stVal..., got %q", ds.Members[1].Ref)
			}
		})
	}
}
