package watch

import (
	"errors"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
)

// Identity identifies a directory independently of its path, so that a
// directory removed and recreated under the same path, or another directory
// moved there, is told apart from the one captured at startup. Inode numbers
// are reused at once after a removal (LOT 0, F18), so the inode alone is not
// enough:
//
//   - Linux: the device, the inode, the birth time where recorded, and the
//     file handle of name_to_handle_at where the filesystem provides one. The
//     handle carries the inode generation, which changes on reuse; the birth
//     time does not always, because ext4 timestamps are coarse.
//   - macOS: the device, the inode and the birth time.
//   - Windows: the volume serial number and the file index, whose NTFS
//     sequence number changes on reuse; the creation time is left out
//     because NTFS tunneling can restore it.
type Identity struct {
	Volume uint64
	File   uint64
	Birth  int64  // nanoseconds since the Unix epoch; 0 when not recorded
	Handle string // opaque file handle (Linux); empty when not provided
}

// ErrNotDirectory reports that a path names something other than a
// directory.
var ErrNotDirectory = errors.New("not a directory")

// IdentityOf returns the identity of the directory at path, following a
// final link as os.Stat does. A missing path matches os.ErrNotExist, and
// anything but a directory ErrNotDirectory.
func IdentityOf(path string) (Identity, error) { return identityOf(path) }

// StructuralSet returns the directories an observation entered, root
// included, in record order: the root unless maxDepth is 0, and every
// directory record whose depth (its number of path components) is below
// maxDepth, all of them when maxDepth is nil. Links and junctions are never
// in it, and neither is what the capture excluded, since it has no record.
func StructuralSet(records []identity.Record, maxDepth *int) []artifact.Path {
	if maxDepth != nil && *maxDepth == 0 {
		return nil
	}
	set := make([]artifact.Path, 0, len(records))
	for _, r := range records {
		if r.Kind != artifact.KindDirectory {
			continue
		}
		if r.Path != artifact.RootPath && maxDepth != nil && strings.Count(string(r.Path), "/")+1 >= *maxDepth {
			continue
		}
		set = append(set, r.Path)
	}
	return set
}
