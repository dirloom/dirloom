package config

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

func TestPublicConfigurationExamplesUseTheRealSchema(t *testing.T) {
	pattern := regexp.MustCompile(`(?s)<!-- dirloom-config-example:([a-z-]+) -->\r?\n` + "```yaml" + `\r?\n(.*?)\r?\n` + "```")
	paths := []string{
		filepath.Join("..", "..", "docs", "configuration.md"),
		filepath.Join("..", "..", "README.md"),
	}
	found := make([]string, 0, 6)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		matches := pattern.FindAllSubmatch(data, -1)
		for _, match := range matches {
			name := string(match[1])
			found = append(found, name)
			values, err := parseDocument(match[2], path+"#"+name)
			if err != nil {
				t.Errorf("example %q is invalid: %v", name, err)
				continue
			}
			switch name {
			case "quick-start":
				if !values.Depth.Set || values.Depth.Value != 4 || len(values.IgnorePatterns) != 2 {
					t.Errorf("quick-start values = %#v", values)
				}
			case "complete":
				if !values.Format.Set || values.Format.Value != FormatMarkdown || !values.Style.Set || values.Style.Value != StyleASCII || len(values.IgnorePatterns) != 3 {
					t.Errorf("complete values = %#v", values)
				}
			case "team":
				if len(values.IgnorePatterns) != 2 {
					t.Errorf("team values = %#v", values)
				}
			case "user":
				if !values.Depth.Set || values.Depth.Value != 3 || values.Style.Value != StyleASCII {
					t.Errorf("user values = %#v", values)
				}
			case "unlimited":
				if !values.Depth.Set || !values.Depth.Unlimited {
					t.Errorf("unlimited values = %#v", values)
				}
			case "readme":
				if !values.Depth.Set || values.Depth.Value != 4 || values.Format.Value != FormatText || len(values.IgnorePatterns) != 1 {
					t.Errorf("README values = %#v", values)
				}
			default:
				t.Errorf("example %q has no contract assertion", name)
			}
		}
	}
	sort.Strings(found)
	want := []string{"complete", "quick-start", "readme", "team", "unlimited", "user"}
	if len(found) != len(want) {
		t.Fatalf("found %d tested YAML examples, want %d: %#v", len(found), len(want), found)
	}
	for index := range want {
		if found[index] != want[index] {
			t.Fatalf("example names = %#v, want %#v", found, want)
		}
	}
}
