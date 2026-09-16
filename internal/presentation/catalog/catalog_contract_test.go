package catalog

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
	"go.yaml.in/yaml/v3"
)

type classificationFixture struct {
	ID           string        `yaml:"id"`
	Name         string        `yaml:"name"`
	RelativePath string        `yaml:"relativePath"`
	Type         tree.NodeType `yaml:"type"`
	Kind         Kind          `yaml:"kind"`
	Roles        []Role        `yaml:"roles"`
	Source       MatchSource   `yaml:"matchedBy"`
	MatcherKey   string        `yaml:"matcherKey"`
}

type classificationFixtureDocument struct {
	CatalogVersion int                     `yaml:"catalogVersion"`
	Cases          []classificationFixture `yaml:"cases"`
}

func TestCatalogV1ContractAndIndependentFixtures(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/classification-v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document classificationFixtureDocument
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.CatalogVersion != Version || len(document.Cases) != EntryCount {
		t.Fatalf("fixture contract = version %d, cases %d", document.CatalogVersion, len(document.Cases))
	}
	seenIDs := make(map[string]struct{}, len(document.Cases))
	seenMatchers := make(map[string]struct{}, len(document.Cases))
	counts := make(map[MatchSource]int)
	for _, fixture := range document.Cases {
		if _, duplicate := seenIDs[fixture.ID]; duplicate || fixture.ID == "" {
			t.Fatalf("duplicate or empty fixture id %q", fixture.ID)
		}
		seenIDs[fixture.ID] = struct{}{}
		identity := string(fixture.Source) + ":" + strings.ToLower(fixture.MatcherKey)
		if _, duplicate := seenMatchers[identity]; duplicate {
			t.Fatalf("duplicate fixture matcher %q", identity)
		}
		seenMatchers[identity] = struct{}{}
		counts[fixture.Source]++
		got := Classify(fixture.Name, fixture.RelativePath, fixture.Type)
		want := Classification{Kind: fixture.Kind, Roles: fixture.Roles, Source: fixture.Source, MatcherKey: fixture.MatcherKey}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v, want %#v", fixture.ID, got, want)
		}
	}
	wantCounts := map[MatchSource]int{SourceFilename: 64, SourceDirectory: 40, SourceSuffix: 32, SourceExtension: 120}
	if !reflect.DeepEqual(counts, wantCounts) {
		t.Fatalf("fixture groups = %#v, want %#v", counts, wantCounts)
	}
}

func TestCatalogRegistryRolesAndDefensiveCopies(t *testing.T) {
	if got := len(Kinds()); got != KindCount {
		t.Fatalf("kinds = %d", got)
	}
	if got, want := Roles(), []Role{
		RoleSecurity, RoleGenerated, RoleVendor, RoleTest, RoleContract, RoleLock,
		RoleInfra, RoleConfig, RoleExecutable, RoleArchive, RoleMedia, RoleData,
		RoleSource, RoleDocument, RoleTooling, RoleGeneric,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("roles = %#v, want %#v", got, want)
	}
	for _, definition := range Kinds() {
		if chain := KindChain(definition.Kind); len(chain) == 0 || len(chain) > 4 {
			t.Errorf("kind %s chain = %#v", definition.Kind, chain)
		}
		unicodeGlyph, _ := Glyphs(definition.Kind)
		if unicodeGlyph == "" {
			t.Errorf("kind %s has no Unicode fallback", definition.Kind)
		}
	}

	entries := Entries()
	entries[0].Roles[0] = RoleGeneric
	entries[0].Matcher.Value = "mutated"
	if fresh := Entries()[0]; fresh.Matcher.Value == "mutated" || fresh.Roles[0] == RoleGeneric {
		t.Fatal("entries are mutable through returned values")
	}
	kinds := Kinds()
	kinds[0].Unicode = "X"
	if fresh, _ := LookupKind(kinds[0].Kind); fresh.Unicode == "X" {
		t.Fatal("kind registry is mutable through returned values")
	}
	roles := Roles()
	roles[0] = RoleGeneric
	if Roles()[0] != RoleSecurity {
		t.Fatal("role registry is mutable through returned values")
	}
}

func TestClassificationPrecedenceCaseFoldingAndFallback(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		nodeType tree.NodeType
		kind     Kind
		roles    []Role
		source   MatchSource
		matcher  string
	}{
		{"README.MD", "README.MD", tree.NodeFile, "document.markdown", []Role{RoleContract, RoleDocument}, SourceFilename, "readme.md"},
		{"USER_TEST.GO", "internal/USER_TEST.GO", tree.NodeFile, "source.go", []Role{RoleTest, RoleSource}, SourceSuffix, "_test.go"},
		{"types.D.MTS", "types.D.MTS", tree.NodeFile, "source.typescript", []Role{RoleGenerated, RoleContract, RoleSource}, SourceSuffix, ".d.mts"},
		{"logo.PNG", "assets/logo.PNG", tree.NodeFile, "media.image.png", []Role{RoleMedia}, SourceExtension, ".png"},
		{"NODE_MODULES", "NODE_MODULES", tree.NodeDirectory, "directory", []Role{RoleVendor}, SourceDirectory, "node_modules"},
		{"README.md", "README.md", tree.NodeSymlink, "symlink", []Role{RoleGeneric}, SourceNodeType, "symlink"},
		{"unknown.名", "unknown.名", tree.NodeFile, "file", []Role{RoleGeneric}, SourceFallback, "file"},
		{"unknown", "unknown", tree.NodeDirectory, "directory", []Role{RoleGeneric}, SourceFallback, "directory"},
	}
	for _, test := range tests {
		got := Classify(test.name, test.path, test.nodeType)
		if got.Kind != test.kind || got.Source != test.source || got.MatcherKey != test.matcher || !reflect.DeepEqual(got.Roles, test.roles) {
			t.Errorf("%s: %#v", test.path, got)
		}
	}
}

func FuzzClassifyIsDeterministic(f *testing.F) {
	for _, seed := range []string{"README.md", "src/main.go", "types.d.ts", "Ω/名.go", "", ".."} {
		f.Add(seed, seed)
	}
	f.Fuzz(func(t *testing.T, name, path string) {
		first := Classify(name, path, tree.NodeFile)
		second := Classify(name, path, tree.NodeFile)
		if !reflect.DeepEqual(first, second) || first.Kind == "" || len(first.Roles) == 0 {
			t.Fatalf("non-deterministic classification: %#v / %#v", first, second)
		}
	})
}

func BenchmarkClassifyExact(b *testing.B) {
	for range b.N {
		_ = Classify("README.md", "README.md", tree.NodeFile)
	}
}

func BenchmarkClassifySuffix(b *testing.B) {
	for range b.N {
		_ = Classify("service.generated.go", "internal/service.generated.go", tree.NodeFile)
	}
}

func BenchmarkClassifyFallback(b *testing.B) {
	for range b.N {
		_ = Classify("unknown", "unknown", tree.NodeFile)
	}
}
