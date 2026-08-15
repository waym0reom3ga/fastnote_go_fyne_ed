// FastNote go_fyne — entry point.
//
// Exactly two permitted flags (spec §5.1): --version and --event-file.
// Unknown flags exit non-zero.

package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `FastNote - markdown editor

Usage:
  fastnote [flags]

Flags:
  --version        print the version
  --event-file P   append phase markers (painted/open/save/etc.) to P
  --help           print this help
`

func runCLI(args []string) int {
	fs := flag.NewFlagSet("fastnote", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	doVersion := fs.Bool("version", false, "")
	eventFile := fs.String("event-file", "", "")
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

	state := NewAppState("")
	state.EventFile = *eventFile
	RunGUI(state, "")
	return 0
}

func main() {
	os.Exit(runCLI(os.Args[1:]))
}
