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

func TestAnnounceListUnmarshalBencode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want AnnounceList
	}{
		{"empty string", "0:", nil},
		{"non-empty string", "3:abc", AnnounceList{{"abc"}}},
		{"flat list of strings", "l3:abc3:defe", AnnounceList{{"abc"}, {"def"}}},
		{"list of lists", "ll3:abcel3:defee", AnnounceList{{"abc"}, {"def"}}},
		{"single tier with two urls", "ll3:abc3:defee", AnnounceList{{"abc", "def"}}},
		{"integer", "i42e", nil},
		{"junk-only tier dropped", "lli1eel3:abcee", AnnounceList{{"abc"}}},
		{"dict", "de", nil},
		{"empty list", "le", nil},
		{"empty string kept inside tier", "ll0:3:abcee", AnnounceList{{"", "abc"}}},
		{"junk skipped inside tier", "ll3:abci7edeee", AnnounceList{{"abc"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var al AnnounceList
			require.NoError(t, al.UnmarshalBencode([]byte(tc.in)))
			assert.Equal(t, tc.want, al)
		})
	}
}

func TestAnnounceListUnmarshalBencodeMalformed(t *testing.T) {
	var al AnnounceList
	assert.Error(t, al.UnmarshalBencode([]byte("l3:ab")))
}

func TestAnnounceListOverridesAnnounce(t *testing.T) {
	cases := []struct {
		name     string
		al       AnnounceList
		announce string
		want     bool
	}{
		{"nil list empty announce", nil, "", false},
		{"nil list with announce", nil, "x", false},
		{"url overrides empty announce", AnnounceList{{"a"}}, "", true},
		{"url overrides announce", AnnounceList{{"a"}}, "x", true},
		{"empty url overrides empty announce", AnnounceList{{""}}, "", true},
		{"empty url does not override announce", AnnounceList{{""}}, "x", false},
		{"empty tier empty announce", AnnounceList{{}}, "", false},
		{"url in later tier", AnnounceList{{}, {"b"}}, "x", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.al.OverridesAnnounce(tc.announce))
		})
	}
}

func TestAnnounceListDistinctValues(t *testing.T) {
	assert.Nil(t, AnnounceList(nil).DistinctValues())
	assert.Equal(t, []string{"a", "b", "c"},
		AnnounceList{{"a", "b"}, {"b", "c"}, {"a"}}.DistinctValues())
	assert.Equal(t, []string{"", "a"},
		AnnounceList{{"", "a"}, {""}}.DistinctValues())
}

func TestAnnounceListClone(t *testing.T) {
	al := AnnounceList{{"a"}, {"b"}}
	clone := al.Clone()
	clone[0] = []string{"z"}
	assert.Equal(t, AnnounceList{{"a"}, {"b"}}, al)
}

func TestUpvertedAnnounceList(t *testing.T) {
	assert.Nil(t, (&MetaInfo{}).UpvertedAnnounceList())

	mi := &MetaInfo{Announce: "a"}
	assert.Equal(t, AnnounceList{{"a"}}, mi.UpvertedAnnounceList())

	mi.AnnounceList = AnnounceList{{""}}
	assert.Equal(t, AnnounceList{{"a"}}, mi.UpvertedAnnounceList())

	mi.AnnounceList = AnnounceList{{"b"}}
	assert.Equal(t, AnnounceList{{"b"}}, mi.UpvertedAnnounceList())
}

func testFileNodesMatch(t *testing.T, file string, nodes []Node) {
	t.Helper()
	mi, err := LoadFromFile(file)
	require.NoError(t, err)
	assert.Equal(t, nodes, mi.Nodes)
}

func TestNodesListStrings(t *testing.T) {
	testFileNodesMatch(t, "testdata/trackerless.torrent", []Node{
		"udp://tracker.openbittorrent.com:80",
		"udp://tracker.openbittorrent.com:80",
	})
}

func TestNodesListPairsBEP5(t *testing.T) {
	testFileNodesMatch(t, "testdata/issue_65a.torrent", []Node{
		"185.34.3.132:5680",
		"185.34.3.103:12340",
		"94.209.253.165:47232",
		"78.46.103.11:34319",
		"195.154.162.70:55011",
		"185.34.3.137:3732",
	})
	testFileNodesMatch(t, "testdata/issue_65b.torrent", []Node{
		"95.211.203.130:6881",
		"84.72.116.169:6889",
		"204.83.98.77:7000",
		"101.187.175.163:19665",
		"37.187.118.32:6881",
		"83.128.223.71:23865",
	})
}

func TestNodeUnmarshalShapes(t *testing.T) {
	var mi MetaInfo
	require.NoError(t, bencode.Unmarshal([]byte("d5:nodesl12:1.2.3.4:5678ee"), &mi))
	assert.Equal(t, []Node{"1.2.3.4:5678"}, mi.Nodes)

	mi = MetaInfo{}
	require.NoError(t, bencode.Unmarshal([]byte("d5:nodesll7:1.2.3.4i5678eeee"), &mi))
	assert.Equal(t, []Node{"1.2.3.4:5678"}, mi.Nodes)
}

func TestNodeUnmarshalMismatch(t *testing.T) {
	var typeErr *bencode.UnmarshalTypeError
	for _, in := range []string{"i42e", "de", "le", "l7:1.2.3.4e", "li1ei2ee", "l7:1.2.3.44:porte"} {
		var n Node
		err := n.UnmarshalBencode([]byte(in))
		require.Error(t, err, in)
		assert.ErrorAs(t, err, &typeErr, in)
	}
}

func TestUrlListUnmarshalBencode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want UrlList
	}{
		{"string", "3:abc", UrlList{"abc"}},
		{"empty string", "0:", UrlList{""}},
		{"list of strings", "l3:abc3:defe", UrlList{"abc", "def"}},
		{"empty list", "le", UrlList{}},
		{"junk skipped in list", "li1e3:abcdee", UrlList{"abc"}},
		{"integer", "i42e", nil},
		{"dict", "de", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var u UrlList
			require.NoError(t, u.UnmarshalBencode([]byte(tc.in)))
			assert.Equal(t, tc.want, u)
		})
	}

	var u UrlList
	assert.Error(t, u.UnmarshalBencode([]byte("l3:ab")))
}

func TestNodesJunkElementsDropped(t *testing.T) {
	mi, err := Load(strings.NewReader("d5:nodesl14:router.io:6881i42eee"))
	require.NoError(t, err)
	assert.Equal(t, []Node{"router.io:6881"}, mi.Nodes)

	mi, err = Load(strings.NewReader("d5:nodesll9:127.0.0.14:6881eee"))
	require.NoError(t, err)
	assert.Empty(t, mi.Nodes)
}
