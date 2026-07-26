// SPDX-License-Identifier: MIT

package domain

// LogicalDevice represents an IEC 61850 logical device (LD).
type LogicalDevice struct {
	Name         string
	LogicalNodes []LogicalNode

	// Summary counters (populated by enriched queries).
	LNCount   int
	DSCount   int
	URCBCount int
	BRCBCount int
}
