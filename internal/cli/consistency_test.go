package cli

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// refusingCommands lists, for each command that refuses flags other commands
// take, its path, the operands that pass its argument check, the subject its
// diagnostics name, and the refused flags.
var refusingCommands = []struct {
	path     []string
	operands []string
	subject  string
	refused  []refusedFlag
}{
	{[]string{"verify"}, []string{"ref.dlm.json"}, "verify", verifyRefusedFlags},
	{[]string{"diff"}, []string{"snapshot:a.dlm.json", "live:."}, "diff", diffRefusedFlags},
	{[]string{"history"}, nil, "history", historyRefusedFlags},
	{[]string{"watch"}, nil, "watch", watchRefusedFlags},
	{[]string{"preset"}, nil, "preset inspection", sourceFlags(reasonNoConfiguration)},
	{[]string{"preset", "explain"}, []string{"ai"}, "preset inspection", sourceFlags(reasonNoConfiguration)},
	{[]string{"theme"}, nil, "theme inspection", sourceFlags(reasonNoConfiguration)},
	{[]string{"theme", "list"}, nil, "theme inspection", sourceFlags(reasonNoConfiguration)},
	{[]string{"theme", "explain"}, []string{"vivid"}, "theme inspection", sourceFlags(reasonNoConfiguration)},
	{[]string{"theme", "validate"}, []string{"theme.yaml"}, "theme inspection", sourceFlags(reasonNoConfiguration)},
	{[]string{"theme", "classify"}, []string{"a.txt"}, "theme classification", sourceFlags(reasonNoConfiguration)},
}

// TestHelpListsOnlyUsableFlags pins A5/D5: the help and the completion of a
// command never offer a flag the command refuses, inherited configuration
// flags included.
func TestHelpListsOnlyUsableFlags(t *testing.T) {
	for _, command := range refusingCommands {
		name := strings.Join(command.path, " ")
		help, stderr, code := executeForTest(t, append(append([]string{}, command.path...), "--help")...)
		if code != 0 || stderr != "" {
			t.Fatalf("%s --help = (%q, %d)", name, stderr, code)
		}
		completion, _, code := executeForTest(t, append(append(append([]string{"__complete"}, command.path...), command.operands...), "--")...)
		if code != 0 {
			t.Fatalf("%s completion exit %d", name, code)
		}
		for _, flag := range command.refused {
			for label, text := range map[string]string{"help": help, "completion": completion} {
				if strings.Contains(text, "--"+flag.name+" ") || strings.Contains(text, "--"+flag.name+"\t") || strings.Contains(text, "--"+flag.name+"\n") {
					t.Errorf("%s %s offers the refused flag --%s:\n%s", name, label, flag.name, text)
				}
			}
		}
		if command.subject != "watch" && strings.Contains(help, "Global Flags") {
			t.Errorf("%s help still lists inherited flags:\n%s", name, help)
		}
	}
}

// TestRefusedFlagsNameTheirReason checks that every refused flag, long or
// short, is a usage error that names the flag, the command and the reason,
// instead of "unknown flag".
func TestRefusedFlagsNameTheirReason(t *testing.T) {
	for _, command := range refusingCommands {
		for _, flag := range command.refused {
			spellings := [][]string{{"--" + flag.name}}
			if !flag.boolean {
				spellings[0] = append(spellings[0], "value")
			}
			if flag.shorthand != "" {
				spellings = append(spellings, []string{"-" + flag.shorthand, "value"})
			}
			for _, spelling := range spellings {
				args := append(append(append([]string{}, command.path...), command.operands...), spelling...)
				stdout, stderr, code := executeForTest(t, args...)
				want := "Error: --" + flag.name + " is not supported by " + command.subject + ": " + flag.reason + "\n"
				if code != 2 || stdout != "" || stderr != want {
					t.Errorf("dirloom %q = (%q, %q, %d), want exit 2 and %q", args, stdout, stderr, code, want)
				}
			}
		}
	}
	// An inherited flag given before the command name is refused the same way.
	stdout, stderr, code := executeForTest(t, "--no-config", "verify", "ref.dlm.json")
	if want := "Error: --no-config is not supported by verify: " + reasonCaptureSemantics + "\n"; code != 2 || stdout != "" || stderr != want {
		t.Errorf("--no-config verify = (%q, %q, %d), want %q", stdout, stderr, code, want)
	}
}

// TestInvalidFormatIsReportedUniformly checks that every command with a
// --format flag reports an empty or unknown value the same way, with its
// valid values and the help to read.
func TestInvalidFormatIsReportedUniformly(t *testing.T) {
	for _, command := range []struct {
		args   []string
		values []string
		help   string
	}{
		{[]string{}, []string{"text", "markdown", "json", "d2"}, "formats"},
		{[]string{"fingerprint"}, []string{"text", "json"}, "fingerprint"},
		{[]string{"verify", "ref.dlm.json"}, []string{"text", "json"}, "verify"},
		{[]string{"diff", "snapshot:a.dlm.json", "live:."}, []string{"text", "json"}, "diff"},
		{[]string{"history"}, []string{"text", "json"}, "history"},
		{[]string{"watch"}, []string{"text", "ndjson"}, "watch"},
	} {
		args := append(append([]string{}, command.args...), "--format", "banana")
		stdout, stderr, code := executeForTest(t, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, `Error: invalid value "banana" for --format`+"\n\nValid values:\n") ||
			!strings.Contains(stderr, "Run 'dirloom help "+command.help+"' for details.") {
			t.Errorf("dirloom %q = (%q, %q, %d)", args, stdout, stderr, code)
		}
		for _, value := range command.values {
			if !strings.Contains(stderr, "\n  "+value+"\n") {
				t.Errorf("dirloom %q does not list %q:\n%s", args, value, stderr)
			}
		}
		args = append(append([]string{}, command.args...), "--format", "")
		stdout, stderr, code = executeForTest(t, args...)
		if code != 2 || stdout != "" || stderr != "Error: --format requires a non-empty value\n" {
			t.Errorf("dirloom %q = (%q, %q, %d)", args, stdout, stderr, code)
		}
	}
}

// TestSharedFlagsShareTheirWording checks that a flag several commands take
// is described the same way everywhere.
func TestSharedFlagsShareTheirWording(t *testing.T) {
	root := NewRootCommand(io.Discard, io.Discard, "test")
	for _, check := range []struct {
		flag     string
		usage    string
		commands [][]string
	}{
		{"depth", depthUsage, [][]string{{}, {"config", "explain"}, {"fingerprint"}, {"snapshot"}, {"watch"}}},
		{"dirs-only", dirsOnlyUsage, [][]string{{}, {"config", "explain"}, {"fingerprint"}, {"snapshot"}, {"watch"}}},
		{"no-moves", noMovesUsage, [][]string{{"diff"}, {"history"}, {"watch"}}},
	} {
		for _, path := range check.commands {
			command := findCommand(t, root, path)
			flag := command.Flags().Lookup(check.flag)
			if flag == nil || flag.Usage != check.usage {
				t.Errorf("dirloom %v --%s usage = %v, want %q", path, check.flag, flag, check.usage)
			}
		}
	}
}

// TestHelpHasNoTabs checks every command help and every help topic for a
// TAB, which renders differently from terminal to terminal.
func TestHelpHasNoTabs(t *testing.T) {
	var visit func(path []string, command *cobra.Command)
	visit = func(path []string, command *cobra.Command) {
		help, stderr, code := executeForTest(t, append(append([]string{}, path...), "--help")...)
		if code != 0 || stderr != "" || strings.Contains(help, "\t") {
			t.Errorf("dirloom %v --help = (%q, %d), contains a TAB: %v", path, stderr, code, strings.Contains(help, "\t"))
		}
		for _, child := range command.Commands() {
			if child.IsAvailableCommand() {
				visit(append(append([]string{}, path...), child.Name()), child)
			}
		}
	}
	visit(nil, NewRootCommand(io.Discard, io.Discard, "test"))
	for _, name := range helpTopicNames() {
		if topic, _, _ := executeForTest(t, "help", name); strings.Contains(topic, "\t") {
			t.Errorf("help topic %s contains a TAB", name)
		}
	}
}

// TestStructuralPresentationFlagsAreValidatedThenIgnored pins F3: fingerprint
// and snapshot validate --color, --icons and --theme like the tree output,
// then ignore a valid value. A --theme path is checked for its form only:
// the file is not read.
func TestStructuralPresentationFlagsAreValidatedThenIgnored(t *testing.T) {
	root := t.TempDir()
	for _, command := range []string{"fingerprint", "snapshot"} {
		plain, stderr, code := executeForTest(t, command, root, "--no-config")
		if code != 0 || stderr != "" {
			t.Fatalf("%s = (%q, %d)", command, stderr, code)
		}
		for _, invalid := range [][]string{{"--color", "banana"}, {"--icons", "banana"}, {"--theme", "banana"}} {
			stdout, stderr, code := executeForTest(t, append([]string{command, root, "--no-config"}, invalid...)...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, `"banana"`) {
				t.Errorf("%s %v = (%q, %q, %d), want exit 2", command, invalid, stdout, stderr, code)
			}
		}
		missingTheme := filepath.Join(t.TempDir(), "missing.yaml")
		valid, stderr, code := executeForTest(t, command, root, "--no-config", "--color", "always", "--icons", "nerd", "--theme", missingTheme)
		if code != 0 || stderr != "" || valid != plain {
			t.Errorf("%s with valid presentation flags = (%q, %d), output changed: %v", command, stderr, code, valid != plain)
		}
		help, _, _ := executeForTest(t, command, "--help")
		if strings.Contains(help, "accepted and ignored") || strings.Count(help, "validated, then ignored") != 3 {
			t.Errorf("%s help does not say the presentation flags are validated:\n%s", command, help)
		}
	}
}

func findCommand(t *testing.T, root *cobra.Command, path []string) *cobra.Command {
	t.Helper()
	command := root
	for _, name := range path {
		var next *cobra.Command
		for _, child := range command.Commands() {
			if child.Name() == name {
				next = child
			}
		}
		if next == nil {
			t.Fatalf("no command %v", path)
		}
		command = next
	}
	return command
}
