// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

// Package bencode implements encoding and decoding of bencode data as used
// in the BitTorrent metainfo format (BEP 3).
//
// The decoder tolerates non-canonical input seen in the wild: integer
// literals and string length prefixes may carry leading zeros, i-0e decodes
// as zero, and dict keys may be unsorted or duplicated (the last value
// wins). It stays strict where safety requires it: values nested deeper
// than 512 levels are rejected, and integers must fit the target type, with
// no big-integer fallback.
package bencode

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// Marshaler is implemented by types that can encode themselves as bencode.
type Marshaler interface {
	MarshalBencode() ([]byte, error)
}

// Unmarshaler is implemented by types that can decode themselves from bencode.
type Unmarshaler interface {
	UnmarshalBencode([]byte) error
}

// Marshal returns the bencode encoding of v.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal decodes a single bencode value from data into the value pointed
// to by v, failing with ErrUnusedTrailingBytes if input remains after it.
func Unmarshal(data []byte, v any) error {
	d := &Decoder{data: data}
	if err := d.Decode(v); err != nil {
		return err
	}
	if n := len(data) - d.pos; n > 0 {
		return ErrUnusedTrailingBytes{NumUnusedBytes: n}
	}
	return nil
}

// NewDecoder returns a Decoder reading bencode values from r.
func NewDecoder(r io.Reader) *Decoder {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &Decoder{r: br}
}

// NewEncoder returns an Encoder writing bencode values to w.
func NewEncoder(w io.Writer) *Encoder {
	// A bytes.Buffer cannot fail and already takes byte and string writes,
	// so wrapping it in bufio would only add a copy of everything written.
	if buf, ok := w.(*bytes.Buffer); ok {
		return &Encoder{w: buf}
	}
	return &Encoder{w: bufio.NewWriter(w)}
}

// Bytes captures a raw bencode value verbatim, allowing byte-identical
// round-trips of subtrees such as a torrent's info dict.
type Bytes []byte

// UnmarshalBencode stores a copy of the raw value.
func (b *Bytes) UnmarshalBencode(raw []byte) error {
	*b = append([]byte(nil), raw...)
	return nil
}

// MarshalBencode returns the raw value unchanged, or an error if b is empty
// and therefore not a valid bencode value.
func (b Bytes) MarshalBencode() ([]byte, error) {
	if len(b) == 0 {
		return nil, errors.New("bencode: Bytes value is zero-length")
	}
	return b, nil
}
