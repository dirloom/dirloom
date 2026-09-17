package artifact

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func TestCanonicalizePOSIXSeparator(t *testing.T) {
	got, err := Canonicalize("src/core/main.go", POSIXSeparator)
	if err != nil || got != "src/core/main.go" {
		t.Fatalf("got %q (%v)", got, err)
	}
}

func TestCanonicalizeWindowsSeparator(t *testing.T) {
	got, err := Canonicalize(`src\core\main.go`, WindowsSeparator)
	if err != nil || got != "src/core/main.go" {
		t.Fatalf("got %q (%v)", got, err)
	}
	mixed, err := Canonicalize(`src/core\util.go`, WindowsSeparator)
	if err != nil || mixed != "src/core/util.go" {
		t.Fatalf("mixed got %q (%v)", mixed, err)
	}
}

func TestCanonicalizePOSIXKeepsBackslashInName(t *testing.T) {
	got, err := Canonicalize(`src\core`, POSIXSeparator)
	if err != nil || got != `src\core` {
		t.Fatalf("POSIX backslash should be a name character, got %q (%v)", got, err)
	}
}

func TestCanonicalizeSpacesLeadingDotsAndUTF8(t *testing.T) {
	cases := []struct {
		in   string
		want Path
	}{
		{"my file.txt", "my file.txt"},
		{".env", ".env"},
		{".hidden/file", ".hidden/file"},
		{"你好.txt", "你好.txt"},
		{"src/./lib", "src/lib"},
		{"./src", "src"},
		{".", "."},
	}
	for _, test := range cases {
		got, err := Canonicalize(test.in, POSIXSeparator)
		if err != nil || got != test.want {
			t.Errorf("%q -> %q (%v), want %q", test.in, got, err, test.want)
		}
	}
}

func TestCanonicalizeNFDToNFC(t *testing.T) {
	nfd := "cafe\u0301.txt"
	got, err := Canonicalize(nfd, POSIXSeparator)
	if err != nil {
		t.Fatal(err)
	}
	want := Path(norm.NFC.String(nfd))
	if got != want || got != "café.txt" && string(got) != norm.NFC.String(nfd) {
		t.Fatalf("NFC(%q) = %q, want %q", nfd, got, want)
	}
	if string(got) != norm.NFC.String(string(got)) {
		t.Fatal("result is not NFC")
	}
}

func TestCanonicalizeNFCIdempotent(t *testing.T) {
	first, err := Canonicalize("café.txt", POSIXSeparator)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Canonicalize(string(first), POSIXSeparator)
	if err != nil || first != second {
		t.Fatalf("idempotence: %q vs %q (%v)", first, second, err)
	}
}

func TestCanonicalizePreservesCase(t *testing.T) {
	left, err := Canonicalize("Auth", POSIXSeparator)
	if err != nil {
		t.Fatal(err)
	}
	right, err := Canonicalize("auth", POSIXSeparator)
	if err != nil {
		t.Fatal(err)
	}
	if left == right || left != "Auth" || right != "auth" {
		t.Fatalf("case collapsed: %q %q", left, right)
	}
}

func TestCanonicalizeVeryLongName(t *testing.T) {
	name := strings.Repeat("a", 255)
	got, err := Canonicalize(name, POSIXSeparator)
	if err != nil || got != Path(name) {
		t.Fatalf("long name: %v %d", err, len(got))
	}
}

func TestCanonicalizeDeepRelativePath(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString("d")
	}
	got, err := Canonicalize(b.String(), POSIXSeparator)
	if err != nil || got != Path(b.String()) {
		t.Fatalf("deep: %q %v", got, err)
	}
}

func TestCanonicalizeRejectsTraversalAndAbsolute(t *testing.T) {
	cases := []struct {
		in  string
		sep byte
	}{
		{"..", POSIXSeparator},
		{"../src", POSIXSeparator},
		{"a/../../b", POSIXSeparator},
		{"/src", POSIXSeparator},
		{`C:\src`, WindowsSeparator},
		{`\\server\share`, WindowsSeparator},
		{"", POSIXSeparator},
	}
	for _, test := range cases {
		if _, err := Canonicalize(test.in, test.sep); err == nil {
			t.Errorf("accepted %q", test.in)
		}
	}
}

func TestCanonicalizeLexicalDotDotInsideRoot(t *testing.T) {
	got, err := Canonicalize("src/../lib/main.go", POSIXSeparator)
	if err != nil || got != "lib/main.go" {
		t.Fatalf("got %q (%v)", got, err)
	}
}

func TestCollisionSetUnicode(t *testing.T) {
	set := NewCollisionSet()
	nfc, err := Canonicalize("café.txt", POSIXSeparator)
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Add("café.txt", nfc); err != nil {
		t.Fatal(err)
	}
	nfd := "cafe\u0301.txt"
	nfdPath, err := Canonicalize(nfd, POSIXSeparator)
	if err != nil {
		t.Fatal(err)
	}
	err = set.Add(nfd, nfdPath)
	if err == nil {
		t.Fatal("expected Unicode collision")
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeCollision {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestCanonicalizeRejectsNULAndInvalidUTF8(t *testing.T) {
	if _, err := Canonicalize("a\x00b", POSIXSeparator); err == nil {
		t.Fatal("NUL accepted")
	}
	invalid := string([]byte{0xff, 0xfe, 0xfd})
	if utf8.ValidString(invalid) {
		t.Fatal("fixture is valid UTF-8")
	}
	if _, err := Canonicalize(invalid, POSIXSeparator); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}
