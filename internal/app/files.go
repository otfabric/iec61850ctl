// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"io"

	iec61850 "github.com/otfabric/go-iec61850"
)

// ListFiles returns file directory entries matching pattern.
func (a *App) ListFiles(pattern string) ([]iec61850.FileEntry, error) {
	return a.conn.ListFiles(context.Background(), pattern)
}

// DownloadFile writes a remote file to w and returns metadata.
func (a *App) DownloadFile(name string, w io.Writer) (*iec61850.FileEntry, error) {
	return a.conn.DownloadFile(context.Background(), name, w)
}
