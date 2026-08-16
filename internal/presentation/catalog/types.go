// Package catalog classifies filesystem entries into stable technical kinds and structural roles.
package catalog

import (
	"sort"

	"github.com/dirloom/dirloom/internal/tree"
)

const (
	// Version is the semantic catalog contract shipped with Dirloom v0.2.
	Version = 1

	EntryCount = 256
	KindCount  = 96
	RoleCount  = 16
)

// Kind identifies the technical nature of an entry.
type Kind string

// Role identifies one structural function of an entry.
type Role string

// MatchSource explains which catalog matcher selected a kind.
type MatchSource string

const (
	SourceNodeType  MatchSource = "node-type"
	SourceFilename  MatchSource = "filename"
	SourceDirectory MatchSource = "directory"
	SourceSuffix    MatchSource = "suffix"
	SourceExtension MatchSource = "extension"
	SourceFallback  MatchSource = "fallback"
)

const (
	RoleSecurity   Role = "security"
	RoleGenerated  Role = "generated"
	RoleVendor     Role = "vendor"
	RoleTest       Role = "test"
	RoleContract   Role = "contract"
	RoleLock       Role = "lock"
	RoleInfra      Role = "infra"
	RoleConfig     Role = "config"
	RoleExecutable Role = "executable"
	RoleArchive    Role = "archive"
	RoleMedia      Role = "media"
	RoleData       Role = "data"
	RoleSource     Role = "source"
	RoleDocument   Role = "document"
	RoleTooling    Role = "tooling"
	RoleGeneric    Role = "generic"
)

var canonicalRoles = []Role{
	RoleSecurity, RoleGenerated, RoleVendor, RoleTest,
	RoleContract, RoleLock, RoleInfra, RoleConfig,
	RoleExecutable, RoleArchive, RoleMedia, RoleData,
	RoleSource, RoleDocument, RoleTooling, RoleGeneric,
}

var roleRank = func() map[Role]int {
	result := make(map[Role]int, len(canonicalRoles))
	for index, role := range canonicalRoles {
		result[role] = index
	}
	return result
}()

// Matcher identifies one immutable built-in catalog match.
type Matcher struct {
	Source MatchSource `json:"source" yaml:"source"`
	Value  string      `json:"value" yaml:"value"`
}

// Entry maps a matcher to a technical kind and ordered structural roles.
type Entry struct {
	Matcher Matcher `json:"matcher" yaml:"matcher"`
	Kind    Kind    `json:"kind" yaml:"kind"`
	Roles   []Role  `json:"roles" yaml:"roles"`
}

// KindDefinition defines inheritance and portable glyph fallbacks.
type KindDefinition struct {
	Kind    Kind   `json:"kind" yaml:"kind"`
	Parent  Kind   `json:"parent,omitempty" yaml:"parent,omitempty"`
	Unicode string `json:"unicode" yaml:"unicode"`
	Nerd    string `json:"nerd" yaml:"nerd"`
}

// Classification is the deterministic result for one filesystem entry.
type Classification struct {
	Kind       Kind        `json:"kind"`
	Roles      []Role      `json:"roles"`
	Source     MatchSource `json:"matchedBy"`
	MatcherKey string      `json:"matcherKey"`
}

// Roles returns the public role order defensively.
func Roles() []Role {
	return append([]Role(nil), canonicalRoles...)
}

// IsRole reports whether a role is in catalog v1.
func IsRole(value string) bool {
	_, ok := roleRank[Role(value)]
	return ok
}

func normalizeRoles(values []Role) []Role {
	seen := make(map[Role]struct{}, len(values))
	result := make([]Role, 0, len(values))
	for _, value := range values {
		if _, ok := roleRank[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.SliceStable(result, func(left, right int) bool {
		return roleRank[result[left]] < roleRank[result[right]]
	})
	if len(result) == 0 {
		return []Role{RoleGeneric}
	}
	return result
}

func cloneEntry(value Entry) Entry {
	value.Roles = append([]Role(nil), value.Roles...)
	return value
}

// NodeType is accepted as tree.NodeType so the classifier remains aligned with
// the canonical model without depending on presentation or rendering.
type NodeType = tree.NodeType
