package catalog

import (
	"reflect"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

func TestV033ClassificationPromotions(t *testing.T) {
	corrections := v033ClassificationCorrections()
	if len(corrections) != 6 {
		t.Fatalf("v0.3.3 corrections = %d, want 6", len(corrections))
	}
	seen := map[string]struct{}{}
	for _, correction := range corrections {
		if _, duplicate := seen[correction.Name]; duplicate {
			t.Fatalf("duplicate correction %q", correction.Name)
		}
		seen[correction.Name] = struct{}{}
		got := Classify(correction.Name, correction.Name, tree.NodeFile)
		if got.Source != SourceFilename || got.MatcherKey != correction.Current.MatcherKey {
			t.Errorf("%s matchedBy = %s/%s, want filename/%s", correction.Name, got.Source, got.MatcherKey, correction.Current.MatcherKey)
		}
		if !reflect.DeepEqual(got, correction.Current) {
			t.Errorf("%s = %#v, want %#v (%s)", correction.Name, got, correction.Current, correction.Justification)
		}
		if reflect.DeepEqual(got, correction.Previous) {
			t.Errorf("%s was not promoted away from %#v", correction.Name, correction.Previous)
		}
		if !containsRole(got.Roles, RoleInfra) {
			t.Errorf("%s lost infra role: %#v", correction.Name, got.Roles)
		}
		if correction.Name == ".terraform.lock.hcl" && !containsRole(got.Roles, RoleLock) {
			t.Errorf("terraform lock lost lock role: %#v", got.Roles)
		}
	}
}

func TestV033ClassificationCorrectionsAreTheOnlyKindChanges(t *testing.T) {
	allowed := v033CorrectionByMatcherIdentity()
	if len(allowed) != 6 {
		t.Fatalf("correction identities = %d", len(allowed))
	}
	changed := 0
	for _, entry := range Entries() {
		identity := string(entry.Matcher.Source) + ":" + string(entry.Matcher.Value)
		if _, ok := allowed[identity]; !ok {
			continue
		}
		changed++
		want := allowed[identity]
		if entry.Kind != want.Kind || !reflect.DeepEqual(entry.Roles, want.Roles) || entry.Matcher.Source != want.Source || entry.Matcher.Value != want.MatcherKey {
			t.Errorf("%s matcher contract = %#v, want %#v", identity, entry, want)
		}
	}
	if changed != 6 {
		t.Fatalf("catalog contains %d v0.3.3 corrections, want 6", changed)
	}
}

func TestV033ClassificationNegativeControlsAndPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		kind    Kind
		source  MatchSource
		matcher string
	}{
		{"random.yaml", "data.yaml", SourceExtension, ".yaml"},
		{"settings.hcl", "source.hcl", SourceExtension, ".hcl"},
		{"application.hcl", "source.hcl", SourceExtension, ".hcl"},
		{"Dockerfile", "manifest.container", SourceFilename, "dockerfile"},
		{"Containerfile", "manifest.container", SourceFilename, "containerfile"},
		{"main.tf", "manifest.terraform", SourceExtension, ".tf"},
		{"variables.tf", "manifest.terraform", SourceExtension, ".tf"},
		{"terraform.tfvars", "manifest.terraform", SourceExtension, ".tfvars"},
	}
	for _, test := range tests {
		got := Classify(test.name, test.name, tree.NodeFile)
		if got.Kind != test.kind || got.Source != test.source || got.MatcherKey != test.matcher {
			t.Errorf("%s = %#v, want kind=%s matchedBy=%s/%s", test.name, got, test.kind, test.source, test.matcher)
		}
	}
	lock := Classify("random.lock", "random.lock", tree.NodeFile)
	if lock.Kind == "manifest.terraform" {
		t.Fatalf("random.lock must not become terraform: %#v", lock)
	}
	compose := Classify("docker-compose.yml", "docker-compose.yml", tree.NodeFile)
	genericYAML := Classify("random.yaml", "random.yaml", tree.NodeFile)
	if compose.Source != SourceFilename || genericYAML.Source != SourceExtension {
		t.Fatalf("filename must beat extension: compose=%#v yaml=%#v", compose, genericYAML)
	}
	bake := Classify("docker-bake.hcl", "docker-bake.hcl", tree.NodeFile)
	genericHCL := Classify("settings.hcl", "settings.hcl", tree.NodeFile)
	if bake.Source != SourceFilename || genericHCL.Source != SourceExtension || genericHCL.Kind != "source.hcl" {
		t.Fatalf("bake filename must beat generic hcl: bake=%#v hcl=%#v", bake, genericHCL)
	}
}
