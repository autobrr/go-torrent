// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

// Package metainfo parses .torrent files (BEP 3, BEP 12, BEP 19, BEP 52),
// tolerating the malformed optional fields that real-world trackers serve.
package metainfo

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/autobrr/go-torrent/bencode"
)

// MetaInfo is a decoded .torrent file (BEP 3).
type MetaInfo struct {
	InfoBytes    bencode.Bytes     `bencode:"info,omitempty"`
	Announce     string            `bencode:"announce,omitempty"`
	AnnounceList AnnounceList      `bencode:"announce-list,omitempty"`
	Nodes        []Node            `bencode:"nodes,omitempty,ignore_unmarshal_type_error"`
	CreationDate int64             `bencode:"creation date,omitempty,ignore_unmarshal_type_error"`
	Comment      string            `bencode:"comment,omitempty,ignore_unmarshal_type_error"`
	CreatedBy    string            `bencode:"created by,omitempty,ignore_unmarshal_type_error"`
	Encoding     string            `bencode:"encoding,omitempty,ignore_unmarshal_type_error"`
	UrlList      UrlList           `bencode:"url-list,omitempty"`
	PieceLayers  map[string]string `bencode:"piece layers,omitempty,ignore_unmarshal_type_error"`
}

// Load decodes a single bencode value from r as a MetaInfo. Trailing ASCII
// whitespace and NUL bytes are tolerated, a deviation from anacrolix/torrent:
// some tracker webservers append a newline after the torrent payload.
func Load(r io.Reader) (*MetaInfo, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var mi MetaInfo
	err = bencode.Unmarshal(data, &mi)
	if err == nil {
		return &mi, nil
	}
	var trailing bencode.ErrUnusedTrailingBytes
	if !errors.As(err, &trailing) {
		return nil, err
	}
	for _, b := range data[len(data)-trailing.NumUnusedBytes:] {
		switch b {
		case ' ', '\t', '\r', '\n', 0:
		default:
			return nil, fmt.Errorf("error after decoding metainfo: %w", err)
		}
	}
	return &mi, nil
}

// LoadFromFile decodes a MetaInfo from the named file.
func LoadFromFile(filename string) (*MetaInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// UnmarshalInfo decodes the raw info dict.
func (mi *MetaInfo) UnmarshalInfo() (Info, error) {
	var info Info
	err := bencode.Unmarshal(mi.InfoBytes, &info)
	return info, err
}

// HashInfoBytes returns the v1 infohash of the raw info dict.
func (mi *MetaInfo) HashInfoBytes() Hash {
	return HashBytes(mi.InfoBytes)
}

// Write encodes mi as bencode to w.
func (mi *MetaInfo) Write(w io.Writer) error {
	return bencode.NewEncoder(w).Encode(mi)
}

// UpvertedAnnounceList returns the announce-list, converted from the single
// announce field when the list does not override it.
func (mi *MetaInfo) UpvertedAnnounceList() AnnounceList {
	if mi.AnnounceList.OverridesAnnounce(mi.Announce) {
		return mi.AnnounceList
	}
	if mi.Announce != "" {
		return AnnounceList{{mi.Announce}}
	}
	return nil
}
