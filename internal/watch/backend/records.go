package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// inotifyHeader is the fixed part of a struct inotify_event: wd, mask,
// cookie and len, four 32-bit fields in native byte order.
const inotifyHeader = 16

// inotifyEvent is one decoded inotify record. Name is empty for an event on
// the watched directory itself.
type inotifyEvent struct {
	wd   uint32
	mask uint32
	name string
}

var errTruncatedRecord = errors.New("truncated inotify record")

// parseInotify decodes a buffer read from an inotify descriptor, record by
// record. Kernel data is untrusted: every length is checked against the
// buffer, and a record that does not fit is an error, never a panic. The
// name, padded with NUL bytes, ends at its first NUL.
func parseInotify(buf []byte, each func(inotifyEvent)) error {
	for len(buf) > 0 {
		if len(buf) < inotifyHeader {
			return errTruncatedRecord
		}
		wd := binary.NativeEndian.Uint32(buf[0:])
		mask := binary.NativeEndian.Uint32(buf[4:])
		size := binary.NativeEndian.Uint32(buf[12:])
		rest := buf[inotifyHeader:]
		if uint64(size) > uint64(len(rest)) {
			return errTruncatedRecord
		}
		name := rest[:size]
		if i := bytes.IndexByte(name, 0); i >= 0 {
			name = name[:i]
		}
		each(inotifyEvent{wd: wd, mask: mask, name: string(name)})
		buf = rest[size:]
	}
	return nil
}

// Linux filesystem types refused in native mode (Structural Watch v1,
// filesystem policy), by statfs f_type. The values are the kernel's magic
// numbers; a Linux test checks them against golang.org/x/sys/unix.
var linuxRefusedTypes = map[int64]string{
	0x6969:     "nfs",
	0x517b:     "smb",
	0xfe534d42: "smb2",
	0xff534d42: "cifs",
	0x01021997: "9p",
	0x5346414f: "afs",
	0x00c36400: "ceph",
	0x73757245: "coda",
}

// classifyLinux returns the name of a refused filesystem type, or "" when
// the type is accepted. Only the low 32 bits of f_type are significant.
func classifyLinux(fsType int64) string {
	return linuxRefusedTypes[fsType&0xffffffff]
}
