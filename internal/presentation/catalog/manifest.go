package catalog

import "fmt"

type entrySpec struct {
	value string
	kind  Kind
	roles []Role
}

func spec(value string, kind Kind, roles ...Role) entrySpec {
	return entrySpec{value: value, kind: kind, roles: roles}
}

var manifest = buildManifest()

func buildManifest() []Entry {
	filenames := filenameSpecs()
	directories := directorySpecs()
	suffixes := suffixSpecs()
	extensions := extensionSpecs()
	if len(filenames) != FilenameEntryCount || len(directories) != DirectoryEntryCount || len(suffixes) != SuffixEntryCount || len(extensions) != ExtensionEntryCount {
		panic(fmt.Sprintf("invalid semantic catalog groups: filenames=%d directories=%d suffixes=%d extensions=%d", len(filenames), len(directories), len(suffixes), len(extensions)))
	}
	result := make([]Entry, 0, EntryCount)
	appendSpecs := func(source MatchSource, values []entrySpec) {
		for _, value := range values {
			result = append(result, Entry{Matcher: Matcher{Source: source, Value: value.value}, Kind: value.kind, Roles: normalizeRoles(value.roles)})
		}
	}
	appendSpecs(SourceFilename, filenames)
	appendSpecs(SourceDirectory, directories)
	appendSpecs(SourceSuffix, suffixes)
	appendSpecs(SourceExtension, extensions)
	return result
}

// Entries returns a defensive copy of the semantic catalog manifest.
func Entries() []Entry {
	result := make([]Entry, len(manifest))
	for index, value := range manifest {
		result[index] = cloneEntry(value)
	}
	return result
}
