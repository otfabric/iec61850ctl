// SPDX-License-Identifier: MIT

package view

// FileEntry is the projection of domain.FileEntry for display/API output.
type FileEntry struct {
	Name         string `json:"name"`
	Size         string `json:"size"`
	SizeBytes    uint32 `json:"size_bytes"`
	LastModified string `json:"last_modified"`
}
