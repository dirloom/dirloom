package backend

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
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

// notify encodes FILE_NOTIFY_INFORMATION records, chained in order.
func notify(records ...notifyRecord) []byte {
	var out []byte
	for i, r := range records {
		units := utf16.Encode([]rune(strings.ReplaceAll(r.path, "/", `\`)))
		rec := make([]byte, notifyHeader+2*len(units))
		for len(rec)%4 != 0 {
			rec = append(rec, 0)
		}
		if i < len(records)-1 {
			binary.LittleEndian.PutUint32(rec[0:], uint32(len(rec)))
		}
		binary.LittleEndian.PutUint32(rec[4:], r.action)
		binary.LittleEndian.PutUint32(rec[8:], uint32(2*len(units)))
		for j, u := range units {
			binary.LittleEndian.PutUint16(rec[notifyHeader+2*j:], u)
		}
		out = append(out, rec...)
	}
	return out
}

func TestParseNotify(t *testing.T) {
	want := []notifyRecord{
		{actionAdded, "a/new.txt"},
		{actionRenamedOld, "Old Name"},
		{actionRenamedNew, "été/日本"},
		{actionModified, "a/.gitignore"},
	}
	var got []notifyRecord
	if err := parseNotify(notify(want...), func(r notifyRecord) { got = append(got, r) }); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records %+v, want %+v", got, want)
	}
}

func TestParseNotifyRejectsMalformedBuffers(t *testing.T) {
	two := notify(notifyRecord{actionAdded, "a"}, notifyRecord{actionRemoved, "b"})
	odd := notify(notifyRecord{actionAdded, "ab"})
	binary.LittleEndian.PutUint32(odd[8:], 3)
	backwards := append([]byte{}, two...)
	binary.LittleEndian.PutUint32(backwards[0:], 4)
	beyond := append([]byte{}, two...)
	binary.LittleEndian.PutUint32(beyond[0:], 4096)
	for name, buf := range map[string][]byte{
		"short header":     two[:8],
		"name beyond":      two[:notifyHeader+1],
		"odd name length":  odd,
		"offset too small": backwards,
		"offset beyond":    beyond,
		"empty":            nil,
	} {
		if err := parseNotify(buf, func(notifyRecord) {}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func FuzzParseNotify(f *testing.F) {
	f.Add(notify(notifyRecord{actionAdded, "a"}))
	f.Add(notify(notifyRecord{actionAdded, "a/b"}, notifyRecord{actionModified, "c"}))
	f.Add([]byte{0, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 0xd8, 0})
	f.Fuzz(func(t *testing.T, buf []byte) {
		count := 0
		_ = parseNotify(buf, func(notifyRecord) { count++ })
		if count > len(buf)/notifyHeader+1 {
			t.Fatalf("%d records from %d bytes", count, len(buf))
		}
	})
}

func TestClassifyWindowsDrive(t *testing.T) {
	for driveType, want := range map[uint32]string{0: "", 1: "", 2: "", 3: "", 4: "remote drive", 5: "", 6: ""} {
		if got := classifyWindowsDrive(driveType); got != want {
			t.Errorf("classifyWindowsDrive(%d) = %q, want %q", driveType, got, want)
		}
	}
}

func TestMayBeShortName(t *testing.T) {
	for p, want := range map[string]bool{
		"PROGRA~1":          true,
		"LONGFI~1.TXT":      true,
		"a/b/DOCUME~2/x":    true,
		"~1":                true,
		"file~":             false,
		"my~file":           false,
		"abcdefghi~1":       false, // longer than eight characters
		"a~1.tar.gz":        false,
		"README~1.markdown": false,
		"":                  false,
	} {
		if got := mayBeShortName(p); got != want {
			t.Errorf("mayBeShortName(%q) = %v, want %v", p, got, want)
		}
	}
}
