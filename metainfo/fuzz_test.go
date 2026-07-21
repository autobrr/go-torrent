// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func FuzzLoad(f *testing.F) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".torrent") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata", e.Name()))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	for _, s := range []string{
		"de",
		"d4:infodee",
		"d4:infod4:name1:x12:piece lengthi1e6:pieces20:aaaaaaaaaaaaaaaaaaaaee",
		"d13:announce-list0:8:announce3:urle",
		"d13:announce-listll3:abcei1eee",
		"d5:nodes0:e",
		"d5:nodesll7:1.2.3.4i5678eeee",
		"d8:url-list0:e",
		"d4:infod12:meta versioni2e9:file treed1:ad0:d6:lengthi5e11:pieces root0:eeeee",
		"d13:creation date23:29.03.2018 22:18:14 UTC4:infodee",
		"de\n",
	} {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		mi, err := Load(bytes.NewReader(data))
		if err != nil {
			return
		}
		_ = mi.HashInfoBytes()
		_ = mi.UpvertedAnnounceList().DistinctValues()
		if info, err := mi.UnmarshalInfo(); err == nil {
			_ = info.TotalLength()
			_ = info.NumPieces()
			_ = info.BestName()
			_ = info.IsDir()
			for _, fi := range info.UpvertedFiles() {
				_ = fi.DisplayPath(&info)
			}
		}
		if err := mi.Write(io.Discard); err != nil {
			t.Fatalf("write of loaded metainfo failed: %v", err)
		}
	})
}
