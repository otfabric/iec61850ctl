// SPDX-License-Identifier: MIT

package value

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

func TestFormatUtcTimeValue(t *testing.T) {
	const ms = uint64(1737646499123) // 2025-01-23 15:34:59.123 UTC

	tests := []struct {
		name         string
		ms           uint64
		tq           uint8
		wantContains []string
		wantAbsent   []string
	}{
		{
			"leap unknown only",
			ms, 0x00,
			[]string{"2025-01-23 15:34:59.123 UTC", "leap-unknown"},
			[]string{"clock-failure", "clock-not-synced", "acc<="},
		},
		{
			"leap known no flags",
			ms, 0x80,
			[]string{"2025-01-23 15:34:59.123 UTC"},
			[]string{"[", "leap-unknown", "clock-failure"},
		},
		{
			"clock failure",
			ms, 0x40,
			[]string{"clock-failure", "leap-unknown"},
			nil,
		},
		{
			"clock not synced",
			ms, 0x20,
			[]string{"clock-not-synced", "leap-unknown"},
			nil,
		},
		{
			"accuracy class",
			ms, 0x80 | 0x0A, // leap known + accuracy class 10
			[]string{"acc<=", "UTC"},
			[]string{"leap-unknown"},
		},
		{
			"all quality bits",
			ms, 0x40 | 0x20 | 0x05, // failure + not synced + leap unknown + acc
			[]string{"clock-failure", "clock-not-synced", "leap-unknown", "acc<="},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatUtcTimeValue(tt.ms, tt.tq)
			for _, sub := range tt.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("FormatUtcTimeValue() = %q, want containing %q", got, sub)
				}
			}
			for _, sub := range tt.wantAbsent {
				if strings.Contains(got, sub) {
					t.Errorf("FormatUtcTimeValue() = %q, want absent %q", got, sub)
				}
			}
		})
	}
}

func TestFormatUtcTimeFromTime(t *testing.T) {
	tm := time.UnixMilli(1737646499123).UTC()
	got := FormatUtcTimeFromTime(tm, 0x80)
	if !strings.Contains(got, "2025-01-23 15:34:59.123 UTC") {
		t.Errorf("FormatUtcTimeFromTime() = %q", got)
	}
	if strings.Contains(got, "[") {
		t.Errorf("FormatUtcTimeFromTime() unexpected flags: %q", got)
	}
}

func TestFormatUtcTimestamp(t *testing.T) {
	ts := domain.Timestamp{UnixMs: 1737646499123, TimeQuality: 0x00}
	got := FormatUtcTimestamp(ts)
	if !strings.Contains(got, "UTC") || !strings.Contains(got, "leap-unknown") {
		t.Errorf("FormatUtcTimestamp() = %q", got)
	}
}

func TestFormatBinaryTime(t *testing.T) {
	four := make([]byte, 4)
	binary.BigEndian.PutUint32(four, 3661001) // 01:01:01.001

	six := make([]byte, 6)
	binary.BigEndian.PutUint16(six[0:2], 1) // 1 day since 1984-01-01
	binary.BigEndian.PutUint32(six[2:6], 0)

	tests := []struct {
		name       string
		value      interface{}
		wantExact  string
		wantSubstr []string
	}{
		{"nil", nil, "(nil)", nil},
		{
			"4-byte midnight ms",
			four,
			"",
			[]string{"ms-since-midnight=3661001", "01:01:01.001"},
		},
		{
			"6-byte days+ms",
			six,
			"",
			[]string{"days=1", "ms=0", "1984-01-02"},
		},
		{
			"odd length bytes",
			[]byte{0x01, 0x02, 0x03},
			"",
			[]string{"raw BINARY_TIME", "len=3"},
		},
		{"uint64 zero", uint64(0), "Unknown", nil},
		{
			"uint64 nonzero",
			uint64(1737646499123),
			"",
			[]string{"2025-01-23 15:34:59.123", "(ms=1737646499123)"},
		},
		{"int64 zero", int64(0), "Unknown", nil},
		{
			"int64 nonzero",
			int64(1737646499123),
			"",
			[]string{"2025-01-23 15:34:59.123", "(ms=1737646499123)"},
		},
		{
			"uint32 unix seconds",
			uint32(1730289600),
			"",
			[]string{"2024-10-30 12:00:00 UTC", "(1730289600)"},
		},
		{
			"int unix seconds",
			int(1730289600),
			"",
			[]string{"2024-10-30 12:00:00 UTC", "(1730289600)"},
		},
		{"unsupported", "nope", "nope (raw)", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBinaryTime(tt.value)
			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("FormatBinaryTime() = %q, want %q", got, tt.wantExact)
			}
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(got, sub) {
					t.Errorf("FormatBinaryTime() = %q, want containing %q", got, sub)
				}
			}
		})
	}
}

func TestFormatUnixTimestamp(t *testing.T) {
	const sec int64 = 1730289600 // 2024-10-30 12:00:00 UTC

	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"int64", int64(sec), "2024-10-30 12:00:00 UTC (1730289600)"},
		{"uint64", uint64(sec), "2024-10-30 12:00:00 UTC (1730289600)"},
		{"int32", int32(sec), "2024-10-30 12:00:00 UTC (1730289600)"},
		{"uint32", uint32(sec), "2024-10-30 12:00:00 UTC (1730289600)"},
		{"int", int(sec), "2024-10-30 12:00:00 UTC (1730289600)"},
		{"unsupported", "abc", "abc (raw)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatUnixTimestamp(tt.value)
			if got != tt.want {
				t.Errorf("FormatUnixTimestamp(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
