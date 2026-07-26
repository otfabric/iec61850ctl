// SPDX-License-Identifier: MIT

package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestValueToModel_UTCTime_SerializesTimeQuality(t *testing.T) {
	ts := time.UnixMilli(1662761017061) // 2022-09-09 22:03:37.061 UTC
	val, errMsg := ValueToModel(ts, mms.ValueTypeUTCTime)
	if errMsg != "" {
		t.Fatalf("ValueToModel: %s", errMsg)
	}
	if val == nil || val.Type != domain.TypeUtcTime {
		t.Fatalf("expected type UTC_TIME, got %v", val)
	}
	jsonBytes, err := json.Marshal(val.Raw)
	if err != nil {
		t.Fatalf("marshal UTC_TIME from Raw: %v", err)
	}
	var decoded struct {
		Seconds      int64  `json:"seconds"`
		Milliseconds uint16 `json:"milliseconds"`
		TimeQuality  uint8  `json:"time_quality"`
	}
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("unmarshal UTC_TIME JSON: %v", err)
	}
	if decoded.Seconds != 1662761017 {
		t.Errorf("seconds = %d, want 1662761017", decoded.Seconds)
	}
	if decoded.Milliseconds != 61 {
		t.Errorf("milliseconds = %d, want 61", decoded.Milliseconds)
	}
}
