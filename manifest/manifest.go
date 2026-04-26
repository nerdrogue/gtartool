// Package manifest handles the {archive}.manifest.json sidecar.
//
// Example manifest.json:
//
//	{
//	  "brotli_quality": 9,
//	  "brotli_window": 22,
//	  "overrides": {
//	    "assets/ui/font.bin":         { "codec": "lz4" },
//	    "scripts/mission_01.lua":     { "codec": "brotli", "brotli_quality": 11 },
//	    "data/navmesh.bin":           { "codec": "none" }
//	  }
//	}
//
// All paths in overrides are matched after canonicalization (lowercase, forward slash).
// The top-level brotli_quality is REQUIRED if any file uses brotli without a per-file override.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nerdrogue/gtartool/gtar"
)

// Override holds per-file settings.
type Override struct {
	Codec         string `json:"codec"`          // "none", "lz4", "zstd", "brotli"
	BrotliQuality *int   `json:"brotli_quality"` // nil = use top-level
}

// Manifest is the parsed .manifest.json.
type Manifest struct {
	// BrotliQuality is the global default brotli quality (required if any file uses brotli).
	BrotliQuality *int `json:"brotli_quality"`
	// BrotliWindow is the global brotli window (default 22).
	BrotliWindow int `json:"brotli_window"`
	// Overrides maps canonical path -> per-file settings.
	Overrides map[string]Override `json:"overrides"`

	path string
}

// ManifestPath returns the expected manifest path for a given archive path.
// e.g. "world_base.gtar" -> "world_base.manifest.json"
func ManifestPath(archivePath string) string {
	base := strings.TrimSuffix(archivePath, filepath.Ext(archivePath))
	return base + ".manifest.json"
}

// Load reads the manifest from disk. Returns (nil, nil) if the file does not exist.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", path, err)
	}
	if m.BrotliWindow == 0 {
		m.BrotliWindow = 22
	}
	m.path = path

	// Canonicalize override keys
	canon := make(map[string]Override, len(m.Overrides))
	for k, v := range m.Overrides {
		canon[gtar.CanonicalPath(k)] = v
	}
	m.Overrides = canon

	return &m, nil
}

// ResolveCodec returns the codec and brotli quality for a file.
// path should be the canonical path.
// Returns an error if brotli quality is needed but not configured.
func (m *Manifest) ResolveCodec(canonPath string, detectedCodec uint16) (uint16, int, error) {
	codec := detectedCodec
	var brotliQuality *int

	if m != nil {
		if ov, ok := m.Overrides[canonPath]; ok {
			if ov.Codec != "" {
				c, err := parseCodecName(ov.Codec)
				if err != nil {
					return 0, 0, fmt.Errorf("override for %q: %w", canonPath, err)
				}
				codec = c
			}
			if ov.BrotliQuality != nil {
				brotliQuality = ov.BrotliQuality
			}
		}
		if brotliQuality == nil {
			brotliQuality = m.BrotliQuality
		}
	}

	q := 0
	if codec == gtar.CodecBrotli {
		if brotliQuality == nil {
			return 0, 0, fmt.Errorf(
				"file %q uses brotli but no brotli_quality is set in manifest.json (add it at top level or per-file override)",
				canonPath,
			)
		}
		q = *brotliQuality
		if q < 0 || q > 11 {
			return 0, 0, fmt.Errorf("brotli_quality must be 0-11, got %d", q)
		}
	}

	return codec, q, nil
}

func parseCodecName(s string) (uint16, error) {
	switch strings.ToLower(s) {
	case "none", "raw":
		return gtar.CodecNone, nil
	case "lz4":
		return gtar.CodecLZ4, nil
	case "zstd":
		return gtar.CodecZstd, nil
	case "brotli":
		return gtar.CodecBrotli, nil
	default:
		return 0, fmt.Errorf("unknown codec %q (valid: none, lz4, zstd, brotli)", s)
	}
}
