package filter

import "testing"

func TestIgnoreMatcher(t *testing.T) {
	matcher, err := NewIgnoreMatcher([]string{
		"node_modules",
		"*.log",
		"src/**/generated?.go",
		"literal,comma",
		"cache/",
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path  string
		dir   bool
		match bool
	}{
		{path: "node_modules", dir: true, match: true},
		{path: "apps/web/node_modules", dir: true, match: true},
		{path: "logs/debug.log", match: true},
		{path: "logs/debug.LOG", match: false},
		{path: "src/generated1.go", match: true},
		{path: "src/a/b/generatedZ.go", match: true},
		{path: "src/a/b/generated.go", match: false},
		{path: "literal,comma", match: true},
		{path: "cache", dir: true, match: true},
		{path: "cache", dir: false, match: false},
	}

	for _, test := range tests {
		if got := matcher.Match(test.path, test.dir); got != test.match {
			t.Errorf("Match(%q, %v) = %v, want %v", test.path, test.dir, got, test.match)
		}
	}
}

func TestIgnoreMatcherRejectsInvalidPatterns(t *testing.T) {
	for _, pattern := range []string{"", "/absolute", `C:\absolute`, "C:drive-relative", "../outside", "foo//bar", "[unterminated"} {
		t.Run(pattern, func(t *testing.T) {
			if _, err := NewIgnoreMatcher([]string{pattern}); err == nil {
				t.Fatalf("expected %q to be rejected", pattern)
			}
		})
	}
}

func TestIgnoreMatcherNormalizesSeparators(t *testing.T) {
	matcher, err := NewIgnoreMatcher([]string{`src\**\generated.go`})
	if err != nil {
		t.Fatal(err)
	}
	if !matcher.Match("src/pkg/generated.go", false) {
		t.Fatal("Windows-style separators were not normalized")
	}
}
