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

// Phase 5 forward BRCB journeys (isolated servers).
//
// | Assertion                                      | libiec61850 | iec61850bean |
// |------------------------------------------------|-------------|--------------|
// | get report exposes BRCB EntryID opt field      | required    | required     |
// | subscribe --type BR --purge-buf + GI           | required    | required     |
// | resume with --entry-id from prior report       | required    | best-effort  |

func TestBRCB_ClientPurgeAndResume(t *testing.T) {
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
				t.Fatalf("start isolated %s: %v", stack, err)
			}
			t.Cleanup(h.stop)

			brcbName := discoverBRCB(t, h)
			get := runCtl(t, h.ctlEnv(), commandTimeout,
				"get", "report",
				"--ld", "InteropLD",
				"--ln", "LLN0",
				"--report", brcbName,
				"--format", "json",
			)
			requireExitZero(t, get, h)
			var envelope struct {
				Report struct {
					Buffered       bool `json:"buffered"`
					OptionalFields struct {
						EntryID        bool `json:"entry_id"`
						BufferOverflow bool `json:"buffer_overflow"`
					} `json:"optional_fields"`
				} `json:"report"`
			}
			if err := decodeExactJSON(get.Stdout, &envelope); err != nil {
				t.Fatalf("decode get report: %v", err)
			}
			if !envelope.Report.Buffered {
				t.Fatal("expected buffered RCB")
			}
			if !envelope.Report.OptionalFields.EntryID {
				t.Fatal("expected optional_fields.entry_id true for BRCB")
			}

			// Purge + enable + GI: receive at least one report before any resume claim.
			sub1 := runCtl(t, h.ctlEnv(), 30*time.Second,
				"subscribe", "report",
				"--ld", "InteropLD",
				"--ln", "LLN0",
				"--report", brcbName,
				"--type", "BR",
				"--purge-buf",
				"--interrogation",
				"--max-reports", "1",
				"--duration", "15s",
				"--format", "jsonl",
			)
			if sub1.ExitCode != 0 {
				logCommandFailure(t, sub1, h)
				t.Fatalf("purge subscribe exit=%d", sub1.ExitCode)
			}
			entryID := firstReportEntryID(t, sub1.Stdout)
			if entryID == "" && stack == stackLibIEC61850 {
				t.Fatal("libiec61850 BRCB report missing entry_id")
			}

			if entryID == "" {
				// Bean may omit EntryID in indications; purge path already validated.
				return
			}

			sub2 := runCtl(t, h.ctlEnv(), 30*time.Second,
				"subscribe", "report",
				"--ld", "InteropLD",
				"--ln", "LLN0",
				"--report", brcbName,
				"--type", "BR",
				"--entry-id", entryID,
				"--interrogation",
				"--max-reports", "1",
				"--duration", "15s",
				"--format", "jsonl",
			)
			if sub2.ExitCode != 0 {
				logCommandFailure(t, sub2, h)
				t.Fatalf("resume subscribe exit=%d", sub2.ExitCode)
			}
			if !jsonlHasEvent(sub2.Stdout, "report") {
				t.Fatal("resume subscribe: no report event")
			}
		})
	}
}

func discoverBRCB(t *testing.T, h *serverHandle) string {
	t.Helper()
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
	for _, r := range listed {
		if r.Buffered && strings.HasPrefix(r.Name, "brcb01") {
			return r.Name
		}
	}
	t.Fatalf("no BRCB brcb01* in %v", listed)
	return ""
}

func firstReportEntryID(t *testing.T, stdout []byte) string {
	t.Helper()
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev["event"] != "report" {
			continue
		}
		if id, ok := ev["entry_id"].(string); ok {
			return id
		}
	}
	return ""
}

func jsonlHasEvent(stdout []byte, event string) bool {
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev["event"] == event {
			return true
		}
	}
	return false
}
