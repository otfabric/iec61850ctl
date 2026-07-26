// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"testing"
)

func TestEncodeReadyEvent(t *testing.T) {
	b, err := EncodeReadyEvent("127.0.0.1:1102", "iec61850-v2", "abc123", "InteropIED")
	if err != nil {
		t.Fatal(err)
	}
	if b[len(b)-1] != '\n' {
		t.Fatal("expected trailing newline")
	}
	var ev ReadyEvent
	if err := json.Unmarshal(b[:len(b)-1], &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Event != "ready" || ev.Adapter != "iec61850ctl" || ev.IEDName != "InteropIED" {
		t.Fatalf("unexpected event: %+v", ev)
	}
	if ev.Fixture != "iec61850-v2" || ev.Address != "127.0.0.1:1102" || ev.Version != "abc123" {
		t.Fatalf("unexpected fields: %+v", ev)
	}
}

func TestEncodeReadyEvent_OmitsEmptyFixture(t *testing.T) {
	b, err := EncodeReadyEvent("0.0.0.0:102", "", "dev", "IED1")
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b[:len(b)-1], &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["fixture"]; ok {
		t.Fatalf("fixture should be omitted when empty, got %v", raw)
	}
}
