package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nerdrogue/gtartool/gtar"
	"github.com/nerdrogue/gtartool/manifest"
	"github.com/nerdrogue/gtartool/strtab"
)

const packUsage = `Usage: gtartool pack [options] <output.gtar> <file|dir> [file|dir ...]

Pack files into a .gtar archive.

A companion .strtab sidecar is always written alongside the archive.
A .manifest.json file can be placed next to the output archive to configure
codec overrides and brotli quality. The manifest is loaded automatically if found.

Options:
  -v    Verbose: print each file being packed
  -strip-prefix <prefix>
        Strip this prefix from all input paths before storing in the archive.
        Useful when packing from a build directory.

Examples:
  gtartool pack assets.gtar ./build/assets/
  gtartool pack -strip-prefix build/assets/ assets.gtar build/assets/
`

func RunPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	verbose := fs.Bool("v", false, "verbose output")
	stripPrefix := fs.String("strip-prefix", "", "strip prefix from stored paths")
	fs.Usage = func() { fmt.Fprint(os.Stderr, packUsage) }

	if err := fs.Parse(args); err != nil {
		return err
	}
	remaining := fs.Args()
	if len(remaining) < 2 {
		fs.Usage()
		return fmt.Errorf("requires <output.gtar> and at least one input path")
	}

	outPath := remaining[0]
	inputs := remaining[1:]

	// Load manifest if present
	manifestPath := manifest.ManifestPath(outPath)
	mf, err := manifest.Load(manifestPath)
	if err != nil {
		return err
	}
	if mf != nil {
		fmt.Fprintf(os.Stderr, "manifest: loaded %s\n", manifestPath)
	}

	// Collect all input files
	var allFiles []string
	for _, inp := range inputs {
		info, err := os.Stat(inp)
		if err != nil {
			return fmt.Errorf("stat %q: %w", inp, err)
		}
		if info.IsDir() {
			walked, err := walkDir(inp)
			if err != nil {
				return err
			}
			allFiles = append(allFiles, walked...)
		} else {
			allFiles = append(allFiles, inp)
		}
	}

	if len(allFiles) == 0 {
		return fmt.Errorf("no files found in inputs")
	}

	// Build FileEntry list
	fileEntries := make([]gtar.FileEntry, 0, len(allFiles))
	for _, fp := range allFiles {
		data, err := os.ReadFile(fp)
		if err != nil {
			return fmt.Errorf("read %q: %w", fp, err)
		}

		storedPath := fp
		if *stripPrefix != "" {
			storedPath = strings.TrimPrefix(storedPath, *stripPrefix)
		}
		// Normalize separators for display
		storedPath = filepath.ToSlash(storedPath)
		canonPath := gtar.CanonicalPath(storedPath)

		// Detect codec
		detectedCodec := gtar.DetectCodec(storedPath, data)

		// Apply manifest overrides
		codec, brotliQ, err := mf.ResolveCodec(canonPath, detectedCodec)
		if err != nil {
			return err
		}

		if *verbose {
			fmt.Printf("  pack  %-60s  codec=%-8s  raw=%s\n",
				canonPath, gtar.CodecName(codec), humanSize(len(data)))
		}

		fileEntries = append(fileEntries, gtar.FileEntry{
			Path:          storedPath,
			Data:          data,
			Codec:         codec,
			BrotliQuality: brotliQ,
		})
	}

	// Write archive
	if err := gtar.WriteArchive(outPath, fileEntries); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}

	// Write strtab
	paths := gtar.PathsFromFiles(fileEntries)
	strtabPath := strings.TrimSuffix(outPath, filepath.Ext(outPath)) + ".strtab"
	if err := strtab.Write(strtabPath, paths); err != nil {
		return fmt.Errorf("write strtab: %w", err)
	}

	// Print summary
	outInfo, _ := os.Stat(outPath)
	totalRaw := 0
	for _, fe := range fileEntries {
		totalRaw += len(fe.Data)
	}
	fmt.Printf("packed %d files -> %s (raw %s, stored %s, ratio %.1f%%)\n",
		len(fileEntries), outPath,
		humanSize(totalRaw),
		humanSize(int(outInfo.Size())),
		100.0*float64(outInfo.Size())/float64(totalRaw),
	)
	fmt.Printf("strtab -> %s\n", strtabPath)

	return nil
}

func walkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func humanSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKiB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1fMiB", float64(n)/(1024*1024))
	default:
		return fmt.Sprintf("%.1fGiB", float64(n)/(1024*1024*1024))
	}
}
