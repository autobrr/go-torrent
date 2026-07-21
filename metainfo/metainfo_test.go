// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/autobrr/go-torrent/bencode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var torrentFileCases = []struct {
	file         string
	infoHash     string
	name         string
	totalLength  int64
	numPieces    int
	announce     string
	announceList AnnounceList
	noInfo       bool
}{
	{
		file:        "23516C72685E8DB0C8F15553382A927F185C4F01.torrent",
		infoHash:    "23516c72685e8db0c8f15553382a927f185c4f01",
		name:        "Z.Nation.S01E12.HDTV.x264-LOL[ettv]",
		totalLength: 529171764,
		numPieces:   2019,
		announce:    "udp://open.demonii.com:1337/announce",
		announceList: AnnounceList{
			{"udp://open.demonii.com:1337/announce"},
			{"udp://tracker.publicbt.com:80/announce"},
			{"udp://tracker.openbittorrent.com:80/announce"},
			{"udp://tracker.istole.it:80/announce"},
			{"http://mgtracker.org:2710/announce"},
			{"udp://coppersurfer.tk:6969/announce"},
			{"udp://tracker.token.ro:80/announce"},
			{"udp://castradio.net:6969/announce"},
			{"udp://ipv4.tracker.harry.lu:80/announce"},
			{"udp://9.rarbg.com:2710/announce"},
		},
	},
	{
		file:        "Debian.13.5.0.AMD64.NETINST-FAKE.torrent",
		infoHash:    "f4a1456a329be438b36f7b5a2b647be72f21089f",
		name:        "Debian.13.5.0.AMD64.NETINST-FAKE",
		totalLength: 791676305,
		numPieces:   378,
		announce:    "https://tracker.example.org:2711/00000000000000000000000000000000/announce",
	},
	{
		file:        "SKODAOCTAVIA336x280_archive.torrent",
		infoHash:    "d4b197dff199aad447a9a352e31528adbbd97922",
		name:        "SKODAOCTAVIA336x280",
		totalLength: 5448139,
		numPieces:   11,
		announce:    "http://bt1.archive.org:6969/announce",
		announceList: AnnounceList{
			{"http://bt1.archive.org:6969/announce"},
			{"http://bt2.archive.org:6969/announce"},
		},
	},
	{
		file:        "archlinux-2011.08.19-netinstall-i686.iso.torrent",
		infoHash:    "500f29c0c537f5e41c6af676b7633de9d080d237",
		name:        "archlinux-2011.08.19-netinstall-i686.iso",
		totalLength: 189792256,
		numPieces:   362,
		announce:    "http://tracker.archlinux.org:6969/announce",
	},
	{
		file:        "bittorrent-v2-hybrid-test.torrent",
		infoHash:    "631a31dd0a46257d5078c0dee4e66e26f73e42ac",
		name:        "bittorrent-v1-v2-hybrid-test",
		totalLength: 895544883,
		numPieces:   1715,
	},
	{
		file:        "bittorrent-v2-test.torrent",
		infoHash:    "f987ab6bb50f831a861c3754ecd1b47dc2cf2e30",
		name:        "bittorrent-v2-test",
		totalLength: 1534222888,
		numPieces:   371,
	},
	{
		file:        "continuum.torrent",
		infoHash:    "4029ef207642d5d6b8b9a0a484a103262f764710",
		name:        "Continuum.S01.720p.WEB-DL.Rus.Eng.HDCLUB",
		totalLength: 6397469459,
		numPieces:   1526,
		announce:    "udp://bt.rutor.org:2710",
		announceList: AnnounceList{
			{"udp://bt.rutor.org:2710"},
			{"http://retracker.local/announce"},
		},
	},
	{
		file:        "flat-url-list.torrent",
		infoHash:    "9da24e606e4ed9c7b91c1772fb5bf98f82bd9687",
		name:        "SKODAOCTAVIA336x280",
		totalLength: 5448139,
		numPieces:   11,
		announce:    "http://bt1.archive.org:6969/announce",
		announceList: AnnounceList{
			{"http://bt1.archive.org:6969/announce"},
			{"http://bt2.archive.org:6969/announce"},
		},
	},
	{
		file:        "issue_65a.torrent",
		infoHash:    "4fe050208d675e5cd967d41348a31ab11b316806",
		name:        "olo@SIS001@OBA-237",
		totalLength: 1492619433,
		numPieces:   1424,
		announce:    "udp://tracker.coppersurfer.tk:6969/announce",
		announceList: AnnounceList{
			{"udp://tracker.coppersurfer.tk:6969/announce"},
			{"udp://tracker.openbittorrent.com:80/announce"},
			{"udp://glotorrents.pw:6969/announce"},
			{"udp://tracker.publicbt.com:80/announce"},
			{"http://www.iamtracker.com/announce.php"},
		},
	},
	{
		file:        "issue_65b.torrent",
		infoHash:    "d376200e98b64abdcfa257a63f6208a704e42d9f",
		name:        "olo@SIS001@heyzo_hd_0870_full",
		totalLength: 1056819797,
		numPieces:   2016,
		announce:    "udp://tracker.coppersurfer.tk:6969/announce",
		announceList: AnnounceList{
			{"udp://tracker.coppersurfer.tk:6969/announce"},
			{"udp://tracker.openbittorrent.com:80/announce"},
			{"udp://glotorrents.pw:6969/announce"},
			{"udp://tracker.publicbt.com:80/announce"},
			{"udp://tracker.istole.it:80/announce"},
			{"udp://tracker.ccc.de:80/announce"},
			{"udp://tracker.lamsoft.net:6969/announce.php"},
			{"udp://tracker.lamsoft.net:6969/announce"},
			{"http://tracker.bittorrent.am/announce.php"},
			{"http://tracker.bittorrent.am/announce"},
			{"http://www.iamtracker.com/announce.php"},
		},
	},
	{
		file:   "minimal-trailing-newline.torrent",
		noInfo: true,
	},
	{
		file:        "trackerless.torrent",
		infoHash:    "1dc8b6dbbb81c58b71220e20908245f8f565433f",
		name:        "testfile.bin",
		totalLength: 1128,
		numPieces:   1,
	},
}

func TestLoadTestdataTorrents(t *testing.T) {
	for _, tc := range torrentFileCases {
		t.Run(tc.file, func(t *testing.T) {
			mi, err := LoadFromFile(filepath.Join("testdata", tc.file))
			require.NoError(t, err)
			assert.Equal(t, tc.announce, mi.Announce)
			assert.Equal(t, tc.announceList, mi.AnnounceList)
			if tc.noInfo {
				_, err := mi.UnmarshalInfo()
				assert.Error(t, err)
				return
			}
			assert.Equal(t, tc.infoHash, mi.HashInfoBytes().HexString())
			info, err := mi.UnmarshalInfo()
			require.NoError(t, err)
			assert.Equal(t, tc.name, info.BestName())
			assert.Equal(t, tc.totalLength, info.TotalLength())
			assert.Equal(t, tc.numPieces, info.NumPieces())
		})
	}
}

func TestTestdataTableIsComplete(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	require.NoError(t, err)
	var onDisk []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".torrent") {
			onDisk = append(onDisk, e.Name())
		}
	}
	var covered []string
	for _, tc := range torrentFileCases {
		covered = append(covered, tc.file)
	}
	assert.ElementsMatch(t, onDisk, covered)
}

func TestInfoBytesRoundTrip(t *testing.T) {
	for _, file := range []string{
		"archlinux-2011.08.19-netinstall-i686.iso.torrent",
		"continuum.torrent",
		"23516C72685E8DB0C8F15553382A927F185C4F01.torrent",
		"trackerless.torrent",
		"Debian.13.5.0.AMD64.NETINST-FAKE.torrent",
	} {
		t.Run(file, func(t *testing.T) {
			mi, err := LoadFromFile(filepath.Join("testdata", file))
			require.NoError(t, err)
			info, err := mi.UnmarshalInfo()
			require.NoError(t, err)
			b, err := bencode.Marshal(&info)
			require.NoError(t, err)
			assert.Equal(t, []byte(mi.InfoBytes), b)
		})
	}
}

// FunFile serves torrents whose announce-list is the bencode string "0:"
// instead of a list of lists; anacrolix/torrent fails the whole parse.
// TestEmptyStringAnnounceList covers the FunFile quirk: announce-list
// encoded as the bencode string "0:" instead of a list of tiers, which
// fails the whole parse in anacrolix/torrent. The testdata file is
// synthetic but mirrors the real torrent's shape.
func TestEmptyStringAnnounceList(t *testing.T) {
	mi, err := LoadFromFile("testdata/Debian.13.5.0.AMD64.NETINST-FAKE.torrent")
	require.NoError(t, err)
	assert.Nil(t, mi.AnnounceList)
	assert.Equal(t, AnnounceList{
		{"https://tracker.example.org:2711/00000000000000000000000000000000/announce"},
	}, mi.UpvertedAnnounceList())
	assert.Equal(t, "f4a1456a329be438b36f7b5a2b647be72f21089f", mi.HashInfoBytes().String())
	info, err := mi.UnmarshalInfo()
	require.NoError(t, err)
	assert.Equal(t, int64(791676305), info.TotalLength())
	require.NotNil(t, info.Private)
	assert.True(t, *info.Private)
}

func testUnmarshalMetaInfo(t *testing.T, input string, expected *MetaInfo) {
	t.Helper()
	var actual MetaInfo
	err := bencode.Unmarshal([]byte(input), &actual)
	if expected == nil {
		assert.Error(t, err)
		return
	}
	require.NoError(t, err)
	assert.Equal(t, *expected, actual)
}

func TestUnmarshalMetaInfo(t *testing.T) {
	testUnmarshalMetaInfo(t, "de", &MetaInfo{})
	testUnmarshalMetaInfo(t, "d4:infoe", nil)
	testUnmarshalMetaInfo(t, "d4:infoabce", nil)
	testUnmarshalMetaInfo(t, "d4:infodee", &MetaInfo{InfoBytes: bencode.Bytes("de")})
}

// https://github.com/anacrolix/torrent/issues/247
func TestUnmarshalStringCreationDate(t *testing.T) {
	var mi MetaInfo
	require.NoError(t, bencode.Unmarshal([]byte("d13:creation date23:29.03.2018 22:18:14 UTC4:infodee"), &mi))
	assert.Zero(t, mi.CreationDate)
	assert.Equal(t, bencode.Bytes("de"), mi.InfoBytes)
}

func TestUnmarshalEmptyStringNodes(t *testing.T) {
	var mi MetaInfo
	require.NoError(t, bencode.Unmarshal([]byte("d5:nodes0:e"), &mi))
	assert.Nil(t, mi.Nodes)
}

func TestUnmarshalCreationDateOverflow(t *testing.T) {
	var mi MetaInfo
	require.NoError(t, bencode.Unmarshal([]byte("d13:creation datei99999999999999999999999999e4:infodee"), &mi))
	assert.Zero(t, mi.CreationDate)
	assert.Equal(t, bencode.Bytes("de"), mi.InfoBytes)
}

func TestLoadTrailingNewlineFile(t *testing.T) {
	mi, err := LoadFromFile("testdata/minimal-trailing-newline.torrent")
	require.NoError(t, err)
	assert.Equal(t, MetaInfo{}, *mi)
	_, err = mi.UnmarshalInfo()
	assert.Error(t, err)
}

func TestLoadTrailingGarbage(t *testing.T) {
	_, err := Load(strings.NewReader("dexyz"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error after decoding metainfo")
}

func TestLoadTrailingWhitespaceAndNul(t *testing.T) {
	mi, err := Load(strings.NewReader("de \t\r\n\x00"))
	require.NoError(t, err)
	assert.Equal(t, MetaInfo{}, *mi)
}

func TestLoadSyntaxErrors(t *testing.T) {
	var syntaxErr *bencode.SyntaxError

	html := "<!DOCTYPE html><html><head><title>Not Found</title></head><body>404</body></html>"
	_, err := Load(strings.NewReader(html))
	require.Error(t, err)
	assert.ErrorAs(t, err, &syntaxErr)

	data, err := os.ReadFile("testdata/continuum.torrent")
	require.NoError(t, err)
	_, err = Load(bytes.NewReader(data[:100]))
	require.Error(t, err)
	assert.ErrorAs(t, err, &syntaxErr)

	_, err = Load(strings.NewReader(""))
	require.Error(t, err)
	assert.ErrorAs(t, err, &syntaxErr)
}

func TestMetainfoWithListURLList(t *testing.T) {
	mi, err := LoadFromFile("testdata/SKODAOCTAVIA336x280_archive.torrent")
	require.NoError(t, err)
	assert.Equal(t, UrlList{
		"https://archive.org/download/",
		"http://ia601600.us.archive.org/26/items/",
		"http://ia801600.us.archive.org/26/items/",
	}, mi.UrlList)
}

func TestMetainfoWithStringURLList(t *testing.T) {
	mi, err := LoadFromFile("testdata/flat-url-list.torrent")
	require.NoError(t, err)
	assert.Equal(t, UrlList{"https://archive.org/download/"}, mi.UrlList)
}

func TestWriteMetaInfo(t *testing.T) {
	var buf bytes.Buffer
	mi := &MetaInfo{
		InfoBytes: bencode.Bytes("d2:hi5:theree"),
		Nodes:     []Node{"1.2.3.4:5555", "not a hostport"},
	}
	require.NoError(t, mi.Write(&buf))
	assert.Equal(t, "d4:infod2:hi5:theree5:nodesl12:1.2.3.4:555514:not a hostportee", buf.String())
}

func TestWriteLoadRoundTrip(t *testing.T) {
	mi, err := LoadFromFile("testdata/continuum.torrent")
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, mi.Write(&buf))
	again, err := Load(&buf)
	require.NoError(t, err)
	assert.Equal(t, mi, again)
}

func TestHash(t *testing.T) {
	h := HashBytes(nil)
	assert.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", h.HexString())
	assert.Equal(t, h.HexString(), h.String())
	assert.Len(t, h.Bytes(), HashSize)
	assert.False(t, h.IsZero())
	assert.True(t, Hash{}.IsZero())

	parsed, err := NewHashFromHex("da39a3ee5e6b4b0d3255bfef95601890afd80709")
	require.NoError(t, err)
	assert.Equal(t, h, parsed)

	_, err = NewHashFromHex("da39a3ee")
	assert.Error(t, err)
	_, err = NewHashFromHex(strings.Repeat("zz", HashSize))
	assert.Error(t, err)
}
