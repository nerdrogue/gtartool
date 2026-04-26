package gtar

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
)

// Archive holds the parsed index of a .gtar file.
type Archive struct {
	Hdr     Header
	Entries []Entry
	// Paths maps path_hash -> original path (populated from .strtab if available)
	Paths map[uint64]string
	f     *os.File
}

// Open opens a .gtar file for reading.
// If a companion .strtab file exists, paths are loaded from it.
func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	ar := &Archive{f: f, Paths: make(map[uint64]string)}

	if err := binary.Read(f, binary.LittleEndian, &ar.Hdr); err != nil {
		f.Close()
		return nil, fmt.Errorf("read header: %w", err)
	}

	if string(ar.Hdr.Magic[:]) != Magic {
		f.Close()
		return nil, fmt.Errorf("not a gtar file (bad magic: %q)", string(ar.Hdr.Magic[:]))
	}
	if ar.Hdr.Version != Version {
		f.Close()
		return nil, fmt.Errorf("unsupported gtar version: %d (expected %d)", ar.Hdr.Version, Version)
	}

	ar.Entries = make([]Entry, ar.Hdr.EntryCount)
	for i := range ar.Entries {
		if err := binary.Read(f, binary.LittleEndian, &ar.Entries[i]); err != nil {
			f.Close()
			return nil, fmt.Errorf("read entry %d: %w", i, err)
		}
	}

	// Entries must be sorted by path_hash for binary search
	sort.Slice(ar.Entries, func(i, j int) bool {
		return ar.Entries[i].PathHash < ar.Entries[j].PathHash
	})

	return ar, nil
}

// Close releases the file handle.
func (ar *Archive) Close() error {
	return ar.f.Close()
}

// Find looks up an entry by path (canonicalized and hashed).
// Returns nil if not found.
func (ar *Archive) Find(path string) *Entry {
	h := HashPath(path)
	return ar.FindByHash(h)
}

// FindByHash looks up an entry by pre-computed hash via binary search.
func (ar *Archive) FindByHash(h uint64) *Entry {
	idx := sort.Search(len(ar.Entries), func(i int) bool {
		return ar.Entries[i].PathHash >= h
	})
	if idx < len(ar.Entries) && ar.Entries[idx].PathHash == h {
		return &ar.Entries[idx]
	}
	return nil
}

// ReadBlob reads and decompresses the blob for the given entry.
func (ar *Archive) ReadBlob(e *Entry) ([]byte, error) {
	if e.Codec == CodecZstdDict {
		return nil, fmt.Errorf("codec zstd_dict (3) not supported: archive was packed with an external dictionary")
	}

	compressed := make([]byte, e.SizeStored)
	if _, err := ar.f.ReadAt(compressed, int64(e.BlobOffset)); err != nil {
		return nil, fmt.Errorf("read blob at offset %d: %w", e.BlobOffset, err)
	}

	// Verify checksum
	got := ChecksumBlob(compressed)
	if got != e.Checksum {
		return nil, fmt.Errorf("checksum mismatch: got %08x, expected %08x", got, e.Checksum)
	}

	return Decompress(e.Codec, compressed, e.SizeRaw)
}

// ReadBlobRaw reads the raw (compressed) bytes without decompressing.
func (ar *Archive) ReadBlobRaw(e *Entry) ([]byte, error) {
	raw := make([]byte, e.SizeStored)
	if _, err := ar.f.ReadAt(raw, int64(e.BlobOffset)); err != nil {
		return nil, fmt.Errorf("read raw blob: %w", err)
	}
	return raw, nil
}

// PathName returns the human-readable path for an entry if known, otherwise the hash in hex.
func (ar *Archive) PathName(e *Entry) string {
	if p, ok := ar.Paths[e.PathHash]; ok {
		return p
	}
	return fmt.Sprintf("<hash:%016x>", e.PathHash)
}

// WriteHeader writes only the gtar header and sorted entries to w (for inspect/dump).
func WriteHeaderTo(ar *Archive, w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, ar.Hdr); err != nil {
		return err
	}
	for _, e := range ar.Entries {
		if err := binary.Write(w, binary.LittleEndian, e); err != nil {
			return err
		}
	}
	return nil
}
