package gitsource

import (
	"bytes"
	"encoding/hex"

	"github.com/dirloom/dirloom/internal/artifact"
)

// Mode is a tree entry mode accepted by Git Tree Source v1.
type Mode uint8

const (
	// ModeTree is 40000: a directory.
	ModeTree Mode = iota + 1
	// ModeFile is 100644, 100755 or 100664: a file. The executable bit is not
	// identity.
	ModeFile
	// ModeLink is 120000: a symbolic link whose blob is the target.
	ModeLink
	// ModeGitlink is 160000: a submodule, an empty directory never entered.
	ModeGitlink
)

func parseMode(raw []byte) (Mode, bool) {
	switch string(raw) {
	case "40000":
		return ModeTree, true
	case "100644", "100755", "100664":
		return ModeFile, true
	case "120000":
		return ModeLink, true
	case "160000":
		return ModeGitlink, true
	default:
		return 0, false
	}
}

// TreeEntry is one validated tree entry. Name is as recorded; ID is
// lowercase hex.
type TreeEntry struct {
	Mode Mode
	Name string
	ID   string
}

// maxModeLength is the longest accepted mode; a space must follow it.
const maxModeLength = 6

// ParseTree parses a raw tree object strictly: entries are
// `<mode> SP <name> NUL <raw id>`. Names pass canonical validation but are
// returned as recorded, since NFC normalization and sibling collisions belong
// to artifact construction.
func ParseTree(data []byte, format ObjectFormat) ([]TreeEntry, error) {
	var entries []TreeEntry
	seen := make(map[string]struct{})
	for offset := 0; offset < len(data); {
		window := data[offset:min(len(data), offset+maxModeLength+1)]
		space := bytes.IndexByte(window, ' ')
		if space <= 0 {
			return nil, treeInvalid("tree has a malformed entry at byte %d", offset)
		}
		mode, ok := parseMode(window[:space])
		if !ok {
			return nil, treeInvalid("tree has an entry with unsupported mode %s", quoteToken(string(window[:space])))
		}
		nameStart := offset + space + 1
		nameLength := bytes.IndexByte(data[nameStart:], 0)
		if nameLength < 0 {
			return nil, treeInvalid("tree has a malformed entry at byte %d", offset)
		}
		name := string(data[nameStart : nameStart+nameLength])
		idStart := nameStart + nameLength + 1
		idEnd := idStart + format.RawSize
		if idEnd > len(data) {
			return nil, treeInvalid("tree has a truncated entry at byte %d", offset)
		}
		if _, err := artifact.CanonicalizeName(name); err != nil {
			return nil, treeInvalid("tree has an entry with invalid name %s", quoteName(name))
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, treeInvalid("tree has two entries named %s", quoteName(name))
		}
		seen[name] = struct{}{}
		entries = append(entries, TreeEntry{Mode: mode, Name: name, ID: hex.EncodeToString(data[idStart:idEnd])})
		offset = idEnd
	}
	return entries, nil
}
