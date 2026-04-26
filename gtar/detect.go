package gtar

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// DetectCodec determines the best codec for a file given its content and path.
// Strategy:
//  1. Magic bytes (like the `file` command)
//  2. File extension fallback for engine-specific formats
//  3. UTF-8 text heuristic
//  4. Default: lz4
func DetectCodec(path string, data []byte) uint16 {
	ext := strings.ToLower(filepath.Ext(path))

	// --- Magic byte detection ---
	if c, ok := detectByMagic(data, ext); ok {
		return c
	}

	// --- Extension fallback for engine-specific formats ---
	switch ext {
	case ".dcell":
		return CodecZstd
	case ".cbor", ".thing", ".def", ".cdef":
		return CodecZstd
	case ".lua":
		return CodecBrotli
	case ".mesh", ".col", ".nav", ".ozz":
		return CodecLZ4
	case ".sub", ".strtab", ".txt", ".md", ".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".glsl", ".hlsl", ".vert", ".frag", ".comp":
		return CodecBrotli
	case ".webm", ".mp4", ".mkv":
		return CodecNone // pre-compressed video
	}

	// --- UTF-8 text heuristic ---
	if isLikelyText(data) {
		return CodecBrotli
	}

	// Default: lz4 (good ratio, fast decompression)
	return CodecLZ4
}

type magic struct {
	offset int
	bytes  []byte
	codec  uint16
}

var magicTable = []magic{
	// Textures — already compressed or near-incompressible
	{0, []byte("DDS "), CodecNone},                          // DirectDraw Surface
	{0, []byte("\xabKTX 20\xbb\r\n\x1a\n"), CodecNone},     // KTX2
	{0, []byte("\x89PNG\r\n\x1a\n"), CodecNone},             // PNG
	{0, []byte("\xff\xd8\xff"), CodecNone},                  // JPEG
	{0, []byte("RIFF"), CodecNone},                          // RIFF (check further below for type)
	{0, []byte("fLaC"), CodecNone},                          // FLAC audio
	{0, []byte("OggS"), CodecNone},                          // OGG (Vorbis/Opus audio)
	{0, []byte("ID3"), CodecNone},                           // MP3 with ID3 tag
	{0, []byte("\xff\xfb"), CodecNone},                      // MP3 frame sync
	{0, []byte("\xff\xf3"), CodecNone},                      // MP3 frame sync variant
	{0, []byte("\xff\xf2"), CodecNone},                      // MP3 frame sync variant
	{0, []byte("\x1a\x45\xdf\xa3"), CodecNone},              // WebM / MKV
	{0, []byte("\x00\x00\x00\x18ftypmp4"), CodecNone},       // MP4
	// Already-compressed blobs (shouldn't be in a gtar but handle gracefully)
	{0, []byte("\x04\x22\x4d\x18"), CodecNone},              // LZ4 frame
	{0, []byte("\x28\xb5\x2f\xfd"), CodecNone},              // zstd frame
	// Lua bytecode
	{0, []byte("\x1bLua"), CodecBrotli},
	// ELF (shouldn't appear in game assets, but just in case)
	{0, []byte("\x7fELF"), CodecLZ4},
}

func detectByMagic(data []byte, ext string) (uint16, bool) {
	if len(data) < 4 {
		return CodecNone, false
	}

	for _, m := range magicTable {
		end := m.offset + len(m.bytes)
		if len(data) >= end && string(data[m.offset:end]) == string(m.bytes) {
			// Special case: RIFF — check the form type at offset 8
			if m.bytes[0] == 'R' && m.bytes[1] == 'I' {
				if len(data) >= 12 {
					form := string(data[8:12])
					if form == "WAVE" || form == "AVI " {
						return CodecNone, true
					}
				}
				// Unknown RIFF form — fall through
				return CodecLZ4, true
			}
			return m.codec, true
		}
	}

	return 0, false
}

// isLikelyText returns true if data looks like UTF-8 text.
// Samples the first 512 bytes. If >90% are valid UTF-8 printable runes, it's text.
func isLikelyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	if !utf8.Valid(sample) {
		return false
	}
	// Count control bytes (excluding tab, newline, CR)
	control := 0
	total := 0
	for _, b := range sample {
		total++
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			control++
		}
	}
	return total == 0 || float64(control)/float64(total) < 0.05
}
