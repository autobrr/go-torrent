// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package bencode

import (
	"bufio"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Encoder writes bencode values to an output stream.
type Encoder struct {
	w encodeWriter
}

type encodeWriter interface {
	io.Writer
	io.ByteWriter
	io.StringWriter
}

// Encode writes the bencode encoding of v and flushes it to the underlying
// writer before returning.
func (e *Encoder) Encode(v any) error {
	if v == nil {
		return &MarshalTypeError{}
	}
	if err := e.encodeValue(reflect.ValueOf(v)); err != nil {
		return err
	}
	if bw, ok := e.w.(*bufio.Writer); ok {
		return bw.Flush()
	}
	return nil
}

func (e *Encoder) encodeValue(v reflect.Value) error {
	if !v.IsValid() {
		return &MarshalTypeError{}
	}
	if v.Kind() != reflect.Pointer || !v.IsNil() {
		if m, t, ok := marshalerFor(v); ok {
			return e.encodeMarshaler(m, t)
		}
	}
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			e.writeString("i1e")
		} else {
			e.writeString("i0e")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.writeByte('i')
		e.writeString(strconv.FormatInt(v.Int(), 10))
		e.writeByte('e')
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		e.writeByte('i')
		e.writeString(strconv.FormatUint(v.Uint(), 10))
		e.writeByte('e')
	case reflect.String:
		e.writeStringValue(v.String())
	case reflect.Slice, reflect.Array:
		return e.encodeSequence(v)
	case reflect.Map:
		return e.encodeMap(v)
	case reflect.Struct:
		return e.encodeStruct(v)
	case reflect.Interface:
		return e.encodeValue(v.Elem())
	case reflect.Pointer:
		// A nil pointer is encoded as the zero value of its pointee, a
		// documented deviation from anacrolix/torrent which panics here.
		if v.IsNil() {
			return e.encodeValue(reflect.Zero(v.Type().Elem()))
		}
		return e.encodeValue(v.Elem())
	default:
		return &MarshalTypeError{Type: v.Type()}
	}
	return nil
}

func marshalerFor(v reflect.Value) (Marshaler, reflect.Type, bool) {
	if m, ok := v.Interface().(Marshaler); ok {
		return m, v.Type(), true
	}
	if v.Kind() != reflect.Pointer && v.CanAddr() {
		if m, ok := v.Addr().Interface().(Marshaler); ok {
			return m, v.Addr().Type(), true
		}
	}
	return nil, nil, false
}

func (e *Encoder) encodeMarshaler(m Marshaler, t reflect.Type) error {
	data, err := m.MarshalBencode()
	if err != nil {
		return &MarshalerError{Type: t, Err: err}
	}
	e.writeBytes(data)
	return nil
}

func (e *Encoder) encodeSequence(v reflect.Value) error {
	if v.Type().Elem().Kind() == reflect.Uint8 {
		var b []byte
		if v.Kind() == reflect.Slice {
			b = v.Bytes()
		} else {
			b = make([]byte, v.Len())
			reflect.Copy(reflect.ValueOf(b), v)
		}
		e.writeString(strconv.Itoa(len(b)))
		e.writeByte(':')
		e.writeBytes(b)
		return nil
	}
	e.writeByte('l')
	for i := 0; i < v.Len(); i++ {
		if err := e.encodeValue(v.Index(i)); err != nil {
			return err
		}
	}
	e.writeByte('e')
	return nil
}

func (e *Encoder) encodeMap(v reflect.Value) error {
	if v.Type().Key().Kind() != reflect.String {
		return &MarshalTypeError{Type: v.Type()}
	}
	keys := v.MapKeys()
	slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
	e.writeByte('d')
	for _, k := range keys {
		e.writeStringValue(k.String())
		if err := e.encodeValue(v.MapIndex(k)); err != nil {
			return err
		}
	}
	e.writeByte('e')
	return nil
}

func (e *Encoder) encodeStruct(v reflect.Value) error {
	info := structInfoFor(v.Type())
	e.writeByte('d')
	for _, f := range info.fields {
		fv := fieldForEncode(v, f.index)
		if !fv.IsValid() {
			continue
		}
		if f.omitEmpty && isEmptyValue(fv) {
			continue
		}
		e.writeStringValue(f.key)
		if err := e.encodeValue(fv); err != nil {
			return err
		}
	}
	e.writeByte('e')
	return nil
}

// isEmptyValue deliberately treats a non-nil empty slice or map as non-empty:
// omitempty only drops nil ones, so an intentionally empty []byte still
// encodes as "0:". A struct is empty when all its exported fields are empty,
// matching anacrolix/torrent so that fields like metainfo's zero FileTree are
// omitted.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if t.Field(i).PkgPath != "" {
				continue
			}
			if !isEmptyValue(v.Field(i)) {
				return false
			}
		}
		return true
	}
	return false
}

func (e *Encoder) writeStringValue(s string) {
	e.writeString(strconv.Itoa(len(s)))
	e.writeByte(':')
	e.writeString(s)
}

// Write errors are ignored at each call site because the writer is either a
// bytes.Buffer, which cannot fail, or a bufio.Writer, which latches the
// first failure and reports it from the Flush at the end of Encode.
func (e *Encoder) writeByte(b byte) { _ = e.w.WriteByte(b) }

func (e *Encoder) writeString(s string) { _, _ = e.w.WriteString(s) }

func (e *Encoder) writeBytes(b []byte) { _, _ = e.w.Write(b) }
