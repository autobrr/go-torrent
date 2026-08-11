// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package bencode

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireDecodes[T any](t *testing.T, input string, want T) {
	t.Helper()
	var got T
	require.NoError(t, Unmarshal([]byte(input), &got))
	assert.Equal(t, want, got)
}

func TestUnmarshalString(t *testing.T) {
	requireDecodes(t, "4:spam", "spam")
	requireDecodes(t, "0:", "")
	requireDecodes(t, "3:\x00\x01\xff", "\x00\x01\xff")
	requireDecodes(t, "03:abc", "abc")
	requireDecodes(t, "4:\x00abc", []byte{0, 'a', 'b', 'c'})
}

func TestUnmarshalInt(t *testing.T) {
	requireDecodes(t, "i42e", 42)
	requireDecodes(t, "i-1e", -1)
	requireDecodes(t, "i03e", int64(3))
	requireDecodes(t, "i-0e", 0)
	requireDecodes(t, "i18446744073709551615e", uint64(18446744073709551615))
	requireDecodes(t, "i-128e", int8(-128))
	requireDecodes(t, "i127e", int8(127))
	requireDecodes(t, "i7e", uint(7))
}

func TestUnmarshalBool(t *testing.T) {
	requireDecodes(t, "i0e", false)
	requireDecodes(t, "i1e", true)
	requireDecodes(t, "i-5e", true)
	requireDecodes(t, "i-0e", false)

	// Strings decode as bools the way anacrolix parses them: ParseBool
	// first, then any non-empty string is true.
	requireDecodes(t, "1:1", true)
	requireDecodes(t, "1:0", false)
	requireDecodes(t, "4:true", true)
	requireDecodes(t, "5:false", false)
	requireDecodes(t, "0:", false)
	requireDecodes(t, "1:x", true)

	var pb *bool
	require.NoError(t, Unmarshal([]byte("i1e"), &pb))
	require.NotNil(t, pb)
	assert.True(t, *pb)
}

func TestUnmarshalPointer(t *testing.T) {
	var p *int
	require.NoError(t, Unmarshal([]byte("i9e"), &p))
	require.NotNil(t, p)
	assert.Equal(t, 9, *p)
}

func TestUnmarshalList(t *testing.T) {
	requireDecodes(t, "li1ei2ei3ee", []int{1, 2, 3})
	requireDecodes(t, "l4:spam4:eggse", []string{"spam", "eggs"})
	requireDecodes(t, "lli1eeli2ei3eee", [][]int{{1}, {2, 3}})

	var got []int
	require.NoError(t, Unmarshal([]byte("le"), &got))
	require.NotNil(t, got)
	assert.Empty(t, got)
}

// Torrents in the wild encode scalar fields as one-element lists
// (anacrolix/torrent issue #297).
func TestUnmarshalSingletonList(t *testing.T) {
	requireDecodes(t, "l4:spame", "spam")
	requireDecodes(t, "li42ee", int64(42))
	requireDecodes(t, "ll4:spamee", "spam")

	var s string
	var typeErr *UnmarshalTypeError
	require.ErrorAs(t, Unmarshal([]byte("le"), &s), &typeErr)
	require.ErrorAs(t, Unmarshal([]byte("l4:spam4:eggse"), &s), &typeErr)
}

func TestUnmarshalMap(t *testing.T) {
	requireDecodes(t, "d1:ai1e1:bi2ee", map[string]int{"a": 1, "b": 2})
	requireDecodes(t, "d3:cow3:moo4:spaml1:a1:bee", map[string]any{
		"cow":  "moo",
		"spam": []any{"a", "b"},
	})
}

func TestUnmarshalMapDuplicateKeysLastWins(t *testing.T) {
	requireDecodes(t, "d1:ai1e1:ai2ee", map[string]int{"a": 2})
}

func TestUnmarshalMapUnsortedKeys(t *testing.T) {
	requireDecodes(t, "d1:bi1e1:ai2ee", map[string]int{"a": 2, "b": 1})
}

func TestUnmarshalMapNonStringKeyTarget(t *testing.T) {
	var got map[int]int
	err := Unmarshal([]byte("d1:ai1ee"), &got)
	var typeErr *UnmarshalTypeError
	require.ErrorAs(t, err, &typeErr)
	assert.Equal(t, "dict", typeErr.BencodeTypeName)
}

type taggedStruct struct {
	Renamed string `bencode:"other name"`
	Hidden  string `bencode:"-"`
	Plain   int
}

func TestUnmarshalStructTags(t *testing.T) {
	var got taggedStruct
	require.NoError(t, Unmarshal([]byte("d6:Hidden1:x5:Plaini2e10:other name1:ve"), &got))
	assert.Equal(t, "v", got.Renamed)
	assert.Equal(t, 2, got.Plain)
	assert.Empty(t, got.Hidden)
}

func TestUnmarshalStructUnknownKeysSkipped(t *testing.T) {
	type target struct {
		B int `bencode:"b"`
	}
	for _, input := range []string{
		"d1:a4:junk1:bi7ee",
		"d1:ali1eli2eee1:bi7ee",
		"d1:ad1:xd1:y0:ee1:bi7ee",
		"d1:bi7e1:zli1ei2eee",
	} {
		var got target
		require.NoError(t, Unmarshal([]byte(input), &got), "input %q", input)
		assert.Equal(t, 7, got.B, "input %q", input)
	}
}

func TestUnmarshalStructDuplicateKeysLastWins(t *testing.T) {
	type target struct {
		A int `bencode:"a"`
	}
	var got target
	require.NoError(t, Unmarshal([]byte("d1:ai1e1:ai2ee"), &got))
	assert.Equal(t, 2, got.A)
}

type EmbInner struct {
	X int    `bencode:"x"`
	Y string `bencode:"y"`
}

func TestUnmarshalEmbeddedStruct(t *testing.T) {
	type outer struct {
		EmbInner
		Z int `bencode:"z"`
	}
	var got outer
	require.NoError(t, Unmarshal([]byte("d1:xi1e1:y2:hi1:zi3ee"), &got))
	assert.Equal(t, 1, got.X)
	assert.Equal(t, "hi", got.Y)
	assert.Equal(t, 3, got.Z)
}

func TestUnmarshalEmbeddedStructPointer(t *testing.T) {
	type outer struct {
		*EmbInner
		Z int `bencode:"z"`
	}
	var got outer
	require.NoError(t, Unmarshal([]byte("d1:xi1e1:zi3ee"), &got))
	require.NotNil(t, got.EmbInner)
	assert.Equal(t, 1, got.X)
	assert.Equal(t, 3, got.Z)
}

func TestUnmarshalInterfaceShapes(t *testing.T) {
	requireDecodes[any](t, "4:spam", "spam")
	requireDecodes[any](t, "i5e", int64(5))
	requireDecodes[any](t, "li1e4:spame", []any{int64(1), "spam"})
	requireDecodes[any](t, "d1:al1:bd1:ci1eeee", map[string]any{
		"a": []any{"b", map[string]any{"c": int64(1)}},
	})
	requireDecodes[any](t, "le", []any{})
	requireDecodes[any](t, "de", map[string]any{})
}

func TestUnmarshalTypeErrorCases(t *testing.T) {
	tests := []struct {
		input    string
		target   any
		wireType string
	}{
		{"4:spam", new(int), "string"},
		{"4:spam", new([][]string), "string"},
		{"i1e", new(string), "integer"},
		{"li1ee", new(string), "integer"},
		{"l1:a1:be", new(string), "list"},
		{"le", new(map[string]int), "list"},
		{"d1:ai1ee", new(int), "dict"},
		{"de", new([]int), "dict"},
	}
	for _, tc := range tests {
		err := Unmarshal([]byte(tc.input), tc.target)
		var typeErr *UnmarshalTypeError
		require.ErrorAs(t, err, &typeErr, "input %q", tc.input)
		assert.Equal(t, tc.wireType, typeErr.BencodeTypeName, "input %q", tc.input)
		assert.Equal(t, reflect.TypeOf(tc.target).Elem(), typeErr.UnmarshalTargetType, "input %q", tc.input)
	}
}

func TestUnmarshalTypeErrorMessage(t *testing.T) {
	var n int
	err := Unmarshal([]byte("4:spam"), &n)
	var typeErr *UnmarshalTypeError
	require.ErrorAs(t, err, &typeErr)
	assert.Equal(t, "can't unmarshal a bencode string into a int", typeErr.Error())
}

func TestUnmarshalIntRangeErrors(t *testing.T) {
	tests := []struct {
		input  string
		target any
	}{
		{"i128e", new(int8)},
		{"i-129e", new(int8)},
		{"i256e", new(uint8)},
		{"i-1e", new(uint16)},
		{"i9223372036854775808e", new(int64)},
		{"i18446744073709551616e", new(uint64)},
		{"i9223372036854775808e", new(any)},
	}
	for _, tc := range tests {
		err := Unmarshal([]byte(tc.input), tc.target)
		var typeErr *UnmarshalTypeError
		require.ErrorAs(t, err, &typeErr, "input %q into %T", tc.input, tc.target)
	}
}

func TestUnmarshalIgnoreTypeErrorTruncatesSlice(t *testing.T) {
	type target struct {
		N []int `bencode:"n,ignore_unmarshal_type_error"`
	}

	var got target
	require.NoError(t, Unmarshal([]byte("d1:nli1e1:xi2eee"), &got))
	assert.Equal(t, []int{1}, got.N)

	got = target{}
	require.NoError(t, Unmarshal([]byte("d1:nl1:xee"), &got))
	assert.Empty(t, got.N)
}

type issue247 struct {
	CreationDate int64 `bencode:"creation date,ignore_unmarshal_type_error"`
	Info         Bytes `bencode:"info"`
}

func TestIgnoreUnmarshalTypeErrorIssue247(t *testing.T) {
	input := "d13:creation date23:29.03.2018 22:18:14 UTC4:infodee"
	var got issue247
	require.NoError(t, Unmarshal([]byte(input), &got))
	assert.Zero(t, got.CreationDate)
	assert.Equal(t, Bytes("de"), got.Info)
}

func TestUnmarshalTypeErrorWithoutIgnoreTag(t *testing.T) {
	type strict struct {
		CreationDate int64 `bencode:"creation date"`
		Info         Bytes `bencode:"info"`
	}
	var got strict
	err := Unmarshal([]byte("d13:creation date23:29.03.2018 22:18:14 UTC4:infodee"), &got)
	var typeErr *UnmarshalTypeError
	require.ErrorAs(t, err, &typeErr)
	assert.Equal(t, "string", typeErr.BencodeTypeName)
}

func TestIgnoreUnmarshalTypeErrorMidValue(t *testing.T) {
	type target struct {
		A []int `bencode:"a,ignore_unmarshal_type_error"`
		B int   `bencode:"b"`
	}
	var got target
	require.NoError(t, Unmarshal([]byte("d1:ali1e3:abci9ee1:bi7ee"), &got))
	assert.Equal(t, 7, got.B)
}

func TestIgnoreUnmarshalTypeErrorKeepsSyntaxErrors(t *testing.T) {
	type target struct {
		A int `bencode:"a,ignore_unmarshal_type_error"`
	}
	var got target
	err := Unmarshal([]byte("d1:ai--1ee"), &got)
	var syntaxErr *SyntaxError
	require.ErrorAs(t, err, &syntaxErr)
}

type wrappedTypeErrUnmarshaler struct{}

func (w *wrappedTypeErrUnmarshaler) UnmarshalBencode([]byte) error {
	return fmt.Errorf("wrapped: %w", &UnmarshalTypeError{
		BencodeTypeName:     "string",
		UnmarshalTargetType: reflect.TypeOf(w),
	})
}

func TestIgnoreUnmarshalTypeErrorThroughWrapping(t *testing.T) {
	type target struct {
		A wrappedTypeErrUnmarshaler `bencode:"a,ignore_unmarshal_type_error"`
		B int                       `bencode:"b"`
	}
	var got target
	require.NoError(t, Unmarshal([]byte("d1:a2:hi1:bi7ee"), &got))
	assert.Equal(t, 7, got.B)
}

type rawRecorder struct {
	raw []byte
}

func (r *rawRecorder) UnmarshalBencode(raw []byte) error {
	r.raw = append([]byte(nil), raw...)
	return nil
}

func TestUnmarshalerReceivesRawValue(t *testing.T) {
	type target struct {
		Foo rawRecorder `bencode:"foo"`
	}
	var got target
	require.NoError(t, Unmarshal([]byte("d3:food1:b0:1:a0:ee"), &got))
	assert.Equal(t, []byte("d1:b0:1:a0:e"), got.Foo.raw)
}

var errBoom = errors.New("boom")

type failingUnmarshaler struct{}

func (f *failingUnmarshaler) UnmarshalBencode([]byte) error { return errBoom }

func TestUnmarshalerErrorWrapped(t *testing.T) {
	var got failingUnmarshaler
	err := Unmarshal([]byte("i1e"), &got)
	var wrapErr *UnmarshalerError
	require.ErrorAs(t, err, &wrapErr)
	assert.ErrorIs(t, err, errBoom)
}

type typeErrUnmarshaler struct{}

func (u *typeErrUnmarshaler) UnmarshalBencode([]byte) error {
	return &UnmarshalTypeError{BencodeTypeName: "string", UnmarshalTargetType: reflect.TypeOf(u)}
}

func TestUnmarshalerTypeErrorPassthrough(t *testing.T) {
	var got typeErrUnmarshaler
	err := Unmarshal([]byte("4:spam"), &got)
	var typeErr *UnmarshalTypeError
	require.ErrorAs(t, err, &typeErr)
	var wrapErr *UnmarshalerError
	assert.NotErrorAs(t, err, &wrapErr)
}

func TestUnmarshalInvalidArg(t *testing.T) {
	var invalidErr *UnmarshalInvalidArgError

	require.ErrorAs(t, Unmarshal([]byte("i1e"), nil), &invalidErr)
	require.ErrorAs(t, Unmarshal([]byte("i1e"), 5), &invalidErr)

	var p *int
	require.ErrorAs(t, Unmarshal([]byte("i1e"), p), &invalidErr)
}

func TestSyntaxErrors(t *testing.T) {
	inputs := []string{
		"",
		"<html><body>x</body></html>",
		"{",
		"x",
		"e",
		":",
		"i",
		"ie",
		"i-e",
		"i--1e",
		"i1.5e",
		"i12x",
		"-1:x",
		"3:ab",
		"5:spa",
		"99:x",
		"9999999999:",
		"2147483649:",
		"l",
		"li1e",
		"lxe",
		"d",
		"d3:fooe",
		"di1ei2ee",
		"d4:spamxe",
		strings.Repeat("l", 600),
	}
	for _, input := range inputs {
		var got any
		err := Unmarshal([]byte(input), &got)
		var syntaxErr *SyntaxError
		require.ErrorAs(t, err, &syntaxErr, "input %q", input)
	}
}

func TestSyntaxErrorOffsets(t *testing.T) {
	tests := []struct {
		input      string
		offset     int64
		wantEOF    bool
		wantOffset bool
	}{
		{input: "", offset: 0, wantEOF: true, wantOffset: true},
		{input: "<html>", offset: 0, wantOffset: true},
		{input: "lxe", offset: 1, wantOffset: true},
		{input: "d4:spamxe", offset: 7, wantOffset: true},
		{input: "i-e", offset: 0, wantOffset: true},
		{input: "3:ab", wantEOF: true},
	}
	for _, tc := range tests {
		var got any
		err := Unmarshal([]byte(tc.input), &got)
		var syntaxErr *SyntaxError
		require.ErrorAs(t, err, &syntaxErr, "input %q", tc.input)
		if tc.wantOffset {
			assert.Equal(t, tc.offset, syntaxErr.Offset, "input %q", tc.input)
		}
		if tc.wantEOF {
			assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "input %q", tc.input)
		}
	}
}

func TestSyntaxErrorMessage(t *testing.T) {
	var got any
	err := Unmarshal([]byte("i-e"), &got)
	var syntaxErr *SyntaxError
	require.ErrorAs(t, err, &syntaxErr)
	assert.Equal(t, fmt.Sprintf("bencode: syntax error (offset: 0): %s", syntaxErr.What), syntaxErr.Error())
}

func TestTruncatedPrefixes(t *testing.T) {
	valid := "d1:ali-1ei0el2:xyd1:a0:eee4:infod6:lengthi42e6:pieces8:AAAAAAAAee"

	var whole any
	require.NoError(t, Unmarshal([]byte(valid), &whole))

	for i := 0; i < len(valid); i++ {
		var got any
		err := Unmarshal([]byte(valid[:i]), &got)
		require.Error(t, err, "prefix %q", valid[:i])
		var syntaxErr *SyntaxError
		require.ErrorAs(t, err, &syntaxErr, "prefix %q", valid[:i])
	}
}

func TestNestingDepth(t *testing.T) {
	var got any
	ok := strings.Repeat("l", 512) + strings.Repeat("e", 512)
	require.NoError(t, Unmarshal([]byte(ok), &got))

	bomb := strings.Repeat("l", 600) + strings.Repeat("e", 600)
	err := Unmarshal([]byte(bomb), &got)
	var syntaxErr *SyntaxError
	require.ErrorAs(t, err, &syntaxErr)
}

func TestUnmarshalTrailingBytes(t *testing.T) {
	var n int
	err := Unmarshal([]byte("i1ei2e"), &n)
	var trailing ErrUnusedTrailingBytes
	require.ErrorAs(t, err, &trailing)
	assert.Equal(t, 1, n)
	assert.Equal(t, 3, trailing.NumUnusedBytes)
	assert.Equal(t, "3 unused trailing bytes", trailing.Error())
}

func TestDecoderSequentialValues(t *testing.T) {
	d := NewDecoder(strings.NewReader("i1e4:spam"))

	var n int
	require.NoError(t, d.Decode(&n))
	assert.Equal(t, 1, n)
	require.Error(t, d.ReadEOF())

	var s string
	require.NoError(t, d.Decode(&s))
	assert.Equal(t, "spam", s)
	require.NoError(t, d.ReadEOF())
}

func TestDecoderTrailingGarbage(t *testing.T) {
	d := NewDecoder(strings.NewReader("i1egarbage"))

	var n int
	require.NoError(t, d.Decode(&n))
	assert.Equal(t, 1, n)
	require.Error(t, d.ReadEOF())
}
