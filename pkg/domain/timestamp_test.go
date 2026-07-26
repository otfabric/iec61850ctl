// SPDX-License-Identifier: MIT

package domain

import (
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
)

func TestParseTimeToUnixMs(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    uint64
		wantErr string
	}{
		{
			name: "RFC3339",
			in:   "2024-10-30T12:00:00Z",
			want: uint64(time.Date(2024, 10, 30, 12, 0, 0, 0, time.UTC).UnixMilli()),
		},
		{
			name: "RFC3339Nano",
			in:   "2024-10-30T12:00:00.123Z",
			want: uint64(time.Date(2024, 10, 30, 12, 0, 0, 123000000, time.UTC).UnixMilli()),
		},
		{
			name: "T without Z",
			in:   "2024-10-30T12:00:00",
			want: uint64(time.Date(2024, 10, 30, 12, 0, 0, 0, time.UTC).UnixMilli()),
		},
		{
			name: "space UTC with ms",
			in:   "2024-10-30 12:00:00.123",
			want: uint64(time.Date(2024, 10, 30, 12, 0, 0, 123000000, time.UTC).UnixMilli()),
		},
		{
			name: "space UTC without ms",
			in:   "2024-10-30 12:00:00",
			want: uint64(time.Date(2024, 10, 30, 12, 0, 0, 0, time.UTC).UnixMilli()),
		},
		{
			name: "ms epoch",
			in:   "1737646499123",
			want: 1737646499123,
		},
		{
			name: "trimmed digits",
			in:   "  42  ",
			want: 42,
		},
		{
			name:    "empty",
			in:      "   ",
			wantErr: "empty time string",
		},
		{
			name:    "unsupported",
			in:      "not-a-time",
			wantErr: "unsupported time format",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeToUnixMs(tt.in)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseTimeToUnixMs(%q) error = nil, want %q", tt.in, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseTimeToUnixMs(%q) error = %v, want containing %q", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTimeToUnixMs(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseTimeToUnixMs(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatTimestampMs(t *testing.T) {
	if got := FormatTimestampMs(0); got != "" {
		t.Errorf("FormatTimestampMs(0) = %q, want \"\"", got)
	}
	got := FormatTimestampMs(1737646499123)
	want := time.UnixMilli(1737646499123).UTC().Format("2006-01-02 15:04:05.000 UTC")
	if got != want {
		t.Errorf("FormatTimestampMs() = %q, want %q", got, want)
	}
}

func TestFromUtcTimeValue(t *testing.T) {
	ts := FromUtcTimeValue(1_700_000_000_000, 0x80|0x40|0x20|0x0A)
	if ts.UnixMs != 1_700_000_000_000 {
		t.Errorf("UnixMs = %d", ts.UnixMs)
	}
	if ts.TimeQuality != 0xE0|0x0A {
		t.Errorf("TimeQuality = %#x", ts.TimeQuality)
	}
	if !ts.LeapSecondKnown {
		t.Error("LeapSecondKnown = false")
	}
	if !ts.ClockFailure {
		t.Error("ClockFailure = false")
	}
	if !ts.ClockNotSynchronized {
		t.Error("ClockNotSynchronized = false")
	}
	if ts.SubSecondPrecision != 0x0A {
		t.Errorf("SubSecondPrecision = %d, want 10", ts.SubSecondPrecision)
	}

	clean := FromUtcTimeValue(1000, 0)
	if clean.LeapSecondKnown || clean.ClockFailure || clean.ClockNotSynchronized {
		t.Errorf("unexpected flags on clean quality: %+v", clean)
	}
}

func TestTimestamp_Methods(t *testing.T) {
	ts := Timestamp{
		UnixMs:               1_700_000_000_123,
		LeapSecondKnown:      false,
		ClockFailure:         true,
		ClockNotSynchronized: true,
		SubSecondPrecision:   10,
	}

	gotTime := ts.Time()
	wantTime := time.UnixMilli(1_700_000_000_123).UTC()
	if !gotTime.Equal(wantTime) {
		t.Errorf("Time() = %v, want %v", gotTime, wantTime)
	}

	s := ts.String()
	if !strings.Contains(s, "UTC") {
		t.Errorf("String() = %q, want containing UTC", s)
	}
	if !strings.Contains(s, "clock-failure") || !strings.Contains(s, "clock-not-synchronized") || !strings.Contains(s, "leap-second-unknown") {
		t.Errorf("String() = %q, want quality flags", s)
	}

	reliable := Timestamp{UnixMs: 1000, LeapSecondKnown: true}
	if !strings.Contains(reliable.String(), "UTC") {
		t.Errorf("reliable String() = %q", reliable.String())
	}
	if strings.Contains(reliable.String(), "[") {
		t.Errorf("reliable String() unexpectedly has flags: %q", reliable.String())
	}

	if got := ts.RFC3339(); got != wantTime.Format(time.RFC3339Nano) {
		t.Errorf("RFC3339() = %q, want %q", got, wantTime.Format(time.RFC3339Nano))
	}

	if ts.IsReliable() {
		t.Error("IsReliable() = true, want false")
	}
	if !reliable.IsReliable() {
		t.Error("IsReliable() = false for clean timestamp")
	}
}

func TestFromLibTimestamp(t *testing.T) {
	if got := FromLibTimestamp(iec61850.Timestamp{}); got != (Timestamp{}) {
		t.Errorf("FromLibTimestamp(zero) = %+v, want zero", got)
	}

	lib := iec61850.Timestamp{
		Time: time.UnixMilli(1_700_000_000_000).UTC(),
		Quality: iec61850.TimeQuality{
			LeapSecondKnown:      true,
			ClockFailure:         true,
			ClockNotSynchronized: false,
			TimeAccuracy:         12,
		},
	}
	got := FromLibTimestamp(lib)
	if got.UnixMs != 1_700_000_000_000 {
		t.Errorf("UnixMs = %d", got.UnixMs)
	}
	if !got.LeapSecondKnown || !got.ClockFailure || got.ClockNotSynchronized {
		t.Errorf("flags = %+v", got)
	}
	if got.SubSecondPrecision != 12 {
		t.Errorf("SubSecondPrecision = %d, want 12", got.SubSecondPrecision)
	}
}

func TestIsDigitsOnly(t *testing.T) {
	if !isDigitsOnly("123") {
		t.Error("isDigitsOnly(123) = false")
	}
	if isDigitsOnly("") {
		t.Error("isDigitsOnly(\"\") = true")
	}
	if isDigitsOnly("12a") {
		t.Error("isDigitsOnly(12a) = true")
	}
}
