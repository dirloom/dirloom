package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dirloom/dirloom/internal/releaseartifacts"
)

func main() {
	prepare := flag.NewFlagSet("prepare", flag.ExitOnError)
	prepareDist := prepare.String("dist", "dist", "GoReleaser dist directory")
	prepareSyft := prepare.String("syft", "syft", "syft executable")

	verify := flag.NewFlagSet("verify", flag.ExitOnError)
	verifyDist := verify.String("dist", "dist", "GoReleaser dist directory")

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: release-artifacts prepare|verify [--dist dir]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "prepare":
		_ = prepare.Parse(os.Args[2:])
		if err := releaseartifacts.Prepare(*prepareDist, *prepareSyft); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "verify":
		_ = verify.Parse(os.Args[2:])
		if err := releaseartifacts.Verify(*verifyDist); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := releaseartifacts.VerifyArchivePayloads(*verifyDist); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}
