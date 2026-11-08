package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// denyACL denies the current user rights on path, through icacls, until the
// test ends.
func denyACL(t *testing.T, path, rights string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	if output, err := exec.Command("icacls", path, "/deny", "*"+sid+":"+rights).CombinedOutput(); err != nil {
		t.Fatalf("icacls %s /deny: %v\n%s", path, err, output)
	}
	t.Cleanup(func() { _ = exec.Command("icacls", path, "/remove:d", "*"+sid).Run() })
}

// TestWindowsACLDeniedDirectory pins the LOT 0 facts of the v0.4.0-rc1 plan
// on a directory the user may not list and a file the user may not read:
//   - every command that reads the tree fails, naming the directory: exit 1
//     in the 0/1/2 family, OBSERVATION_ERROR with source_read_failure, exit 5,
//     in verify and diff; no output is written;
//   - ignoring the directory avoids the error;
//   - a denied file changes nothing: its content is never read, and its
//     metadata comes from the parent listing.
func TestWindowsACLDeniedDirectory(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{{"open/a.txt", "a"}, {"locked/secret.txt", "s"}, {"denied-file.txt", "d"}})
	locked := filepath.Join(root, "locked")
	snap := filepath.Join(t.TempDir(), "ref.dlm.json")
	if _, stderr, code := executeForTest(t, "snapshot", root, "--no-config", "--output", snap); code != 0 {
		t.Fatalf("snapshot with access = %d: %s", code, stderr)
	}
	before, stderr, code := executeForTest(t, "fingerprint", root, "--no-config", "--ignore", "locked/")
	if code != 0 {
		t.Fatalf("fingerprint with access = %d: %s", code, stderr)
	}

	denyACL(t, locked, "(RD)")
	denyACL(t, filepath.Join(root, "denied-file.txt"), "(R)")
	if _, err := os.ReadDir(locked); !errors.Is(err, os.ErrPermission) {
		t.Skipf("the denied directory is still listable here (%v): this token bypasses the ACL", err)
	}

	const denied = "Error: read directory \"locked\": permission denied\n"
	output := filepath.Join(t.TempDir(), "never.dlm.json")
	for name, args := range map[string][]string{
		"tree":             {root, "--no-config"},
		"tree json":        {root, "--no-config", "--format", "json"},
		"fingerprint json": {"fingerprint", root, "--no-config", "--format", "json"},
		"snapshot":         {"snapshot", root, "--no-config", "--output", output},
	} {
		stdout, stderr, code := executeForTest(t, args...)
		if code != 1 || stdout != "" || stderr != denied {
			t.Fatalf("%s = (%q, %q, %d), want exit 1 and %q", name, stdout, stderr, code, denied)
		}
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot --output left a file: %v", err)
	}

	stdout, stderr, code := executeForTest(t, "verify", snap, root, "--format", "json")
	verified := assertVerifyJSON(t, stdout, stderr, code, 5, "OBSERVATION_ERROR", false)
	if verified.Diagnostic == nil || verified.Diagnostic.Code != "source_read_failure" {
		t.Fatalf("verify diagnostic = %+v", verified.Diagnostic)
	}
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+snap, "live:"+root, "--format", "json")
	diffed := assertDiffJSON(t, stdout, stderr, code, 5, "OBSERVATION_ERROR")
	if diffed.Diagnostic == nil || diffed.Diagnostic.Source != "b" || diffed.Diagnostic.Code != "source_read_failure" {
		t.Fatalf("diff diagnostic = %+v", diffed.Diagnostic)
	}

	after, stderr, code := executeForTest(t, "fingerprint", root, "--no-config", "--ignore", "locked/")
	if code != 0 || after != before {
		t.Fatalf("fingerprint without the denied directory = (%q, %q, %d), want %q: the denied file must change nothing", after, stderr, code, before)
	}
}
