package catalog

import (
	"path/filepath"
	"strings"

	"github.com/dirloom/dirloom/internal/tree"
)

type suffixNode struct {
	children map[rune]*suffixNode
	entry    *Entry
}

type catalogIndexes struct {
	filenames   map[string]Entry
	directories map[string]Entry
	extensions  map[string]Entry
	suffixes    *suffixNode
}

var indexes = buildIndexes()

func buildIndexes() catalogIndexes {
	result := catalogIndexes{
		filenames: make(map[string]Entry), directories: make(map[string]Entry),
		extensions: make(map[string]Entry), suffixes: &suffixNode{children: make(map[rune]*suffixNode)},
	}
	for _, entry := range manifest {
		key := strings.ToLower(entry.Matcher.Value)
		switch entry.Matcher.Source {
		case SourceFilename:
			result.filenames[key] = entry
		case SourceDirectory:
			result.directories[key] = entry
		case SourceExtension:
			result.extensions[key] = entry
		case SourceSuffix:
			node := result.suffixes
			runes := []rune(key)
			for index := len(runes) - 1; index >= 0; index-- {
				if node.children[runes[index]] == nil {
					node.children[runes[index]] = &suffixNode{children: make(map[rune]*suffixNode)}
				}
				node = node.children[runes[index]]
			}
			copyEntry := cloneEntry(entry)
			node.entry = &copyEntry
		}
	}
	return result
}

// Classify applies the stable catalog precedence to one canonical tree entry.
func Classify(name, relativePath string, nodeType tree.NodeType) Classification {
	_ = relativePath // Reserved for future catalog versions; theme rules use it today.
	switch nodeType {
	case tree.NodeSymlink:
		return Classification{Kind: "symlink", Roles: []Role{RoleGeneric}, Source: SourceNodeType, MatcherKey: "symlink"}
	case tree.NodeDirectory:
		if entry, ok := indexes.directories[strings.ToLower(name)]; ok {
			return classificationFrom(entry)
		}
		return Classification{Kind: "directory", Roles: []Role{RoleGeneric}, Source: SourceFallback, MatcherKey: "directory"}
	default:
		lower := strings.ToLower(name)
		if entry, ok := indexes.filenames[lower]; ok {
			return classificationFrom(entry)
		}
		if entry, ok := matchSuffix(lower); ok {
			return classificationFrom(entry)
		}
		if entry, ok := indexes.extensions[strings.ToLower(filepath.Ext(name))]; ok {
			return classificationFrom(entry)
		}
		return Classification{Kind: "file", Roles: []Role{RoleGeneric}, Source: SourceFallback, MatcherKey: "file"}
	}
}

func matchSuffix(name string) (Entry, bool) {
	node := indexes.suffixes
	var matched *Entry
	runes := []rune(name)
	for index := len(runes) - 1; index >= 0; index-- {
		next := node.children[runes[index]]
		if next == nil {
			break
		}
		node = next
		if node.entry != nil {
			value := cloneEntry(*node.entry)
			matched = &value
		}
	}
	if matched == nil {
		return Entry{}, false
	}
	return *matched, true
}

func classificationFrom(entry Entry) Classification {
	return Classification{Kind: entry.Kind, Roles: append([]Role(nil), entry.Roles...), Source: entry.Matcher.Source, MatcherKey: entry.Matcher.Value}
}
