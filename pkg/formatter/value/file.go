// SPDX-License-Identifier: MIT

package value

import "fmt"

// FormatFileSize formats a size in bytes to a human-readable string (GB, MB, KB).
// Returns the size with 2 decimal places and the original byte count in parentheses.
func FormatFileSize(bytes uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB (%d bytes)", float64(bytes)/float64(GB), bytes)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB (%d bytes)", float64(bytes)/float64(MB), bytes)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB (%d bytes)", float64(bytes)/float64(KB), bytes)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
