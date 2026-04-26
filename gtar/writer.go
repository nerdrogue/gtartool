package gtar

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// FileEntry is used as input to the writer.
type FileEntry struct {
	Path  string // original path (will be canonicalized)
	Data  []byte // uncompressed content
	Codec uint16 // codec to use
	// BrotliQuality is only relevant when Codec == CodecBrotli (0-11).
	BrotliQuality int
}

// WriteArchive creates a .gtar file at dst from the given entries.
// Entries are sorted by path hash before writing.
func WriteArchive(dst string, files []FileEntry) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to pack")
	}

	// Compress all blobs first (need sizes to calculate offsets)
	type packed struct {
		hash       uint64
		path       string
		canonical  string
		codec      uint16
		rawSize    uint32
		compressed []byte
		checksum   uint32
	}

	pkgs := make([]packed, 0, len(files))
	for _, f := range files {
		canonical := CanonicalPath(f.Path)
		h := HashPath(f.Path)

		compressed, err := Compress(f.Codec, f.Data, f.BrotliQuality)
		if err != nil {
			return fmt.Errorf("compress %q: %w", f.Path, err)
		}

		pkgs = append(pkgs, packed{
			hash:       h,
			path:       f.Path,
			canonical:  canonical,
			codec:      f.Codec,
			rawSize:    uint32(len(f.Data)),
			compressed: compressed,
			checksum:   ChecksumBlob(compressed),
		})
	}

	// Sort by path hash (required for binary search lookup in engine)
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].hash < pkgs[j].hash
	})

	// Calculate blob offsets
	// Index section: HeaderSize + N*EntrySize, then padded to BlobAlignment
	indexSize := int64(HeaderSize) + int64(len(pkgs))*int64(EntrySize)
	firstBlobOffset := alignUp(indexSize, BlobAlignment)

	offsets := make([]int64, len(pkgs))
	cur := firstBlobOffset
	for i, p := range pkgs {
		offsets[i] = cur
		cur = alignUp(cur+int64(len(p.compressed)), BlobAlignment)
	}
	totalSize := cur

	// Write file
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	// Pre-allocate (helps with fragmentation on Linux)
	if err := f.Truncate(totalSize); err != nil {
		// Non-fatal; just try writing
		_ = err
	}

	// Write header
	hdr := Header{}
	copy(hdr.Magic[:], Magic)
	hdr.Version = Version
	hdr.Flags = 0
	hdr.EntryCount = uint32(len(pkgs))
	if err := binary.Write(f, binary.LittleEndian, hdr); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write index entries
	for i, p := range pkgs {
		e := Entry{
			PathHash:   p.hash,
			BlobOffset: uint64(offsets[i]),
			SizeRaw:    p.rawSize,
			SizeStored: uint32(len(p.compressed)),
			Codec:      p.codec,
			Flags:      0,
			Checksum:   p.checksum,
		}
		if err := binary.Write(f, binary.LittleEndian, e); err != nil {
			return fmt.Errorf("write entry %d: %w", i, err)
		}
	}

	// Pad index section to BlobAlignment
	padIndex := firstBlobOffset - indexSize
	if padIndex > 0 {
		pad := make([]byte, padIndex)
		if _, err := f.Write(pad); err != nil {
			return fmt.Errorf("write index padding: %w", err)
		}
	}

	// Write blobs (each followed by padding to BlobAlignment)
	for i, p := range pkgs {
		if _, err := f.Write(p.compressed); err != nil {
			return fmt.Errorf("write blob %q: %w", p.path, err)
		}
		// Pad to alignment
		written := int64(len(p.compressed))
		nextOffset := alignUp(offsets[i]+written, BlobAlignment)
		padBlob := nextOffset - (offsets[i] + written)
		if padBlob > 0 {
			pad := make([]byte, padBlob)
			if _, err := f.Write(pad); err != nil {
				return fmt.Errorf("write blob padding: %w", err)
			}
		}
	}

	return f.Sync()
}

// alignUp rounds x up to the nearest multiple of align.
func alignUp(x, align int64) int64 {
	return (x + align - 1) &^ (align - 1)
}

// PathsFromFiles extracts the original paths from a slice of FileEntry.
func PathsFromFiles(files []FileEntry) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
