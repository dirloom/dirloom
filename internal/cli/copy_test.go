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

	"github.com/dirloom/dirloom/internal/clipboard"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/presentation"
)

func TestCopyWritesExactRenderAndSilentStdout(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	want, stderr, code := executeForTest(t, root, "--no-config", "--color", "never")
	if code != 0 || stderr != "" {
		t.Fatalf("baseline=(%q,%q,%d)", want, stderr, code)
	}
	clip := &clipboard.Buffer{}
	stdout, stderr, code := executeForTestWithClipboard(t, clip, root, "--no-config", "--copy")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("copy=(%q,%q,%d)", stdout, stderr, code)
	}
	if !bytes.Equal(clip.Data, []byte(want)) {
		t.Fatalf("clipboard = %q, want %q", clip.Data, want)
	}
	if !bytes.HasSuffix(clip.Data, []byte("\n")) || bytes.HasSuffix(clip.Data, []byte("\n\n")) {
		t.Fatalf("newline contract = %q", clip.Data)
	}
}

func TestCopyOutputConflictBeforeConfigAndScan(t *testing.T) {
	stdout, stderr, code := executeForTest(t,
		filepath.Join(t.TempDir(), "missing-root"),
		"--copy", "--output", "tree.md",
		"--config", filepath.Join(t.TempDir(), "missing.yaml"),
	)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("conflict=(%q,%q,%d)", stdout, stderr, code)
	}
	if strings.Contains(stderr, "does not exist") || strings.Contains(strings.ToLower(stderr), "scan") {
		t.Fatalf("conflict leaked later work: %q", stderr)
	}
}

func TestCopyFailureDoesNotLeakContent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "secret.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	clip := clipboard.Fail{Err: errors.New("wl-copy missing")}
	stdout, stderr, code := executeForTestWithClipboard(t, clip, root, "--no-config", "--copy")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "clipboard") {
		t.Fatalf("failure=(%q,%q,%d)", stdout, stderr, code)
	}
	if strings.Contains(stderr, "secret.go") {
		t.Fatalf("error leaked tree content: %q", stderr)
	}
}

func TestCopyPresentationContracts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("disabled")
	}))
	tty := presentation.NewEvaluator(
		presentation.WithEnvironment(func(string) (string, bool) { return "", false }),
		presentation.WithTerminalDetection(func(io.Writer) bool { return true }),
		presentation.WithANSIPreparation(func(io.Writer) (func() error, error) { return func() error { return nil }, nil }),
		presentation.WithWindowsTerminalCompatibility(false),
	)
	interactive, stderr, code := executeForTestWithDependencies(t, loader, tty, root, "--no-config", "--icons", "unicode")
	if code != 0 || stderr != "" || !strings.ContainsRune(interactive, '\x1b') || !strings.Contains(interactive, "•") {
		t.Fatalf("interactive=(%q,%q,%d)", interactive, stderr, code)
	}

	clip := &clipboard.Buffer{}
	stdout, stderr, code := executeWithRuntime(t, loader, tty, clip, root, "--no-config", "--icons", "unicode", "--copy")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("copy unicode=(%q,%q,%d)", stdout, stderr, code)
	}
	if bytes.Contains(clip.Data, []byte{0x1b}) {
		t.Fatalf("clipboard contains ANSI: %q", clip.Data)
	}
	if !strings.Contains(string(clip.Data), "•") {
		t.Fatalf("clipboard dropped icons: %q", clip.Data)
	}

	autoClip := &clipboard.Buffer{}
	stdout, stderr, code = executeWithRuntime(t, loader, tty, autoClip, root, "--no-config", "--icons", "auto", "--copy")
	if code != 0 || stdout != "" || stderr != "" || !strings.Contains(string(autoClip.Data), "•") || bytes.Contains(autoClip.Data, []byte{0x1b}) {
		t.Fatalf("copy auto icons=(%q,%q,%d data=%q)", stdout, stderr, code, autoClip.Data)
	}

	neverClip := &clipboard.Buffer{}
	_, _, code = executeWithRuntime(t, loader, tty, neverClip, root, "--no-config", "--icons", "never", "--copy")
	if code != 0 || strings.Contains(string(neverClip.Data), "•") {
		t.Fatalf("copy never icons = %q code=%d", neverClip.Data, code)
	}

	forced := &clipboard.Buffer{}
	_, _, code = executeWithRuntime(t, loader, tty, forced, root, "--no-config", "--color", "always", "--icons", "unicode", "--copy")
	if code != 0 || !bytes.Contains(forced.Data, []byte{0x1b}) {
		t.Fatalf("forced color copy = %q code=%d", forced.Data, code)
	}

	jsonClip := &clipboard.Buffer{}
	canonical, _, _ := executeForTest(t, root, "--no-config", "--format", "json")
	_, _, code = executeForTestWithClipboard(t, jsonClip, root, "--no-config", "--format", "json", "--copy")
	if code != 0 || string(jsonClip.Data) != canonical || bytes.Contains(jsonClip.Data, []byte{0x1b}) {
		t.Fatalf("json copy = %q want %q", jsonClip.Data, canonical)
	}

	mermaidClip := &clipboard.Buffer{}
	canonicalMermaid, _, _ := executeForTest(t, root, "--no-config", "--format", "mermaid")
	_, _, code = executeForTestWithClipboard(t, mermaidClip, root, "--no-config", "--format", "mermaid", "--copy")
	if code != 0 || string(mermaidClip.Data) != canonicalMermaid || bytes.Contains(mermaidClip.Data, []byte{0x1b}) {
		t.Fatalf("mermaid copy = %q want %q", mermaidClip.Data, canonicalMermaid)
	}
}

func TestCopyDoesNotAffectHelpVersionOrSubcommands(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "--copy", "--help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage:") {
		t.Fatalf("help=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "--copy", "--version")
	if code != 0 || stderr != "" || stdout != "dirloom v0.1.0-test\n" {
		t.Fatalf("version=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "theme", "list", "--copy")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unknown flag") {
		t.Fatalf("subcommand copy=(%q,%q,%d)", stdout, stderr, code)
	}
}

func executeForTestWithClipboard(t *testing.T, clip clipboard.Writer, args ...string) (string, string, int) {
	t.Helper()
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	return executeWithRuntime(t, loader, presentation.NewEvaluator(), clip, args...)
}

func executeWithRuntime(t *testing.T, loader *configuration.Loader, evaluator *presentation.Evaluator, clip clipboard.Writer, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := execute(context.Background(), args, &stdout, &stderr, "v0.1.0-test", commandDependencies{
		loader:    loader,
		evaluator: evaluator,
		clipboard: clip,
	})
	return stdout.String(), stderr.String(), code
}
