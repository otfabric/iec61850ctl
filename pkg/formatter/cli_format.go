// SPDX-License-Identifier: MIT

package formatter

import (
	"fmt"
	"strings"
)

// ParseCLIFormat accepts only text or json. Unknown formats return an error
// (no silent fallback to text).
func ParseCLIFormat(s string) (OutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "text":
		return OutputFormatText, nil
	case "json":
		return OutputFormatJSON, nil
	default:
		return OutputFormatText, fmt.Errorf("invalid --format %q (supported: text, json)", s)
	}
}

// StreamFormat identifies streaming output modes.
type StreamFormat string

const (
	StreamFormatText  StreamFormat = "text"
	StreamFormatJSONL StreamFormat = "jsonl"
)

// ParseStreamFormat accepts text or jsonl for streaming commands.
func ParseStreamFormat(s string) (StreamFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "text":
		return StreamFormatText, nil
	case "jsonl":
		return StreamFormatJSONL, nil
	default:
		return "", fmt.Errorf("invalid --format %q (supported: text, jsonl)", s)
	}
}
