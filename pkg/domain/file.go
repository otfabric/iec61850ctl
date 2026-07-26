// SPDX-License-Identifier: MIT

package domain

// FileEntry represents an entry from the MMS file directory service.
type FileEntry struct {
	Name         string
	Size         uint32
	LastModified uint64
}
