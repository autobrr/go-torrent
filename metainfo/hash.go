// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

// HashSize is the length of a v1 infohash in bytes.
const HashSize = 20

// Hash is a v1 (SHA-1) infohash.
type Hash [HashSize]byte

// Bytes returns the hash as a byte slice.
func (h Hash) Bytes() []byte {
	return h[:]
}

// String returns the lowercase hex encoding of the hash.
func (h Hash) String() string {
	return h.HexString()
}

// HexString returns the lowercase hex encoding of the hash.
func (h Hash) HexString() string {
	return hex.EncodeToString(h[:])
}

// IsZero reports whether the hash is all zero bytes.
func (h Hash) IsZero() bool {
	return h == Hash{}
}

// NewHashFromHex decodes a 40-character hex string into a Hash.
func NewHashFromHex(s string) (Hash, error) {
	var h Hash
	if len(s) != 2*HashSize {
		return h, fmt.Errorf("hash hex string has length %d, expected %d", len(s), 2*HashSize)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return h, err
	}
	copy(h[:], b)
	return h, nil
}

// HashBytes returns the SHA-1 digest of b.
func HashBytes(b []byte) Hash {
	return sha1.Sum(b)
}
