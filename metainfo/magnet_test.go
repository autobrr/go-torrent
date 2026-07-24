// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var exampleMagnetURI = `magnet:?xt=urn:btih:51340689c960f0778a4387aef9b4b52fd08390cd&dn=Some+Movie+%281985%29+1337p+-+Eru&tr=http%3A%2F%2Fhttp.was.great%21&tr=udp%3A%2F%2Fanti.piracy.honeypot%3A6969`

func exampleMagnet() Magnet {
	ih, err := NewHashFromHex("51340689c960f0778a4387aef9b4b52fd08390cd")
	if err != nil {
		panic(err)
	}
	return Magnet{
		InfoHash:    ih,
		DisplayName: "Some Movie (1985) 1337p - Eru",
		Trackers: []string{
			"http://http.was.great!",
			"udp://anti.piracy.honeypot:6969",
		},
	}
}

func TestMagnetString(t *testing.T) {
	m, err := ParseMagnetUri(exampleMagnet().String())
	require.NoError(t, err)
	assert.Equal(t, exampleMagnet(), m)
}

func TestParseMagnetUri(t *testing.T) {
	// A base32-encoded v1 infohash.
	m, err := ParseMagnetUri("magnet:?xt=urn:btih:ZOCMZQIPFFW7OLLMIC5HUB6BPCSDEOQU")
	require.NoError(t, err)
	assert.Equal(t, "cb84ccc10f296df72d6c40ba7a07c178a4323a14", m.InfoHash.HexString())

	m, err = ParseMagnetUri(exampleMagnetURI)
	require.NoError(t, err)
	assert.Equal(t, exampleMagnet(), m)

	// Empty string is not a magnet URI.
	_, err = ParseMagnetUri("")
	assert.Error(t, err)

	// Wrong scheme.
	_, err = ParseMagnetUri("https://example.com")
	assert.Error(t, err)

	// An unrecognized xt URN is kept as a param, not an error.
	m, err = ParseMagnetUri("magnet:?xt=urn:sha1:YNCKHTQCWBTRNJIV4WNAE52SJUQCZO5C")
	require.NoError(t, err)
	assert.True(t, m.InfoHash.IsZero())
	assert.Equal(t, []string{"urn:sha1:YNCKHTQCWBTRNJIV4WNAE52SJUQCZO5C"}, m.Params["xt"])

	// A broken v1 infohash is an error.
	_, err = ParseMagnetUri("magnet:?xt=urn:btih:this hash is really broken")
	assert.Error(t, err)

	// Duplicate v1 infohashes are an error.
	_, err = ParseMagnetUri("magnet:?xt=urn:btih:51340689c960f0778a4387aef9b4b52fd08390cd&xt=urn:btih:51340689c960f0778a4387aef9b4b52fd08390cd")
	assert.Error(t, err)
}

func TestParseMagnetUriV2Only(t *testing.T) {
	const v2Only = "magnet:?xt=urn:btmh:1220caf1e1c30e81cb361b9ee167c4aa64228a7fa4fa9f6105232b28ad099f3a302e&dn=bittorrent-v2-test"

	m, err := ParseMagnetUri(v2Only)
	require.NoError(t, err)
	assert.True(t, m.InfoHash.IsZero())
	assert.Equal(t, "caf1e1c30e81cb361b9ee167c4aa64228a7fa4fa9f6105232b28ad099f3a302e", m.InfoHashV2.HexString())
	assert.Equal(t, "bittorrent-v2-test", m.DisplayName)
	assert.Len(t, m.Params, 0)

	// Round-trip.
	m2, err := ParseMagnetUri(m.String())
	require.NoError(t, err)
	assert.Equal(t, m, m2)

	// A bad multihash prefix is an error.
	_, err = ParseMagnetUri("magnet:?xt=urn:btmh:1120caf1e1c30e81cb361b9ee167c4aa64228a7fa4fa9f6105232b28ad099f3a302e")
	assert.Error(t, err)
}

func TestParseMagnetUriHybrid(t *testing.T) {
	const hybrid = "magnet:?xt=urn:btih:631a31dd0a46257d5078c0dee4e66e26f73e42ac&xt=urn:btmh:1220d8dd32ac93357c368556af3ac1d95c9d76bd0dff6fa9833ecdac3d53134efabb&dn=bittorrent-v1-v2-hybrid-test"

	m, err := ParseMagnetUri(hybrid)
	require.NoError(t, err)
	assert.Equal(t, "631a31dd0a46257d5078c0dee4e66e26f73e42ac", m.InfoHash.HexString())
	assert.Equal(t, "d8dd32ac93357c368556af3ac1d95c9d76bd0dff6fa9833ecdac3d53134efabb", m.InfoHashV2.HexString())
	assert.Equal(t, "bittorrent-v1-v2-hybrid-test", m.DisplayName)
	assert.Len(t, m.Params, 0)

	// Round-trip preserves both infohashes.
	m2, err := ParseMagnetUri(m.String())
	require.NoError(t, err)
	assert.Equal(t, m, m2)
}

func TestMetaInfoMagnetV1(t *testing.T) {
	mi, err := LoadFromFile("testdata/archlinux-2011.08.19-netinstall-i686.iso.torrent")
	require.NoError(t, err)

	m, err := mi.Magnet()
	require.NoError(t, err)
	assert.Equal(t, "archlinux-2011.08.19-netinstall-i686.iso", m.DisplayName)
	assert.Equal(t, mi.HashInfoBytes(), m.InfoHash)
	assert.True(t, m.InfoHashV2.IsZero())
	assert.Equal(t, mi.UpvertedAnnounceList().DistinctValues(), m.Trackers)
}

func TestMetaInfoMagnetV2Only(t *testing.T) {
	mi, err := LoadFromFile("testdata/bittorrent-v2-test.torrent")
	require.NoError(t, err)

	m, err := mi.Magnet()
	require.NoError(t, err)
	assert.True(t, m.InfoHash.IsZero())
	assert.Equal(t, "caf1e1c30e81cb361b9ee167c4aa64228a7fa4fa9f6105232b28ad099f3a302e", m.InfoHashV2.HexString())
}

func TestMetaInfoMagnetHybrid(t *testing.T) {
	mi, err := LoadFromFile("testdata/bittorrent-v2-hybrid-test.torrent")
	require.NoError(t, err)

	m, err := mi.Magnet()
	require.NoError(t, err)
	assert.Equal(t, "631a31dd0a46257d5078c0dee4e66e26f73e42ac", m.InfoHash.HexString())
	assert.Equal(t, "d8dd32ac93357c368556af3ac1d95c9d76bd0dff6fa9833ecdac3d53134efabb", m.InfoHashV2.HexString())

	// The generated link parses back with both infohashes intact.
	m2, err := ParseMagnetUri(m.String())
	require.NoError(t, err)
	assert.Equal(t, m.InfoHash, m2.InfoHash)
	assert.Equal(t, m.InfoHashV2, m2.InfoHashV2)
}

func TestMetaInfoMagnetWebSeeds(t *testing.T) {
	mi, err := LoadFromFile("testdata/flat-url-list.torrent")
	require.NoError(t, err)

	m, err := mi.Magnet()
	require.NoError(t, err)
	assert.Equal(t, []string(mi.UrlList), m.Params["ws"])

	m2, err := ParseMagnetUri(m.String())
	require.NoError(t, err)
	assert.Equal(t, m.Params["ws"], m2.Params["ws"])
}
