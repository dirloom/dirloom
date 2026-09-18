package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/spf13/cobra"
)

func TestNormalizeOptionalAutoFlags(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "help is not an icon value", in: []string{"--icons", "--help"}, want: []string{"--icons=auto", "--help"}},
		{name: "version is not a color value", in: []string{"--color", "--version"}, want: []string{"--color=auto", "--version"}},
		{name: "separated known icon mode", in: []string{"root", "--no-config", "--icons", "unicode"}, want: []string{"root", "--no-config", "--icons=unicode"}},
		{name: "separated ascii icon mode", in: []string{"--icons", "ascii"}, want: []string{"--icons=ascii"}},
		{name: "equals icon mode is unchanged", in: []string{"--icons=nerd"}, want: []string{"--icons=nerd"}},
		{name: "separated known color mode", in: []string{"--color", "always"}, want: []string{"--color=always"}},
		{name: "equals color mode is unchanged", in: []string{"--color=never"}, want: []string{"--color=never"}},
		{name: "next flag stays independent", in: []string{"--icons", "--depth", "3"}, want: []string{"--icons=auto", "--depth", "3"}},
		{name: "format after bare icons", in: []string{"--icons", "--format", "json"}, want: []string{"--icons=auto", "--format", "json"}},
		{name: "theme after bare icons", in: []string{"--icons", "--theme", "vivid"}, want: []string{"--icons=auto", "--theme", "vivid"}},
		{name: "boolean after bare icons", in: []string{"--icons", "--no-config"}, want: []string{"--icons=auto", "--no-config"}},
		{name: "bare color then icons value", in: []string{"--color", "--icons", "nerd"}, want: []string{"--color=auto", "--icons=nerd"}},
		{name: "terminator after bare icons", in: []string{"--icons", "--", "./directory"}, want: []string{"--icons=auto", "--", "./directory"}},
		{name: "no rewrite after terminator", in: []string{"--", "--icons", "unicode"}, want: []string{"--", "--icons", "unicode"}},
		{name: "invalid separated value stays a flag value", in: []string{"--icons", "banana"}, want: []string{"--icons=banana"}},
		{name: "invalid color value stays a flag value", in: []string{"--color", "banana"}, want: []string{"--color=banana"}},
		{name: "completion empty toComplete", in: []string{"__complete", "--icons", ""}, want: []string{"__complete", "--icons", ""}},
		{name: "completion partial icon value", in: []string{"__complete", "--icons", "un"}, want: []string{"__complete", "--icons", "un"}},
		{name: "completion partial color value", in: []string{"__completeNoDesc", "--color", "al"}, want: []string{"__completeNoDesc", "--color", "al"}},
		{name: "completion next flag becomes auto", in: []string{"__complete", "--icons", "--format", ""}, want: []string{"__complete", "--icons=auto", "--format", ""}},
		{name: "trailing bare flag becomes auto", in: []string{"--icons"}, want: []string{"--icons=auto"}},
		{name: "bare pair", in: []string{"--color", "--icons"}, want: []string{"--color=auto", "--icons=auto"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := normalizeOptionalAutoFlags(test.in)
			if strings.Join(got, "\x00") != strings.Join(test.want, "\x00") {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestHelpTopicRegistryIsValid(t *testing.T) {
	if problems := helpTopicRegistryProblems(); len(problems) > 0 {
		t.Fatalf("help topic registry: %s", strings.Join(problems, "; "))
	}
	commands := NewRootCommand(io.Discard, io.Discard, "test")
	commands.InitDefaultHelpCmd()
	for _, command := range commands.Commands() {
		if !command.IsAvailableCommand() {
			continue
		}
		if _, ok := lookupHelpTopic(command.Name()); ok {
			t.Errorf("command %q collides with a help topic; commands must keep precedence", command.Name())
		}
		if command.Name() == helpTopicsMetaName {
			t.Errorf("command %q collides with the reserved help meta-target; command lookup would hide the topic catalog", command.Name())
		}
		for _, alias := range command.Aliases {
			if _, ok := lookupHelpTopic(alias); ok {
				t.Errorf("command alias %q collides with a help topic", alias)
			}
		}
	}
}

func TestIconsAndColorAcceptImplicitAutoAndExplicitModes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	implicitIcons, stderr, code := executeForTest(t, root, "--no-config", "--icons")
	if code != 0 || stderr != "" {
		t.Fatalf("implicit icons=(%q,%q,%d)", implicitIcons, stderr, code)
	}
	equalsIcons, stderr, code := executeForTest(t, root, "--no-config", "--icons=auto")
	if code != 0 || stderr != "" || equalsIcons != implicitIcons {
		t.Fatalf("icons=auto differs\nimplicit=%q\nequals=%q\nstderr=%q code=%d", implicitIcons, equalsIcons, stderr, code)
	}
	spacedIcons, stderr, code := executeForTest(t, root, "--no-config", "--icons", "auto")
	if code != 0 || stderr != "" || spacedIcons != implicitIcons {
		t.Fatalf("icons auto differs\nimplicit=%q\nspaced=%q\nstderr=%q code=%d", implicitIcons, spacedIcons, stderr, code)
	}

	implicitColor, stderr, code := executeForTest(t, root, "--no-config", "--color")
	if code != 0 || stderr != "" {
		t.Fatalf("implicit color=(%q,%q,%d)", implicitColor, stderr, code)
	}
	equalsColor, stderr, code := executeForTest(t, root, "--no-config", "--color=auto")
	if code != 0 || stderr != "" || equalsColor != implicitColor {
		t.Fatalf("color=auto differs\nimplicit=%q\nequals=%q\nstderr=%q code=%d", implicitColor, equalsColor, stderr, code)
	}
	spacedColor, stderr, code := executeForTest(t, root, "--no-config", "--color", "auto")
	if code != 0 || stderr != "" || spacedColor != implicitColor {
		t.Fatalf("color auto differs\nimplicit=%q\nspaced=%q\nstderr=%q code=%d", implicitColor, spacedColor, stderr, code)
	}

	for _, args := range [][]string{
		{root, "--no-config", "--icons", "never"},
		{root, "--no-config", "--icons", "ascii"},
		{root, "--no-config", "--icons", "unicode"},
		{root, "--no-config", "--icons", "nerd"},
		{root, "--no-config", "--icons", "auto"},
		{root, "--no-config", "--color", "never"},
		{root, "--no-config", "--color", "always"},
		{root, "--no-config", "--color", "auto"},
	} {
		stdout, stderr, code := executeForTest(t, args...)
		if code != 0 || stderr != "" || stdout == "" {
			t.Fatalf("%v=(%q,%q,%d)", args, stdout, stderr, code)
		}
	}

	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return "", errors.New("disabled") }))
	evaluator := presentation.NewEvaluator(
		presentation.WithEnvironment(func(string) (string, bool) { return "", false }),
		presentation.WithTerminalDetection(func(io.Writer) bool { return true }),
		presentation.WithANSIPreparation(func(io.Writer) (func() error, error) { return func() error { return nil }, nil }),
		presentation.WithWindowsTerminalCompatibility(false),
	)
	implicitTTY, stderr, code := executeForTestWithDependencies(t, loader, evaluator, root, "--no-config", "--theme", "vivid", "--icons")
	if code != 0 || stderr != "" {
		t.Fatalf("implicit TTY icons=(%q,%q,%d)", implicitTTY, stderr, code)
	}
	explicitTTY, stderr, code := executeForTestWithDependencies(t, loader, evaluator, root, "--no-config", "--theme", "vivid", "--icons=auto")
	if code != 0 || stderr != "" || explicitTTY != implicitTTY || !strings.Contains(implicitTTY, "•") {
		t.Fatalf("TTY auto icons differ\nimplicit=%q\nexplicit=%q\nstderr=%q code=%d", implicitTTY, explicitTTY, stderr, code)
	}
}

func TestOptionalValueFlagsDoNotConsumeHelpOrVersion(t *testing.T) {
	for _, args := range [][]string{{"--icons", "--help"}, {"--color", "--help"}} {
		stdout, stderr, code := executeForTest(t, args...)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage:") {
			t.Fatalf("%v=(%q,%q,%d)", args, stdout, stderr, code)
		}
		assertCanonicalHelp(t, stdout)
	}
	for _, args := range [][]string{{"--icons", "--version"}, {"--color", "--version"}} {
		stdout, stderr, code := executeForTest(t, args...)
		if code != 0 || stderr != "" || stdout != "dirloom v0.1.0-test\n" {
			t.Fatalf("%v=(%q,%q,%d)", args, stdout, stderr, code)
		}
	}

	stdout, stderr, code := executeForTest(t, "--theme", "--help")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
		t.Fatalf("theme must still require a value=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestInvalidEnumeratedFlagsAreActionable(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "--icons", "invalid")
	if code != 2 || stdout != "" {
		t.Fatalf("icons invalid=(%q,%q,%d)", stdout, stderr, code)
	}
	for _, want := range []string{`invalid value "invalid" for --icons`, "Valid values:", "auto", "ascii", "unicode", "nerd", "never", "dirloom help icons"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("icons diagnostic missing %q\n%s", want, stderr)
		}
	}

	stdout, stderr, code = executeForTest(t, "--icons", "banana")
	if code != 2 || stdout != "" || !strings.Contains(stderr, `invalid value "banana" for --icons`) || strings.Contains(stderr, "does not exist") {
		t.Fatalf("icons banana must stay an invalid mode=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "--color", "banana")
	if code != 2 || stdout != "" || !strings.Contains(stderr, `invalid value "banana" for --color`) {
		t.Fatalf("color banana must stay an invalid mode=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "--color", "invalid")
	if code != 2 || stdout != "" || !strings.Contains(stderr, `invalid value "invalid" for --color`) || !strings.Contains(stderr, "dirloom help colors") {
		t.Fatalf("color invalid=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "--format", "wat")
	if code != 2 || stdout != "" || !strings.Contains(stderr, `invalid value "wat" for --format`) || !strings.Contains(stderr, "dirloom help formats") {
		t.Fatalf("format invalid=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "--diagram-view", "structure")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "dirloom help diagrams") {
		t.Fatalf("diagram mismatch=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestHelpTopicsAreCanonicalAndDeterministic(t *testing.T) {
	names := append([]string{"topics", "concepts", "examples"}, helpTopicNames()...)
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		first, stderr, code := executeForTest(t, "help", name)
		if code != 0 || stderr != "" || first == "" || !strings.HasSuffix(first, "\n") || !utf8.ValidString(first) {
			t.Fatalf("help %s=(len=%d, stderr=%q, code=%d)", name, len(first), stderr, code)
		}
		assertCanonicalHelp(t, first)
		second, _, code := executeForTest(t, "help", name)
		if code != 0 || first != second {
			t.Fatalf("help %s is not deterministic", name)
		}
	}

	stdout, stderr, code := executeForTest(t, "help", "topics")
	if code != 0 || stderr != "" {
		t.Fatalf("topics=(%q,%q,%d)", stdout, stderr, code)
	}
	if strings.Count(stdout, "Help topics:") != 1 {
		t.Fatalf("topics header=(%q)", stdout)
	}
	previous := ""
	for _, name := range helpTopicNames() {
		if !strings.Contains(stdout, name) {
			t.Errorf("topics list missing %q\n%s", name, stdout)
		}
		if previous != "" && name < previous {
			t.Errorf("topic order %q follows %q", name, previous)
		}
		previous = name
	}
}

func TestHelpResolvesCommandsBeforeTopics(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "help", "theme")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Inspect and validate terminal themes") || strings.Contains(stdout, "ICON MODES") {
		t.Fatalf("help theme=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "help", "icons")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "ICON MODES") || !strings.Contains(stdout, "--icons without a value") {
		t.Fatalf("help icons=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "help", "config")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Inspect Dirloom configuration") {
		t.Fatalf("help config=(%q,%q,%d)", stdout, stderr, code)
	}

	root := NewRootCommand(io.Discard, io.Discard, "test")
	root.InitDefaultHelpCmd()
	for _, command := range root.Commands() {
		if !command.IsAvailableCommand() {
			continue
		}
		stdout, stderr, code = executeForTest(t, "help", command.Name())
		if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage:") {
			t.Fatalf("help %s must keep Cobra command help=(%q,%q,%d)", command.Name(), stdout, stderr, code)
		}
		if topic, ok := lookupHelpTopic(command.Name()); ok {
			t.Fatalf("command %q is shadowed by topic %q", command.Name(), topic.Name)
		}
		if command.Name() == helpTopicsMetaName {
			t.Fatalf("command %q would hide the reserved %s catalog", command.Name(), helpTopicsMetaName)
		}
	}
}

func TestHelpTopicsMetaDoesNotBypassCommandPrecedence(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRootCommand(&stdout, &stderr, "v0.1.0-test")
	root.AddCommand(&cobra.Command{
		Use:   helpTopicsMetaName,
		Short: "synthetic topics command",
		Run:   func(*cobra.Command, []string) {},
	})
	root.SetArgs([]string{"help", helpTopicsMetaName})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if stderr.Len() != 0 {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if !strings.Contains(out, "synthetic topics command") {
		t.Fatalf("command named %q must win command-first lookup:\n%s", helpTopicsMetaName, out)
	}
	if strings.Contains(out, "Help topics:") {
		t.Fatalf("topics meta-target must not run before command lookup:\n%s", out)
	}

	catalog, catalogErr, code := executeForTest(t, "help", "topics")
	if code != 0 || catalogErr != "" || !strings.Contains(catalog, "Help topics:") {
		t.Fatalf("public help topics catalog=(%q,%q,%d)", catalog, catalogErr, code)
	}
}

func TestUnknownHelpTargetsSuggestNearbyNames(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "help", "icon")
	if code != 2 || stdout != "" || !strings.Contains(stderr, `unknown help topic or command "icon"`) || !strings.Contains(stderr, "Did you mean:") || !strings.Contains(stderr, "icons") || !strings.Contains(stderr, "dirloom help topics") {
		t.Fatalf("icon=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "help", "formts")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "formats") {
		t.Fatalf("formts=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "help", "banana")
	if code != 2 || stdout != "" || strings.Contains(stderr, "Did you mean:") {
		t.Fatalf("banana should not suggest=(%q,%q,%d)", stdout, stderr, code)
	}
	if !strings.Contains(stderr, "dirloom help topics") {
		t.Fatalf("banana missing catalog hint=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestHelpRemainsPresentationNeutral(t *testing.T) {
	cases := [][]string{
		{"--help"},
		{"--theme", "vivid", "--icons", "nerd", "--color", "always", "--help"},
		{"help", "icons"},
		{"help", "topics"},
	}
	for _, args := range cases {
		stdout, stderr, code := executeForTest(t, args...)
		if code != 0 || stderr != "" || stdout == "" {
			t.Fatalf("%v=(%q,%q,%d)", args, stdout, stderr, code)
		}
		assertCanonicalHelp(t, stdout)
	}
}

func TestHelpDoesNotLoadConfiguration(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\ndefaults:\n  format: yaml\n")
	t.Chdir(root)
	for _, args := range [][]string{{"--help"}, {"help", "icons"}, {"help", "topics"}, {"help", "examples"}, {"help", "concepts"}} {
		stdout, stderr, code := executeForTest(t, args...)
		if code != 0 || stderr != "" || stdout == "" {
			t.Fatalf("%v=(%q,%q,%d)", args, stdout, stderr, code)
		}
	}

	invalid := filepath.Join(root, ".dirloom.yaml")
	stdout, stderr, code := executeForTest(t, "--config", invalid, "help", "icons")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "ICON MODES") {
		t.Fatalf("help with invalid config=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestHelpAndGuidanceExitCodes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		code int
	}{
		{[]string{"--help"}, 0},
		{[]string{"help", "icons"}, 0},
		{[]string{"help", "topics"}, 0},
		{[]string{root, "--no-config", "--icons"}, 0},
		{[]string{"--icons", "bad"}, 2},
		{[]string{"help", "unknown-topic"}, 2},
		{[]string{filepath.Join(root, "missing")}, 1},
	}
	for _, test := range cases {
		stdout, stderr, code := executeForTest(t, test.args...)
		if code != test.code {
			t.Fatalf("%v code=%d want=%d stdout=%q stderr=%q", test.args, code, test.code, stdout, stderr)
		}
		if test.code != 0 && stdout != "" {
			t.Fatalf("%v expected empty stdout, got %q", test.args, stdout)
		}
	}

	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return "", errors.New("disabled") }))
	var stderr bytes.Buffer
	code := executeWithLoader(context.Background(), []string{"help", "icons"}, failingWriter{}, &stderr, "v0.1.0-test", loader)
	if code != 1 || !strings.Contains(stderr.String(), "Error: ") {
		t.Fatalf("help write failure code=%d stderr=%q", code, stderr.String())
	}
}

func TestRootHelpMentionsTopicsAndOptionalIconValue(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=(%q,%q,%d)", stdout, stderr, code)
	}
	for _, want := range []string{"help topics", "--icons", "--color", "Usage:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("root help missing %q\n%s", want, stdout)
		}
	}
}

func TestOptionalAutoFlagsKeepIndependentFlagsAndTerminator(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := executeForTest(t, "--no-config", "--icons", "--", root)
	if code != 0 || stderr != "" || stdout == "" {
		t.Fatalf("terminator after icons=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "--", "--icons")
	if code == 2 || strings.Contains(stderr, "invalid value") {
		t.Fatalf("tokens after -- must not be parsed as flags=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, root, "--no-config", "--icons", "--depth", "0")
	if code != 0 || stderr != "" || !strings.Contains(stdout, filepath.Base(root)+"/") {
		t.Fatalf("icons then depth=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, root, "--no-config", "--color", "--icons", "never")
	if code != 0 || stderr != "" || stdout == "" {
		t.Fatalf("color then icons=(%q,%q,%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, root, "--no-config", "--icons", "--format", "json")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--icons auto cannot be used with --format json") {
		t.Fatalf("bare icons must not consume --format=(%q,%q,%d)", stdout, stderr, code)
	}

	for _, args := range [][]string{
		{root, "--no-config", "--icons=unicode"},
		{root, "--no-config", "--icons=ascii"},
		{root, "--no-config", "--icons=nerd"},
		{root, "--no-config", "--icons=never"},
		{root, "--no-config", "--icons=auto"},
		{root, "--no-config", "--color=always"},
		{root, "--no-config", "--color=never"},
		{root, "--no-config", "--color=auto"},
	} {
		stdout, stderr, code = executeForTest(t, args...)
		if code != 0 || stderr != "" || stdout == "" {
			t.Fatalf("%v=(%q,%q,%d)", args, stdout, stderr, code)
		}
	}
}

func TestOptionalHelpTopicsStayCompactIndexes(t *testing.T) {
	examples, ok := lookupHelpTopic("examples")
	if !ok {
		t.Fatal("missing examples topic")
	}
	if strings.Count(examples.Body, "dirloom") < 5 || strings.Contains(examples.Body, "ICON MODES") {
		t.Fatalf("examples should stay a recipe list:\n%s", examples.Body)
	}
	if strings.Count(examples.Body, "\n") > 40 {
		t.Fatalf("examples topic is too long:\n%s", examples.Body)
	}

	concepts, ok := lookupHelpTopic("concepts")
	if !ok {
		t.Fatal("missing concepts topic")
	}
	if !strings.Contains(concepts.Body, "CONCEPT INDEX") || strings.Contains(concepts.Body, "COLOR MODES") || strings.Contains(concepts.Body, "ICON MODES") {
		t.Fatalf("concepts should stay an index:\n%s", concepts.Body)
	}
	if strings.Count(concepts.Body, "\n") > 40 {
		t.Fatalf("concepts topic is too long:\n%s", concepts.Body)
	}
}

func assertCanonicalHelp(t *testing.T, text string) {
	t.Helper()
	if containsANSI(text) {
		t.Fatalf("help contains ANSI: %q", text)
	}
	if containsNerdGlyph(text) {
		t.Fatalf("help contains Nerd Font glyphs")
	}
}
