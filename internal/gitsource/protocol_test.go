package gitsource

import (
	"bufio"
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
)

var testID = strings.Repeat("ab", 20)

func response(t *testing.T, input string, kind objectKind) ([]byte, *bufio.Reader, error) {
	t.Helper()
	reader := bufio.NewReaderSize(strings.NewReader(input), 128)
	data, err := readResponse(reader, testID, kind)
	return data, reader, err
}

func TestReadResponseAccepts(t *testing.T) {
	data, reader, err := response(t, testID+" tree 3\nabc\nNEXT", treeObject)
	if err != nil || string(data) != "abc" {
		t.Fatalf("%q %v", data, err)
	}
	rest, _ := reader.ReadString(0)
	if rest != "NEXT" {
		t.Fatalf("consumed past the response: rest %q", rest)
	}
	data, _, err = response(t, testID+" blob 0\n\n", linkObject)
	if err != nil || len(data) != 0 {
		t.Fatalf("empty blob: %q %v", data, err)
	}
	data, _, err = response(t, testID+" blob 4096\n"+strings.Repeat("x", 4096)+"\n", linkObject)
	if err != nil || len(data) != 4096 {
		t.Fatalf("blob at the limit: %d %v", len(data), err)
	}
}

func TestReadResponseRejects(t *testing.T) {
	other := strings.Repeat("cd", 20)
	cases := []struct {
		name  string
		input string
		code  string
		parts []string
	}{
		{"missing", testID + " missing\n", artifact.CodeGitObjectUnavailable, []string{"not available"}},
		{"missing other id", other + " missing\n", artifact.CodeGitTreeInvalid, []string{"malformed header"}},
		{"other id", other + " tree 3\nabc\n", artifact.CodeGitTreeInvalid, []string{"malformed header"}},
		{"extra field", testID + " tree 3 x\nabc\n", artifact.CodeGitTreeInvalid, []string{"malformed header"}},
		{"short header", testID + " tree\n", artifact.CodeGitTreeInvalid, []string{"malformed header"}},
		{"ambiguous", testID + " ambiguous\n", artifact.CodeGitTreeInvalid, []string{"malformed header"}},
		{"long header", strings.Repeat("a", 63) + "\n", artifact.CodeGitTreeInvalid, []string{"malformed header"}},
		{"no newline in buffer", strings.Repeat("a", 200), artifact.CodeGitTreeInvalid, []string{"malformed header"}},
		{"wrong type", testID + " blob 3\nabc\n", artifact.CodeGitTreeInvalid, []string{`"blob"`, "expected a tree"}},
		{"empty size", testID + " tree \nabc\n", artifact.CodeGitTreeInvalid, []string{"malformed size"}},
		{"signed size", testID + " tree +3\nabc\n", artifact.CodeGitTreeInvalid, []string{"malformed size"}},
		{"negative size", testID + " tree -3\nabc\n", artifact.CodeGitTreeInvalid, []string{"malformed size"}},
		{"leading zero", testID + " tree 03\nabc\n", artifact.CodeGitTreeInvalid, []string{"malformed size"}},
		{"hex size", testID + " tree 1a\nabc\n", artifact.CodeGitTreeInvalid, []string{"malformed size"}},
		{"huge size", testID + " tree 99999999999999999999\n", artifact.CodeGitTreeInvalid, []string{"malformed size"}},
		{"above limit", testID + " tree 33554433\n", artifact.CodeGitTreeInvalid, []string{"33554433 bytes", "32 MiB limit"}},
		{"truncated header", testID + " tree 3", artifact.CodeGitObjectUnavailable, []string{"truncated"}},
		{"empty response", "", artifact.CodeGitObjectUnavailable, []string{"truncated"}},
		{"truncated body", testID + " tree 10\nabc", artifact.CodeGitObjectUnavailable, []string{"truncated"}},
		{"missing terminator", testID + " tree 3\nabc", artifact.CodeGitObjectUnavailable, []string{"truncated"}},
		{"wrong terminator", testID + " tree 3\nabcd\n", artifact.CodeGitObjectUnavailable, []string{"truncated"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := response(t, tc.input, treeObject)
			requireCode(t, err, tc.code)
			requireMessage(t, err, tc.parts...)
		})
	}
	_, _, err := response(t, testID+" blob 1048577\n", gitIgnoreObject)
	requireMessage(t, err, ".gitignore blob", "1 MiB limit")
	_, _, err = response(t, testID+" blob 4097\n", linkObject)
	requireMessage(t, err, "link target blob", "4096 bytes limit")
}

func FuzzReadResponse(f *testing.F) {
	f.Add([]byte(testID + " tree 3\nabc\n"))
	f.Add([]byte(testID + " missing\n"))
	f.Add([]byte(testID + " tree 70\n" + strings.Repeat("x", 70) + "\n"))
	f.Add([]byte(testID + " blob 2\nab\n"))
	f.Add([]byte(testID + " tree 5\nab"))
	kind := objectKind{typ: "tree", limit: 64, label: "fuzz tree"}
	f.Fuzz(func(t *testing.T, input []byte) {
		source := bytes.NewReader(input)
		reader := bufio.NewReaderSize(source, 16)
		data, err := readResponse(reader, testID, kind)
		if err != nil {
			var typed *artifact.Error
			if !asArtifactError(err, &typed) || typed.Internal {
				t.Fatalf("untyped or internal error %v", err)
			}
			return
		}
		header := bytes.IndexByte(input, '\n') + 1
		consumed := len(input) - source.Len() - reader.Buffered()
		if int64(len(data)) > kind.limit || consumed != header+len(data)+1 {
			t.Fatalf("data %d bytes, consumed %d, header %d", len(data), consumed, header)
		}
		if !bytes.Equal(data, input[header:header+len(data)]) || input[header+len(data)] != '\n' {
			t.Fatal("data does not match the input")
		}
	})
}

func asArtifactError(err error, target **artifact.Error) bool {
	return errors.As(err, target)
}

func entry(mode, name string, id byte, size int) []byte {
	out := append([]byte(mode+" "+name), 0)
	return append(out, bytes.Repeat([]byte{id}, size)...)
}

func TestParseTreeAccepts(t *testing.T) {
	var data []byte
	modes := []string{"40000", "100644", "100755", "100664", "120000", "160000"}
	want := []Mode{ModeTree, ModeFile, ModeFile, ModeFile, ModeLink, ModeGitlink}
	names := []string{"dir", "file", "tool", "group", "link", "module"}
	for i, mode := range modes {
		data = append(data, entry(mode, names[i], byte(i), 20)...)
	}
	// Names Git sorts oddly, NFD, a backslash and a Windows device name stay
	// as recorded.
	extra := []string{"-a", "!", "é", `a\b`, "CON", ".hidden", "..."}
	for _, name := range extra {
		data = append(data, entry("100644", name, 0xff, 20)...)
	}
	entries, err := ParseTree(data, SHA1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(modes)+len(extra) {
		t.Fatalf("%d entries", len(entries))
	}
	for i := range modes {
		if entries[i].Mode != want[i] || entries[i].Name != names[i] || entries[i].ID != strings.Repeat(string("0123456789abcdef"[i/16])+string("0123456789abcdef"[i%16]), 20) {
			t.Fatalf("entry %d: %+v", i, entries[i])
		}
	}
	for i, name := range extra {
		if got := entries[len(modes)+i]; got.Name != name || got.ID != strings.Repeat("ff", 20) {
			t.Fatalf("extra %d: %+v", i, got)
		}
	}

	sha256, err := ParseTree(entry("40000", "dir", 0x01, 32), SHA256)
	if err != nil || len(sha256) != 1 || sha256[0].ID != strings.Repeat("01", 32) {
		t.Fatalf("sha256: %+v %v", sha256, err)
	}
	if empty, err := ParseTree(nil, SHA1); err != nil || len(empty) != 0 {
		t.Fatalf("empty tree: %+v %v", empty, err)
	}
}

func TestParseTreeRejects(t *testing.T) {
	valid := entry("100644", "ok", 1, 20)
	cases := map[string][]byte{
		"zero-padded tree mode":   entry("040000", "dir", 1, 20),
		"unknown mode":            entry("100600", "file", 1, 20),
		"short mode":              entry("644", "file", 1, 20),
		"long mode":               entry("1006440", "file", 1, 20),
		"empty mode":              entry("", "file", 1, 20),
		"no space":                []byte("100644"),
		"no NUL":                  []byte("100644 name"),
		"truncated id":            entry("100644", "file", 1, 19),
		"sha256 read as sha1":     append(entry("100644", "file", 1, 32), valid...),
		"empty name":              entry("100644", "", 1, 20),
		"dot":                     entry("40000", ".", 1, 20),
		"dot dot":                 entry("40000", "..", 1, 20),
		"slash":                   entry("100644", "a/b", 1, 20),
		"invalid UTF-8":           entry("100644", "\xff", 1, 20),
		"duplicate":               append(append([]byte{}, valid...), valid...),
		"duplicate across modes":  append(entry("100644", "x", 1, 20), entry("40000", "x", 2, 20)...),
		"trailing garbage":        append(append([]byte{}, valid...), 'x'),
		"mode with trailing junk": entry("100644x", "file", 1, 20),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseTree(data, SHA1)
			requireCode(t, err, artifact.CodeGitTreeInvalid)
		})
	}
	_, err := ParseTree(entry("100600", "f", 1, 20), SHA1)
	requireMessage(t, err, `unsupported mode "100600"`)
	_, err = ParseTree(append(append([]byte{}, valid...), valid...), SHA1)
	requireMessage(t, err, `two entries named "ok"`)
	_, err = ParseTree(entry("100644", strings.Repeat("n", 1000)+"/", 1, 20), SHA1)
	if len(err.Error()) > 400 {
		t.Fatalf("unbounded message: %d bytes", len(err.Error()))
	}
}

func FuzzParseTree(f *testing.F) {
	f.Add(entry("100644", "file", 1, 20))
	f.Add(append(entry("40000", "dir", 1, 20), entry("120000", "link", 2, 20)...))
	f.Add(entry("160000", "module", 3, 20))
	f.Add(entry("100644", "é", 4, 19))
	f.Fuzz(func(t *testing.T, data []byte) {
		entries, err := ParseTree(data, SHA1)
		if err != nil {
			var typed *artifact.Error
			if !asArtifactError(err, &typed) || typed.Code != artifact.CodeGitTreeInvalid {
				t.Fatalf("unexpected error %v", err)
			}
			return
		}
		total := 0
		seen := make(map[string]bool)
		for _, e := range entries {
			modeLength := 6
			if e.Mode == ModeTree {
				modeLength = 5
			}
			total += modeLength + 1 + len(e.Name) + 1 + SHA1.RawSize
			if !SHA1.isObjectID(e.ID) || seen[e.Name] {
				t.Fatalf("entry %+v", e)
			}
			if _, err := artifact.CanonicalizeName(e.Name); err != nil {
				t.Fatalf("accepted name %q: %v", e.Name, err)
			}
			seen[e.Name] = true
		}
		if total != len(data) {
			t.Fatalf("entries cover %d of %d bytes", total, len(data))
		}
	})
}

func FuzzParseVersion(f *testing.F) {
	f.Add("git version 2.56.0.windows.2\n")
	f.Add("git version 2.39.5 (Apple Git-154)\n")
	f.Add("git version 2.45")
	f.Fuzz(func(t *testing.T, output string) {
		version, ok := ParseVersion(output)
		if !ok {
			return
		}
		if version.Major < 0 || version.Minor < 0 || !strings.HasPrefix(output, "git version "+version.Text) {
			t.Fatalf("%q parsed as %+v", output, version)
		}
		prefix := strconv.Itoa(version.Major) + "." + strconv.Itoa(version.Minor)
		if !strings.HasPrefix(version.Text, prefix) {
			t.Fatalf("%q parsed as %+v", output, version)
		}
	})
}
