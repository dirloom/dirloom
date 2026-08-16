package catalog

import (
	"fmt"
	"strings"
)

// Validate checks every compiled-in catalog invariant.
func Validate() error {
	if len(manifest) != EntryCount {
		return fmt.Errorf("catalog has %d entries; expected %d", len(manifest), EntryCount)
	}
	if len(canonicalRoles) != RoleCount {
		return fmt.Errorf("catalog has %d roles; expected %d", len(canonicalRoles), RoleCount)
	}
	if err := validateKinds(); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(manifest))
	counts := make(map[MatchSource]int)
	for _, entry := range manifest {
		if _, ok := kindRegistry[entry.Kind]; !ok {
			return fmt.Errorf("matcher %s:%s references unknown kind %q", entry.Matcher.Source, entry.Matcher.Value, entry.Kind)
		}
		if len(entry.Roles) == 0 {
			return fmt.Errorf("matcher %s:%s has no role", entry.Matcher.Source, entry.Matcher.Value)
		}
		for _, role := range entry.Roles {
			if !IsRole(string(role)) {
				return fmt.Errorf("matcher %s:%s references unknown role %q", entry.Matcher.Source, entry.Matcher.Value, role)
			}
		}
		identity := string(entry.Matcher.Source) + ":" + strings.ToLower(entry.Matcher.Value)
		if _, duplicate := seen[identity]; duplicate {
			return fmt.Errorf("duplicate catalog matcher %s", identity)
		}
		seen[identity] = struct{}{}
		counts[entry.Matcher.Source]++
	}
	expected := map[MatchSource]int{SourceFilename: 64, SourceDirectory: 40, SourceSuffix: 32, SourceExtension: 120}
	for source, count := range expected {
		if counts[source] != count {
			return fmt.Errorf("catalog has %d %s matchers; expected %d", counts[source], source, count)
		}
	}
	return nil
}
