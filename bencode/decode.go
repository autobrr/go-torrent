// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package bencode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
)

// maxDepth bounds nesting so hostile input such as a long run of "l" bytes
// cannot exhaust the stack.
const maxDepth = 512

const maxStringLen = 1 << 31

// readChunkSize caps per-step allocation while reading string contents, so a
// length prefix larger than the remaining input cannot force a huge
// allocation before the read fails.
const readChunkSize = 4 << 20

// Decoder reads bencode values from an input stream.
type Decoder struct {
	r *bufio.Reader
	// data enables the zero-copy path used by Unmarshal: values decode
	// directly from the slice, captures are subslices, and skips are pure
	// scans. offset is the running stream position, or the absolute base
	// offset of data when set.
	data   []byte
	pos    int
	offset int64
}

func (d *Decoder) curOffset() int64 {
	if d.r == nil {
		return d.offset + int64(d.pos)
	}
	return d.offset
}

// Decode reads the next bencode value from the input and stores it in the
// value pointed to by v.
func (d *Decoder) Decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &UnmarshalInvalidArgError{Type: reflect.TypeOf(v)}
	}
	return d.decodeValue(rv.Elem(), 0)
}

// ReadEOF returns nil only if the underlying reader is exhausted.
func (d *Decoder) ReadEOF() error {
	if d.r == nil {
		if d.pos < len(d.data) {
			return errors.New("bencode: expected EOF")
		}
		return nil
	}
	_, err := d.r.ReadByte()
	if err == nil {
		_ = d.r.UnreadByte()
		return errors.New("bencode: expected EOF")
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return fmt.Errorf("bencode: expected EOF: %w", err)
}

func (d *Decoder) decodeValue(v reflect.Value, depth int) error {
	if depth > maxDepth {
		return d.depthError()
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if u, ok := v.Interface().(Unmarshaler); ok {
			return d.decodeUnmarshaler(u, v.Type(), depth)
		}
		v = v.Elem()
	}
	if v.CanAddr() {
		if u, ok := v.Addr().Interface().(Unmarshaler); ok {
			return d.decodeUnmarshaler(u, v.Addr().Type(), depth)
		}
	}
	if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
		x, err := d.decodeInterface(depth)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(x))
		return nil
	}

	b, err := d.readByte()
	if err != nil {
		return err
	}
	switch {
	case b == 'i':
		return d.decodeInt(v)
	case b >= '0' && b <= '9':
		return d.decodeString(b, v)
	case b == 'l':
		return d.decodeList(v, depth)
	case b == 'd':
		return d.decodeDict(v, depth)
	}
	return d.typeCharError(b)
}

func (d *Decoder) decodeUnmarshaler(u Unmarshaler, t reflect.Type, depth int) error {
	raw, err := d.captureValue(depth)
	if err != nil {
		return err
	}
	err = u.UnmarshalBencode(raw)
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*UnmarshalTypeError](err); ok {
		return err
	}
	return &UnmarshalerError{Type: t, Err: err}
}

func (d *Decoder) decodeInterface(depth int) (any, error) {
	if depth > maxDepth {
		return nil, d.depthError()
	}
	b, err := d.readByte()
	if err != nil {
		return nil, err
	}
	switch {
	case b == 'i':
		lit, err := d.readIntLiteral()
		if err != nil {
			return nil, err
		}
		n, err := strconv.ParseInt(lit, 10, 64)
		if err != nil {
			return nil, &UnmarshalTypeError{BencodeTypeName: "integer", UnmarshalTargetType: reflect.TypeFor[int64]()}
		}
		return n, nil
	case b >= '0' && b <= '9':
		_, content, err := d.readStringParts(b)
		if err != nil {
			return nil, err
		}
		return string(content), nil
	case b == 'l':
		list := []any{}
		for {
			c, err := d.peekByte()
			if err != nil {
				return nil, err
			}
			if c == 'e' {
				d.discardByte()
				return list, nil
			}
			elem, err := d.decodeInterface(depth + 1)
			if err != nil {
				return nil, err
			}
			list = append(list, elem)
		}
	case b == 'd':
		dict := map[string]any{}
		for {
			c, err := d.peekByte()
			if err != nil {
				return nil, err
			}
			if c == 'e' {
				d.discardByte()
				return dict, nil
			}
			_, key, err := d.readDictKey()
			if err != nil {
				return nil, err
			}
			val, err := d.decodeInterface(depth + 1)
			if err != nil {
				return nil, err
			}
			dict[string(key)] = val
		}
	}
	return nil, d.typeCharError(b)
}

// decodeInt reports range and sign mismatches as UnmarshalTypeError, not
// SyntaxError: readIntLiteral already validated the syntax, and type errors
// are what ignore_unmarshal_type_error fields swallow, so an absurd value in
// junk metadata cannot fail a whole parse.
func (d *Decoder) decodeInt(v reflect.Value) error {
	lit, err := d.readIntLiteral()
	if err != nil {
		return err
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(lit, 10, v.Type().Bits())
		if err != nil {
			return &UnmarshalTypeError{BencodeTypeName: "integer", UnmarshalTargetType: v.Type()}
		}
		v.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(lit, 10, v.Type().Bits())
		if err != nil {
			return &UnmarshalTypeError{BencodeTypeName: "integer", UnmarshalTargetType: v.Type()}
		}
		v.SetUint(n)
		return nil
	case reflect.Bool:
		n, err := strconv.ParseInt(lit, 10, 64)
		if err != nil {
			// An out-of-range literal is still a non-zero integer.
			v.SetBool(true)
			return nil //nolint:nilerr
		}
		v.SetBool(n != 0)
		return nil
	}
	return &UnmarshalTypeError{BencodeTypeName: "integer", UnmarshalTargetType: v.Type()}
}

func (d *Decoder) decodeString(first byte, v reflect.Value) error {
	_, content, err := d.readStringParts(first)
	if err != nil {
		return err
	}
	switch {
	case v.Kind() == reflect.String:
		v.SetString(string(content))
		return nil
	case v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8:
		// In data mode content aliases the caller's input, so hand out a copy.
		if d.r == nil {
			content = slices.Clone(content)
		}
		v.SetBytes(content)
		return nil
	}
	return &UnmarshalTypeError{BencodeTypeName: "string", UnmarshalTargetType: v.Type()}
}

func (d *Decoder) decodeList(v reflect.Value, depth int) error {
	if v.Kind() != reflect.Slice {
		return &UnmarshalTypeError{BencodeTypeName: "list", UnmarshalTargetType: v.Type()}
	}
	n := 0
	for {
		c, err := d.peekByte()
		if err != nil {
			return err
		}
		if c == 'e' {
			d.discardByte()
			break
		}
		if n >= v.Len() {
			v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
		}
		if err := d.decodeValue(v.Index(n), depth+1); err != nil {
			// Drop the zero placeholder appended above so a caller that
			// swallows the error is left with only fully decoded elements.
			v.SetLen(n)
			return err
		}
		n++
	}
	if n < v.Len() {
		v.SetLen(n)
	}
	if n == 0 {
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	}
	return nil
}

func (d *Decoder) decodeDict(v reflect.Value, depth int) error {
	switch v.Kind() {
	case reflect.Map:
		return d.decodeDictToMap(v, depth)
	case reflect.Struct:
		return d.decodeDictToStruct(v, depth)
	}
	return &UnmarshalTypeError{BencodeTypeName: "dict", UnmarshalTargetType: v.Type()}
}

func (d *Decoder) decodeDictToMap(v reflect.Value, depth int) error {
	t := v.Type()
	if t.Key().Kind() != reflect.String {
		return &UnmarshalTypeError{BencodeTypeName: "dict", UnmarshalTargetType: t}
	}
	if v.IsNil() {
		v.Set(reflect.MakeMap(t))
	}
	for {
		c, err := d.peekByte()
		if err != nil {
			return err
		}
		if c == 'e' {
			d.discardByte()
			return nil
		}
		_, keyRaw, err := d.readDictKey()
		if err != nil {
			return err
		}
		key := string(keyRaw)
		elem := reflect.New(t.Elem()).Elem()
		if err := d.decodeValue(elem, depth+1); err != nil {
			return fmt.Errorf("parsing value for key %q: %w", key, err)
		}
		kv := reflect.ValueOf(key)
		if kv.Type() != t.Key() {
			kv = kv.Convert(t.Key())
		}
		v.SetMapIndex(kv, elem)
	}
}

func (d *Decoder) decodeDictToStruct(v reflect.Value, depth int) error {
	info := structInfoFor(v.Type())
	for {
		c, err := d.peekByte()
		if err != nil {
			return err
		}
		if c == 'e' {
			d.discardByte()
			return nil
		}
		_, keyRaw, err := d.readDictKey()
		if err != nil {
			return err
		}
		key := string(keyRaw)
		f, known := info.byKey[key]
		if !known {
			if err := d.skipValue(depth + 1); err != nil {
				return err
			}
			continue
		}
		fv := fieldForDecode(v, f.index)
		if f.ignoreTypeError {
			if err := d.decodeIgnoringTypeError(fv, key, depth); err != nil {
				return err
			}
			continue
		}
		if err := d.decodeValue(fv, depth+1); err != nil {
			return fmt.Errorf("parsing value for key %q: %w", key, err)
		}
	}
}

// decodeIgnoringTypeError captures the whole value up front so that a
// swallowed UnmarshalTypeError cannot leave the stream mid-value, which would
// desync every subsequent key (anacrolix/torrent issue #247).
func (d *Decoder) decodeIgnoringTypeError(fv reflect.Value, key string, depth int) error {
	start := d.curOffset()
	raw, err := d.captureValue(depth + 1)
	if err != nil {
		return err
	}
	sub := &Decoder{data: raw, offset: start}
	err = sub.decodeValue(fv, depth+1)
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*UnmarshalTypeError](err); ok {
		return nil
	}
	return fmt.Errorf("parsing value for key %q: %w", key, err)
}

// skipValue validates and consumes the next value without building it.
func (d *Decoder) skipValue(depth int) error {
	if depth > maxDepth {
		return d.depthError()
	}
	b, err := d.readByte()
	if err != nil {
		return err
	}
	switch {
	case b == 'i':
		_, err := d.readIntLiteral()
		return err
	case b >= '0' && b <= '9':
		_, _, err := d.readStringParts(b)
		return err
	case b == 'l', b == 'd':
		isDict := b == 'd'
		for {
			c, err := d.peekByte()
			if err != nil {
				return err
			}
			if c == 'e' {
				d.discardByte()
				return nil
			}
			if isDict {
				if _, _, err := d.readDictKey(); err != nil {
					return err
				}
			}
			if err := d.skipValue(depth + 1); err != nil {
				return err
			}
		}
	}
	return d.typeCharError(b)
}

func (d *Decoder) captureValue(depth int) ([]byte, error) {
	if d.r == nil {
		start := d.pos
		if err := d.skipValue(depth); err != nil {
			return nil, err
		}
		return d.data[start:d.pos:d.pos], nil
	}
	var buf []byte
	if err := d.captureInto(&buf, depth); err != nil {
		return nil, err
	}
	return buf, nil
}

// captureInto appends the next value's raw bytes exactly as read, preserving
// original key order and redundant leading zeros so captured subtrees
// round-trip byte-identically.
func (d *Decoder) captureInto(buf *[]byte, depth int) error {
	if depth > maxDepth {
		return d.depthError()
	}
	b, err := d.readByte()
	if err != nil {
		return err
	}
	switch {
	case b == 'i':
		lit, err := d.readIntLiteral()
		if err != nil {
			return err
		}
		*buf = append(*buf, 'i')
		*buf = append(*buf, lit...)
		*buf = append(*buf, 'e')
		return nil
	case b >= '0' && b <= '9':
		prefix, content, err := d.readStringParts(b)
		if err != nil {
			return err
		}
		*buf = appendStringValue(*buf, prefix, content)
		return nil
	case b == 'l', b == 'd':
		*buf = append(*buf, b)
		isDict := b == 'd'
		for {
			c, err := d.peekByte()
			if err != nil {
				return err
			}
			if c == 'e' {
				d.discardByte()
				*buf = append(*buf, 'e')
				return nil
			}
			if isDict {
				prefix, key, err := d.readDictKey()
				if err != nil {
					return err
				}
				*buf = appendStringValue(*buf, prefix, key)
			}
			if err := d.captureInto(buf, depth+1); err != nil {
				return err
			}
		}
	}
	return d.typeCharError(b)
}

func appendStringValue(buf, prefix, content []byte) []byte {
	buf = append(buf, prefix...)
	buf = append(buf, ':')
	return append(buf, content...)
}

func (d *Decoder) readDictKey() (prefix []byte, content []byte, err error) {
	b, err := d.readByte()
	if err != nil {
		return nil, nil, err
	}
	if b < '0' || b > '9' {
		return nil, nil, &SyntaxError{Offset: d.offset - 1, What: errors.New("dict key is not a string")}
	}
	return d.readStringParts(b)
}

func (d *Decoder) readIntLiteral() (string, error) {
	start := d.curOffset() - 1
	if d.r == nil {
		end := bytes.IndexByte(d.data[d.pos:], 'e')
		if end < 0 {
			d.pos = len(d.data)
			return "", d.inputError(io.EOF)
		}
		lit := d.data[d.pos : d.pos+end]
		d.pos += end + 1
		if !validIntLiteral(lit) {
			return "", &SyntaxError{Offset: start, What: fmt.Errorf("invalid integer literal %q", lit)}
		}
		return string(lit), nil
	}
	var lit []byte
	for {
		b, err := d.readByte()
		if err != nil {
			return "", err
		}
		if b == 'e' {
			break
		}
		lit = append(lit, b)
	}
	if !validIntLiteral(lit) {
		return "", &SyntaxError{Offset: start, What: fmt.Errorf("invalid integer literal %q", lit)}
	}
	return string(lit), nil
}

func validIntLiteral(lit []byte) bool {
	if len(lit) > 0 && lit[0] == '-' {
		lit = lit[1:]
	}
	if len(lit) == 0 {
		return false
	}
	for _, b := range lit {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func (d *Decoder) readStringParts(first byte) (prefix []byte, content []byte, err error) {
	start := d.curOffset() - 1
	if d.r == nil {
		n := int64(first - '0')
		i := d.pos
		for i < len(d.data) && d.data[i] >= '0' && d.data[i] <= '9' {
			if n <= maxStringLen {
				n = n*10 + int64(d.data[i]-'0')
			}
			i++
		}
		if i >= len(d.data) {
			d.pos = len(d.data)
			return nil, nil, d.inputError(io.EOF)
		}
		prefix = d.data[d.pos-1 : i]
		if d.data[i] != ':' {
			return nil, nil, &SyntaxError{Offset: start, What: fmt.Errorf("invalid character %q in string length", d.data[i])}
		}
		if n > maxStringLen {
			return nil, nil, &SyntaxError{Offset: start, What: fmt.Errorf("string length %s out of range", prefix)}
		}
		i++
		if int64(len(d.data)-i) < n {
			d.pos = len(d.data)
			return nil, nil, d.inputError(io.EOF)
		}
		content = d.data[i : i+int(n)]
		d.pos = i + int(n)
		return prefix, content, nil
	}
	prefix = []byte{first}
	for {
		b, err := d.readByte()
		if err != nil {
			return nil, nil, err
		}
		if b == ':' {
			break
		}
		if b < '0' || b > '9' {
			return nil, nil, &SyntaxError{Offset: start, What: fmt.Errorf("invalid character %q in string length", b)}
		}
		prefix = append(prefix, b)
	}
	n, perr := strconv.ParseInt(string(prefix), 10, 64)
	if perr != nil || n > maxStringLen {
		return nil, nil, &SyntaxError{Offset: start, What: fmt.Errorf("string length %s out of range", prefix)}
	}
	content, err = d.readStringContent(n)
	if err != nil {
		return nil, nil, err
	}
	return prefix, content, nil
}

func (d *Decoder) readStringContent(n int64) ([]byte, error) {
	if n == 0 {
		return []byte{}, nil
	}
	var buf []byte
	for int64(len(buf)) < n {
		step := int(min(n-int64(len(buf)), readChunkSize))
		start := len(buf)
		buf = slices.Grow(buf, step)[:start+step]
		m, err := io.ReadFull(d.r, buf[start:])
		d.offset += int64(m)
		if err != nil {
			return nil, d.inputError(err)
		}
	}
	return buf, nil
}

func (d *Decoder) readByte() (byte, error) {
	if d.r == nil {
		if d.pos >= len(d.data) {
			return 0, d.inputError(io.EOF)
		}
		b := d.data[d.pos]
		d.pos++
		return b, nil
	}
	b, err := d.r.ReadByte()
	if err != nil {
		return 0, d.inputError(err)
	}
	d.offset++
	return b, nil
}

func (d *Decoder) peekByte() (byte, error) {
	if d.r == nil {
		if d.pos >= len(d.data) {
			return 0, d.inputError(io.EOF)
		}
		return d.data[d.pos], nil
	}
	b, err := d.r.Peek(1)
	if err != nil {
		return 0, d.inputError(err)
	}
	return b[0], nil
}

// discardByte is only called after a successful peekByte, so the byte is
// buffered and Discard cannot fail.
func (d *Decoder) discardByte() {
	if d.r == nil {
		d.pos++
		return
	}
	_, _ = d.r.Discard(1)
	d.offset++
}

func (d *Decoder) inputError(err error) error {
	if errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return &SyntaxError{Offset: d.curOffset(), What: err}
}

func (d *Decoder) depthError() error {
	return &SyntaxError{Offset: d.curOffset(), What: errors.New("nesting depth exceeds limit")}
}

func (d *Decoder) typeCharError(b byte) error {
	return &SyntaxError{Offset: d.curOffset() - 1, What: fmt.Errorf("invalid value type character %q", b)}
}
