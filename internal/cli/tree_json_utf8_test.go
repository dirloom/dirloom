package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dirloom/dirloom/internal/clipboard"
)

// invalidUTF8Unit is how Go spells an unpaired UTF-16 surrogate (U+D800) in
// a Windows file name (WTF-8). On Linux the same three bytes are a raw,
// non-UTF-8 name.
const invalidUTF8Unit = "\xed\xa0\x80"

// escapedInvalidUTF8Unit is how a diagnostic quotes those bytes.
const escapedInvalidUTF8Unit = `\xed\xa0\x80`

// createInvalidUTF8Entry creates a file or a directory whose name is not
// valid UTF-8: on Windows a real NTFS name holding an unpaired surrogate. It
// skips where the filesystem refuses or rewrites such a name (macOS APFS).
func createInvalidUTF8Entry(t *testing.T, parent, name string, directory bool) string {
	t.Helper()
	path := filepath.Join(parent, name)
	var err error
	if directory {
		err = os.Mkdir(path, 0o755)
	} else {
		err = os.WriteFile(path, nil, 0o644)
	}
	if err != nil {
		t.Skipf("this filesystem refuses a non-UTF-8 name: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == name {
			return path
		}
	}
	t.Skipf("this filesystem rewrote the non-UTF-8 name %q", name)
	return ""
}

// TestTreeJSONRefusesInvalidUTF8Name pins the A11 correction on a real
// directory: tree JSON refuses a name that is not valid UTF-8, as the text
// output does, instead of writing U+FFFD with exit 0; nothing reaches stdout,
// the --output file or the clipboard.
func TestTreeJSONRefusesInvalidUTF8Name(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	name := "bad" + invalidUTF8Unit + ".txt"
	createInvalidUTF8Entry(t, root, name, false)

	assertTreeJSONRefused(t, `entry "bad`+escapedInvalidUTF8Unit+`.txt": name is not valid UTF-8`, root, "--no-config", "--format", "json")

	// The text output refuses the same tree, with the same exit status.
	if stdout, stderr, code := executeForTest(t, root, "--no-config"); code != 1 || stdout != "" || stderr == "" {
		t.Fatalf("text = (%q, %q, %d), want exit 1", stdout, stderr, code)
	}

	// --output leaves an existing file unchanged and no temporary file.
	outputDir := t.TempDir()
	output := filepath.Join(outputDir, "tree.json")
	if err := os.WriteFile(output, []byte("previous\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertTreeJSONRefused(t, "not valid UTF-8", root, "--no-config", "--format", "json", "--output", output)
	if data, err := os.ReadFile(output); err != nil || string(data) != "previous\n" {
		t.Fatalf("--output file = %q, %v; want it unchanged", data, err)
	}
	if entries, err := os.ReadDir(outputDir); err != nil || len(entries) != 1 {
		t.Fatalf("output directory holds %d entries (%v), want only the unchanged file", len(entries), err)
	}

	// --copy leaves the clipboard untouched.
	clip := &clipboard.Buffer{}
	stdout, stderr, code := executeForTestWithClipboard(t, clip, root, "--no-config", "--format", "json", "--copy")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "not valid UTF-8") || clip.Data != nil {
		t.Fatalf("--copy = (%q, %q, %d), clipboard %q; want exit 1 and no clipboard write", stdout, stderr, code, clip.Data)
	}

	// The tree JSON describes the rendered tree: an excluded name is no
	// refusal.
	stdout, stderr, code = executeForTest(t, root, "--no-config", "--format", "json", "--ignore", "bad*")
	if code != 0 || stderr != "" {
		t.Fatalf("--ignore = (%q, %q, %d), want exit 0", stdout, stderr, code)
	}
	assertValidTreeJSON(t, stdout)
	if stdout, stderr, code = executeForTest(t, root, "--no-config", "--format", "json", "--depth", "0"); code != 0 || stderr != "" {
		t.Fatalf("--depth 0 = (%q, %q, %d), want exit 0", stdout, stderr, code)
	}
}

// TestTreeJSONRefusesInvalidUTF8DirectoryAndRoot covers a directory name
// below the root and the name of the inspected root itself.
func TestTreeJSONRefusesInvalidUTF8DirectoryAndRoot(t *testing.T) {
	parent := t.TempDir()
	directory := createInvalidUTF8Entry(t, parent, "top"+invalidUTF8Unit, true)
	if err := os.WriteFile(filepath.Join(directory, "ok.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	assertTreeJSONRefused(t, `entry "top`+escapedInvalidUTF8Unit+`": name is not valid UTF-8`, parent, "--no-config", "--format", "json")
	assertTreeJSONRefused(t, `the root: name "top`+escapedInvalidUTF8Unit+`" is not valid UTF-8`, directory, "--no-config", "--format", "json")
}

// TestTreeJSONRefusesInvalidUTF8SymlinkTarget covers a symbolic link whose
// recorded target is not valid UTF-8. The link is dangling: the target is
// recorded, never followed.
func TestTreeJSONRefusesInvalidUTF8SymlinkTarget(t *testing.T) {
	root := t.TempDir()
	target := "bad" + invalidUTF8Unit
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if recorded, err := os.Readlink(filepath.Join(root, "link")); err != nil || recorded != target {
		t.Skipf("this filesystem rewrote the link target: %q, %v", recorded, err)
	}
	assertTreeJSONRefused(t, `entry "link": target "bad`+escapedInvalidUTF8Unit+`" is not valid UTF-8`, root, "--no-config", "--format", "json")
}

func assertTreeJSONRefused(t *testing.T, want string, args ...string) {
	t.Helper()
	stdout, stderr, code := executeForTest(t, args...)
	if code != 1 || stdout != "" {
		t.Fatalf("dirloom %q = (stdout %q, stderr %q, exit %d), want exit 1 and empty stdout", args, stdout, stderr, code)
	}
	if !strings.HasPrefix(stderr, "Error: ") || !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want it to mention %q", stderr, want)
	}
	if !utf8.ValidString(stderr) || strings.ContainsRune(stderr, utf8.RuneError) {
		t.Fatalf("stderr %q must stay UTF-8 without U+FFFD", stderr)
	}
}

func assertValidTreeJSON(t *testing.T, stdout string) {
	t.Helper()
	var document struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal([]byte(stdout), &document); err != nil || document.SchemaVersion != 1 {
		t.Fatalf("tree JSON %q: %v", stdout, err)
	}
	if strings.Contains(stdout, `�`) || strings.ContainsRune(stdout, utf8.RuneError) {
		t.Fatalf("tree JSON holds U+FFFD: %q", stdout)
	}
}
