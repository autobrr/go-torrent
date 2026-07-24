// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package metainfo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HashV2Size is the length of a v2 infohash in bytes.
const HashV2Size = 32

// HashV2 is a v2 (SHA-256) infohash (BEP 52).
type HashV2 [HashV2Size]byte

// Bytes returns the hash as a byte slice.
func (h HashV2) Bytes() []byte {
	return h[:]
}

// String returns the lowercase hex encoding of the hash.
func (h HashV2) String() string {
	return h.HexString()
}

// HexString returns the lowercase hex encoding of the hash.
func (h HashV2) HexString() string {
	return hex.EncodeToString(h[:])
}

// IsZero reports whether the hash is all zero bytes.
func (h HashV2) IsZero() bool {
	return h == HashV2{}
}

// ToShort truncates the hash to 20 bytes for use in auxiliary interfaces,
// like DHT and trackers (BEP 52).
func (h HashV2) ToShort() Hash {
	var short Hash
	copy(short[:], h[:HashSize])
	return short
}

// NewHashV2FromHex decodes a 64-character hex string into a HashV2.
func NewHashV2FromHex(s string) (HashV2, error) {
	var h HashV2
	if len(s) != 2*HashV2Size {
		return h, fmt.Errorf("hash hex string has length %d, expected %d", len(s), 2*HashV2Size)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return h, err
	}
	copy(h[:], b)
	return h, nil
}

// HashV2Bytes returns the SHA-256 digest of b.
func HashV2Bytes(b []byte) HashV2 {
	return sha256.Sum256(b)
}
