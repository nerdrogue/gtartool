package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nerdrogue/gtartool/gtar"
	"github.com/nerdrogue/gtartool/strtab"
)

const extractUsage = `Usage: gtartool extract [options] <archive.gtar> [path ...]

Extract files from a .gtar archive.
If no paths are given, all files are extracted.
If a .strtab sidecar exists, original paths are used; otherwise files are
named by their hash (e.g. <hash>.bin).

Options:
  -o <dir>   Output directory (default: current directory)
  -v         Verbose: print each extracted file
  -raw       Write compressed blobs as-is (do not decompress)

Examples:
  gtartool extract assets.gtar -o ./out/
  gtartool extract assets.gtar scripts/mission_01.lua
`

func RunExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	outDir := fs.String("o", ".", "output directory")
	verbose := fs.Bool("v", false, "verbose output")
	raw := fs.Bool("raw", false, "write raw compressed blobs, do not decompress")
	fs.Usage = func() { fmt.Fprint(os.Stderr, extractUsage) }

	if err := fs.Parse(args); err != nil {
		return err
	}
	remaining := fs.Args()
	if len(remaining) < 1 {
		fs.Usage()
		return fmt.Errorf("requires <archive.gtar>")
	}

	archivePath := remaining[0]
	filterPaths := remaining[1:]

	ar, err := gtar.Open(archivePath)
	if err != nil {
		return err
	}
	defer ar.Close()

	// Load strtab if present
	strtabPath := strings.TrimSuffix(archivePath, filepath.Ext(archivePath)) + ".strtab"
	if paths, err := strtab.Load(strtabPath); err == nil {
		ar.Paths = paths
	}

	// Build filter set
	filterHashes := make(map[uint64]bool)
	for _, p := range filterPaths {
		filterHashes[gtar.HashPath(p)] = true
	}

	extracted := 0
	totalBytes := 0

	for i := range ar.Entries {
		e := &ar.Entries[i]

		if len(filterHashes) > 0 && !filterHashes[e.PathHash] {
			continue
		}

		// Determine output path
		name := ar.PathName(e)
		outPath := filepath.Join(*outDir, filepath.FromSlash(name))

		// Ensure output directory exists
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("mkdir %q: %w", filepath.Dir(outPath), err)
		}

		var data []byte
		if *raw {
			data, err = ar.ReadBlobRaw(e)
			if err != nil {
				return fmt.Errorf("read raw blob %q: %w", name, err)
			}
		} else {
			data, err = ar.ReadBlob(e)
			if err != nil {
				return fmt.Errorf("decompress %q: %w", name, err)
			}
		}

		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return fmt.Errorf("write %q: %w", outPath, err)
		}

		if *verbose {
			fmt.Printf("  extract  %-60s  %s\n", name, humanSize(len(data)))
		}

		extracted++
		totalBytes += len(data)
	}

	if len(filterPaths) > 0 && extracted < len(filterPaths) {
		fmt.Fprintf(os.Stderr, "warning: %d of %d requested paths not found in archive\n",
			len(filterPaths)-extracted, len(filterPaths))
	}

	fmt.Printf("extracted %d files -> %s (%s)\n", extracted, *outDir, humanSize(totalBytes))
	return nil
}
