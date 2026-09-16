package clipboard

import (
	"encoding/binary"
	"unicode/utf16"
)

func utf16LEWithNUL(data []byte) []byte {
	encoded := utf16.Encode([]rune(string(data)))
	out := make([]byte, 2*(len(encoded)+1))
	for index, unit := range encoded {
		binary.LittleEndian.PutUint16(out[index*2:], unit)
	}
	return out
}

func utf16LEWithBOM(data []byte) []byte {
	payload := utf16LEWithNUL(data)
	out := make([]byte, 2+len(payload))
	out[0], out[1] = 0xff, 0xfe
	copy(out[2:], payload)
	return out
}
