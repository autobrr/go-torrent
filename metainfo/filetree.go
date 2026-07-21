// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"maps"
	"reflect"
	"slices"

	"github.com/autobrr/go-torrent/bencode"
)

// FileTreeFile holds the properties of a leaf file in a FileTree.
type FileTreeFile struct {
	Length     int64  `bencode:"length"`
	PiecesRoot string `bencode:"pieces root"`
}

// FileTree is the BEP 52 v2 file tree.
type FileTree struct {
	File FileTreeFile
	Dir  map[string]FileTree
}

// UnmarshalBencode decodes the wire shape: a dict whose empty key holds the
// file properties for a leaf and whose other keys are child names. The whole
// tree is decoded in a single pass; recursing with a fresh Unmarshal per
// level would retain a raw copy of every subtree once per ancestor, letting a
// small crafted torrent allocate hundreds of times its size.
func (ft *FileTree) UnmarshalBencode(b []byte) error {
	var v any
	if err := bencode.Unmarshal(b, &v); err != nil {
		return err
	}
	return ft.fromValue(v)
}

func (ft *FileTree) fromValue(v any) error {
	dir, ok := v.(map[string]any)
	if !ok {
		return &bencode.UnmarshalTypeError{
			BencodeTypeName:     bencodeTypeName(v),
			UnmarshalTargetType: reflect.TypeFor[FileTree](),
		}
	}
	if props, ok := dir[""]; ok {
		if err := ft.File.fromValue(props); err != nil {
			return err
		}
	}
	ft.Dir = make(map[string]FileTree, len(dir))
	for name, sub := range dir {
		if name == "" {
			continue
		}
		var child FileTree
		if err := child.fromValue(sub); err != nil {
			return err
		}
		ft.Dir[name] = child
	}
	return nil
}

func (f *FileTreeFile) fromValue(v any) error {
	props, ok := v.(map[string]any)
	if !ok {
		return &bencode.UnmarshalTypeError{
			BencodeTypeName:     bencodeTypeName(v),
			UnmarshalTargetType: reflect.TypeFor[FileTreeFile](),
		}
	}
	if raw, ok := props["length"]; ok {
		n, ok := raw.(int64)
		if !ok {
			return &bencode.UnmarshalTypeError{
				BencodeTypeName:     bencodeTypeName(raw),
				UnmarshalTargetType: reflect.TypeFor[int64](),
			}
		}
		f.Length = n
	}
	if raw, ok := props["pieces root"]; ok {
		s, ok := raw.(string)
		if !ok {
			return &bencode.UnmarshalTypeError{
				BencodeTypeName:     bencodeTypeName(raw),
				UnmarshalTargetType: reflect.TypeFor[string](),
			}
		}
		f.PiecesRoot = s
	}
	return nil
}

// MarshalBencode reproduces the wire shape decoded by UnmarshalBencode.
func (ft *FileTree) MarshalBencode() ([]byte, error) {
	dir := make(map[string]bencode.Bytes, len(ft.Dir)+1)
	if ft.IsDir() {
		for name, sub := range ft.Dir {
			if name == "" {
				continue
			}
			raw, err := sub.MarshalBencode()
			if err != nil {
				return nil, err
			}
			dir[name] = raw
		}
	} else {
		raw, err := bencode.Marshal(&ft.File)
		if err != nil {
			return nil, err
		}
		dir[""] = raw
	}
	return bencode.Marshal(dir)
}

// IsDir reports whether the node has child entries.
func (ft *FileTree) IsDir() bool {
	return ft.NumEntries() != 0
}

// NumEntries returns the number of child entries, excluding the file
// properties key.
func (ft *FileTree) NumEntries() int {
	n := len(ft.Dir)
	if _, ok := ft.Dir[""]; ok {
		n--
	}
	return n
}

// appendFiles walks leaves in sorted key order. v2 files are piece aligned,
// so each file's offset advances to the next piece boundary (BEP 52).
func (ft *FileTree) appendFiles(out *[]FileInfo, pieceLength int64, path []string, offset *int64) {
	if ft.IsDir() {
		for _, name := range slices.Sorted(maps.Keys(ft.Dir)) {
			if name == "" {
				continue
			}
			sub := ft.Dir[name]
			sub.appendFiles(out, pieceLength, append(path, name), offset)
		}
		return
	}
	*out = append(*out, FileInfo{
		Length:        ft.File.Length,
		Path:          slices.Clone(path),
		PathUtf8:      slices.Clone(path),
		TorrentOffset: *offset,
	})
	*offset += pieceAligned(ft.File.Length, pieceLength)
}

func pieceAligned(length, pieceLength int64) int64 {
	if pieceLength <= 0 {
		return length
	}
	return (length + pieceLength - 1) / pieceLength * pieceLength
}
