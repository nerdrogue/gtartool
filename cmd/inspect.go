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

const inspectUsage = `Usage: gtartool inspect [options] <archive.gtar>

Dump the raw binary header and index of a .gtar archive in human-readable form.
Useful for debugging archive integrity without extracting anything.

Options:
  -entry <path>   Inspect a single entry by path (uses strtab for lookup)
  -hex            Also dump the raw entry bytes in hex

Examples:
  gtartool inspect assets.gtar
  gtartool inspect -entry scripts/mission_01.lua assets.gtar
`

func RunInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	entryPath := fs.String("entry", "", "inspect a single entry by path")
	showHex := fs.Bool("hex", false, "dump raw entry bytes in hex")
	fs.Usage = func() { fmt.Fprint(os.Stderr, inspectUsage) }

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("requires <archive.gtar>")
	}

	archivePath := fs.Arg(0)
	ar, err := gtar.Open(archivePath)
	if err != nil {
		return err
	}
	defer ar.Close()

	// Load strtab if present
	strtabPath := strings.TrimSuffix(archivePath, filepath.Ext(archivePath)) + ".strtab"
	strtabLoaded := false
	if paths, err := strtab.Load(strtabPath); err == nil {
		ar.Paths = paths
		strtabLoaded = true
	}

	fi, _ := os.Stat(archivePath)

	// --- File header ---
	fmt.Println("=== GTAR HEADER ===")
	fmt.Printf("  magic        : %q\n", string(ar.Hdr.Magic[:]))
	fmt.Printf("  version      : %d\n", ar.Hdr.Version)
	fmt.Printf("  flags        : 0x%04x\n", ar.Hdr.Flags)
	fmt.Printf("  entry_count  : %d\n", ar.Hdr.EntryCount)
	fmt.Printf("  file_size    : %s (%d bytes)\n", humanSize(int(fi.Size())), fi.Size())
	fmt.Printf("  index_size   : %d bytes (header=%d + entries=%d×%d)\n",
		gtar.HeaderSize+int(ar.Hdr.EntryCount)*gtar.EntrySize,
		gtar.HeaderSize, ar.Hdr.EntryCount, gtar.EntrySize,
	)
	fmt.Printf("  blob_align   : %d bytes\n", gtar.BlobAlignment)
	fmt.Printf("  strtab       : ")
	if strtabLoaded {
		fmt.Printf("%s (%d paths loaded)\n", strtabPath, len(ar.Paths))
	} else {
		fmt.Println("not found")
	}
	fmt.Println()

	// --- Single entry mode ---
	if *entryPath != "" {
		e := ar.Find(*entryPath)
		if e == nil {
			return fmt.Errorf("entry not found: %q (hash %016x)", *entryPath, gtar.HashPath(*entryPath))
		}
		printEntry(ar, e, *showHex)
		return nil
	}

	// --- Full index dump ---
	fmt.Printf("=== INDEX (%d entries) ===\n", len(ar.Entries))
	fmt.Printf("  %-4s  %-16s  %-16s  %-10s  %-10s  %-8s  %8s  %s\n",
		"#", "path_hash", "blob_offset", "size_raw", "size_stored", "codec", "checksum", "path")
	fmt.Println("  " + strings.Repeat("-", 110))

	for i := range ar.Entries {
		e := &ar.Entries[i]
		name := ar.PathName(e)
		ratio := 100.0
		if e.SizeRaw > 0 {
			ratio = 100.0 * float64(e.SizeStored) / float64(e.SizeRaw)
		}
		fmt.Printf("  %-4d  %016x  %016x  %10s  %10s  %-8s  %08x  %s  (%.1f%%)\n",
			i,
			e.PathHash,
			e.BlobOffset,
			humanSize(int(e.SizeRaw)),
			humanSize(int(e.SizeStored)),
			gtar.CodecName(e.Codec),
			e.Checksum,
			name,
			ratio,
		)
		if *showHex {
			dumpEntryHex(e)
		}
	}

	return nil
}

func printEntry(ar *gtar.Archive, e *gtar.Entry, showHex bool) {
	name := ar.PathName(e)
	ratio := 100.0
	if e.SizeRaw > 0 {
		ratio = 100.0 * float64(e.SizeStored) / float64(e.SizeRaw)
	}
	fmt.Println("=== ENTRY ===")
	fmt.Printf("  path         : %s\n", name)
	fmt.Printf("  path_hash    : %016x\n", e.PathHash)
	fmt.Printf("  blob_offset  : %016x  (%d)\n", e.BlobOffset, e.BlobOffset)
	fmt.Printf("  size_raw     : %s (%d bytes)\n", humanSize(int(e.SizeRaw)), e.SizeRaw)
	fmt.Printf("  size_stored  : %s (%d bytes)\n", humanSize(int(e.SizeStored)), e.SizeStored)
	fmt.Printf("  ratio        : %.1f%%\n", ratio)
	fmt.Printf("  codec        : %s (%d)\n", gtar.CodecName(e.Codec), e.Codec)
	fmt.Printf("  flags        : 0x%04x\n", e.Flags)
	fmt.Printf("  checksum     : %08x  (xxHash32 of compressed blob)\n", e.Checksum)
	if showHex {
		fmt.Println()
		dumpEntryHex(e)
	}
}

func dumpEntryHex(e *gtar.Entry) {
	// Manually lay out the 32 entry bytes in little-endian
	b := make([]byte, gtar.EntrySize)
	putU64LE(b[0:], e.PathHash)
	putU64LE(b[8:], e.BlobOffset)
	putU32LE(b[16:], e.SizeRaw)
	putU32LE(b[20:], e.SizeStored)
	putU16LE(b[24:], e.Codec)
	putU16LE(b[26:], e.Flags)
	putU32LE(b[28:], e.Checksum)

	fmt.Printf("  raw bytes: ")
	for i, byt := range b {
		if i > 0 && i%8 == 0 {
			fmt.Printf(" ")
		}
		fmt.Printf("%02x", byt)
	}
	fmt.Println()
}

func putU64LE(b []byte, v uint64) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}
func putU32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
func putU16LE(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}
