package gtar

import (
	"encoding/binary"
	"math/bits"
	"path/filepath"
	"strings"

	xxhash64 "github.com/cespare/xxhash/v2"
)

// HashPath canonicalizes a path and returns its xxHash64 (seed=0).
// Canonicalization: lowercase, forward slashes, no leading slash.
func HashPath(p string) uint64 {
	clean := strings.ToLower(filepath.ToSlash(p))
	clean = strings.TrimPrefix(clean, "/")
	return xxhash64.Sum64String(clean)
}

// CanonicalPath returns the canonical form of a path (lowercase, forward slashes).
func CanonicalPath(p string) string {
	clean := strings.ToLower(filepath.ToSlash(p))
	return strings.TrimPrefix(clean, "/")
}

// ChecksumBlob computes xxHash32 of a blob (the compressed bytes on disk).
// Implemented from scratch since the dep only provides xxHash64.
func ChecksumBlob(data []byte) uint32 {
	return xxHash32(data, 0)
}

// xxHash32 is a pure-Go implementation of the xxHash32 algorithm.
// Spec: https://github.com/Cyan4973/xxHash/blob/dev/doc/xxhash_spec.md
func xxHash32(data []byte, seed uint32) uint32 {
	const (
		prime1 = uint32(0x9E3779B1)
		prime2 = uint32(0x85EBCA77)
		prime3 = uint32(0xC2B2AE3D)
		prime4 = uint32(0x27D4EB2F)
		prime5 = uint32(0x165667B1)
	)

	n := len(data)
	i := 0
	var h32 uint32

	if n >= 16 {
		v1 := seed + prime1 + prime2
		v2 := seed + prime2
		v3 := seed
		v4 := seed - prime1

		for ; i <= n-16; i += 16 {
			v1 = bits.RotateLeft32(v1+binary.LittleEndian.Uint32(data[i:])*prime2, 13) * prime1
			v2 = bits.RotateLeft32(v2+binary.LittleEndian.Uint32(data[i+4:])*prime2, 13) * prime1
			v3 = bits.RotateLeft32(v3+binary.LittleEndian.Uint32(data[i+8:])*prime2, 13) * prime1
			v4 = bits.RotateLeft32(v4+binary.LittleEndian.Uint32(data[i+12:])*prime2, 13) * prime1
		}
		h32 = bits.RotateLeft32(v1, 1) + bits.RotateLeft32(v2, 7) +
			bits.RotateLeft32(v3, 12) + bits.RotateLeft32(v4, 18)
	} else {
		h32 = seed + prime5
	}

	h32 += uint32(n)

	for ; i <= n-4; i += 4 {
		h32 += binary.LittleEndian.Uint32(data[i:]) * prime3
		h32 = bits.RotateLeft32(h32, 17) * prime4
	}
	for ; i < n; i++ {
		h32 += uint32(data[i]) * prime5
		h32 = bits.RotateLeft32(h32, 11) * prime1
	}

	h32 ^= h32 >> 15
	h32 *= prime2
	h32 ^= h32 >> 13
	h32 *= prime3
	h32 ^= h32 >> 16

	return h32
}
