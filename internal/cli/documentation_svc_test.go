package cli

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		"upgrading-to-v0.4.md", "../SECURITY.md", "candidates for the v0.4 freeze", "**experimental**", "File contents are never read."} {
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
		"`dirloom ./diff`", "reference/exit-codes.md", "../README.md#build-from-source"} {
		if !strings.Contains(upgrade, want) {
			t.Errorf("upgrade note missing %q", want)
		}
	}
}

// The commands that the Structural Version Control guides show must exist,
// with every flag they pass, so that the examples cannot drift from the CLI.
// The guides' examples were also run by hand; this test keeps them honest
// when a command or a flag changes.
func TestStructuralVersionControlGuidesUseExistingCommandsAndFlags(t *testing.T) {
	root := NewRootCommand(io.Discard, io.Discard, "test")
	for _, document := range [][]string{
		{"docs", "structural-version-control.md"},
		{"docs", "ci-integration.md"},
		{"docs", "upgrading-to-v0.4.md"},
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
	codeBlockPattern  = regexp.MustCompile("(?ms)^(```+|````+)[a-z]*\n(.*?)^```+$")
	inlineCodePattern = regexp.MustCompile("`(dirloom [^`]+)`")
	commandPattern    = regexp.MustCompile(`(?:^|[\s;(|&]|\$ )dirloom\s+(.*)$`)
)

// dirloomCommandLines returns the dirloom invocations of a Markdown document:
// lines of fenced code blocks and inline code spans that run "dirloom".
func dirloomCommandLines(text string) []string {
	var lines []string
	for _, block := range codeBlockPattern.FindAllStringSubmatch(text, -1) {
		for _, line := range strings.Split(block[2], "\n") {
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

// checkDirloomCommandLine resolves the command of a "dirloom ..." line and
// reports the first flag that the command does not accept, or accepts only
// to refuse it.
func checkDirloomCommandLine(root *cobra.Command, line string) string {
	var args []string
	for _, field := range strings.Fields(strings.TrimPrefix(line, "dirloom ")) {
		if field == "|" || field == "||" || field == "&&" || field == ";" || field == "\\" ||
			strings.HasPrefix(field, ">") || strings.HasPrefix(field, "2>") || strings.HasPrefix(field, ")") {
			break
		}
		args = append(args, strings.Trim(field, `"'`))
	}
	command, _, err := root.Find(args)
	if err != nil {
		return err.Error()
	}
	command.InitDefaultHelpFlag()
	command.InitDefaultVersionFlag()
	for _, arg := range args {
		var flag *pflag.Flag
		switch {
		case strings.HasPrefix(arg, "--"):
			name, _, _ := strings.Cut(arg[2:], "=")
			flag = lookupFlag(command, name)
			if flag == nil {
				return "unknown flag --" + name + " for " + command.CommandPath()
			}
		case len(arg) == 2 && arg[0] == '-' && arg[1] != '-':
			flag = command.Flags().ShorthandLookup(arg[1:])
			if flag == nil {
				flag = command.InheritedFlags().ShorthandLookup(arg[1:])
			}
			if flag == nil {
				return "unknown shorthand " + arg + " for " + command.CommandPath()
			}
		default:
			continue
		}
		if flag.Hidden {
			return "--" + flag.Name + " is refused by " + command.CommandPath()
		}
	}
	return ""
}

func lookupFlag(command *cobra.Command, name string) *pflag.Flag {
	if flag := command.Flags().Lookup(name); flag != nil {
		return flag
	}
	return command.InheritedFlags().Lookup(name)
}
