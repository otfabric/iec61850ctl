// SPDX-License-Identifier: MIT

package domain

import (
	"fmt"
	"strings"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
)

// Timestamp wraps an IEC 61850 timestamp with decoded quality flags.
type Timestamp struct {
	UnixMs               uint64
	TimeQuality          uint8
	LeapSecondKnown      bool
	ClockFailure         bool
	ClockNotSynchronized bool
	SubSecondPrecision   int
}

func (t Timestamp) Time() time.Time {
	sec := int64(t.UnixMs / 1000)
	nsec := int64((t.UnixMs % 1000) * 1_000_000)
	return time.Unix(sec, nsec).UTC()
}

func (t Timestamp) String() string {
	s := t.Time().Format("2006-01-02 15:04:05.000 UTC")
	var flags []string
	if t.ClockFailure {
		flags = append(flags, "clock-failure")
	}
	if t.ClockNotSynchronized {
		flags = append(flags, "clock-not-synchronized")
	}
	if !t.LeapSecondKnown {
		flags = append(flags, "leap-second-unknown")
	}
	if len(flags) > 0 {
		s += " [" + strings.Join(flags, ", ") + "]"
	}
	return s
}

func (t Timestamp) RFC3339() string {
	return t.Time().Format(time.RFC3339Nano)
}

func (t Timestamp) IsReliable() bool {
	return !t.ClockFailure && !t.ClockNotSynchronized
}

// FromLibTimestamp converts go-iec61850.Timestamp to domain Timestamp.
func FromLibTimestamp(ts iec61850.Timestamp) Timestamp {
	if ts.IsZero() {
		return Timestamp{}
	}
	return Timestamp{
		UnixMs:               uint64(ts.Time.UnixMilli()),
		LeapSecondKnown:      ts.Quality.LeapSecondKnown,
		ClockFailure:         ts.Quality.ClockFailure,
		ClockNotSynchronized: ts.Quality.ClockNotSynchronized,
		SubSecondPrecision:   ts.Quality.TimeAccuracy,
	}
}

// FromUtcTimeValue builds a Timestamp from milliseconds and a time-quality byte.
func FromUtcTimeValue(ms uint64, tq uint8) Timestamp {
	return Timestamp{
		UnixMs:               ms,
		TimeQuality:          tq,
		LeapSecondKnown:      tq&0x80 != 0,
		ClockFailure:         tq&0x40 != 0,
		ClockNotSynchronized: tq&0x20 != 0,
		SubSecondPrecision:   int(tq & 0x1F),
	}
}

func FormatTimestampMs(ms uint64) string {
	if ms == 0 {
		return ""
	}
	sec := int64(ms / 1000)
	nsec := int64((ms % 1000) * 1_000_000)
	return time.Unix(sec, nsec).UTC().Format("2006-01-02 15:04:05.000 UTC")
}

const TimeFormatForJournal = "2006-01-02 15:04:05.000"

func ParseTimeToUnixMs(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty time string")
	}
	if isDigitsOnly(s) {
		var ms uint64
		if _, err := fmt.Sscanf(s, "%d", &ms); err != nil {
			return 0, fmt.Errorf("invalid number: %w", err)
		}
		return ms, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return uint64(t.UnixNano() / 1e6), nil
		}
	}
	for _, layout := range []string{"2006-01-02 15:04:05.000", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return uint64(t.UnixNano() / 1e6), nil
		}
	}
	return 0, fmt.Errorf("unsupported time format: use RFC3339 (e.g. 2024-10-30T12:00:00Z), %q, or milliseconds", TimeFormatForJournal)
}

func isDigitsOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
