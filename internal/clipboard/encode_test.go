package clipboard

import (
	"bytes"
	"testing"
)

func TestUTF16LEEncoding(t *testing.T) {
	got := utf16LEWithNUL([]byte("A\n"))
	want := []byte{'A', 0, '\n', 0, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("utf16LEWithNUL = %v, want %v", got, want)
	}
	withBOM := utf16LEWithBOM([]byte("A"))
	if !bytes.HasPrefix(withBOM, []byte{0xff, 0xfe}) || !bytes.Equal(withBOM[2:4], []byte{'A', 0}) {
		t.Fatalf("utf16LEWithBOM = %v", withBOM)
	}
}

func TestBufferStoresExactBytes(t *testing.T) {
	payload := []byte("tree/\n└── file\n")
	var buffer Buffer
	if err := buffer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer.Data, payload) {
		t.Fatalf("buffer = %q", buffer.Data)
	}
	payload[0] = 'X'
	if bytes.Equal(buffer.Data, payload) {
		t.Fatal("buffer must store a copy")
	}
}
