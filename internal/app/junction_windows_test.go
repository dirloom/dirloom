//go:build windows

package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/render"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/tree"
)

func makeJunction(t testing.TB, link, target string) {
	t.Helper()
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J %s: %v\n%s", link, err, output)
	}
}

// setJunctionTarget retargets the existing junction at link in one step, by
// replacing its mount-point reparse data: an observation sees the old target
// or the new one, never no junction.
func setJunctionTarget(t testing.TB, link, target string) {
	t.Helper()
	substitute := utf16.Encode([]rune(`\??\` + target))
	print := utf16.Encode([]rune(target))
	// REPARSE_DATA_BUFFER for IO_REPARSE_TAG_MOUNT_POINT: an 8-byte header,
	// four uint16 name offsets and lengths, then both names, NUL-terminated.
	path := append(append(append(substitute, 0), print...), 0)
	buffer := make([]byte, 16+2*len(path))
	binary.LittleEndian.PutUint32(buffer[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:], uint16(8+2*len(path)))
	binary.LittleEndian.PutUint16(buffer[8:], 0)
	binary.LittleEndian.PutUint16(buffer[10:], uint16(2*len(substitute)))
	binary.LittleEndian.PutUint16(buffer[12:], uint16(2*len(substitute)+2))
	binary.LittleEndian.PutUint16(buffer[14:], uint16(2*len(print)))
	for i, unit := range path {
		binary.LittleEndian.PutUint16(buffer[16+2*i:], unit)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatalf("open junction %s: %v", link, err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var returned uint32
	if err := windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil); err != nil {
		t.Fatalf("retarget junction %s: %v", link, err)
	}
	if got, err := os.Readlink(link); err != nil || got != target {
		t.Fatalf("junction %s now targets %q (%v), want %q", link, got, err, target)
	}
}

// junctionTree returns a root holding target/inside.txt, an empty other/,
// and junction pointing at target.
func junctionTree(t *testing.T) (root, junction, target, other string) {
	t.Helper()
	root = t.TempDir()
	target = filepath.Join(root, "target")
	other = filepath.Join(root, "other")
	mkdirAll(t, target, other)
	writeFiles(t, filepath.Join(target, "inside.txt"))
	junction = filepath.Join(root, "junction")
	makeJunction(t, junction, target)
	return root, junction, target, other
}

func TestFingerprintWindowsJunctionTargetIsIdentity(t *testing.T) {
	root, junction, target, other := junctionTree(t)
	fingerprint := func() string {
		t.Helper()
		result, err := Fingerprint(context.Background(), InspectRequest{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		return result.Fingerprint.String()
	}
	original := fingerprint()
	must(t, os.Remove(junction))
	makeJunction(t, junction, other)
	if retargeted := fingerprint(); retargeted == original {
		t.Fatalf("retargeting the junction kept fingerprint %s", original)
	}
	must(t, os.Remove(junction))
	makeJunction(t, junction, target)
	if restored := fingerprint(); restored != original {
		t.Fatalf("restored junction fingerprint %s, want %s", restored, original)
	}
}

// TestSnapshotWindowsJunctionCapturedAsFile is the migration of a snapshot
// taken before junctions were recognized, when the scanner recorded a
// junction as a plain file: it stays valid, and it differs from the same
// unchanged tree observed now.
func TestSnapshotWindowsJunctionCapturedAsFile(t *testing.T) {
	root, _, _, _ := junctionTree(t)
	current, err := snapshot.LoadBytes(mustFilesystemSnapshot(t, root))
	if err != nil {
		t.Fatal(err)
	}
	legacy := current.Artifact
	found := false
	for i := range legacy.Root.Children {
		if node := &legacy.Root.Children[i]; node.Name == "junction" {
			if node.Kind != artifact.KindJunction || node.Target != "target" {
				t.Fatalf("observed junction = %+v", node)
			}
			node.Kind, node.Target, found = artifact.KindFile, "", true
		}
	}
	if !found {
		t.Fatal("junction missing")
	}
	raw := mustSnapshotBytes(t, legacy, current.Document.Capture)

	// It still loads and verifies its own integrity.
	loaded, err := snapshot.LoadBytes(raw)
	if err != nil {
		t.Fatalf("legacy snapshot no longer loads: %v", err)
	}
	if loaded.Fingerprint == current.Fingerprint {
		t.Fatal("legacy and current snapshots share a fingerprint")
	}
	// Against the unchanged tree it is a mismatch, a normal result.
	assertSnapshotStatus(t, raw, root, VerifyMismatch)
	snap := mustDiffSnapshotFile(t, t.TempDir(), "legacy.dlm.json", raw)
	diff := mustDiff(t, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	if got := strings.Join(describeChanges(diff.Changes), "; "); got != "CHANGED junction (file->junction:target)" {
		t.Fatalf("legacy diff = %s", got)
	}
	// A new snapshot is the new reference.
	assertSnapshotStatus(t, mustFilesystemSnapshot(t, root), root, VerifyMatch)
}

// TestInspectWindowsJunctionPresentation checks every public tree format: a
// junction is shown exactly as a symlink with the same target, so no format
// gains a type value or a different graph node id.
func TestInspectWindowsJunctionPresentation(t *testing.T) {
	root, _, target, _ := junctionTree(t)
	model, err := Inspect(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	junctions := 0
	symlink := cloneAsSymlinks(model, &junctions)
	if junctions != 1 {
		t.Fatalf("the scan found %d junctions, want 1", junctions)
	}
	slashTarget := filepath.ToSlash(target)
	for _, test := range []struct{ format, style, want string }{
		{render.FormatText, render.StyleUnicode, "junction -> " + slashTarget},
		{render.FormatText, render.StyleASCII, "junction -> " + slashTarget},
		{render.FormatMarkdown, render.StyleUnicode, "junction -> " + slashTarget},
		{render.FormatMarkdownTree, render.StyleUnicode, "`junction` -> `" + slashTarget + "`"},
		{render.FormatJSON, render.StyleUnicode, `{"name":"junction","type":"symlink","target":"` + slashTarget + `"}`},
		{render.FormatMermaid, render.StyleUnicode, "junction -&gt; " + slashTarget},
		{render.FormatGraphviz, render.StyleUnicode, "junction -> " + slashTarget},
		{render.FormatD2, render.StyleUnicode, "junction -> " + slashTarget},
	} {
		renderer, err := render.New(test.format, test.style)
		if err != nil {
			t.Fatal(err)
		}
		var out, asSymlink bytes.Buffer
		if err := renderer.Render(&out, model); err != nil {
			t.Fatalf("%s: %v", test.format, err)
		}
		if err := renderer.Render(&asSymlink, symlink); err != nil {
			t.Fatalf("%s: %v", test.format, err)
		}
		if out.String() != asSymlink.String() {
			t.Fatalf("%s/%s shows the junction unlike a symlink:\n%s\nsymlink:\n%s", test.format, test.style, out.String(), asSymlink.String())
		}
		got := out.String()
		if test.format == render.FormatJSON {
			got = strings.Join(strings.Fields(got), "")
		}
		if !strings.Contains(got, test.want) {
			t.Fatalf("%s/%s lacks %q:\n%s", test.format, test.style, test.want, out.String())
		}
	}
}

// cloneAsSymlinks copies a tree with every junction made a plain symlink, and
// counts the junctions.
func cloneAsSymlinks(node *tree.Node, junctions *int) *tree.Node {
	clone := *node
	if clone.Junction {
		*junctions++
		clone.Junction = false
	}
	clone.Children = nil
	if node.Children != nil {
		clone.Children = make([]*tree.Node, 0, len(node.Children))
		for _, child := range node.Children {
			clone.Children = append(clone.Children, cloneAsSymlinks(child, junctions))
		}
	}
	return &clone
}
