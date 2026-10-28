// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/otfabric/go-mms"
)

// DirFileProvider serves the regular files of one directory (non-recursive,
// read-only) over MMS file services. Paths are resolved through os.Root, so
// requests cannot escape the directory.
type DirFileProvider struct {
	root *os.Root
}

var _ mms.FileProvider = (*DirFileProvider)(nil)

// NewDirFileProvider opens dir for serving.
func NewDirFileProvider(dir string) (*DirFileProvider, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &DirFileProvider{root: root}, nil
}

// Release closes the directory handle.
func (p *DirFileProvider) Release() error { return p.root.Close() }

func cleanName(name string) string {
	return strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(name, "\\", "/")), "/")
}

// List returns regular files whose name matches req.FileSpec (path.Match
// syntax; empty or "*" lists everything), sorted by name.
func (p *DirFileProvider) List(_ context.Context, req mms.FileListRequest) (*mms.FileListResult, error) {
	f, err := p.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	infos, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	spec := cleanName(req.FileSpec)
	if _, err := path.Match(spec, ""); err != nil {
		return nil, fmt.Errorf("file spec %q: %w", req.FileSpec, err)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name() < infos[j].Name() })
	res := &mms.FileListResult{}
	for _, fi := range infos {
		if !fi.Mode().IsRegular() {
			continue
		}
		name := fi.Name()
		if spec != "" {
			if ok, _ := path.Match(spec, name); !ok {
				continue
			}
		}
		if req.ContinueAfter != "" && name <= req.ContinueAfter {
			continue
		}
		if req.MaxEntries > 0 && len(res.Entries) == req.MaxEntries {
			res.MoreFollows = true
			break
		}
		res.Entries = append(res.Entries, mms.FileEntry{Name: name, Size: fi.Size(), LastModified: fi.ModTime().UTC()})
	}
	return res, nil
}

// Open opens a file for reading.
func (p *DirFileProvider) Open(_ context.Context, name string) (mms.FileHandle, mms.FileAttributes, error) {
	f, err := p.root.Open(cleanName(name))
	if err != nil {
		return nil, mms.FileAttributes{}, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, mms.FileAttributes{}, err
	}
	if !fi.Mode().IsRegular() {
		_ = f.Close()
		return nil, mms.FileAttributes{}, fs.ErrNotExist
	}
	return f, mms.FileAttributes{Size: fi.Size(), LastModified: fi.ModTime().UTC()}, nil
}

// Read returns the next chunk; moreFollows is false on the final chunk.
func (p *DirFileProvider) Read(_ context.Context, h mms.FileHandle, maxBytes int) ([]byte, bool, error) {
	f, ok := h.(*os.File)
	if !ok {
		return nil, false, errors.New("invalid file handle")
	}
	if maxBytes <= 0 {
		maxBytes = 512
	}
	buf := make([]byte, maxBytes)
	n, err := io.ReadFull(f, buf)
	switch {
	case err == nil:
		// Peek whether more data follows without consuming it.
		var one [1]byte
		m, perr := f.Read(one[:])
		if m == 1 {
			if _, serr := f.Seek(-1, io.SeekCurrent); serr != nil {
				return nil, false, serr
			}
			return buf[:n], true, nil
		}
		if perr != nil && !errors.Is(perr, io.EOF) {
			return nil, false, perr
		}
		return buf[:n], false, nil
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return buf[:n], false, nil
	default:
		return nil, false, err
	}
}

// Close closes a handle returned by Open.
func (p *DirFileProvider) Close(_ context.Context, h mms.FileHandle) error {
	f, ok := h.(*os.File)
	if !ok {
		return errors.New("invalid file handle")
	}
	return f.Close()
}

// Delete is refused: the provider is read-only.
func (p *DirFileProvider) Delete(context.Context, string) error { return mms.ErrFileAccessDenied }

// Rename is refused: the provider is read-only.
func (p *DirFileProvider) Rename(context.Context, string, string) error {
	return mms.ErrFileAccessDenied
}

// ObtainFile is refused: the provider is read-only.
func (p *DirFileProvider) ObtainFile(context.Context, string, string) error {
	return mms.ErrFileAccessDenied
}
