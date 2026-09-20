package identity

import (
	"encoding/binary"
	"io"
	"math"

	"github.com/dirloom/dirloom/internal/artifact"
)

const (
	magic               = "DLMI"
	flagHasTarget  byte = 0x01
	maxRecordBytes      = 1 << 30
)

// EncodeV1 writes Canonical Identity Encoding v1 to writer. Records must
// already be Identity Projection v1 (sorted). Writer errors are returned.
func EncodeV1(writer io.Writer, records []Record) error {
	count, err := boundedU32(len(records))
	if err != nil {
		return err
	}
	if err := writeFull(writer, []byte(magic)); err != nil {
		return artifact.EncodingFailure(err)
	}
	if err := writeFull(writer, []byte{byte(ProjectionVersion), byte(EncodingVersion)}); err != nil {
		return artifact.EncodingFailure(err)
	}
	if err := writeU32(writer, count); err != nil {
		return artifact.EncodingFailure(err)
	}
	for _, record := range records {
		if err := encodeRecord(writer, record); err != nil {
			return err
		}
	}
	return nil
}

func encodeRecord(writer io.Writer, record Record) error {
	if err := writeString(writer, string(record.Path)); err != nil {
		return err
	}
	code, err := record.Kind.Code()
	if err != nil {
		return err
	}
	flags := byte(0)
	if record.Kind.HasTarget() {
		flags |= flagHasTarget
	}
	if err := writeFull(writer, []byte{code, flags}); err != nil {
		return artifact.EncodingFailure(err)
	}
	if flags&flagHasTarget != 0 {
		if err := writeString(writer, record.Target); err != nil {
			return err
		}
	}
	return nil
}

func writeString(writer io.Writer, value string) error {
	if len(value) > maxRecordBytes {
		return artifact.InternalError("identity string exceeds encoding limit")
	}
	length, err := boundedU32(len(value))
	if err != nil {
		return err
	}
	if err := writeU32(writer, length); err != nil {
		return artifact.EncodingFailure(err)
	}
	if err := writeFull(writer, []byte(value)); err != nil {
		return artifact.EncodingFailure(err)
	}
	return nil
}

func boundedU32(n int) (uint32, error) {
	if n < 0 || uint64(n) > math.MaxUint32 {
		return 0, artifact.InternalError("identity length exceeds encoding limit")
	}
	return uint32(n), nil //nolint:gosec // n is checked against math.MaxUint32
}

func writeU32(writer io.Writer, value uint32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], value)
	return writeFull(writer, buf[:])
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
