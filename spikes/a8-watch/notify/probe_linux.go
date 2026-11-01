package notify

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

var linuxTypes = map[int64]string{
	unix.NFS_SUPER_MAGIC:       "nfs",
	unix.SMB_SUPER_MAGIC:       "smb",
	unix.SMB2_SUPER_MAGIC:      "smb2",
	unix.CIFS_SUPER_MAGIC:      "cifs",
	unix.V9FS_MAGIC:            "9p",
	unix.AFS_SUPER_MAGIC:       "afs",
	unix.CEPH_SUPER_MAGIC:      "ceph",
	unix.CODA_SUPER_MAGIC:      "coda",
	unix.FUSE_SUPER_MAGIC:      "fuse",
	unix.TMPFS_MAGIC:           "tmpfs",
	unix.EXT4_SUPER_MAGIC:      "ext2/3/4",
	unix.OVERLAYFS_SUPER_MAGIC: "overlayfs",
	unix.XFS_SUPER_MAGIC:       "xfs",
	unix.BTRFS_SUPER_MAGIC:     "btrfs",
	unix.PROC_SUPER_MAGIC:      "proc",
	unix.SYSFS_MAGIC:           "sysfs",
}

// RemoteTypes is the candidate refusal list for Linux (F13).
var RemoteTypes = map[string]bool{"nfs": true, "smb": true, "smb2": true, "cifs": true, "9p": true, "afs": true, "ceph": true, "coda": true}

// FSType describes the filesystem holding path.
func FSType(path string) (string, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return "", err
	}
	name := linuxTypes[int64(st.Type)]
	if name == "" {
		name = "unknown"
	}
	return fmt.Sprintf("%s (0x%x)", name, uint64(st.Type)), nil
}

// Classify returns the short type name and whether it is on the refusal list.
func Classify(path string) (string, bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return "", false, err
	}
	name := linuxTypes[int64(st.Type)]
	return name, RemoteTypes[name], nil
}

// OpenHandles counts the process's open descriptors.
func OpenHandles() (int, error) {
	entries, err := os.ReadDir("/proc/self/fd")
	return len(entries), err
}
