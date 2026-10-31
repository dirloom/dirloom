package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestPublicClipboardDocumentationMatchesCLI(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "clipboard-and-completions.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"dirloom --format markdown --copy",
		"dirloom --format json --copy",
		"dirloom completion bash",
		"dirloom completion powershell",
		"mutually exclusive",
		"no ANSI",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("clipboard guide missing %q", want)
		}
	}
	pattern := regexp.MustCompile(`(?s)<!-- dirloom-clipboard-command:([a-z-]+) -->\r?\n` + "```(?:bash)?\r?\n(.*?)\r?\n" + "```")
	matches := pattern.FindAllSubmatch(data, -1)
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, string(match[1]))
		if !strings.Contains(string(match[2]), "dirloom") {
			t.Errorf("clipboard command %q does not invoke Dirloom", match[1])
		}
	}
	wantIDs := []string{"completion-bash", "json", "markdown", "text"}
	sort.Strings(ids)
	if strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("clipboard command IDs = %#v, want %#v", ids, wantIDs)
	}
}

func TestPublicDistributionDocumentationMatchesReleaseContract(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "distribution.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"winget install Dirloom.Dirloom",
		"brew install --cask dirloom/tap/dirloom",
		"scoop install dirloom",
		"13 release artifacts",
		"exactly 12 hash",
		"Release Done",
		"Distribution Verified",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("distribution guide missing %q", want)
		}
	}
}

func TestREADMEAndUseCasesExposeCopyAndCompletions(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readmeText := string(readme)
	for _, want := range []string{
		"--copy",
		"dirloom completion bash",
		"docs/clipboard-and-completions.md",
		"docs/distribution.md",
		"winget install Dirloom.Dirloom",
		"brew install --cask dirloom/tap/dirloom",
	} {
		if !strings.Contains(readmeText, want) {
			t.Errorf("README missing %q", want)
		}
	}
	useCases, err := os.ReadFile(filepath.Join("..", "..", "docs", "use-cases.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(useCases), "| `--copy` natif |") {
		t.Fatal("use cases still list native --copy as unavailable")
	}
	if !strings.Contains(string(useCases), "dirloom --format markdown --copy") {
		t.Fatal("use cases missing native Markdown copy")
	}
}

func TestPublicHelpDocumentationMatchesCLIContracts(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile(filepath.Join("..", "..", "docs", "contextual-help.md"))
	if err != nil {
		t.Fatal(err)
	}
	themes, err := os.ReadFile(filepath.Join("..", "..", "docs", "themes.md"))
	if err != nil {
		t.Fatal(err)
	}
	completions, err := os.ReadFile(filepath.Join("..", "..", "docs", "clipboard-and-completions.md"))
	if err != nil {
		t.Fatal(err)
	}
	readmeText := string(readme)
	guideText := string(guide)
	themeText := string(themes)
	completionText := string(completions)
	for _, want := range []string{
		"dirloom help icons",
		"dirloom help topics",
		"docs/contextual-help.md",
		"dirloom --icons",
	} {
		if !strings.Contains(readmeText, want) {
			t.Errorf("README missing %q", want)
		}
	}
	for _, want := range []string{
		"dirloom --help",
		"dirloom help <topic>",
		"--icons",
		"--color",
		"never",
		"ascii",
		"unicode",
		"nerd",
		"auto",
		"always",
		"help vs explain",
	} {
		if !strings.Contains(guideText, want) {
			t.Errorf("contextual help guide missing %q", want)
		}
	}
	for _, mode := range []string{"never", "ascii", "unicode", "nerd", "auto"} {
		if !strings.Contains(themeText, mode) {
			t.Errorf("themes guide missing icon mode %q", mode)
		}
	}
	for _, mode := range []string{"never", "always", "auto"} {
		if !strings.Contains(themeText, mode) {
			t.Errorf("themes guide missing color mode %q", mode)
		}
	}
	if !strings.Contains(themeText, "dirloom help icons") || !strings.Contains(themeText, "without a value is equivalent") {
		t.Fatal("themes guide missing implicit --icons documentation")
	}
	if !strings.Contains(completionText, "dirloom help") || !strings.Contains(completionText, "help topics") {
		t.Fatal("completion guide missing contextual help completions")
	}
	for _, name := range helpTopicNames() {
		if !strings.Contains(guideText, name) {
			t.Errorf("contextual help guide missing topic %q", name)
		}
	}
}

func TestReleaseWorkflowDocumentsPre1Versioning(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "release-workflow.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"Pre-1.0 versioning policy",
		"Release branch integration",
		"0.Y.0",
		"0.Y.Z",
		"v0.3.0",
		"v0.3.1",
		"v0.3.2",
		"v0.3.3",
		"v0.4.0",
		"v0.4.0-a1",
		"PRESENTATION",
		"CHANGE",
		"release/vX.Y.Z",
		"PR → release/vX.Y.Z",
		"PR finale → main",
		"Do not open versioned feature or fix pull requests against `main`.",
		"scope freeze",
		"Do not create `chore/v0.3.2-freeze`.",
		"[0.3.2] - 2026-09-18",
		"Do not create `chore/v0.3.3-freeze`.",
		"[0.3.3] - 2026-09-18",
		"Do not create `chore/v0.4.0-a1-freeze`.",
		"[0.4.0-a1] - 2026-09-20",
		"Do not create `chore/v0.4.0-a2-freeze`.",
		"[0.4.0-a2] - 2026-09-24",
		"prerelease / CLI package",
		"release/v0.4.0-a2",
		"release/v0.4.0-a4",
		"feat/v0.4.0-a4-structural-diff",
		"Optional extended terminal matrix",
		"Release blocker: NO",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("release workflow missing %q", want)
		}
	}
	roadmap, err := os.ReadFile(filepath.Join("..", "..", "docs", "product", "roadmap.md"))
	if err != nil {
		t.Fatal(err)
	}
	roadmapText := string(roadmap)
	if !strings.Contains(roadmapText, "v0.3.1") || !strings.Contains(roadmapText, "CLI GUIDANCE") {
		t.Fatal("roadmap missing v0.3.1 CLI guidance refinement")
	}
	if strings.Contains(roadmapText, "v0.4.0-beta") || strings.Contains(roadmapText, "v0.4.0-b") {
		t.Fatal("roadmap must not present contextual help as a v0.4.0 beta")
	}
}

func TestVerifyDocumentationDiscoverable(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	reference, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "verify.md"))
	if err != nil {
		t.Fatal(err)
	}
	contract, err := os.ReadFile(filepath.Join("..", "..", "docs", "contracts", "verify-result-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	adr, err := os.ReadFile(filepath.Join("..", "..", "docs", "adr", "0003-snapshot-verification.md"))
	if err != nil {
		t.Fatal(err)
	}
	architecture, err := os.ReadFile(filepath.Join("..", "..", "docs", "architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "dirloom verify") || !strings.Contains(string(readme), "docs/reference/verify.md") {
		t.Fatal("README missing verify discoverability")
	}
	for _, want := range []string{"Capture Semantics", "exit", "MATCH", "MISMATCH", "dirloom diff", "PowerShell", "GitLab"} {
		if !strings.Contains(string(reference), want) {
			t.Errorf("verify reference missing %q", want)
		}
	}
	for _, want := range []string{"schemaVersion", "INVALID_SNAPSHOT", "snapshot_fingerprint_mismatch", "verify_observation_failure"} {
		if !strings.Contains(string(contract), want) {
			t.Errorf("verify contract missing %q", want)
		}
	}
	if !strings.Contains(string(adr), "Fingerprint v1") || !strings.Contains(string(architecture), "dirloom verify") {
		t.Fatal("architecture decision or architecture doc missing verify")
	}
}

func TestDiffDocumentationDiscoverable(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	reference, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "diff.md"))
	if err != nil {
		t.Fatal(err)
	}
	contract, err := os.ReadFile(filepath.Join("..", "..", "docs", "contracts", "diff-result-v3.md"))
	if err != nil {
		t.Fatal(err)
	}
	gitContract, err := os.ReadFile(filepath.Join("..", "..", "docs", "contracts", "git-tree-source-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	gitADR, err := os.ReadFile(filepath.Join("..", "..", "docs", "adr", "0006-git-sources.md"))
	if err != nil {
		t.Fatal(err)
	}
	moves, err := os.ReadFile(filepath.Join("..", "..", "docs", "contracts", "move-detection-structural-v1.md"))
	if err != nil {
		t.Fatal(err)
	}
	adr, err := os.ReadFile(filepath.Join("..", "..", "docs", "adr", "0004-structural-diff.md"))
	if err != nil {
		t.Fatal(err)
	}
	moveADR, err := os.ReadFile(filepath.Join("..", "..", "docs", "adr", "0005-move-detection.md"))
	if err != nil {
		t.Fatal(err)
	}
	architecture, err := os.ReadFile(filepath.Join("..", "..", "docs", "architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"dirloom diff", "docs/reference/diff.md", "--no-moves", "Diff Result Schema v3", "git:<revision>", "Git 2.45"} {
		if !strings.Contains(string(readme), want) {
			t.Errorf("README missing %q", want)
		}
	}
	for _, want := range []string{"snapshot:<path>", "live:<directory>", "Capture Semantics", "ADDED", "REMOVED", "CHANGED", "MOVED", "--no-moves", "structural-v1", "not proof", "No structural differences.", "PowerShell", "GitLab", "diff-result-v2.md", "diff-result-v3.md",
		"git:<revision>", "## Git sources", "Not `git diff`", "git_revision_not_found", "git_tree_invalid", "2.45", "index entry", "fetch-depth", "git-tree-source-v1.md"} {
		if !strings.Contains(string(reference), want) {
			t.Errorf("diff reference missing %q", want)
		}
	}
	for _, want := range []string{"schemaVersion", "NO_DIFFERENCES", "DIFFERENCES", "INVALID_SNAPSHOT", "UNSUPPORTED_SNAPSHOT", "SNAPSHOT_READ_ERROR", "OBSERVATION_ERROR", "INTERNAL_ERROR", `"operation"`, "`sources`", "`moveDetection`", "`MOVED`", "--no-moves",
		"schemaVersion: 3", "`tree`", "`commit`", "git_object_unavailable", "## Evolution policy"} {
		if !strings.Contains(string(contract), want) {
			t.Errorf("diff contract missing %q", want)
		}
	}
	for _, want := range []string{"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "protocol.allow=never", "index entry", "## Equivalence with a checkout", "## Complexity", "32 MiB", "git_unavailable"} {
		if !strings.Contains(string(gitContract), want) {
			t.Errorf("Git Tree Source contract missing %q", want)
		}
	}
	if !strings.Contains(string(gitADR), "Diff Result Schema v3") || !strings.Contains(string(gitADR), "go-git") {
		t.Error("ADR 0006 missing the schema or the library decision")
	}
	for _, want := range []string{"R1", "R2", "not proof of a move", "Not detected", "## Complexity"} {
		if !strings.Contains(string(moves), want) {
			t.Errorf("move detection contract missing %q", want)
		}
	}
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "diff", "v3", "golden", "snapshot-read-error.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(golden), `"status": "SNAPSHOT_READ_ERROR"`) {
		t.Fatal("snapshot-read-error golden does not encode SNAPSHOT_READ_ERROR")
	}
	if !strings.Contains(string(adr), "Identity Projection v1") || !strings.Contains(string(moveADR), "structural-v1") {
		t.Fatal("architecture decisions missing diff or move detection")
	}
	if !strings.Contains(string(architecture), "Comparison Engine") || !strings.Contains(string(architecture), "Move Detector") {
		t.Fatal("architecture doc missing diff or move detection")
	}
	for _, want := range []string{"internal/gitsource", "Git backend", "Git Tree Source v1", "v0.4-a6 benchmarks"} {
		if !strings.Contains(string(architecture), want) {
			t.Errorf("architecture doc missing %q", want)
		}
	}
}

func TestHistoryDocumentationDiscoverable(t *testing.T) {
	read := func(parts ...string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return strings.ReplaceAll(string(data), "\r\n", "\n")
	}
	readme := read("README.md")
	reference := read("docs", "reference", "history.md")
	contract := read("docs", "contracts", "structural-history-v1.md")
	schema := read("docs", "contracts", "history-result-v1.md")
	adr := read("docs", "adr", "0007-structural-history.md")
	architecture := read("docs", "architecture.md")
	changelog := read("CHANGELOG.md")
	for _, want := range []string{"dirloom history", "docs/reference/history.md", "History Result Schema v1", "--structural-only", "dirloom ./history"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README missing %q", want)
		}
	}
	for _, want := range []string{"## The walk", "first parent", "--from", "--limit", "--structural-only", "--no-moves", "Not `git log`", "### Shallow clones", "end: shallow",
		"git_commit_invalid", "git_shallow_invalid", "history_from_not_reached", "dirloom ./history", "No commits to show.", "There is no exit 1", "merge-base",
		"PowerShell", "GitLab", "structural-history-v1.md", "history-result-v1.md", "v0.4-a7.md"} {
		if !strings.Contains(reference, want) {
			t.Errorf("history reference missing %q", want)
		}
	}
	// The reference shows the text golden verbatim, so the example cannot drift.
	if golden := read("testdata", "history", "v1", "golden", "history.txt"); !strings.Contains(reference, golden) {
		t.Error("history reference does not show the history.txt golden verbatim")
	}
	for _, want := range []string{"## Walk", "## Shallow boundary list", "## Commit objects", "## Complexity", "## Reference outcomes", "GIT_NO_REPLACE_OBJECTS=1", "253402300799", "16 MiB"} {
		if !strings.Contains(contract, want) {
			t.Errorf("Structural History contract missing %q", want)
		}
	}
	for _, want := range []string{"schemaVersion: 1", "first-parent", "## Transactional output", "## Evolution policy", "history_observation_failure"} {
		if !strings.Contains(schema, want) {
			t.Errorf("History Result Schema missing %q", want)
		}
	}
	if !strings.Contains(adr, "History Result Schema v1") || !strings.Contains(adr, "No persistent cache") {
		t.Error("ADR 0007 missing the schema or the cache decision")
	}
	for _, want := range []string{"internal/history", "Structural History v1", "v0.4-a7 benchmarks"} {
		if !strings.Contains(architecture, want) {
			t.Errorf("architecture doc missing %q", want)
		}
	}
	if !strings.Contains(changelog, "dirloom history") || !strings.Contains(changelog, "dirloom ./history") {
		t.Error("CHANGELOG missing the history command or the directory name change")
	}
}
