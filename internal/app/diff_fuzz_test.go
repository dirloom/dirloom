package app

import (
	"testing"
)

func FuzzParseDiffSource(f *testing.F) {
	f.Add("snapshot:a.dlm.json")
	f.Add("live:dir")
	f.Add(`snapshot:C:\data\x.dlm.json`)
	f.Add("live:relative:with:colons")
	f.Add("snapshot:")
	f.Add("live:")
	f.Add(":path")
	f.Add("a.dlm.json")
	f.Add("SNAPSHOT:x")
	f.Add("git:HEAD")
	f.Add("")
	f.Add(":")
	f.Add("snapshot:snapshot:live:")
	f.Fuzz(func(t *testing.T, expression string) {
		spec, err := ParseDiffSource(expression)
		if err != nil {
			// Every rejection is a usage failure, never a panic or a
			// differently-shaped error.
			if !DiffUsage(err) {
				t.Fatalf("%q error %v is not a usage failure", expression, err)
			}
			return
		}
		// A parsed spec is accepted by the request validator.
		if err2 := validateDiffSpec(spec, "a"); err2 != nil {
			t.Fatalf("%q parsed but rejected: %v", expression, err2)
		}
		// Parsing is deterministic.
		again, err3 := ParseDiffSource(expression)
		if err3 != nil || again != spec {
			t.Fatalf("%q not deterministic: %#v %v", expression, again, err3)
		}
	})
}
