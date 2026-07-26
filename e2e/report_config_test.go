//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"strings"
	"testing"
)

func TestReportConfig_ListAndGet(t *testing.T) {
	for _, stack := range externalStacks() {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			h := sharedServer(t, stack)

			listRes := runCtl(t, h.ctlEnv(), commandTimeout,
				"list", "reports",
				"--ld", "InteropLD",
				"--ln", "LLN0",
				"--format", "json",
			)
			requireExitZero(t, listRes, h)

			var listed []struct {
				Name     string `json:"name"`
				Buffered bool   `json:"buffered"`
				LD       string `json:"ld"`
				LN       string `json:"ln"`
			}
			if err := decodeExactJSON(listRes.Stdout, &listed); err != nil {
				logCommandFailure(t, listRes, h)
				t.Fatalf("decode list reports JSON: %v", err)
			}

			var urcbName, brcbName string
			var urcbBuffered, brcbBuffered bool
			var sawURCB, sawBRCB bool
			names := make([]string, 0, len(listed))
			for _, r := range listed {
				names = append(names, r.Name)
				if strings.HasPrefix(r.Name, "urcb01") {
					sawURCB = true
					urcbName = r.Name
					urcbBuffered = r.Buffered
				}
				if strings.HasPrefix(r.Name, "brcb01") {
					sawBRCB = true
					brcbName = r.Name
					brcbBuffered = r.Buffered
				}
			}
			if !sawURCB || !sawBRCB {
				logCommandFailure(t, listRes, h)
				t.Fatalf("want urcb01* and brcb01* in list reports, got %v", names)
			}
			if urcbBuffered {
				t.Fatalf("urcb %q buffered=%v, want false", urcbName, urcbBuffered)
			}
			if !brcbBuffered {
				t.Fatalf("brcb %q buffered=%v, want true", brcbName, brcbBuffered)
			}

			assertGetReport := func(name string, wantBuffered bool) {
				t.Helper()
				getRes := runCtl(t, h.ctlEnv(), commandTimeout,
					"get", "report",
					"--ld", "InteropLD",
					"--ln", "LLN0",
					"--report", name,
					"--format", "json",
				)
				requireExitZero(t, getRes, h)

				var envelope struct {
					Report struct {
						Name     string `json:"name"`
						Buffered bool   `json:"buffered"`
						DatSet   string `json:"dat_set"`
						RptID    string `json:"rpt_id"`
					} `json:"report"`
				}
				if err := decodeExactJSON(getRes.Stdout, &envelope); err != nil {
					logCommandFailure(t, getRes, h)
					t.Fatalf("decode get report JSON for %s: %v", name, err)
				}
				if envelope.Report.Buffered != wantBuffered {
					logCommandFailure(t, getRes, h)
					t.Fatalf("%s buffered: got %v, want %v", name, envelope.Report.Buffered, wantBuffered)
				}
				if !strings.Contains(envelope.Report.DatSet, "dsInterop") {
					logCommandFailure(t, getRes, h)
					t.Fatalf("%s dat_set: got %q, want to contain dsInterop", name, envelope.Report.DatSet)
				}
			}

			assertGetReport(urcbName, false)
			assertGetReport(brcbName, true)
		})
	}
}
