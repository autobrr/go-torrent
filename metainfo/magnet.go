// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	btihPrefix = "urn:btih:"
	btmhPrefix = "urn:btmh:"

	// multihashSHA256Code is the multihash code for SHA2-256 (BEP 52 wraps
	// the v2 infohash in a multihash for the btmh URN).
	multihashSHA256Code = 0x12
)

// Magnet is the components of a magnet link (BEP 9). It supports v1, hybrid,
// and v2 (BEP 52) links.
type Magnet struct {
	InfoHash    Hash       // v1 infohash; zero when absent
	InfoHashV2  HashV2     // v2 infohash; zero when absent
	Trackers    []string   // "tr" values
	DisplayName string     // "dn" value, if not empty
	Params      url.Values // All other values, such as "x.pe", "as", "xs" etc.
}

// String renders the magnet link as a URI.
func (m Magnet) String() string {
	// Deep-copy m.Params
	vs := make(url.Values, len(m.Params)+len(m.Trackers)+2)
	for k, v := range m.Params {
		vs[k] = append([]string(nil), v...)
	}

	for _, tr := range m.Trackers {
		vs.Add("tr", tr)
	}
	if m.DisplayName != "" {
		vs.Add("dn", m.DisplayName)
	}

	// Transmission and Deluge both expect "urn:btih:" to be unescaped, and
	// Deluge wants it at the start of the magnet link.
	u := url.URL{
		Scheme: "magnet",
	}
	var queryParts []string
	if !m.InfoHash.IsZero() {
		queryParts = append(queryParts, "xt="+btihPrefix+m.InfoHash.HexString())
	}
	if !m.InfoHashV2.IsZero() {
		queryParts = append(queryParts, "xt="+btmhPrefix+multihashHexPrefix()+m.InfoHashV2.HexString())
	}
	if rem := vs.Encode(); rem != "" {
		queryParts = append(queryParts, rem)
	}
	u.RawQuery = strings.Join(queryParts, "&")
	return u.String()
}

// ParseMagnetUri parses a magnet-formatted URI into a Magnet. At most one v1
// and one v2 infohash may be present; neither is required.
func ParseMagnetUri(uri string) (m Magnet, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return m, fmt.Errorf("error parsing uri: %w", err)
	}
	if u.Scheme != "magnet" {
		return m, fmt.Errorf("unexpected scheme %q", u.Scheme)
	}
	q := u.Query()
	var gotV1, gotV2 bool
	for _, xt := range q["xt"] {
		if encoded, found := strings.CutPrefix(xt, btihPrefix); found {
			if gotV1 {
				return m, errors.New("more than one v1 infohash found in magnet link")
			}
			m.InfoHash, err = parseEncodedV1Infohash(encoded)
			if err != nil {
				return m, fmt.Errorf("error parsing infohash %q: %w", encoded, err)
			}
			gotV1 = true
		} else if encoded, found := strings.CutPrefix(xt, btmhPrefix); found {
			if gotV2 {
				return m, errors.New("more than one v2 infohash found in magnet link")
			}
			m.InfoHashV2, err = parseV2Infohash(encoded)
			if err != nil {
				return m, fmt.Errorf("error parsing infohash %q: %w", encoded, err)
			}
			gotV2 = true
		} else {
			lazyAddParam(&m.Params, "xt", xt)
		}
	}
	q.Del("xt")
	m.DisplayName = popFirstValue(q, "dn")
	m.Trackers = q["tr"]
	q.Del("tr")
	// Add everything we haven't consumed.
	copyParams(&m.Params, q)
	return m, nil
}

// Magnet creates a Magnet from mi. It supports v1, hybrid, and v2 torrents.
func (mi *MetaInfo) Magnet() (m Magnet, err error) {
	m.Trackers = append(m.Trackers, mi.UpvertedAnnounceList().DistinctValues()...)
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return
	}
	m.DisplayName = info.BestName()
	if info.HasV1() {
		m.InfoHash = mi.HashInfoBytes()
	}
	if info.HasV2() {
		m.InfoHashV2 = HashV2Bytes(mi.InfoBytes)
	}
	if len(mi.UrlList) != 0 {
		m.Params = url.Values{"ws": mi.UrlList}
	}
	return
}

func parseEncodedV1Infohash(encoded string) (ih Hash, err error) {
	var decode func(dst, src []byte) (int, error)
	switch len(encoded) {
	case 40:
		decode = hex.Decode
	case 32:
		decode = base32.StdEncoding.Decode
	default:
		return ih, fmt.Errorf("unhandled xt parameter encoding (encoded length %d)", len(encoded))
	}
	n, err := decode(ih[:], []byte(encoded))
	if err != nil {
		return ih, fmt.Errorf("error decoding xt: %w", err)
	}
	if n != HashSize {
		return ih, fmt.Errorf("decoded xt length %d, expected %d", n, HashSize)
	}
	return ih, nil
}

func parseV2Infohash(encoded string) (ih HashV2, err error) {
	b, err := hex.DecodeString(encoded)
	if err != nil {
		return
	}
	if len(b) != 2+HashV2Size || b[0] != multihashSHA256Code || b[1] != HashV2Size {
		return ih, errors.New("bad multihash")
	}
	copy(ih[:], b[2:])
	return ih, nil
}

// multihashHexPrefix returns the hex encoding of the multihash header for a
// SHA2-256 digest ("1220").
func multihashHexPrefix() string {
	return hex.EncodeToString([]byte{multihashSHA256Code, HashV2Size})
}

func lazyAddParam(vs *url.Values, k, v string) {
	if *vs == nil {
		*vs = make(url.Values)
	}
	vs.Add(k, v)
}

func copyParams(dest *url.Values, src url.Values) {
	for k, vs := range src {
		for _, v := range vs {
			lazyAddParam(dest, k, v)
		}
	}
}

// popFirstValue removes and returns the first value for key, leaving any
// remaining values in place.
func popFirstValue(vs url.Values, key string) string {
	sl := vs[key]
	switch len(sl) {
	case 0:
		return ""
	case 1:
		vs.Del(key)
		return sl[0]
	default:
		vs[key] = sl[1:]
		return sl[0]
	}
}
