// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package bencode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testInfoDict struct {
	Length      int64  `bencode:"length"`
	Name        string `bencode:"name"`
	PieceLength int64  `bencode:"piece length"`
	Pieces      []byte `bencode:"pieces"`
}

type testTorrent struct {
	Announce string `bencode:"announce"`
	Info     Bytes  `bencode:"info"`
}

func TestRoundTripTorrentBytes(t *testing.T) {
	original := "d8:announce7:udp://x4:infod6:lengthi1e4:name1:f12:piece lengthi2e6:pieces0:ee"

	var tor testTorrent
	require.NoError(t, Unmarshal([]byte(original), &tor))
	assert.Equal(t, "udp://x", tor.Announce)

	reencoded, err := Marshal(tor)
	require.NoError(t, err)
	assert.Equal(t, original, string(reencoded))

	var info testInfoDict
	require.NoError(t, Unmarshal(tor.Info, &info))
	assert.Equal(t, int64(1), info.Length)
	assert.Equal(t, "f", info.Name)
	assert.Equal(t, int64(2), info.PieceLength)
	require.NotNil(t, info.Pieces)
	assert.Empty(t, info.Pieces)

	infoBytes, err := Marshal(info)
	require.NoError(t, err)
	assert.Equal(t, []byte(tor.Info), infoBytes)
}

func TestBytesPreservesUnsortedKeys(t *testing.T) {
	original := "d4:infod1:b0:1:a0:ee"

	var tor struct {
		Info Bytes `bencode:"info"`
	}
	require.NoError(t, Unmarshal([]byte(original), &tor))
	assert.Equal(t, Bytes("d1:b0:1:a0:e"), tor.Info)

	reencoded, err := Marshal(tor)
	require.NoError(t, err)
	assert.Equal(t, original, string(reencoded))
}

func TestBytesTopLevel(t *testing.T) {
	var b Bytes
	require.NoError(t, Unmarshal([]byte("li1ei2ee"), &b))
	assert.Equal(t, Bytes("li1ei2ee"), b)

	out, err := Marshal(b)
	require.NoError(t, err)
	assert.Equal(t, "li1ei2ee", string(out))
}

func TestBytesZeroLengthMarshalError(t *testing.T) {
	_, err := Bytes{}.MarshalBencode()
	require.Error(t, err)

	_, err = Marshal(struct {
		Info Bytes `bencode:"info"`
	}{})
	var wrapErr *MarshalerError
	require.ErrorAs(t, err, &wrapErr)
}

func TestRoundTripStruct(t *testing.T) {
	type nested struct {
		Values []string       `bencode:"values"`
		Extra  map[string]int `bencode:"extra"`
	}
	type target struct {
		Name   string `bencode:"name"`
		Count  int64  `bencode:"count"`
		Flag   bool   `bencode:"flag"`
		Nested nested `bencode:"nested"`
	}

	in := target{
		Name:  "x",
		Count: -3,
		Flag:  true,
		Nested: nested{
			Values: []string{"a", "b"},
			Extra:  map[string]int{"k": 1},
		},
	}
	data, err := Marshal(in)
	require.NoError(t, err)

	var out target
	require.NoError(t, Unmarshal(data, &out))
	assert.Equal(t, in, out)
}
