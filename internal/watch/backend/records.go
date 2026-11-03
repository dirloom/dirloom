package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf16"
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

// macOS filesystem types refused in native mode, by statfs f_fstypename.
var darwinRefusedTypes = map[string]bool{"nfs": true, "smbfs": true, "afpfs": true, "webdav": true}

// classifyDarwin returns the name of a refused filesystem type, or "" when
// the type is accepted.
func classifyDarwin(fsType string) string {
	if darwinRefusedTypes[fsType] {
		return fsType
	}
	return ""
}

// notifyHeader is the fixed part of a FILE_NOTIFY_INFORMATION record:
// NextEntryOffset, Action and FileNameLength, little-endian 32-bit fields.
const notifyHeader = 12

// Windows FILE_ACTION_* values.
const (
	actionAdded      = 1
	actionRemoved    = 2
	actionModified   = 3
	actionRenamedOld = 4
	actionRenamedNew = 5
)

// notifyRecord is one decoded FILE_NOTIFY_INFORMATION record: the action
// and the path relative to the watched directory, '/'-separated.
type notifyRecord struct {
	action uint32
	path   string
}

var errBadNotifyRecord = errors.New("malformed FILE_NOTIFY_INFORMATION record")

// parseNotify decodes a buffer filled by ReadDirectoryChangesW. Every
// offset and length is checked against the buffer, and the chain must move
// forward; a malformed buffer is an error, never a panic. Names are UTF-16;
// an unpaired surrogate decodes to U+FFFD, so the path is still placed.
func parseNotify(buf []byte, each func(notifyRecord)) error {
	for {
		if len(buf) < notifyHeader {
			return errBadNotifyRecord
		}
		next := binary.LittleEndian.Uint32(buf[0:])
		action := binary.LittleEndian.Uint32(buf[4:])
		size := binary.LittleEndian.Uint32(buf[8:])
		name := buf[notifyHeader:]
		if size%2 != 0 || uint64(size) > uint64(len(name)) {
			return errBadNotifyRecord
		}
		units := make([]uint16, size/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(name[2*i:])
		}
		each(notifyRecord{action: action, path: strings.ReplaceAll(string(utf16.Decode(units)), `\`, "/")})
		if next == 0 {
			return nil
		}
		if uint64(next) < notifyHeader || uint64(next) > uint64(len(buf)) {
			return errBadNotifyRecord
		}
		buf = buf[next:]
	}
}

// driveRemote is GetDriveType's DRIVE_REMOTE.
const driveRemote = 4

// classifyWindowsDrive returns the name of a refused drive type, or "" when
// the type is accepted.
func classifyWindowsDrive(driveType uint32) string {
	if driveType == driveRemote {
		return "remote drive"
	}
	return ""
}

// mayBeShortName reports whether a segment of a '/'-separated path may be
// an 8.3 short name (NAME~N.EXT), which can name an entry of the last state
// under another spelling.
func mayBeShortName(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		base, ext, _ := strings.Cut(seg, ".")
		i := strings.IndexByte(base, '~')
		if i >= 0 && i+1 < len(base) && base[i+1] >= '0' && base[i+1] <= '9' && len(base) <= 8 && len(ext) <= 3 {
			return true
		}
	}
	return false
}
