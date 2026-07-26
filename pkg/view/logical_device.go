// SPDX-License-Identifier: MIT

// Package view contains projection structs for CLI and REST output.
//
// View types are transport-safe (JSON-tagged), contain pre-computed presentation
// fields, and never expose raw MMS library types. They are produced by projection
// functions in package service and consumed by formatter (CLI) and HTTP handlers
// (REST). Domain types must never leak through this layer.
package view

// LogicalDevice is the projection of domain.LogicalDevice for display/API output.
type LogicalDevice struct {
	Name      string `json:"name"`
	LNCount   int    `json:"ln_count"`
	DSCount   int    `json:"ds_count"`
	URCBCount int    `json:"urcb_count"`
	BRCBCount int    `json:"brcb_count"`
}
