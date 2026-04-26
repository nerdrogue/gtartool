package gtar

// Magic bytes and version
const (
	Magic   = "GTAR"
	Version = uint16(1)
)

// Header is the fixed 12-byte file header (little-endian).
//
//	[0:4]  magic "GTAR"
//	[4:6]  version u16
//	[6:8]  flags   u16
//	[8:12] entry_count u32
type Header struct {
	Magic      [4]byte
	Version    uint16
	Flags      uint16
	EntryCount uint32
}

const HeaderSize = 12

// Entry is a single 32-byte sorted index entry (little-endian).
//
//	[0:8]   path_hash   u64  xxHash64(canonicalized path, seed=0)
//	[8:16]  blob_offset u64  byte offset of compressed blob in file
//	[16:20] size_raw    u32  uncompressed size
//	[20:24] size_stored u32  compressed size on disk
//	[24:26] codec       u16
//	[26:28] flags       u16
//	[28:32] checksum    u32  xxHash32(compressed blob)
type Entry struct {
	PathHash   uint64
	BlobOffset uint64
	SizeRaw    uint32
	SizeStored uint32
	Codec      uint16
	Flags      uint16
	Checksum   uint32
}

const EntrySize = 32

// Codec IDs
const (
	CodecNone     uint16 = 0
	CodecLZ4      uint16 = 1
	CodecZstd     uint16 = 2
	CodecZstdDict uint16 = 3 // unsupported in gtartool; needs external dict
	CodecBrotli   uint16 = 4
)

// CodecName returns the human-readable codec name.
func CodecName(c uint16) string {
	switch c {
	case CodecNone:
		return "none"
	case CodecLZ4:
		return "lz4"
	case CodecZstd:
		return "zstd"
	case CodecZstdDict:
		return "zstd_dict"
	case CodecBrotli:
		return "brotli"
	default:
		return "unknown"
	}
}

// BlobAlignment is the byte alignment for each blob in the archive.
// Enables mmap + madvise without unaligned reads.
const BlobAlignment = 4096
