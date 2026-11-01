// Command summary turns an evidence JSONL file into a Markdown table.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type record struct {
	Fact    string `json:"fact"`
	OS      string `json:"os"`
	Verdict string `json:"verdict"`
	Detail  string `json:"detail"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: summary <evidence.jsonl>")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("No evidence file: %v\n", err)
		return
	}
	defer f.Close()
	var records []record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var r record
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			records = append(records, r)
		}
	}
	sort.SliceStable(records, func(i, j int) bool { return factNumber(records[i].Fact) < factNumber(records[j].Fact) })
	fmt.Println("| Fact | OS | Verdict | Detail |")
	fmt.Println("| --- | --- | --- | --- |")
	for _, r := range records {
		detail := strings.ReplaceAll(strings.ReplaceAll(r.Detail, "|", "\\|"), "\n", " ")
		fmt.Printf("| %s | %s | %s | %s |\n", r.Fact, r.OS, r.Verdict, detail)
	}
}

func factNumber(fact string) int {
	var n int
	_, _ = fmt.Sscanf(fact, "F%d", &n)
	return n
}
