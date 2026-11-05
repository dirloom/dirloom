package cli

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecialEntryExitCodes pins what a user sees for a socket in the
// observed view: the tree view still lists it, fingerprint exits 1 and verify
// reports OBSERVATION_ERROR with unsupported_node_type, exit 5.
func TestSpecialEntryExitCodes(t *testing.T) {
	// A socket path is limited to about 100 bytes; t.TempDir includes the
	// test name.
	root, err := os.MkdirTemp("", "dl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := filepath.Join(t.TempDir(), "ref.dlm.json")
	if _, stderr, code := executeForTest(t, "snapshot", root, "--no-config", "--output", snap); code != 0 {
		t.Fatalf("snapshot = %d: %s", code, stderr)
	}
	listener, err := net.Listen("unix", filepath.Join(root, "s.sock"))
	if err != nil {
		t.Skipf("AF_UNIX sockets unavailable: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	if stdout, stderr, code := executeForTest(t, root, "--no-config"); code != 0 || !strings.Contains(stdout, "s.sock") {
		t.Fatalf("tree = (%q, %q, %d), want the socket listed", stdout, stderr, code)
	}
	if stdout, stderr, code := executeForTest(t, "fingerprint", root, "--no-config"); code != 1 || stdout != "" || !strings.Contains(stderr, `unsupported node type "socket" at "s.sock"`) {
		t.Fatalf("fingerprint = (%q, %q, %d), want exit 1", stdout, stderr, code)
	}
	stdout, stderr, code := executeForTest(t, "verify", snap, root, "--format", "json")
	var document struct {
		Status     string `json:"status"`
		Diagnostic struct {
			Code string `json:"code"`
		} `json:"diagnostic"`
	}
	if err := json.Unmarshal([]byte(stdout), &document); err != nil || code != 5 || stderr != "" ||
		document.Status != "OBSERVATION_ERROR" || document.Diagnostic.Code != "unsupported_node_type" {
		t.Fatalf("verify = (%q, %q, %d), want OBSERVATION_ERROR unsupported_node_type, exit 5", stdout, stderr, code)
	}
}
