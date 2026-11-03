package watch

import (
	"os"

	"golang.org/x/sys/unix"
)

func identityOf(path string) (Identity, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return Identity{}, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return Identity{}, ErrNotDirectory
	}
	return Identity{
		Volume: uint64(uint32(st.Dev)), //nolint:gosec // G115: the device number is an opaque 32-bit value, reinterpreted.
		File:   st.Ino,
		Birth:  st.Btim.Sec*1e9 + st.Btim.Nsec,
	}, nil
}
