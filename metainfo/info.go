// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

// Info is the torrent info dictionary (BEP 3, BEP 52).
type Info struct {
	PieceLength int64  `bencode:"piece length"`
	Pieces      []byte `bencode:"pieces,omitempty"`
	Name        string `bencode:"name"`
	NameUtf8    string `bencode:"name.utf-8,omitempty"`
	Length      int64  `bencode:"length,omitempty"`
	ExtendedFileAttrs
	Private     *bool      `bencode:"private,omitempty,ignore_unmarshal_type_error"`
	Source      string     `bencode:"source,omitempty"`
	Files       []FileInfo `bencode:"files,omitempty"`
	MetaVersion int64      `bencode:"meta version,omitempty"`
	FileTree    FileTree   `bencode:"file tree,omitempty"`
}

// TotalLength returns the sum of all file lengths, using the v2 file tree
// when present.
func (info *Info) TotalLength() int64 {
	var total int64
	for _, fi := range info.UpvertedFiles() {
		total += fi.Length
	}
	return total
}

// NumPieces returns the number of pieces in the torrent.
func (info *Info) NumPieces() int {
	if info.HasV2() {
		if info.PieceLength <= 0 {
			return 0
		}
		var num int64
		for _, fi := range info.UpvertedFiles() {
			num += (fi.Length + info.PieceLength - 1) / info.PieceLength
		}
		return int(num)
	}
	return len(info.Pieces) / HashSize
}

// IsDir reports whether the torrent describes a directory of files.
func (info *Info) IsDir() bool {
	if info.HasV2() {
		return info.FileTree.IsDir()
	}
	return len(info.Files) != 0
}

// UpvertedFiles returns the torrent's files in a common form: the v2 file
// tree when present, otherwise the v1 files list, with a single-file torrent
// converted to one entry with a nil Path.
func (info *Info) UpvertedFiles() []FileInfo {
	if info.HasV2() {
		var files []FileInfo
		var offset int64
		info.FileTree.appendFiles(&files, info.PieceLength, nil, &offset)
		return files
	}
	if len(info.Files) == 0 {
		return []FileInfo{{Length: info.Length}}
	}
	files := make([]FileInfo, 0, len(info.Files))
	var offset int64
	for _, fi := range info.Files {
		fi.TorrentOffset = offset
		offset += fi.Length
		files = append(files, fi)
	}
	return files
}

// BestName returns the UTF-8 name when present, otherwise the plain name.
func (info *Info) BestName() string {
	if info.NameUtf8 != "" {
		return info.NameUtf8
	}
	return info.Name
}

// HasV2 reports whether the info is a v2 (BEP 52) info dict.
func (info *Info) HasV2() bool {
	return info.MetaVersion == 2
}

// HasV1 reports whether the info can be used as a v1 info dict.
func (info *Info) HasV1() bool {
	return info.MetaVersion == 0 || info.MetaVersion == 1 ||
		info.Files != nil || info.Length != 0 || len(info.Pieces) != 0
}
