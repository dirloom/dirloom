// Command perfbaseline measures complete Dirloom workflows for the
// v0.4.0-rc1 performance and memory baselines (decisions D6 and D8 of the
// rc1 plan, LOT 6). It generates the fixtures, runs every scenario in fresh
// processes that call cli.Execute exactly as cmd/dirloom does, and keeps
// for each run the Go heap, the process memory, the allocations, the time
// and, for watch, the convergence. On Windows each run is held in a job
// object, which records the peak commit and can enforce a memory limit.
//
// The method and the results are in docs/benchmarks/v0.4-rc1.md.
//
//	go build -o perfbaseline.exe ./tools/perfbaseline
//	perfbaseline gen -fixtures F
//	perfbaseline run -fixtures F -out results.jsonl [-scenarios RE] [-runs 5] [-mem-limit 8GiB] [-label L]
//	perfbaseline summarize results.jsonl
//	perfbaseline compare -base old.jsonl -new new.jsonl
//	perfbaseline list
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "gen":
		err = runGen(os.Args[2:])
	case "run":
		err = runRunner(os.Args[2:])
	case "exec":
		err = runChild(os.Args[2:])
	case "summarize":
		err = runSummarize(os.Args[2:])
	case "compare":
		err = runCompare(os.Args[2:])
	case "list":
		err = list()
	default:
		usage()
		os.Exit(2)
	}
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "perfbaseline:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: perfbaseline <command> [flags]

  gen        generate the fixtures
  run        run scenarios in fresh processes and append their results
  summarize  print the distribution of each scenario (Markdown or benchstat)
  compare    compare two campaigns against the investigation threshold
  list       list the scenarios and the fixtures
`)
}

func list() error {
	fmt.Println("| Scenario | Priority | Group | Command | What it measures |\n| --- | --- | --- | --- | --- |")
	for _, s := range scenarios() {
		command := "dirloom " + filepath.ToSlash(strings.Join(s.args("F"), " "))
		if s.Dir != "" {
			command = "(in F/" + s.Dir + ") " + command
		}
		fmt.Printf("| `%s` | %s | %s | `%s` | %s |\n", s.Name, s.Priority, s.Group, command, s.About)
	}
	fmt.Println("\n| Fixture | Contents |\n| --- | --- |")
	for _, f := range fixtures() {
		fmt.Printf("| `%s` | %s |\n", f.Name, f.About)
	}
	return nil
}
