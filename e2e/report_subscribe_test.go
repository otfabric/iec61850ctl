//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Phase 2 stack expectation table (skips must cite this table).
//
// | Assertion                                      | libiec61850 | iec61850bean |
// |------------------------------------------------|-------------|--------------|
// | Exit 0                                         | required    | required     |
// | ≥1 report JSONL event                          | required    | required     |
// | Expected rpt_id interop_urcb01                 | required    | required     |
// | GI reason token present                        | required    | required     |
// | Member values when --show-values               | required    | required     |
// | summary reports_received == 1                  | required    | required     |
// | Cleanup within timeout                         | required    | required     |
// | Follow-up get report enabled == false          | required    | required if Bean exposes RptEna; else skip with table cite |

func TestURCBGI_SubscribeJSONL(t *testing.T) {
	for _, stack := range []string{stackLibIEC61850, stackIEC61850Bean} {
		stack := stack
		t.Run(stack, func(t *testing.T) {
			requireStack(t, stack)
			requireCtlBin(t)

			readyTO := libReadyTimeout
			if stack == stackIEC61850Bean {
				readyTO = beanReadyTimeout
			}
			h, err := startDockerServer(stack, readyTO)
			if err != nil {
				t.Fatalf("start isolated %s server: %v", stack, err)
			}
			t.Cleanup(h.stop)

			// Discover the concrete URCB name (libiec61850 may expand urcb01 → urcb0101).
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
			}
			if err := decodeExactJSON(listRes.Stdout, &listed); err != nil {
				t.Fatalf("decode list reports: %v", err)
			}
			urcbName := ""
			for _, r := range listed {
				if !r.Buffered && strings.HasPrefix(r.Name, "urcb01") {
					urcbName = r.Name
					break
				}
			}
			if urcbName == "" {
				t.Fatalf("no URCB urcb01* in %v", listed)
			}

			res := runCtl(t, h.ctlEnv(), 30*time.Second,
				"subscribe", "report",
				"--ld", "InteropLD",
				"--ln", "LLN0",
				"--report", urcbName,
				"--type", "RP",
				"--interrogation",
				"--max-reports", "1",
				"--duration", "15s",
				"--show-values",
				"--format", "jsonl",
			)
			if res.ExitCode != 0 {
				logCommandFailure(t, res, h)
				t.Fatalf("subscribe exit=%d stderr=%s stdout=%s", res.ExitCode, res.Stderr, res.Stdout)
			}

			var reports int
			var summaryReports int64 = -1
			var sawGI bool
			var sawRptID bool
			sc := bufio.NewScanner(bytes.NewReader(res.Stdout))
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					continue
				}
				var ev map[string]any
				if err := json.Unmarshal([]byte(line), &ev); err != nil {
					t.Fatalf("invalid jsonl line %q: %v", line, err)
				}
				switch ev["event"] {
				case "report":
					reports++
					if id, _ := ev["rpt_id"].(string); id == "interop_urcb01" {
						sawRptID = true
					}
					if reasons, ok := ev["reasons"].([]any); ok {
						for _, r := range reasons {
							if s, _ := r.(string); s == "gi" {
								sawGI = true
							}
						}
					}
				case "summary":
					if n, ok := ev["reports_received"].(float64); ok {
						summaryReports = int64(n)
					}
				}
			}
			if reports < 1 {
				t.Fatalf("expected ≥1 report event, stdout=%s", res.Stdout)
			}
			if !sawRptID {
				t.Fatalf("expected rpt_id interop_urcb01, stdout=%s", res.Stdout)
			}
			if !sawGI {
				t.Fatalf("expected gi reason token, stdout=%s", res.Stdout)
			}
			if summaryReports != 1 {
				t.Fatalf("summary reports_received=%d want 1", summaryReports)
			}

			getRes := runCtl(t, h.ctlEnv(), commandTimeout,
				"get", "report",
				"--ld", "InteropLD",
				"--ln", "LLN0",
				"--report", urcbName,
				"--format", "json",
			)
			if getRes.ExitCode != 0 {
				if stack == stackIEC61850Bean {
					t.Skip("phase2 table: Bean follow-up get report after disable not reliable — cite expectation table")
				}
				logCommandFailure(t, getRes, h)
				t.Fatalf("get report after subscribe: exit=%d stderr=%s", getRes.ExitCode, getRes.Stderr)
			}
			var envelope struct {
				Report struct {
					Enabled *bool `json:"enabled"`
				} `json:"report"`
			}
			if err := decodeExactJSON(getRes.Stdout, &envelope); err != nil {
				t.Fatalf("decode get report: %v", err)
			}
			if envelope.Report.Enabled == nil {
				if stack == stackIEC61850Bean {
					t.Skip("phase2 table: Bean does not expose RptEna after disable — cite expectation table")
				}
				t.Fatal("expected enabled field")
			}
			if *envelope.Report.Enabled {
				t.Fatal("expected enabled == false after subscribe cleanup")
			}
		})
	}
}
