package backend

import (
	"encoding/binary"
	"reflect"
	"testing"
)

// record encodes one inotify_event, its name padded to pad bytes.
func record(wd, mask uint32, name string, pad int) []byte {
	size := len(name)
	if pad > size {
		size = pad
	}
	buf := make([]byte, inotifyHeader+size)
	binary.NativeEndian.PutUint32(buf[0:], wd)
	binary.NativeEndian.PutUint32(buf[4:], mask)
	binary.NativeEndian.PutUint32(buf[12:], uint32(size))
	copy(buf[inotifyHeader:], name)
	return buf
}

func TestParseInotify(t *testing.T) {
	var buf []byte
	buf = append(buf, record(1, 0x100, "a.txt", 16)...)
	buf = append(buf, record(2, 0x400, "", 0)...)
	buf = append(buf, record(3, 0x4000, "", 0)...)
	var got []inotifyEvent
	if err := parseInotify(buf, func(ev inotifyEvent) { got = append(got, ev) }); err != nil {
		t.Fatal(err)
	}
	want := []inotifyEvent{{1, 0x100, "a.txt"}, {2, 0x400, ""}, {3, 0x4000, ""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events %+v, want %+v", got, want)
	}
}

func TestParseInotifyRejectsTruncatedRecords(t *testing.T) {
	full := record(1, 0x100, "name", 16)
	for _, buf := range [][]byte{full[:8], full[:inotifyHeader+3], append(append([]byte{}, full...), 1, 2, 3)} {
		if err := parseInotify(buf, func(inotifyEvent) {}); err == nil {
			t.Fatalf("parseInotify(%d bytes) = nil, want an error", len(buf))
		}
	}
	huge := record(1, 0x100, "", 0)
	binary.NativeEndian.PutUint32(huge[12:], 0xffffffff)
	if err := parseInotify(huge, func(inotifyEvent) {}); err == nil {
		t.Fatal("a length beyond the buffer was accepted")
	}
}

func FuzzParseInotify(f *testing.F) {
	f.Add(record(1, 0x100, "a", 16))
	f.Add(append(record(1, 0x100, "", 0), record(2, 0x4000, "x", 4)...))
	f.Add([]byte{1, 2, 3})
	f.Fuzz(func(t *testing.T, buf []byte) {
		total := 0
		err := parseInotify(buf, func(ev inotifyEvent) { total += inotifyHeader + len(ev.name) })
		if err == nil && total > len(buf) {
			t.Fatalf("decoded %d bytes from %d", total, len(buf))
		}
	})
}

func TestClassifyLinux(t *testing.T) {
	for magic, want := range map[int64]string{
		0x6969:           "nfs",
		0xff534d42:       "cifs",
		-0x00acb2be:      "cifs", // 0xff534d42 read as a signed 32-bit f_type
		0x01021997:       "9p",
		0xef53:           "", // ext2/3/4
		0x01021994:       "", // tmpfs
		0x65735546:       "", // FUSE, accepted
		0x794c7630:       "", // overlayfs
		0x7fffffffffffff: "",
	} {
		if got := classifyLinux(magic); got != want {
			t.Errorf("classifyLinux(%#x) = %q, want %q", magic, got, want)
		}
	}
}
