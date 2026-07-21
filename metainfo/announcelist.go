// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"net"
	"reflect"
	"slices"
	"strconv"

	"github.com/autobrr/go-torrent/bencode"
)

// AnnounceList is the tiers of trackers from BEP 12.
type AnnounceList [][]string

// Clone returns a copy of the tier list; the tier slices themselves are
// shared.
func (al AnnounceList) Clone() AnnounceList {
	return slices.Clone(al)
}

// OverridesAnnounce reports whether al should be preferred over the single
// announce URL.
func (al AnnounceList) OverridesAnnounce(announce string) bool {
	for _, tier := range al {
		for _, url := range tier {
			if url != "" || announce == "" {
				return true
			}
		}
	}
	return false
}

// DistinctValues returns the tracker URLs across all tiers, deduplicated in
// order of first appearance.
func (al AnnounceList) DistinctValues() []string {
	var ret []string
	seen := make(map[string]struct{})
	for _, tier := range al {
		for _, url := range tier {
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			ret = append(ret, url)
		}
	}
	return ret
}

// UnmarshalBencode is deliberately lenient because trackers serve malformed
// announce-lists in the wild (FunFile encodes an empty one as the bencode
// string "0:"). A non-empty string decodes as a single tier, a string
// element in the list becomes its own tier, non-string junk inside a tier is
// skipped, tiers left empty are dropped, and any other value type decodes as
// nil. Only malformed bencode is an error.
func (al *AnnounceList) UnmarshalBencode(b []byte) error {
	var v any
	if err := bencode.Unmarshal(b, &v); err != nil {
		return err
	}
	*al = lenientAnnounceList(v)
	return nil
}

func lenientAnnounceList(v any) AnnounceList {
	switch v := v.(type) {
	case string:
		if v == "" {
			return nil
		}
		return AnnounceList{{v}}
	case []any:
		var al AnnounceList
		for _, elem := range v {
			switch elem := elem.(type) {
			case string:
				al = append(al, []string{elem})
			case []any:
				var tier []string
				for _, e := range elem {
					if s, ok := e.(string); ok {
						tier = append(tier, s)
					}
				}
				if len(tier) > 0 {
					al = append(al, tier)
				}
			}
		}
		return al
	}
	return nil
}

// Node is a DHT bootstrap node from BEP 5, as "host:port".
type Node string

// UnmarshalBencode accepts either a "host:port" string or a [host, port]
// pair. Any other shape returns a bencode.UnmarshalTypeError so that fields
// tagged ignore_unmarshal_type_error can drop it.
func (n *Node) UnmarshalBencode(b []byte) error {
	var v any
	if err := bencode.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case string:
		*n = Node(v)
		return nil
	case []any:
		if len(v) == 2 {
			host, hostOK := v[0].(string)
			port, portOK := v[1].(int64)
			if hostOK && portOK {
				*n = Node(net.JoinHostPort(host, strconv.FormatInt(port, 10)))
				return nil
			}
		}
	}
	return &bencode.UnmarshalTypeError{
		BencodeTypeName:     bencodeTypeName(v),
		UnmarshalTargetType: reflect.TypeFor[Node](),
	}
}

// UrlList is the list of web seed URLs from BEP 19.
type UrlList []string

// UnmarshalBencode accepts either a single URL string or a list, with the
// same leniency as AnnounceList: non-string junk inside a list is skipped
// and any other value type decodes as nil.
func (u *UrlList) UnmarshalBencode(b []byte) error {
	var v any
	if err := bencode.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case string:
		*u = UrlList{v}
	case []any:
		list := make(UrlList, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok {
				list = append(list, s)
			}
		}
		*u = list
	default:
		*u = nil
	}
	return nil
}

func bencodeTypeName(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case int64:
		return "integer"
	case []any:
		return "list"
	default:
		return "dict"
	}
}
