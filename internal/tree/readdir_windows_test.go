//go:build windows

package tree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The tests below prove that readDir is os.ReadDir with one difference, the
// FILE_SHARE_DELETE share mode (anomaly A10): same entries in the same
// order, same metadata, same errors, no handle left open. They run on the
// toolchain of go.mod in CI.

// sameListing lists dir with readDir and os.ReadDir and requires the same
// result, entry by entry.
func sameListing(t *testing.T, dir string) int {
	t.Helper()
	got, gotErr := readDir(dir)
	want, wantErr := os.ReadDir(dir)
	sameError(t, "list "+dir, gotErr, wantErr)
	if len(got) != len(want) {
		t.Fatalf("%s: readDir has %d entries, os.ReadDir %d", dir, len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Name() != w.Name() || g.IsDir() != w.IsDir() || g.Type() != w.Type() {
			t.Fatalf("%s entry %d: readDir (%q, dir %t, type %v), os.ReadDir (%q, dir %t, type %v)",
				dir, i, g.Name(), g.IsDir(), g.Type(), w.Name(), w.IsDir(), w.Type())
		}
		gi, gerr := g.Info()
		wi, werr := w.Info()
		sameError(t, "info of "+w.Name(), gerr, werr)
		if werr != nil {
			continue
		}
		if gi.Name() != wi.Name() || gi.Size() != wi.Size() || gi.Mode() != wi.Mode() ||
			!gi.ModTime().Equal(wi.ModTime()) || gi.IsDir() != wi.IsDir() {
			t.Fatalf("%s: metadata of %q differs: readDir (%d, %v, %v), os.ReadDir (%d, %v, %v)",
				dir, w.Name(), gi.Size(), gi.Mode(), gi.ModTime(), wi.Size(), wi.Mode(), wi.ModTime())
		}
		gs, gok := gi.Sys().(*syscall.Win32FileAttributeData)
		ws, wok := wi.Sys().(*syscall.Win32FileAttributeData)
		// The last access time is left out: the file system may update it
		// on any open, whatever the share mode.
		if gok != wok || (wok && (gs.FileAttributes != ws.FileAttributes || gs.CreationTime != ws.CreationTime ||
			gs.LastWriteTime != ws.LastWriteTime || gs.FileSizeHigh != ws.FileSizeHigh || gs.FileSizeLow != ws.FileSizeLow)) {
			t.Fatalf("%s: system metadata of %q differs: readDir %+v, os.ReadDir %+v", dir, w.Name(), gs, ws)
		}
		if !os.SameFile(gi, wi) {
			t.Fatalf("%s: %q is not the same file in both listings", dir, w.Name())
		}
		gj, gjerr := IsJunction(filepath.Join(dir, g.Name()), gi)
		wj, wjerr := IsJunction(filepath.Join(dir, w.Name()), wi)
		if gj != wj || (gjerr == nil) != (wjerr == nil) {
			t.Fatalf("%s: junction of %q: readDir (%t, %v), os.ReadDir (%t, %v)", dir, w.Name(), gj, gjerr, wj, wjerr)
		}
	}
	return len(want)
}

// sameError requires both errors to be nil, or to be the same *os.PathError
// with the same operation, path and system error.
func sameError(t *testing.T, what string, got, want error) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("%s: readDir error %v, os.ReadDir error %v", what, got, want)
	}
	if want == nil {
		return
	}
	var g, w *os.PathError
	if !errors.As(got, &g) || !errors.As(want, &w) {
		if got.Error() != want.Error() {
			t.Fatalf("%s: readDir error %#v, os.ReadDir error %#v", what, got, want)
		}
		return
	}
	if g.Op != w.Op || g.Path != w.Path || g.Err != w.Err {
		t.Fatalf("%s: readDir error {%s %s %#v}, os.ReadDir error {%s %s %#v}", what, g.Op, g.Path, g.Err, w.Op, w.Path, w.Err)
	}
	for _, target := range []error{os.ErrNotExist, os.ErrPermission, os.ErrInvalid} {
		if errors.Is(got, target) != errors.Is(want, target) {
			t.Fatalf("%s: errors.Is(%v) differs", what, target)
		}
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setAttributes(t *testing.T, path string, attrs uint32) {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	current, err := windows.GetFileAttributes(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(p, current|attrs); err != nil {
		t.Fatal(err)
	}
}

func TestReadDirListsLikeOSReadDir(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		sameListing(t, t.TempDir())
	})
	t.Run("files and attributes", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "empty.txt"), 0)
		writeFile(t, filepath.Join(dir, "big.bin"), 70_000)
		writeFile(t, filepath.Join(dir, ".dot"), 1)
		for name, attrs := range map[string]uint32{
			"hidden.txt":   windows.FILE_ATTRIBUTE_HIDDEN,
			"system.txt":   windows.FILE_ATTRIBUTE_SYSTEM,
			"readonly.txt": windows.FILE_ATTRIBUTE_READONLY,
			"archive.txt":  windows.FILE_ATTRIBUTE_ARCHIVE,
		} {
			writeFile(t, filepath.Join(dir, name), 3)
			setAttributes(t, filepath.Join(dir, name), attrs)
		}
		if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		hidden := filepath.Join(dir, "hidden dir")
		if err := os.Mkdir(hidden, 0o755); err != nil {
			t.Fatal(err)
		}
		setAttributes(t, hidden, windows.FILE_ATTRIBUTE_HIDDEN)
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "readonly.txt"), 0o644) })
		if n := sameListing(t, dir); n != 9 {
			t.Fatalf("%d entries, want 9", n)
		}
	})
	t.Run("more entries than one read returns", func(t *testing.T) {
		// Go reads 64 KiB of FILE_ID_BOTH_DIR_INFO records at a time: these
		// entries take several reads.
		dir := t.TempDir()
		for i := range 1500 {
			writeFile(t, filepath.Join(dir, "entry-with-a-rather-long-name-"+strconv.Itoa(i)), 0)
		}
		if n := sameListing(t, dir); n != 1500 {
			t.Fatalf("%d entries, want 1500", n)
		}
	})
	t.Run("names", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"café", "café", "日本語", "emoji-😀", "with space", "UPPER", "upper2", "a.b.c", "~tilde", "#hash"} {
			writeFile(t, filepath.Join(dir, name), 0)
		}
		// An unpaired UTF-16 surrogate, which Go writes as WTF-8.
		writeFile(t, filepath.Join(dir, "bad\xed\xa0\x80.txt"), 0)
		if n := sameListing(t, dir); n != 11 {
			t.Fatalf("%d entries, want 11", n)
		}
	})
	t.Run("links", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "file.txt"), 1)
		if output, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(dir, "junction"), target).CombinedOutput(); err != nil {
			t.Fatalf("mklink /J: %v\n%s", err, output)
		}
		links := 0
		for link, to := range map[string]string{"dirlink": target, "filelink": filepath.Join(dir, "file.txt"), "dangling": filepath.Join(dir, "missing")} {
			if err := os.Symlink(to, filepath.Join(dir, link)); err != nil {
				t.Logf("symlink %s not created (no symlink privilege?): %v", link, err)
				continue
			}
			links++
		}
		if n := sameListing(t, dir); n != 3+links {
			t.Fatalf("%d entries, want %d", n, 3+links)
		}
	})
	t.Run("long path", func(t *testing.T) {
		dir := t.TempDir()
		for len(dir) < 300 {
			dir = filepath.Join(dir, strings.Repeat("d", 50))
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "deep.txt"), 2)
		if n := sameListing(t, dir); n != 1 {
			t.Fatalf("%d entries, want 1", n)
		}
	})
}

func TestReadDirFailsLikeOSReadDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	writeFile(t, file, 1)
	for name, path := range map[string]string{
		"missing":        filepath.Join(dir, "missing"),
		"missing parent": filepath.Join(dir, "missing", "child"),
		"a file":         file,
		"a file's child": filepath.Join(file, "child"),
		"empty path":     "",
		"NUL in path":    filepath.Join(dir, "a\x00b"),
	} {
		t.Run(name, func(t *testing.T) {
			_, gotErr := readDir(path)
			_, wantErr := os.ReadDir(path)
			if wantErr == nil {
				t.Fatal("os.ReadDir succeeded")
			}
			sameError(t, name, gotErr, wantErr)
		})
	}
	t.Run("list permission denied", func(t *testing.T) {
		denied := filepath.Join(dir, "denied")
		if err := os.Mkdir(denied, 0o755); err != nil {
			t.Fatal(err)
		}
		sid := currentUserSID(t)
		if output, err := exec.Command("icacls", denied, "/deny", "*"+sid+":(RD)").CombinedOutput(); err != nil {
			t.Fatalf("icacls /deny: %v\n%s", err, output)
		}
		t.Cleanup(func() { _ = exec.Command("icacls", denied, "/remove:d", "*"+sid).Run() })
		_, gotErr := readDir(denied)
		_, wantErr := os.ReadDir(denied)
		if !errors.Is(wantErr, os.ErrPermission) {
			t.Skipf("the denied directory is still listable here (%v): this token bypasses the ACL", wantErr)
		}
		sameError(t, "denied", gotErr, wantErr)
	})
}

func currentUserSID(t *testing.T) string {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return user.User.Sid.String()
}

var procGetProcessHandleCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")

func handleCount(t *testing.T) int {
	t.Helper()
	var n uint32
	if r, _, err := procGetProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n))); r == 0 {
		t.Fatal(err)
	}
	return int(n)
}

// TestReadDirClosesItsHandle lists and fails many times: the process holds
// no more handles than before.
func TestReadDirClosesItsHandle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), 0)
	file := filepath.Join(dir, "a")
	_, _ = readDir(dir) // warm up lazy system handles
	before := handleCount(t)
	for range 300 {
		if _, err := readDir(dir); err != nil {
			t.Fatal(err)
		}
		if _, err := readDir(file); err == nil {
			t.Fatal("listing a file succeeded")
		}
		if _, err := readDir(filepath.Join(dir, "missing")); err == nil {
			t.Fatal("listing a missing directory succeeded")
		}
	}
	if after := handleCount(t); after > before+10 {
		t.Fatalf("handles: %d before, %d after 900 listings", before, after)
	}
}

// holdForRename opens dir as a rename does, with delete access, sharing
// everything: Windows holds such a handle while it moves the directory.
func holdForRename(t *testing.T, dir string) windows.Handle {
	t.Helper()
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(h) })
	return h
}

// TestReadDirDoesNotBlockTheUser is the first A10 property: while a scan
// holds a directory open, the user can rename and remove it. os.ReadDir's
// handle blocks both, which the test shows first.
func TestReadDirDoesNotBlockTheUser(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(dir) // the handle os.ReadDir lists through
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+"-renamed"); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		_ = old.Close()
		t.Fatalf("rename under os.ReadDir's handle: %v, want the sharing violation of A10", err)
	}
	_ = old.Close()

	f, err := openDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := os.Rename(dir, dir+"-renamed"); err != nil {
		t.Fatalf("rename under readDir's handle: %v", err)
	}
	if err := os.Remove(dir + "-renamed"); err != nil {
		t.Fatalf("remove under readDir's handle: %v", err)
	}
}

// TestReadDirListsADirectoryBeingRenamed is the second A10 property: a scan
// can list a directory that a rename holds. os.ReadDir cannot, which the
// test shows first.
func TestReadDirListsADirectoryBeingRenamed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "inside.txt"), 1)
	holdForRename(t, dir)
	if _, err := os.ReadDir(dir); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("os.ReadDir during a rename: %v, want the sharing violation of A10", err)
	}
	entries, err := readDir(dir)
	if err != nil {
		t.Fatalf("readDir during a rename: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "inside.txt" {
		t.Fatalf("entries %v", entries)
	}
}

// TestScanOfADirectoryBeingRenamed runs the filesystem scanner over a tree
// one of whose directories a rename holds: the scan completes, as it failed
// with ERROR_SHARING_VIOLATION before (A10).
func TestScanOfADirectoryBeingRenamed(t *testing.T) {
	root := t.TempDir()
	held := filepath.Join(root, "src", "lib")
	if err := os.MkdirAll(held, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(held, "a.go"), 1)
	holdForRename(t, held)
	tree, err := NewScanner(ScanOptions{RootAbs: root, RootName: "root"}).Scan(context.Background())
	if err != nil {
		t.Fatalf("scan during a rename: %v", err)
	}
	src := tree.Children[0]
	if src.Name != "src" || len(src.Children) != 1 || src.Children[0].Name != "lib" ||
		len(src.Children[0].Children) != 1 || src.Children[0].Children[0].Name != "a.go" {
		t.Fatalf("scanned tree %+v", src)
	}
}
