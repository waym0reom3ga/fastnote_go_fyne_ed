// FastNote go_gio — entry point.
//
// CLI seams (spec §5): --open/--insert/--save/--export plus --headless,
// --selftest, --version.  Unknown flags exit non-zero (argparse-style, so a
// binary that ignores flags cannot fool the acceptance harness).

package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `FastNote - markdown editor

Usage:
  fastnote [flags] [--open FILE] [--insert TEXT] [--save] [--export PATH]

Flags:
  --open PATH      open a file
  --insert TEXT    insert text at the document end
  --save           write the document to disk
  --export PATH    export HTML (or PDF when PATH ends in .pdf)
  --headless       stay in the CLI; never open a window
  --notes-dir DIR  the user notes directory (default: home)
  --selftest       run the internal self-test suite
  --version        print the version
  --help           print this help
`

func runCLI(args []string) int {
	fs := flag.NewFlagSet("fastnote", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	openPath := fs.String("open", "", "")
	insert := fs.String("insert", "", "")
	doSave := fs.Bool("save", false, "")
	exportPath := fs.String("export", "", "")
	headless := fs.Bool("headless", false, "")
	notesDir := fs.String("notes-dir", "", "")
	doSelftest := fs.Bool("selftest", false, "")
	doVersion := fs.Bool("version", false, "")
	doHelp := fs.Bool("help", false, "")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Print(usage)
			return 0
		}
		return 2
	}

	if *doVersion {
		fmt.Printf("FastNote %s v%s\n", portID, version)
		return 0
	}
	if *doHelp {
		fmt.Print(usage)
		return 0
	}
	if *doSelftest {
		if RunSelfTest() {
			return 0
		}
		return 1
	}

	state := NewAppState(*notesDir)
	if err := RunCLIActions(state, *openPath, *insert, *doSave, *exportPath); err != nil {
		fmt.Fprintf(os.Stderr, "fastnote: %v\n", err)
		return 1
	}

	if *headless {
		return 0
	}

	RunGUI(state, *openPath)
	return 0
}

func main() {
	os.Exit(runCLI(os.Args[1:]))
}