package history

import (
	"strings"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
)

func id(c string) string { return strings.Repeat(c, 40) }

// validResult is D ← C with D listed first; C's parent B is the bound.
func validResult() Result {
	at := time.Unix(1767225600, 0).UTC()
	return Result{
		To: id("d"), From: id("b"), Limit: 5, End: EndFrom, MoveDetection: comparison.MoveDetectionStructuralV1,
		Entries: []Entry{
			{Commit: id("d"), Tree: id("1"), Parents: []string{id("c")}, CommittedAt: at, NodeCount: 3, HasBase: true, Structural: true,
				Summary: comparison.Summary{Added: 1, Total: 1}},
			{Commit: id("c"), Tree: id("2"), Parents: []string{id("b")}, CommittedAt: at, NodeCount: 2, HasBase: true},
		},
	}
}

func TestValidateAcceptsAValidResult(t *testing.T) {
	if err := validResult().Validate(); err != nil {
		t.Fatal(err)
	}
	empty := Result{To: id("d"), From: id("d"), Limit: 1, End: EndFrom, MoveDetection: comparison.MoveDetectionNone, Entries: []Entry{}}
	if err := empty.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsBrokenResults(t *testing.T) {
	cases := map[string]func(*Result){
		"bad to":                 func(r *Result) { r.To = "HEAD" },
		"bad from":               func(r *Result) { r.From = strings.Repeat("b", 64) },
		"limit zero":             func(r *Result) { r.Limit = 0 },
		"unknown end":            func(r *Result) { r.End = "graph" },
		"unknown move detection": func(r *Result) { r.MoveDetection = "fuzzy" },
		"from end without bound": func(r *Result) { r.From = "" },
		"above limit":            func(r *Result) { r.Limit = 1 },
		"limit end short":        func(r *Result) { r.End = EndLimit; r.From = "" },
		"wrong first commit":     func(r *Result) { r.To = id("e") },
		"broken chain":           func(r *Result) { r.Entries[0].Parents = []string{id("f")} },
		"lists the bound":        func(r *Result) { r.Entries[1].Commit = id("b"); r.Entries[0].Parents = []string{id("b")} },
		"from tail mismatch":     func(r *Result) { r.Entries[1].Parents = []string{id("a")} },
		"root tail with parents": func(r *Result) { r.End = EndRoot; r.From = "" },
		"shallow tail with base": func(r *Result) { r.End = EndShallow; r.From = "" },
		"missing base inside": func(r *Result) {
			r.Entries[0].HasBase = false
			r.Entries[0].Structural = false
			r.Entries[0].Summary = comparison.Summary{}
		},
		"missing base at the end": func(r *Result) { r.Entries[1].HasBase = false },
		"summary sum":             func(r *Result) { r.Entries[0].Summary.Total = 2 },
		"structural without changes": func(r *Result) {
			r.Entries[1].Structural = true
		},
		"changes without structural": func(r *Result) { r.Entries[0].Structural = false },
		"local time":                 func(r *Result) { r.Entries[0].CommittedAt = r.Entries[0].CommittedAt.Local() },
		"no root record":             func(r *Result) { r.Entries[0].NodeCount = 0 },
		"short parent":               func(r *Result) { r.Entries[1].Parents = []string{"b"} },
		"empty without bound":        func(r *Result) { r.Entries = []Entry{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			result := validResult()
			mutate(&result)
			if name == "local time" && time.Local == time.UTC {
				t.Skip("the local zone is UTC")
			}
			if err := result.Validate(); !artifact.IsInternal(err) {
				t.Fatalf("accepted: %v", err)
			}
		})
	}
}
