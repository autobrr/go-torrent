// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"strings"
	"testing"

	"github.com/autobrr/go-torrent/bencode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalInfo(t *testing.T) {
	info := Info{Pieces: []byte{}}
	b, err := bencode.Marshal(&info)
	require.NoError(t, err)
	assert.Equal(t, "d4:name0:12:piece lengthi0e6:pieces0:e", string(b))
}

func TestUpvertedFilesSingleFile(t *testing.T) {
	info := Info{Name: "testfile.bin", Length: 1128}
	assert.False(t, info.IsDir())
	files := info.UpvertedFiles()
	require.Len(t, files, 1)
	assert.Equal(t, FileInfo{Length: 1128}, files[0])
	assert.Equal(t, "testfile.bin", files[0].DisplayPath(&info))
}

func TestUpvertedFilesMultiFileOffsets(t *testing.T) {
	mi, err := LoadFromFile("testdata/continuum.torrent")
	require.NoError(t, err)
	info, err := mi.UnmarshalInfo()
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	files := info.UpvertedFiles()
	require.Len(t, files, 4)
	assert.Equal(t, int64(0), files[0].TorrentOffset)
	assert.Equal(t, int64(1609073588), files[1].TorrentOffset)
	assert.Equal(t, int64(3187260840), files[2].TorrentOffset)
	assert.Equal(t, int64(4833293306), files[3].TorrentOffset)
	assert.Equal(t, int64(1564176153), files[3].Length)
	assert.Equal(t, "Continuum.S01E01.720p.WEB-DL.Rus.Eng.HDCLUB.mkv", files[0].DisplayPath(&info))
}

func TestNumPiecesV1(t *testing.T) {
	assert.Equal(t, 0, (&Info{}).NumPieces())
	assert.Equal(t, 1, (&Info{Pieces: make([]byte, 20)}).NumPieces())
	assert.Equal(t, 2, (&Info{Pieces: make([]byte, 40)}).NumPieces())
	assert.Equal(t, 1, (&Info{Pieces: make([]byte, 39)}).NumPieces())
}

func TestBestNameAndPath(t *testing.T) {
	info := Info{Name: "plain"}
	assert.Equal(t, "plain", info.BestName())
	info.NameUtf8 = "utf8"
	assert.Equal(t, "utf8", info.BestName())

	fi := FileInfo{Path: []string{"a", "b"}}
	assert.Equal(t, []string{"a", "b"}, fi.BestPath())
	fi.PathUtf8 = []string{"c"}
	assert.Equal(t, []string{"c"}, fi.BestPath())
}

func TestHasV1HasV2(t *testing.T) {
	assert.True(t, (&Info{}).HasV1())
	assert.False(t, (&Info{}).HasV2())
	assert.True(t, (&Info{MetaVersion: 1}).HasV1())
	assert.False(t, (&Info{MetaVersion: 2}).HasV1())
	assert.True(t, (&Info{MetaVersion: 2}).HasV2())
	assert.True(t, (&Info{MetaVersion: 2, Files: []FileInfo{}}).HasV1())
	assert.True(t, (&Info{MetaVersion: 2, Length: 1}).HasV1())
	assert.True(t, (&Info{MetaVersion: 2, Pieces: []byte("x")}).HasV1())
}

func TestV2Torrent(t *testing.T) {
	mi, err := LoadFromFile("testdata/bittorrent-v2-test.torrent")
	require.NoError(t, err)
	assert.NotEmpty(t, mi.PieceLayers)

	info, err := mi.UnmarshalInfo()
	require.NoError(t, err)
	assert.Equal(t, "bittorrent-v2-test", info.BestName())
	assert.True(t, info.HasV2())
	assert.False(t, info.HasV1())
	assert.True(t, info.IsDir())
	assert.Equal(t, int64(1534222888), info.TotalLength())
	assert.Equal(t, 371, info.NumPieces())

	files := info.UpvertedFiles()
	require.Len(t, files, 11)
	for _, fi := range files {
		assert.Zero(t, fi.TorrentOffset%info.PieceLength)
	}
	assert.Equal(t, []string{"13.Popsy Team - ViP 2.vob.mp4"}, files[0].Path)
	assert.Equal(t, int64(27551708), files[0].Length)
	assert.Equal(t, int64(0), files[0].TorrentOffset)
	assert.Equal(t, []string{"Chameleon by ASD (female voice).mov"}, files[1].Path)
	assert.Equal(t, int64(91862892), files[1].Length)
	assert.Equal(t, int64(29360128), files[1].TorrentOffset)
	assert.Equal(t, []string{"tbl-starstruck-2006.avi"}, files[10].Path)
	assert.Equal(t, int64(229148672), files[10].Length)
	assert.Equal(t, int64(1325400064), files[10].TorrentOffset)
}

func TestHybridTorrent(t *testing.T) {
	mi, err := LoadFromFile("testdata/bittorrent-v2-hybrid-test.torrent")
	require.NoError(t, err)
	info, err := mi.UnmarshalInfo()
	require.NoError(t, err)
	assert.True(t, info.HasV1())
	assert.True(t, info.HasV2())
	assert.Equal(t, "bittorrent-v1-v2-hybrid-test", info.BestName())

	// The v2 view excludes the .pad files present in the v1 files list.
	assert.Equal(t, int64(895544883), info.TotalLength())
	assert.Equal(t, 1715, info.NumPieces())
	assert.Len(t, info.UpvertedFiles(), 9)
	assert.Len(t, info.Files, 17)
	assert.Equal(t, "p", info.Files[1].Attr)
}

func TestFileTreeUnmarshalMarshal(t *testing.T) {
	root := "00000000000000000000000000000000"
	raw := "d1:ad5:b.txtd0:d6:lengthi5e11:pieces root32:" + root + "eee" +
		"5:c.txtd0:d6:lengthi100e11:pieces root32:" + root + "eee"

	var ft FileTree
	require.NoError(t, ft.UnmarshalBencode([]byte(raw)))
	assert.True(t, ft.IsDir())
	assert.Equal(t, 2, ft.NumEntries())
	assert.Equal(t, int64(5), ft.Dir["a"].Dir["b.txt"].File.Length)
	assert.Equal(t, root, ft.Dir["a"].Dir["b.txt"].File.PiecesRoot)
	sub := ft.Dir["a"]
	assert.True(t, sub.IsDir())
	assert.Equal(t, int64(100), ft.Dir["c.txt"].File.Length)

	out, err := ft.MarshalBencode()
	require.NoError(t, err)
	assert.Equal(t, raw, string(out))

	info := Info{MetaVersion: 2, PieceLength: 64, FileTree: ft}
	files := info.UpvertedFiles()
	require.Len(t, files, 2)
	assert.Equal(t, FileInfo{
		Length:        5,
		Path:          []string{"a", "b.txt"},
		PathUtf8:      []string{"a", "b.txt"},
		TorrentOffset: 0,
	}, files[0])
	assert.Equal(t, FileInfo{
		Length:        100,
		Path:          []string{"c.txt"},
		PathUtf8:      []string{"c.txt"},
		TorrentOffset: 64,
	}, files[1])
	assert.Equal(t, int64(105), info.TotalLength())
	assert.Equal(t, 3, info.NumPieces())
}

func TestFileTreeUnmarshalMismatch(t *testing.T) {
	var ft FileTree
	assert.Error(t, ft.UnmarshalBencode([]byte("3:abc")))
	assert.Error(t, ft.UnmarshalBencode([]byte("i42e")))
	assert.Error(t, ft.UnmarshalBencode([]byte("d1:a")))
}

func nestedFileTree(levels int) string {
	var b strings.Builder
	for range levels {
		b.WriteString("d1:a")
	}
	b.WriteString("d0:d6:lengthi7eee")
	for range levels {
		b.WriteString("e")
	}
	return b.String()
}

func TestFileTreeDeepNesting(t *testing.T) {
	var ft FileTree
	require.NoError(t, ft.UnmarshalBencode([]byte(nestedFileTree(400))))

	info := Info{MetaVersion: 2, PieceLength: 16384, FileTree: ft}
	files := info.UpvertedFiles()
	require.Len(t, files, 1)
	assert.Equal(t, int64(7), files[0].Length)
	assert.Len(t, files[0].Path, 400)
	assert.Equal(t, int64(7), info.TotalLength())

	var overdeep FileTree
	var syntaxErr *bencode.SyntaxError
	require.ErrorAs(t, overdeep.UnmarshalBencode([]byte(nestedFileTree(600))), &syntaxErr)
}
