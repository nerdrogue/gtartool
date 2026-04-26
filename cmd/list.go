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

const listUsage = `Usage: gtartool list [options] <archive.gtar>

List contents of a .gtar archive.

Options:
  -l    Long format: show hash, codec, sizes, checksum
  -sort-size   Sort by raw size descending (default: sorted by hash)

Examples:
  gtartool list assets.gtar
  gtartool list -l assets.gtar
`

func RunList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	long := fs.Bool("l", false, "long format")
	sortSize := fs.Bool("sort-size", false, "sort by raw size descending")
	fs.Usage = func() { fmt.Fprint(os.Stderr, listUsage) }

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
	if paths, err := strtab.Load(strtabPath); err == nil {
		ar.Paths = paths
	}

	entries := make([]gtar.Entry, len(ar.Entries))
	copy(entries, ar.Entries)

	if *sortSize {
		// Sort by raw size descending
		for i := 0; i < len(entries)-1; i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[j].SizeRaw > entries[i].SizeRaw {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}
	}

	totalRaw := uint64(0)
	totalStored := uint64(0)

	if *long {
		fmt.Printf("%-16s  %-8s  %10s  %10s  %6s  %8s  %s\n",
			"hash", "codec", "raw", "stored", "ratio", "checksum", "path")
		fmt.Println(strings.Repeat("-", 90))
	}

	for i := range entries {
		e := &entries[i]
		name := ar.PathName(e)
		totalRaw += uint64(e.SizeRaw)
		totalStored += uint64(e.SizeStored)

		if *long {
			ratio := 100.0
			if e.SizeRaw > 0 {
				ratio = 100.0 * float64(e.SizeStored) / float64(e.SizeRaw)
			}
			fmt.Printf("%016x  %-8s  %10s  %10s  %5.1f%%  %08x  %s\n",
				e.PathHash,
				gtar.CodecName(e.Codec),
				humanSize(int(e.SizeRaw)),
				humanSize(int(e.SizeStored)),
				ratio,
				e.Checksum,
				name,
			)
		} else {
			fmt.Println(name)
		}
	}

	if *long {
		fmt.Println(strings.Repeat("-", 90))
		overallRatio := 100.0
		if totalRaw > 0 {
			overallRatio = 100.0 * float64(totalStored) / float64(totalRaw)
		}
		fmt.Printf("%-16s  %-8s  %10s  %10s  %5.1f%%\n",
			fmt.Sprintf("%d files", len(entries)),
			"",
			humanSize(int(totalRaw)),
			humanSize(int(totalStored)),
			overallRatio,
		)
	} else {
		fmt.Printf("\n%d files\n", len(entries))
	}

	return nil
}
