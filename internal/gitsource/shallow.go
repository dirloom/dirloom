package gitsource

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// maxShallowList bounds the shallow boundary list (Structural History v1).
const maxShallowList = 16 << 20

// ShallowBoundaries returns the commits Git records as shallow boundaries:
// the file `rev-parse --git-path shallow` names, read once. An absent file
// means no boundary. It is the only file read directly, and it never changes
// a walk: it only explains a missing parent.
func (s *Session) ShallowBoundaries(ctx context.Context) (map[string]struct{}, error) {
	out, err := s.runner.output(ctx, "rev-parse", "--git-path", "shallow")
	if err != nil {
		return nil, s.runner.classify(ctx, err, shallowInvalid("git did not name the shallow boundary list"))
	}
	path := singleLine(out)
	if path == "" {
		return nil, shallowInvalid("git named an empty shallow boundary list path")
	}
	if !filepath.IsAbs(path) {
		base := s.runner.dir
		if base == "" {
			if base, err = os.Getwd(); err != nil {
				return nil, shallowInvalid("the shallow boundary list cannot be located")
			}
		}
		path = filepath.Join(base, path)
	}
	data, err := readBounded(path, maxShallowList)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]struct{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	return parseShallow(data, s.format)
}

func readBounded(path string, limit int64) ([]byte, error) {
	// #nosec G304 -- the path is the shallow boundary list Git named for this repository.
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return nil, shallowInvalid("the shallow boundary list cannot be read")
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, shallowInvalid("the shallow boundary list cannot be read")
	}
	if int64(len(data)) > limit {
		return nil, shallowInvalid("the shallow boundary list is above the %s limit", formatLimit(limit))
	}
	return data, nil
}

// parseShallow accepts one full object id per line, each line terminated by
// a newline except possibly the last, and nothing else.
func parseShallow(data []byte, format ObjectFormat) (map[string]struct{}, error) {
	boundaries := make(map[string]struct{})
	for rest := data; len(rest) > 0; {
		var line []byte
		if end := bytes.IndexByte(rest, '\n'); end >= 0 {
			line, rest = rest[:end], rest[end+1:]
		} else {
			line, rest = rest, nil
		}
		if !format.isObjectID(string(line)) {
			return nil, shallowInvalid("the shallow boundary list holds a line that is not a full object id")
		}
		boundaries[string(line)] = struct{}{}
	}
	return boundaries, nil
}
