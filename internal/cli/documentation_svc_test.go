package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// readRepositoryDocument reads a file of the repository with LF line endings.
func readRepositoryDocument(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func TestStructuralVersionControlDocumentationDiscoverable(t *testing.T) {
	readme := readRepositoryDocument(t, "README.md")
	for _, want := range []string{"docs/structural-version-control.md", "docs/ci-integration.md", "docs/reference/exit-codes.md",
		"docs/contracts/README.md", "docs/compatibility-policy.md", "docs/upgrading-to-v0.4.md", "SECURITY.md#verifying-a-release"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README missing %q", want)
		}
	}
	overview := readRepositoryDocument(t, "docs", "structural-version-control.md")
	for _, want := range []string{"reference/exit-codes.md", "contracts/README.md", "compatibility-policy.md", "ci-integration.md",
		"upgrading-to-v0.4.md", "../SECURITY.md", "candidates for the v0.4 freeze", "**experimental**", "File contents are never read.",
		"reference/memory.md", "not a guaranteed ceiling"} {
		if !strings.Contains(overview, want) {
			t.Errorf("Structural Version Control overview missing %q", want)
		}
	}
	// The register indexes every machine output by the command that writes it.
	register := readRepositoryDocument(t, "docs", "contracts", "README.md")
	for _, want := range []string{"## Machine outputs by command", "## JSON Schema files (D12)",
		"`dirloom [directory] --format json`", "tree-json-v1.md",
		"`dirloom fingerprint --format json`", "fingerprint-json-v1.md",
		"snapshot-schema-v1.md",
		"`dirloom verify --format json`", "verify-result-v1.md",
		"`dirloom diff --format json`", "diff-result-v3.md",
		"`dirloom history --format json`", "history-result-v1.md",
		"`dirloom watch --format ndjson`", "watch-event-v1.md"} {
		if !strings.Contains(register, want) {
			t.Errorf("contract register missing %q", want)
		}
	}
	// D10: the trust chain is never presented as a signature of the binaries.
	security := readRepositoryDocument(t, "SECURITY.md")
	for _, want := range []string{"## Release trust model", "### Verifying a release", "not an independent cryptographic signature",
		"checksums.txt", ".spdx.json", "gh attestation verify", "--signer-workflow dirloom/dirloom/.github/workflows/release.yml",
		"makes no network request", "protocol.allow=never", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "256 MiB", "2,000,000"} {
		if !strings.Contains(security, want) {
			t.Errorf("SECURITY.md missing %q", want)
		}
	}
	upgrade := readRepositoryDocument(t, "docs", "upgrading-to-v0.4.md")
	for _, want := range []string{"`v0.3.3`", "### Windows junctions", "unsupported_node_type", "### `.gitignore` files", "64 KiB",
		"`dirloom ./diff`", "reference/exit-codes.md", "../README.md#build-from-source", "## Memory", "reference/memory.md"} {
		if !strings.Contains(upgrade, want) {
			t.Errorf("upgrade note missing %q", want)
		}
	}
}

// The memory guidance must keep the cautions that the CTO review of
// 2026-11-28 asked for: what the figures are and what they are not.
func TestMemoryGuidanceKeepsItsCautions(t *testing.T) {
	memory := readRepositoryDocument(t, "docs", "reference", "memory.md")
	for _, want := range []string{
		"../benchmarks/v0.4-rc1.md",
		"**4.1 GiB is the maximum observed, not a guaranteed ceiling.**",
		"**The 8 GiB of the measurements is a budget, not a machine.**",
		"soft limit of the Go runtime, not a hard cap on the process",
		"`GOMEMLIMIT=1792MiB` let both comparisons of two snapshots that were measured pass 5 out of 5",
		"not as a universal setting",
		"a starting point for constrained environments",
		"`fatal error: out of memory`",
		"the process can hang",
	} {
		if !strings.Contains(memory, want) {
			t.Errorf("docs/reference/memory.md missing %q", want)
		}
	}
	security := readRepositoryDocument(t, "SECURITY.md")
	if !strings.Contains(security, "docs/reference/memory.md") {
		t.Error("SECURITY.md must link the memory guidance from its untrusted-input limits")
	}
}

// The commands that the README and the user guides show must exist, with
// every flag they pass and every help topic they name, so that the examples
// cannot drift from the CLI. The guides' examples were also run by hand; this
// test keeps them honest when a command, a flag or a topic changes.
func TestDocumentedDirloomCommandsExist(t *testing.T) {
	root := NewRootCommand(io.Discard, io.Discard, "test")
	for _, document := range [][]string{
		{"README.md"},
		{"docs", "use-cases.md"},
		{"docs", "structural-version-control.md"},
		{"docs", "ci-integration.md"},
		{"docs", "upgrading-to-v0.4.md"},
		{"docs", "reference", "memory.md"},
		{"SECURITY.md"},
	} {
		name := filepath.Join(document...)
		lines := dirloomCommandLines(readRepositoryDocument(t, document...))
		if name != "SECURITY.md" && len(lines) == 0 {
			t.Errorf("%s: no dirloom command found", name)
		}
		for _, line := range lines {
			if err := checkDirloomCommandLine(root, line); err != "" {
				t.Errorf("%s: %q: %s", name, line, err)
			}
		}
	}
}

// The checker must catch each kind of mistake it is meant to catch, and
// accept the forms the documentation legitimately uses.
func TestDocumentedCommandCheckerCatchesMistakes(t *testing.T) {
	root := NewRootCommand(io.Discard, io.Discard, "test")
	for _, line := range []string{
		"dirloom verfy architecture.dlm.json --format json", // misspelled command
		"dirloom snapshott --output architecture.dlm.json",  // misspelled command, valid root flag
		"dirloom help structurel",                           //nolint:misspell // an unknown help topic, misspelled on purpose
		"dirloom help structural sources",                   // help takes one topic
		"dirloom verify architecture.dlm.json --depth 3",    // refused flag
		"dirloom diff snapshot:a.json live:. --nope",        // unknown flag
		"dirloom fingerprint -q",                            // unknown shorthand
		"dirloom snapshot -z2",                              // unknown joined shorthand
	} {
		if checkDirloomCommandLine(root, line) == "" {
			t.Errorf("%q passed the documentation check", line)
		}
	}
	for _, line := range []string{
		"dirloom snapshot -d2 --output architecture.dlm.json",
		"dirloom --depth 3 ./src",
		"dirloom --icons unicode --theme midnight testdata/showcase/infra-terraform-k8s",
		"dirloom --theme vivid --icons",
		"dirloom --version",
		"dirloom help structural",
		"dirloom help topics",
		"dirloom help verify",
		"dirloom ./diff",
		"dirloom . --format json --output structure.json",
		"dirloom node_modules",
		"dirloom [directory] [flags]",
		`dirloom diff "git:origin/$BASE_REF" git:HEAD > "$RUNNER_TEMP/structure-diff.txt" || status=$?`,
	} {
		if problem := checkDirloomCommandLine(root, line); problem != "" {
			t.Errorf("%q: %s", line, problem)
		}
	}
	// Prose inside a code block is not a command.
	block := "```bash\necho \"Update it with dirloom snapshot if the change is intended.\"\n  dirloom verify a.json || status=$?\n```\n"
	if lines := dirloomCommandLines(block); len(lines) != 1 || lines[0] != "dirloom verify a.json || status=$?" {
		t.Errorf("command lines of a block with an echo = %q", lines)
	}
	// Continued lines are one command, and a comment ends it.
	block = "```powershell\ndirloom `\n  --ignore dist `\n  --depth 2   # shallow\n```\n"
	lines := dirloomCommandLines(block)
	if len(lines) != 1 || lines[0] != "dirloom --ignore dist --depth 2   # shallow" {
		t.Fatalf("command lines of a continued block = %q", lines)
	}
	if problem := checkDirloomCommandLine(root, lines[0]); problem != "" {
		t.Errorf("%q: %s", lines[0], problem)
	}
	lines = dirloomCommandLines("```powershell\ndirloom `\n  --ignor dist\n```\n")
	if len(lines) != 1 || checkDirloomCommandLine(root, lines[0]) == "" {
		t.Errorf("a misspelled flag on a continued line passed the documentation check: %q", lines)
	}
}

func TestCIIntegrationWorkflowShowsEveryDocumentedStep(t *testing.T) {
	guide := readRepositoryDocument(t, "docs", "ci-integration.md")
	workflow := regexp.MustCompile("(?ms)^````yaml\n(.*?)^````$").FindStringSubmatch(guide)
	if workflow == nil {
		t.Fatal("CI integration guide has no ````yaml workflow block")
	}
	for _, want := range []string{
		"fetch-depth: 0",
		"shell: bash",
		"grep \"  $asset\\$\" checksums.txt | sha256sum -c -",
		"gh attestation verify \"$asset\" --repo dirloom/dirloom",
		"dirloom diff \"git:origin/$BASE_REF\" git:HEAD > \"$RUNNER_TEMP/structure-diff.txt\" || status=$?",
		"dirloom history --limit 20 --structural-only",
		"dirloom verify architecture.dlm.json || status=$?",
		"dirloom diff snapshot:architecture.dlm.json live:. || [ $? -eq 1 ]",
	} {
		if !strings.Contains(workflow[1], want) {
			t.Errorf("CI workflow missing %q", want)
		}
	}
	// Every Action is pinned to a full commit SHA, as in this repository.
	for _, uses := range regexp.MustCompile(`uses:\s*(\S+)`).FindAllStringSubmatch(workflow[1], -1) {
		if !regexp.MustCompile(`@[0-9a-f]{40}$`).MatchString(uses[1]) {
			t.Errorf("CI workflow action %q is not pinned to a commit SHA", uses[1])
		}
	}
}

var (
	codeBlockPattern  = regexp.MustCompile("(?ms)^[ \t]*(`{3,})[a-z]*\n(.*?)^[ \t]*`{3,}[ \t]*$")
	inlineCodePattern = regexp.MustCompile("`(dirloom [^`]+)`")
	// A code-block line runs dirloom only when "dirloom" is in command
	// position: at the start of the line, after a "$ " prompt, or after a
	// shell separator. "echo ... dirloom snapshot ..." is prose, not a run.
	commandPattern = regexp.MustCompile(`(?:^\s*(?:\$\s+)?|[;|&({]\s*|\b(?:then|do)\s+)dirloom\s+(.*)$`)
)

// dirloomCommandLines returns the dirloom invocations of a Markdown document:
// lines of fenced code blocks and inline code spans that run "dirloom".
func dirloomCommandLines(text string) []string {
	var lines []string
	for _, block := range codeBlockPattern.FindAllStringSubmatch(text, -1) {
		// Join continued lines: a trailing "\" in a POSIX shell, "`" in PowerShell.
		var joined []string
		continued := false
		for _, line := range strings.Split(block[2], "\n") {
			if continued {
				joined[len(joined)-1] += " " + strings.TrimSpace(line)
			} else {
				joined = append(joined, line)
			}
			last := &joined[len(joined)-1]
			trimmed := strings.TrimRight(*last, " \t")
			continued = strings.HasSuffix(trimmed, " \\") || strings.HasSuffix(trimmed, " `") || trimmed == "\\" || trimmed == "`"
			if continued {
				*last = strings.TrimRight(trimmed[:len(trimmed)-1], " \t")
			}
		}
		for _, line := range joined {
			if match := commandPattern.FindStringSubmatch(line); match != nil {
				lines = append(lines, "dirloom "+match[1])
			}
		}
	}
	prose := codeBlockPattern.ReplaceAllString(text, "")
	for _, match := range inlineCodePattern.FindAllStringSubmatch(prose, -1) {
		lines = append(lines, match[1])
	}
	return lines
}

// documentedRootOperands are the bare words that the documentation puts
// where a command name could stand, on purpose; any other bare word there is
// taken for a misspelled command.
var documentedRootOperands = map[string]bool{
	"node_modules": true, // README: an explicitly selected root is never excluded
	"packages":     true, // use cases: the packages directory of a monorepo
	"browse":       true, // README roadmap: a future command, named as such
}

// checkDirloomCommandLine resolves the command of a "dirloom ..." line and
// reports the first problem: a word that names no command, an unknown help
// topic, or a flag that the command does not accept, or accepts only to
// refuse it.
func checkDirloomCommandLine(root *cobra.Command, line string) string {
	var args []string
	for _, field := range strings.Fields(strings.TrimPrefix(line, "dirloom ")) {
		if field == "|" || field == "||" || field == "&&" || field == ";" || field == "\\" ||
			strings.HasPrefix(field, ">") || strings.HasPrefix(field, "2>") || strings.HasPrefix(field, ")") ||
			strings.HasPrefix(field, "#") {
			break
		}
		args = append(args, strings.Trim(field, `"'`))
	}
	// The CLI rewrites a bare --icons or --color before parsing; so does this check.
	args = normalizeOptionalAutoFlags(args)
	// Cobra adds the help command only when it executes.
	root.InitDefaultHelpCmd()
	command, _, err := root.Find(args)
	if err != nil {
		return err.Error()
	}
	command.InitDefaultHelpFlag()
	command.InitDefaultVersionFlag()
	// Words of the command path come first; the rest are operands and flags.
	depth := 0
	for c := command; c != root; c = c.Parent() {
		depth++
	}
	var operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--"):
			name, _, hasValue := strings.Cut(arg[2:], "=")
			found, hidden, takesValue := lookupFlag(command, name, false)
			if problem := flagProblem(command, "--"+name, found, hidden); problem != "" {
				return problem
			}
			if takesValue && !hasValue {
				i++
			}
		case len(arg) > 1 && arg[0] == '-' && arg[1] != '-':
			// Short flags may be joined: "-d2" is -d with the value 2.
			for j := 1; j < len(arg); j++ {
				found, hidden, takesValue := lookupFlag(command, arg[j:j+1], true)
				if problem := flagProblem(command, "-"+arg[j:j+1], found, hidden); problem != "" {
					return problem
				}
				if takesValue {
					if j == len(arg)-1 {
						i++
					}
					break
				}
			}
		case depth > 0:
			depth--
		default:
			operands = append(operands, arg)
		}
	}
	if len(operands) == 0 || isPlaceholder(operands[0]) {
		return ""
	}
	switch {
	case command == root:
		if !isPathOperand(operands[0]) && !documentedRootOperands[operands[0]] {
			return fmt.Sprintf("%q is neither a command nor a path", operands[0])
		}
	case command.Name() == "help":
		// The resolution of runHelp: a command, the topic list, or a topic.
		if target, _, err := root.Find(operands); err == nil && target != root {
			return ""
		}
		if len(operands) == 1 && operands[0] == helpTopicsMetaName {
			return ""
		}
		if _, ok := lookupHelpTopic(operands[0]); len(operands) != 1 || !ok {
			return fmt.Sprintf("unknown help topic or command %q", strings.Join(operands, " "))
		}
	}
	return ""
}

func flagProblem(command *cobra.Command, flag string, found, hidden bool) string {
	switch {
	case !found:
		return "unknown flag " + flag + " for " + command.CommandPath()
	case hidden:
		return flag + " is refused by " + command.CommandPath()
	}
	return ""
}

// isPlaceholder reports a documentation placeholder such as [directory],
// <snapshot>, … or a shell variable.
func isPlaceholder(word string) bool {
	return strings.HasPrefix(word, "[") || strings.HasPrefix(word, "<") || strings.HasPrefix(word, "$") || word == "…"
}

// isPathOperand reports a word written as a path: ".", "./src", "a/b",
// "C:\x", "~/x", or a file name with an extension such as "README.md".
func isPathOperand(word string) bool {
	return strings.HasPrefix(word, "~") || strings.ContainsAny(word, `./\`)
}

// lookupFlag reports whether command accepts the flag, locally or inherited,
// whether it hides it (a hidden flag is one the command refuses), and whether
// it takes a value from the next word.
func lookupFlag(command *cobra.Command, name string, shorthand bool) (found, hidden, takesValue bool) {
	for _, flags := range []func() (bool, bool, bool){
		func() (bool, bool, bool) {
			flag := command.Flags().Lookup(name)
			if shorthand {
				flag = command.Flags().ShorthandLookup(name)
			}
			if flag == nil {
				return false, false, false
			}
			return true, flag.Hidden, flag.NoOptDefVal == ""
		},
		func() (bool, bool, bool) {
			flag := command.InheritedFlags().Lookup(name)
			if shorthand {
				flag = command.InheritedFlags().ShorthandLookup(name)
			}
			if flag == nil {
				return false, false, false
			}
			return true, flag.Hidden, flag.NoOptDefVal == ""
		},
	} {
		if found, hidden, takesValue := flags(); found {
			return true, hidden, takesValue
		}
	}
	return false, false, false
}
