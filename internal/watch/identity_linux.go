package watch

import (
	"os"

	"golang.org/x/sys/unix"
)

func identityOf(path string) (Identity, error) {
	var st unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, path, unix.AT_STATX_SYNC_AS_STAT, unix.STATX_TYPE|unix.STATX_INO|unix.STATX_BTIME, &st)
	if err != nil {
		return Identity{}, &os.PathError{Op: "statx", Path: path, Err: err}
	}
	if uint32(st.Mode)&unix.S_IFMT != unix.S_IFDIR {
		return Identity{}, ErrNotDirectory
	}
	id := Identity{Volume: unix.Mkdev(st.Dev_major, st.Dev_minor), File: st.Ino}
	if st.Mask&unix.STATX_BTIME != 0 {
		id.Birth = st.Btime.Sec*1e9 + int64(st.Btime.Nsec)
	}
	return id, nil
}
