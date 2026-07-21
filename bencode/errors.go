// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package bencode

import (
	"fmt"
	"reflect"
)

// SyntaxError reports malformed bencode input at an absolute byte offset.
type SyntaxError struct {
	Offset int64
	What   error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("bencode: syntax error (offset: %d): %s", e.Offset, e.What)
}

func (e *SyntaxError) Unwrap() error { return e.What }

// UnmarshalTypeError reports a bencode value that cannot be stored in the
// target Go type.
type UnmarshalTypeError struct {
	BencodeTypeName     string
	UnmarshalTargetType reflect.Type
}

func (e *UnmarshalTypeError) Error() string {
	return fmt.Sprintf("can't unmarshal a bencode %v into a %v", e.BencodeTypeName, e.UnmarshalTargetType)
}

// MarshalTypeError reports a Go type that has no bencode representation.
type MarshalTypeError struct {
	Type reflect.Type
}

func (e *MarshalTypeError) Error() string {
	if e.Type == nil {
		return "bencode: unsupported type: nil"
	}
	return "bencode: unsupported type: " + e.Type.String()
}

// UnmarshalInvalidArgError reports a nil or non-pointer target passed to
// Unmarshal or Decoder.Decode.
type UnmarshalInvalidArgError struct {
	Type reflect.Type
}

func (e *UnmarshalInvalidArgError) Error() string {
	if e.Type == nil {
		return "bencode: target must be a non-nil pointer, got nil"
	}
	if e.Type.Kind() != reflect.Pointer {
		return "bencode: target must be a pointer, got " + e.Type.String()
	}
	return "bencode: target pointer must not be nil (" + e.Type.String() + ")"
}

// MarshalerError wraps an error returned by a MarshalBencode call.
type MarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *MarshalerError) Error() string {
	return fmt.Sprintf("bencode: MarshalBencode failed for type %v: %v", e.Type, e.Err)
}

func (e *MarshalerError) Unwrap() error { return e.Err }

// UnmarshalerError wraps an error returned by an UnmarshalBencode call.
type UnmarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *UnmarshalerError) Error() string {
	return fmt.Sprintf("bencode: UnmarshalBencode failed for type %v: %v", e.Type, e.Err)
}

func (e *UnmarshalerError) Unwrap() error { return e.Err }

// ErrUnusedTrailingBytes reports input remaining after a complete value. The
// unconventional name matches the equivalent type in anacrolix/torrent's
// bencode package, which this package is a drop-in replacement for.
type ErrUnusedTrailingBytes struct {
	NumUnusedBytes int
}

func (e ErrUnusedTrailingBytes) Error() string {
	return fmt.Sprintf("%d unused trailing bytes", e.NumUnusedBytes)
}
