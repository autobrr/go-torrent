// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package bencode

import (
	"bytes"
	"reflect"
	"testing"
)

type fuzzTarget struct {
	Name  string         `bencode:"name"`
	Count int64          `bencode:"count,ignore_unmarshal_type_error"`
	Tags  []string       `bencode:"tags,omitempty"`
	Info  Bytes          `bencode:"info,omitempty"`
	Extra map[string]any `bencode:"extra,omitempty"`
	Flag  bool           `bencode:"flag"`
	Nums  []int          `bencode:"nums,omitempty"`
}

func FuzzUnmarshal(f *testing.F) {
	seeds := []string{
		"i42e",
		"i-0e",
		"0:",
		"4:spam",
		"le",
		"de",
		"li1ei2ee",
		"d3:cow3:moo4:spam4:eggse",
		"d4:spaml1:a1:bee",
		"d1:ad1:bd1:cdeeee",
		"d13:creation date23:29.03.2018 22:18:14 UTC4:infodee",
		"llllleeeee",
		"10:aaaaaaaaaa",
		"d4:name0:12:piece lengthi0e6:pieces0:e",
		"d5:counti-1e4:name1:x4:tagsl1:a1:bee",
		"9999999999:",
		"i9223372036854775807e",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var generic any
		genericErr := Unmarshal(data, &generic)
		if genericErr == nil {
			if _, err := Marshal(generic); err != nil {
				t.Fatalf("re-marshal of decoded any failed: %v", err)
			}
		}

		// The slice fast path and the stream path must agree.
		var streamed any
		d := NewDecoder(bytes.NewReader(data))
		streamErr := d.Decode(&streamed)
		if streamErr == nil {
			streamErr = d.ReadEOF()
		}
		if (genericErr == nil) != (streamErr == nil) {
			t.Fatalf("fast/stream disagree: fast=%v stream=%v", genericErr, streamErr)
		}
		if genericErr == nil && !reflect.DeepEqual(generic, streamed) {
			t.Fatalf("fast/stream values differ: %#v vs %#v", generic, streamed)
		}

		var typed fuzzTarget
		if err := Unmarshal(data, &typed); err == nil {
			if _, err := Marshal(typed); err != nil {
				t.Fatalf("re-marshal of decoded struct failed: %v", err)
			}
		}
	})
}
