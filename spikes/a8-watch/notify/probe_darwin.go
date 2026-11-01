package notify

import (
	"bytes"
	"os"

	"golang.org/x/sys/unix"
)

// RemoteTypes is the candidate refusal list for macOS (F13).
var RemoteTypes = map[string]bool{"nfs": true, "smbfs": true, "afpfs": true, "webdav": true, "cifs": true}

// FSType describes the filesystem holding path.
func FSType(path string) (string, error) {
	name, _, err := Classify(path)
	return name, err
}

// Classify returns f_fstypename and whether it is on the refusal list.
func Classify(path string) (string, bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return "", false, err
	}
	raw := st.Fstypename[:]
	if i := bytes.IndexByte(raw, 0); i >= 0 {
		raw = raw[:i]
	}
	name := string(raw)
	return name, RemoteTypes[name], nil
}

// OpenHandles counts the process's open descriptors.
func OpenHandles() (int, error) {
	entries, err := os.ReadDir("/dev/fd")
	return len(entries), err
}
