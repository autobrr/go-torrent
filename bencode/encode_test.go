// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package bencode

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireEncodes(t *testing.T, v any, want string) {
	t.Helper()
	got, err := Marshal(v)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

func TestMarshalScalars(t *testing.T) {
	requireEncodes(t, 42, "i42e")
	requireEncodes(t, int64(-7), "i-7e")
	requireEncodes(t, int8(-128), "i-128e")
	requireEncodes(t, uint8(255), "i255e")
	requireEncodes(t, uint64(18446744073709551615), "i18446744073709551615e")
	requireEncodes(t, true, "i1e")
	requireEncodes(t, false, "i0e")
	requireEncodes(t, "", "0:")
	requireEncodes(t, "spam", "4:spam")
	requireEncodes(t, "\x00\x01\xff", "3:\x00\x01\xff")
	requireEncodes(t, []byte(nil), "0:")
	requireEncodes(t, []byte{}, "0:")
	requireEncodes(t, []byte{0, 1}, "2:\x00\x01")
}

func TestMarshalList(t *testing.T) {
	requireEncodes(t, []int{1, 2, 3}, "li1ei2ei3ee")
	requireEncodes(t, []int(nil), "le")
	requireEncodes(t, []int{}, "le")
	requireEncodes(t, []any{int64(1), "a", []any{}}, "li1e1:alee")
	requireEncodes(t, [][]string{{"a"}, {"b", "c"}}, "ll1:ael1:b1:cee")
}

func TestMarshalMapSortedKeys(t *testing.T) {
	requireEncodes(t, map[string]int{"c": 3, "a": 1, "b": 2}, "d1:ai1e1:bi2e1:ci3ee")
	requireEncodes(t, map[string]int(nil), "de")
	requireEncodes(t, map[string]int{}, "de")
	requireEncodes(t, map[string]any{"z": "last", "piece length": int64(1), "pieces": ""}, "d12:piece lengthi1e6:pieces0:1:z4:laste")
}

func TestMarshalStructSpecExample(t *testing.T) {
	v := struct {
		Name        string `bencode:"name"`
		PieceLength int64  `bencode:"piece length"`
		Pieces      []byte `bencode:"pieces"`
	}{Pieces: []byte{}}
	requireEncodes(t, v, "d4:name0:12:piece lengthi0e6:pieces0:e")
}

func TestMarshalStructTags(t *testing.T) {
	requireEncodes(t, taggedStruct{Renamed: "v", Hidden: "x", Plain: 2}, "d5:Plaini2e10:other name1:ve")
}

func TestMarshalOmitEmpty(t *testing.T) {
	type target struct {
		Str   string         `bencode:"str,omitempty"`
		Num   int            `bencode:"num,omitempty"`
		Unum  uint           `bencode:"unum,omitempty"`
		Flag  bool           `bencode:"flag,omitempty"`
		Ptr   *int           `bencode:"ptr,omitempty"`
		Iface any            `bencode:"iface,omitempty"`
		Slice []int          `bencode:"slice,omitempty"`
		Blob  []byte         `bencode:"blob,omitempty"`
		Dict  map[string]int `bencode:"dict,omitempty"`
	}

	requireEncodes(t, target{}, "de")

	requireEncodes(t, target{
		Blob:  []byte{},
		Slice: []int{},
		Dict:  map[string]int{},
	}, "d4:blob0:4:dictde5:slicelee")

	zero := 0
	requireEncodes(t, target{Ptr: &zero}, "d3:ptri0ee")

	requireEncodes(t, target{Str: "s", Num: 1, Unum: 2, Flag: true, Iface: "x"},
		"d4:flagi1e5:iface1:x3:numi1e3:str1:s4:unumi2ee")
}

func TestMarshalOmitEmptyStruct(t *testing.T) {
	type inner struct {
		A string `bencode:"a,omitempty"`
		B []int  `bencode:"b,omitempty"`
	}
	type target struct {
		Sub inner `bencode:"sub,omitempty"`
	}

	requireEncodes(t, target{}, "de")
	requireEncodes(t, target{Sub: inner{A: "x"}}, "d3:subd1:a1:xee")
	requireEncodes(t, target{Sub: inner{B: []int{}}}, "d3:subd1:bleee")
}

func TestMarshalEmbeddedStruct(t *testing.T) {
	type outer struct {
		EmbInner
		Z int `bencode:"z"`
	}
	requireEncodes(t, outer{EmbInner: EmbInner{X: 1, Y: "hi"}, Z: 3}, "d1:xi1e1:y2:hi1:zi3ee")
}

func TestMarshalEmbeddedNilPointerSkipsFields(t *testing.T) {
	type outer struct {
		*EmbInner
		Z int `bencode:"z"`
	}
	requireEncodes(t, outer{Z: 3}, "d1:zi3ee")
	requireEncodes(t, outer{EmbInner: &EmbInner{X: 1}, Z: 3}, "d1:xi1e1:y0:1:zi3ee")
}

func TestMarshalNilPointerFieldEncodesZeroValue(t *testing.T) {
	type target struct {
		P *int    `bencode:"p"`
		S *string `bencode:"s"`
	}
	requireEncodes(t, target{}, "d1:pi0e1:s0:e")
}

func TestMarshalTypeErrorKinds(t *testing.T) {
	for _, v := range []any{
		3.14,
		float32(1),
		complex(1, 2),
		make(chan int),
		func() {},
		map[int]string{},
	} {
		_, err := Marshal(v)
		var typeErr *MarshalTypeError
		require.ErrorAs(t, err, &typeErr, "value %T", v)
	}

	_, err := Marshal(struct{ F float64 }{})
	var typeErr *MarshalTypeError
	require.ErrorAs(t, err, &typeErr)
	assert.Equal(t, "bencode: unsupported type: float64", typeErr.Error())
}

type constMarshaler struct{}

func (constMarshaler) MarshalBencode() ([]byte, error) { return []byte("5:const"), nil }

type ptrMarshaler struct {
	n int
}

func (p *ptrMarshaler) MarshalBencode() ([]byte, error) {
	return Marshal(p.n * 2)
}

type failingMarshaler struct{}

func (failingMarshaler) MarshalBencode() ([]byte, error) { return nil, errBoom }

func TestMarshalerHonored(t *testing.T) {
	requireEncodes(t, constMarshaler{}, "5:const")

	type target struct {
		C constMarshaler `bencode:"c"`
		P ptrMarshaler   `bencode:"p"`
	}
	got, err := Marshal(&target{P: ptrMarshaler{n: 3}})
	require.NoError(t, err)
	assert.Equal(t, "d1:c5:const1:pi6ee", string(got))
}

func TestMarshalerErrorWrapped(t *testing.T) {
	_, err := Marshal(failingMarshaler{})
	var wrapErr *MarshalerError
	require.ErrorAs(t, err, &wrapErr)
	assert.ErrorIs(t, err, errBoom)
}

func TestEncoderFlushesPerEncode(t *testing.T) {
	var buf bytes.Buffer
	e := NewEncoder(&buf)
	require.NoError(t, e.Encode(1))
	assert.Equal(t, "i1e", buf.String())
	require.NoError(t, e.Encode("hi"))
	assert.Equal(t, "i1e2:hi", buf.String())
}

func TestMarshalNil(t *testing.T) {
	_, err := Marshal(nil)
	var typeErr *MarshalTypeError
	require.ErrorAs(t, err, &typeErr)
}
