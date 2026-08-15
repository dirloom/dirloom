package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
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
				if !values.Preset.Set || values.Preset.Name != "docs" || !values.Format.Set || values.Format.Value != FormatMarkdown || !values.Style.Set || values.Style.Value != StyleASCII || len(values.IgnorePatterns) != 3 {
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

func TestPublicPresetDocumentationMatchesCatalog(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "presets.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	configPattern := regexp.MustCompile(`(?s)<!-- dirloom-preset-config-example:([a-z-]+) -->\r?\n` + "```yaml" + `\r?\n(.*?)\r?\n` + "```")
	configs := configPattern.FindAllSubmatch(data, -1)
	if len(configs) != 2 {
		t.Fatalf("found %d tested preset YAML examples, want 2", len(configs))
	}
	for _, match := range configs {
		name := string(match[1])
		values, err := parseDocument(match[2], path+"#"+name)
		if err != nil {
			t.Errorf("example %q is invalid: %v", name, err)
			continue
		}
		switch name {
		case "project":
			if !values.Preset.Set || values.Preset.Name != "docs" || !values.Depth.Set || values.Depth.Value != 6 || len(values.IgnorePatterns) != 1 {
				t.Errorf("project example = %#v", values)
			}
		case "reset":
			if !values.Preset.Set || !values.Preset.Disabled {
				t.Errorf("reset example = %#v", values)
			}
		default:
			t.Errorf("preset YAML example %q has no assertion", name)
		}
	}

	jsonPattern := regexp.MustCompile(`(?s)<!-- dirloom-preset-json-example:ai -->\r?\n` + "```json" + `\r?\n(.*?)\r?\n` + "```")
	match := jsonPattern.FindSubmatch(data)
	if match == nil {
		t.Fatal("AI preset JSON contract example was not found")
	}
	var documented PresetDefinition
	if err := json.Unmarshal(match[1], &documented); err != nil {
		t.Fatalf("AI preset JSON example is invalid: %v", err)
	}
	actual, ok := LookupPreset("ai")
	if !ok || !reflect.DeepEqual(documented, actual) {
		t.Fatalf("documented AI preset differs from catalog\ndocumented=%#v\nactual=%#v", documented, actual)
	}

	commandPattern := regexp.MustCompile(`(?s)<!-- dirloom-preset-command:([a-z-]+) -->\r?\n` + "```bash" + `\r?\n(.*?)\r?\n` + "```")
	commands := commandPattern.FindAllSubmatch(data, -1)
	commandIDs := make([]string, 0, len(commands))
	for _, command := range commands {
		commandIDs = append(commandIDs, string(command[1]))
		if !strings.Contains(string(command[2]), "dirloom") {
			t.Errorf("command example %q does not invoke Dirloom", command[1])
		}
	}
	sort.Strings(commandIDs)
	wantCommandIDs := []string{"ai", "compact", "docs", "explain", "monorepo", "quick-start"}
	if !reflect.DeepEqual(commandIDs, wantCommandIDs) {
		t.Fatalf("preset command examples = %#v, want %#v", commandIDs, wantCommandIDs)
	}

	for _, row := range []string{
		"| `docs` | Documentation and architecture reviews | `4` | Files and directories | Markdown | None |",
		"| `compact` | A short structural overview | `3` | Directories only | Text | None |",
		"| `monorepo` | Workspace and package topology | `4` | Directories only | Text | `**/dist`, `**/build` |",
		"| `ai` | Structural context for AI-assisted work | `4` | Files and directories | Markdown | `**/dist`, `**/build`, `*.map` |",
	} {
		if !strings.Contains(text, row) {
			t.Errorf("preset catalog is missing canonical row %q", row)
		}
	}
	useCases, err := os.ReadFile(filepath.Join("..", "..", "docs", "use-cases.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(useCases), "| Presets nommés |") {
		t.Fatal("use cases still list named presets as unavailable")
	}
}
