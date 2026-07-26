// SPDX-License-Identifier: MIT

package value

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// timeQualityFlags decodes the IEC 61850 TimeQuality byte into human-readable flags.
// bit 7 = leapSecondsKnown, bit 6 = clockFailure, bit 5 = clockNotSynchronized, bits 0..4 = accuracy class.
func timeQualityFlags(q uint8) string {
	leapSecondsKnown := q&0x80 != 0
	clockFailure := q&0x40 != 0
	clockNotSynchronized := q&0x20 != 0
	accuracyClass := q & 0x1F
	var s []string
	if clockFailure {
		s = append(s, "clock-failure")
	}
	if clockNotSynchronized {
		s = append(s, "clock-not-synced")
	}
	if !leapSecondsKnown {
		s = append(s, "leap-unknown")
	}
	if accuracyClass > 0 {
		s = append(s, fmt.Sprintf("acc<=%dms", 1000/(1<<accuracyClass)))
	}
	if len(s) == 0 {
		return ""
	}
	return " [" + joinStrings(s, " ") + "]"
}

// joinStrings joins a slice of strings with a separator without using strings.Join.
func joinStrings(a []string, sep string) string {
	if len(a) == 0 {
		return ""
	}
	s := a[0]
	for i := 1; i < len(a); i++ {
		s += sep + a[i]
	}
	return s
}

// FormatUtcTimeValue formats an IEC 61850 UTC_TIME value for display.
// Accepts milliseconds since epoch and the TimeQuality byte.
func FormatUtcTimeValue(ms uint64, timeQuality uint8) string {
	sec := ms / 1000
	nsec := (ms % 1000) * 1e6
	t := time.Unix(int64(sec), int64(nsec)).UTC()
	flags := timeQualityFlags(timeQuality)
	return t.Format("2006-01-02 15:04:05.000") + " UTC" + flags
}

// FormatUtcTimeFromTime formats a UTC time with optional TimeQuality byte.
func FormatUtcTimeFromTime(t time.Time, timeQuality uint8) string {
	return FormatUtcTimeValue(uint64(t.UTC().UnixMilli()), timeQuality)
}

// FormatUtcTimestamp formats a domain.Timestamp for display.
func FormatUtcTimestamp(ts domain.Timestamp) string {
	return FormatUtcTimeValue(ts.UnixMs, ts.TimeQuality)
}

// FormatBinaryTime formats IEC 61850 BINARY_TIME for display.
// []byte: 4 bytes = ms since midnight; 6 bytes = days since 1984-01-01 + ms (raw MMS).
// uint64: milliseconds since Unix epoch (1970-01-01).
func FormatBinaryTime(value interface{}) string {
	if value == nil {
		return "(nil)"
	}
	switch v := value.(type) {
	case []byte:
		switch len(v) {
		case 4:
			ms := binary.BigEndian.Uint32(v)
			return fmt.Sprintf("ms-since-midnight=%d (%02d:%02d:%02d.%03d)", ms, ms/3600000, (ms/60000)%60, (ms/1000)%60, ms%1000)
		case 6:
			days := binary.BigEndian.Uint16(v[0:2])
			ms := binary.BigEndian.Uint32(v[2:6])
			const mmsEpochSec = 441763200
			sec := int64(days)*86400 + mmsEpochSec + int64(ms/1000)
			nsec := int64(ms%1000) * 1e6
			t := time.Unix(sec, nsec).UTC()
			return t.Format("2006-01-02 15:04:05.000") + fmt.Sprintf(" (days=%d ms=%d)", days, ms)
		default:
			return fmt.Sprintf("%x (raw BINARY_TIME, len=%d)", v, len(v))
		}
	case uint64:
		if v == 0 {
			return "Unknown"
		}
		sec := int64(v / 1000)
		nsec := int64((v % 1000) * 1e6)
		t := time.Unix(sec, nsec).UTC()
		return t.Format("2006-01-02 15:04:05.000") + fmt.Sprintf(" (ms=%d)", v)
	case int64:
		if v == 0 {
			return "Unknown"
		}
		return FormatBinaryTime(uint64(v))
	case uint32, int:
		return FormatUnixTimestamp(value)
	default:
		return fmt.Sprintf("%v (raw)", value)
	}
}

// FormatUnixTimestamp converts a Unix timestamp (seconds since 1970-01-01) to a human-readable string.
// Supports int64, uint64, int32, uint32, int. Returns RFC 3339 format with Unix timestamp in parentheses.
func FormatUnixTimestamp(value interface{}) string {
	var unixSec int64

	switch v := value.(type) {
	case int64:
		unixSec = v
	case uint64:
		unixSec = int64(v)
	case int32:
		unixSec = int64(v)
	case uint32:
		unixSec = int64(v)
	case int:
		unixSec = int64(v)
	default:
		return fmt.Sprintf("%v (raw)", value)
	}

	t := time.Unix(unixSec, 0).UTC()
	return fmt.Sprintf("%s (%d)", t.Format("2006-01-02 15:04:05 UTC"), unixSec)
}
