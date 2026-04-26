package main

import (
	"fmt"
	"os"

	"github.com/nerdrogue/gtartool/cmd"
)

const usage = `gtartool — create and inspect .gtar archives for the deer engine

Usage:
  gtartool <subcommand> [options] ...

Subcommands:
  pack      Create a .gtar archive from files or directories
  extract   Extract files from a .gtar archive
  list      List archive contents
  inspect   Dump archive header and index

Run 'gtartool <subcommand> -help' for subcommand-specific options.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	sub := os.Args[1]
	args := os.Args[2:]

	var err error
	switch sub {
	case "pack":
		err = cmd.RunPack(args)
	case "extract":
		err = cmd.RunExtract(args)
	case "list":
		err = cmd.RunList(args)
	case "inspect":
		err = cmd.RunInspect(args)
	case "-h", "--help", "help":
		fmt.Fprint(os.Stderr, usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n\n%s", sub, usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
