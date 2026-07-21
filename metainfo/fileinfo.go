// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"strings"
)

// ExtendedFileAttrs are the optional file attributes from BEP 47.
type ExtendedFileAttrs struct {
	Attr        string   `bencode:"attr,omitempty"`
	SymlinkPath []string `bencode:"symlink path,omitempty"`
	Sha1        string   `bencode:"sha1,omitempty"`
}

// FileInfo is one file in a multi-file torrent (BEP 3).
type FileInfo struct {
	Length   int64    `bencode:"length"`
	Path     []string `bencode:"path"`
	PathUtf8 []string `bencode:"path.utf-8,omitempty"`
	ExtendedFileAttrs
	TorrentOffset int64 `bencode:"-"`
}

// BestPath returns the UTF-8 path when present, otherwise the plain path.
func (fi *FileInfo) BestPath() []string {
	if len(fi.PathUtf8) != 0 {
		return fi.PathUtf8
	}
	return fi.Path
}

// DisplayPath returns the file's path within a directory torrent, or the
// torrent name for a single-file torrent.
func (fi *FileInfo) DisplayPath(info *Info) string {
	if info.IsDir() {
		return strings.Join(fi.BestPath(), "/")
	}
	return info.BestName()
}
