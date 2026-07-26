// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"fmt"
)

// ReadyEvent is the structured readiness contract emitted when --ready-json is set.
type ReadyEvent struct {
	Event   string `json:"event"`
	Address string `json:"address"`
	Fixture string `json:"fixture,omitempty"`
	Adapter string `json:"adapter"`
	Version string `json:"version"`
	IEDName string `json:"ied_name"`
}

// EncodeReadyEvent returns one JSON line for a readiness event.
func EncodeReadyEvent(address, fixtureID, version, iedName string) ([]byte, error) {
	ev := ReadyEvent{
		Event:   "ready",
		Address: address,
		Fixture: fixtureID,
		Adapter: "iec61850ctl",
		Version: version,
		IEDName: iedName,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("marshal ready event: %w", err)
	}
	return append(b, '\n'), nil
}
