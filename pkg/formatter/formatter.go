// SPDX-License-Identifier: MIT

// Package formatter provides configurable formatting for IEC 61850 data types
// (JSON, CSV, Table, YAML, text) and standalone format helpers.

package formatter

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/formatter/value"
)

// TimeFormat defines timestamp output formatting.
type TimeFormat int

const (
	// TimeFormatISO uses ISO 8601 format (2006-01-02T15:04:05Z).
	TimeFormatISO TimeFormat = iota
	// TimeFormatUnix uses Unix timestamp in milliseconds.
	TimeFormatUnix
	// TimeFormatRelative uses relative time (e.g., "2h ago").
	TimeFormatRelative
)

// ByteFormat defines file size formatting.
type ByteFormat int

const (
	// ByteFormatSI uses SI units (1 kB = 1000 bytes).
	ByteFormatSI ByteFormat = iota
	// ByteFormatIEC uses IEC units (1 KiB = 1024 bytes).
	ByteFormatIEC
	// ByteFormatBytes displays raw byte count.
	ByteFormatBytes
)

// OutputFormat defines structured output formatting (JSON, CSV, Table, YAML, text).
type OutputFormat int

const (
	// OutputFormatText uses human-readable text format (default).
	OutputFormatText OutputFormat = iota
	// OutputFormatJSON uses JSON format.
	OutputFormatJSON
	// OutputFormatCSV uses CSV format.
	OutputFormatCSV
	// OutputFormatTable uses aligned table format.
	OutputFormatTable
	// OutputFormatYAML uses YAML format.
	OutputFormatYAML
)

// Formatter provides configurable formatting of IEC 61850 data types.
type Formatter struct {
	timeFormat   TimeFormat
	byteFormat   ByteFormat
	outputFormat OutputFormat
}

// NewFormatter creates a new Formatter with default settings.
func NewFormatter() *Formatter {
	return &Formatter{
		timeFormat:   TimeFormatISO,
		byteFormat:   ByteFormatSI,
		outputFormat: OutputFormatText,
	}
}

// WithTimeFormat sets the timestamp formatting style (fluent API).
func (f *Formatter) WithTimeFormat(format TimeFormat) *Formatter {
	f.timeFormat = format
	return f
}

// WithByteFormat sets the file size formatting style (fluent API).
func (f *Formatter) WithByteFormat(format ByteFormat) *Formatter {
	f.byteFormat = format
	return f
}

// WithOutputFormat sets the structured output formatting style (fluent API).
func (f *Formatter) WithOutputFormat(format OutputFormat) *Formatter {
	f.outputFormat = format
	return f
}

// ParseOutputFormat parses a format string ("json", "csv", "table", "yaml", "text") into OutputFormat.
// Returns (format, true) on success, or (OutputFormatText, false) for unknown values.
func ParseOutputFormat(s string) (OutputFormat, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return OutputFormatJSON, true
	case "csv":
		return OutputFormatCSV, true
	case "table":
		return OutputFormatTable, true
	case "yaml":
		return OutputFormatYAML, true
	case "text":
		return OutputFormatText, true
	default:
		return OutputFormatText, false
	}
}

// FileSize formats a byte count into a human-readable string.
func (f *Formatter) FileSize(bytes uint64) string {
	switch f.byteFormat {
	case ByteFormatBytes:
		return fmt.Sprintf("%d bytes", bytes)
	case ByteFormatIEC:
		return f.fileSizeIEC(bytes)
	default:
		return f.fileSizeSI(bytes)
	}
}

func (f *Formatter) fileSizeSI(bytes uint64) string {
	const unit = 1000
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "kMGT"[exp])
}

func (f *Formatter) fileSizeIEC(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGT"[exp])
}

// Timestamp formats a Unix milliseconds timestamp.
func (f *Formatter) Timestamp(unixMs uint64) string {
	t := time.Unix(0, int64(unixMs)*int64(time.Millisecond))
	switch f.timeFormat {
	case TimeFormatUnix:
		return fmt.Sprintf("%d", unixMs)
	case TimeFormatRelative:
		return f.relativeTime(t)
	default:
		return t.UTC().Format(time.RFC3339)
	}
}

func (f *Formatter) relativeTime(t time.Time) string {
	dur := time.Since(t)
	if dur < 0 {
		dur = -dur
		return fmt.Sprintf("in %s", f.formatDuration(dur))
	}
	return fmt.Sprintf("%s ago", f.formatDuration(dur))
}

func (f *Formatter) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// BinaryTime formats a binary time value (Unix ms as interface{}) into ISO string.
func (f *Formatter) BinaryTime(value interface{}) string {
	switch v := value.(type) {
	case []byte:
		if len(v) == 6 {
			rawMs := uint64(v[0])<<40 | uint64(v[1])<<32 | uint64(v[2])<<24 |
				uint64(v[3])<<16 | uint64(v[4])<<8 | uint64(v[5])
			return f.Timestamp(rawMs)
		}
		if len(v) == 4 {
			secs := binary.BigEndian.Uint32(v)
			return f.Timestamp(uint64(secs) * 1000)
		}
	case uint64:
		return f.Timestamp(v)
	case int64:
		return f.Timestamp(uint64(v))
	}
	return fmt.Sprintf("%v", value)
}

// UtcTimeValue formats an IEC 61850 UTC_TIME value for display using the formatter's time style.
func (f *Formatter) UtcTimeValue(ts domain.Timestamp) string {
	tsStr := f.Timestamp(ts.UnixMs)
	qualityStr := f.timeQualityFlags(ts.TimeQuality)
	return tsStr + qualityStr
}

func (f *Formatter) timeQualityFlags(q uint8) string {
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
	return " [" + strings.Join(s, " ") + "]"
}

// DataAttribute formats a domain.DataAttribute into a display string.
func (f *Formatter) DataAttribute(da *domain.DataAttribute) string {
	if da == nil {
		return ""
	}
	return value.FormatDataAttributeValue(da)
}

// TypeSpec formats a go-mms TypeSpec into a human-readable type string.
func (f *Formatter) TypeSpec(spec *mms.TypeSpec) string {
	return value.FormatTypeSpec(spec)
}

// LeafValue formats a leaf value for display based on its MMS type.
func (f *Formatter) LeafValue(val interface{}, mmsType domain.MmsDataType, attrNameOrPath string) string {
	if val == nil {
		return "(null)"
	}
	if ts, ok := val.(domain.Timestamp); ok {
		return value.FormatUtcTimestamp(ts)
	}
	if ts, ok := val.(*domain.Timestamp); ok && ts != nil {
		return value.FormatUtcTimestamp(*ts)
	}
	return value.FormatLeafValue(val, mmsType, attrNameOrPath)
}

// FormatFileSize formats a size in bytes to a human-readable string (KB, MB, GB).
func FormatFileSize(bytes uint64) string {
	return value.FormatFileSize(bytes)
}

// FormatMmsValue converts a go-mms Value to a display string.
func FormatMmsValue(mv *mms.Value) string {
	return value.FormatMmsValue(mv)
}

// FormatDomainValue converts a domain.Value to a display string.
func FormatDomainValue(v *domain.Value) string {
	return value.FormatDomainValue(v)
}

// FormatBinaryTime formats a binary time value (milliseconds since epoch or raw bytes).
func FormatBinaryTime(v interface{}) string {
	return value.FormatBinaryTime(v)
}

// FormatTypeSpec formats a go-mms TypeSpec to a type string.
func FormatTypeSpec(spec *mms.TypeSpec) string {
	return value.FormatTypeSpec(spec)
}

// FormatUtcTimeValue formats an IEC 61850 UTC_TIME value with 'UTC' suffix.
func FormatUtcTimeValue(ms uint64, timeQuality uint8) string {
	return value.FormatUtcTimeValue(ms, timeQuality)
}

// FormatUtcTimestamp formats a domain.Timestamp for display.
func FormatUtcTimestamp(ts domain.Timestamp) string {
	return value.FormatUtcTimestamp(ts)
}

// FormatUnixTimestamp formats a Unix timestamp value to human-readable string.
func FormatUnixTimestamp(v interface{}) string {
	return value.FormatUnixTimestamp(v)
}

// FormatQualityValue formats a quality bitfield value to human-readable flags.
func FormatQualityValue(v interface{}) string {
	return value.FormatQualityValue(v)
}

// FormatLeafValue formats a leaf attribute value for display by MMS type.
func FormatLeafValue(v interface{}, mmsType domain.MmsDataType, attrNameOrPath string) string {
	return value.FormatLeafValue(v, mmsType, attrNameOrPath)
}

// FormatDataAttributeValue formats a DataAttribute into a display string.
func FormatDataAttributeValue(da *domain.DataAttribute) string {
	return value.FormatDataAttributeValue(da)
}
