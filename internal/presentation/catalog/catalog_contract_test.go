package catalog

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

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
	if os.Getenv("DIRLOOM_WRITE_CATALOG_FIXTURE") == "1" {
		if err := writeClassificationV1Fixture(); err != nil {
			t.Fatal(err)
		}
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
	byMatcher := make(map[string]Entry, EntryCount)
	for _, entry := range Entries() {
		byMatcher[string(entry.Matcher.Source)+":"+strings.ToLower(entry.Matcher.Value)] = entry
	}
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
		entry, ok := byMatcher[identity]
		if !ok {
			t.Fatalf("%s encodes unknown matcher %s", fixture.ID, identity)
		}
		if fixture.Kind != entry.Kind || fixture.Source != entry.Matcher.Source || fixture.MatcherKey != entry.Matcher.Value || !reflect.DeepEqual(fixture.Roles, entry.Roles) {
			t.Errorf("%s fixture diverges from matcher contract: %#v vs %#v", fixture.ID, fixture, entry)
		}
		got := Classify(fixture.Name, fixture.RelativePath, fixture.Type)
		want := Classification{Kind: fixture.Kind, Roles: fixture.Roles, Source: fixture.Source, MatcherKey: fixture.MatcherKey}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v, want %#v", fixture.ID, got, want)
		}
	}
	wantCounts := map[MatchSource]int{
		SourceFilename:  FilenameEntryCount,
		SourceDirectory: DirectoryEntryCount,
		SourceSuffix:    SuffixEntryCount,
		SourceExtension: ExtensionEntryCount,
	}
	if !reflect.DeepEqual(counts, wantCounts) {
		t.Fatalf("fixture groups = %#v, want %#v", counts, wantCounts)
	}
	for _, entry := range Entries() {
		identity := string(entry.Matcher.Source) + ":" + strings.ToLower(entry.Matcher.Value)
		if _, ok := seenMatchers[identity]; !ok {
			t.Errorf("missing fixture for matcher %s", identity)
		}
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
	if RoleCount != 16 {
		t.Fatalf("RoleCount = %d", RoleCount)
	}
	for _, definition := range Kinds() {
		if chain := KindChain(definition.Kind); len(chain) == 0 || len(chain) > 4 {
			t.Errorf("kind %s chain = %#v", definition.Kind, chain)
		}
		unicodeGlyph, nerdGlyph := Glyphs(definition.Kind)
		if unicodeGlyph == "" || nerdGlyph == "" {
			t.Errorf("kind %s has no effective glyphs", definition.Kind)
		}
		if err := validateCatalogGlyph(unicodeGlyph); err != nil {
			t.Errorf("kind %s unicode: %v", definition.Kind, err)
		}
		if err := validateCatalogGlyph(nerdGlyph); err != nil {
			t.Errorf("kind %s nerd: %v", definition.Kind, err)
		}
	}
	wantNerd := map[Kind]string{
		"source.go": "󰟓", "source.rust": "󱘗", "source.python": "󰌠", "source.javascript": "󰌞",
		"source.typescript": "󰛦", "source.html": "󰌝", "source.css": "󰌜", "data.json": "󰘦",
		"data.yaml": "󰈙", "data.toml": "󰈙", "document.markdown": "󰍔", "document.pdf": "󰈦",
		"media.image.png": "󰸭", "archive.package": "󰏗", "manifest.container": "󰡨",
		"source.c": "\U000F0671", "source.cpp": "\U000F0672", "source.csharp": "\U000F031B",
		"source.java": "\U000F0B37", "source.kotlin": "\U000F1219", "source.php": "\U000F031F",
		"source.ruby": "\U000F0D2D", "source.swift": "\U000F06E5", "source.lua": "\U000F08B1",
		"source.r": "\U000F07D4", "source.vue": "\U000F0844", "source.svelte": "\U000F059F",
		"source.astro": "\U000F06E4", "source.graphql": "\U000F0877", "source.protobuf": "\U000F0FD8",
		"source.haskell": "\U000F0C92", "source.ocaml": "\U000F0295", "source.nim": "\U000F02D8",
		"source.d": "\U000F01A6", "source.fortran": "\U000F121A", "source.gleam": "\U000F04A0",
		"source.scheme": "\U000F0627", "source.racket": "\U000F0172", "source.elm": "\U000F0405",
		"source.v": "\U000F016C", "source.crystal": "\U000F01C8", "source.nix": "\U000F1105",
		"source.hcl": "\U000F10D6", "source.cue": "\U000F0168", "source.jsonnet": "\U000F0626",
		"source.powershell": "\U000F0A0A", "source.shell": "\U000F1183", "source.fsharp": "\U000F0627",
		"source.dart": "\U000F08C6", "manifest.node": "\U000F0399", "manifest.go": "󰟓",
		"manifest.rust": "󱘗", "manifest.python": "󰌠", "manifest.java": "\U000F0B37",
		"manifest.dotnet": "\U000F0AAE", "manifest.php": "\U000F031F", "manifest.terraform": "\U000F1062",
		"manifest.helm": "\U000F10FE", "manifest.nix": "\U000F1105", "data.database": "\U000F01BC",
		"data.notebook": "\U000F082E", "data.properties": "\U000F0493", "data.plist": "\U000F05C0",
		"data.certificate": "\U000F0124", "data.key": "\U000F0306", "data.localization": "\U000F05CA",
		"data.xml": "\U000F05C0", "media.audio": "\U000F075A", "media.video": "\U000F0567",
		"media.design": "\U000F03D8", "media.image": "\U000F02E9", "archive.compressed": "\U000F05C4",
	}
	for kind, want := range wantNerd {
		_, nerdGlyph := Glyphs(kind)
		if nerdGlyph != want {
			t.Errorf("kind %s nerd = %q, want %q", kind, nerdGlyph, want)
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
		{"src", "src", tree.NodeFile, "file", []Role{RoleGeneric}, SourceFallback, "file"},
		{"package.json", "vendor/package.json", tree.NodeFile, "manifest.node", []Role{RoleConfig, RoleData}, SourceFilename, "package.json"},
		{"service.grpc.pb.go", "api/service.grpc.pb.go", tree.NodeFile, "source.go", []Role{RoleGenerated, RoleSource}, SourceSuffix, ".grpc.pb.go"},
		{"user.pb.go", "api/user.pb.go", tree.NodeFile, "source.go", []Role{RoleGenerated, RoleSource}, SourceSuffix, ".pb.go"},
		{"app.spec.tsx", "app.spec.tsx", tree.NodeFile, "source.typescript", []Role{RoleTest, RoleSource}, SourceSuffix, ".spec.tsx"},
		{"app.tsx", "app.tsx", tree.NodeFile, "source.typescript", []Role{RoleSource}, SourceExtension, ".tsx"},
		{".ENV", ".ENV", tree.NodeFile, "data.env", []Role{RoleSecurity, RoleConfig}, SourceExtension, ".env"},
		{"very-long-safe-basename-without-a-known-extension", "very-long-safe-basename-without-a-known-extension", tree.NodeFile, "file", []Role{RoleGeneric}, SourceFallback, "file"},
		{"😀.unknown", "😀.unknown", tree.NodeFile, "file", []Role{RoleGeneric}, SourceFallback, "file"},
		{"Chart.yaml", "charts/app/Chart.yaml", tree.NodeFile, "manifest.helm", []Role{RoleContract, RoleInfra, RoleConfig}, SourceFilename, "chart.yaml"},
		{"build.gradle.kts", "build.gradle.kts", tree.NodeFile, "manifest.java", []Role{RoleConfig, RoleTooling}, SourceFilename, "build.gradle.kts"},
		{"extra.gradle.kts", "extra.gradle.kts", tree.NodeFile, "manifest.java", []Role{RoleConfig, RoleTooling}, SourceSuffix, ".gradle.kts"},
		{"playwright.config.ts", "playwright.config.ts", tree.NodeFile, "source.typescript", []Role{RoleTest, RoleConfig}, SourceFilename, "playwright.config.ts"},
		{"README", "README", tree.NodeFile, "document.text", []Role{RoleContract, RoleDocument}, SourceFilename, "readme"},
		{"Makefile", "Makefile", tree.NodeFile, "manifest.generic", []Role{RoleExecutable, RoleTooling}, SourceFilename, "makefile"},
		{"LICENSE", "LICENSE", tree.NodeFile, "document.license", []Role{RoleContract, RoleDocument}, SourceFilename, "license"},
		{".env.local", ".env.local", tree.NodeFile, "data.env", []Role{RoleSecurity, RoleConfig}, SourceFilename, ".env.local"},
		{".editorconfig", ".editorconfig", tree.NodeFile, "data.ini", []Role{RoleConfig, RoleTooling}, SourceFilename, ".editorconfig"},
		{"app.blade.php", "resources/app.blade.php", tree.NodeFile, "source.php", []Role{RoleSource}, SourceSuffix, ".blade.php"},
		{"types.d.ts", "types.d.ts", tree.NodeFile, "source.typescript", []Role{RoleGenerated, RoleContract, RoleSource}, SourceSuffix, ".d.ts"},
		{"flake.nix", "flake.nix", tree.NodeFile, "manifest.nix", []Role{RoleContract, RoleConfig}, SourceFilename, "flake.nix"},
		{"module.nix", "module.nix", tree.NodeFile, "source.nix", []Role{RoleConfig, RoleSource}, SourceExtension, ".nix"},
	}
	for _, test := range tests {
		got := Classify(test.name, test.path, test.nodeType)
		if got.Kind != test.kind || got.Source != test.source || got.MatcherKey != test.matcher || !reflect.DeepEqual(got.Roles, test.roles) {
			t.Errorf("%s: %#v", test.path, got)
		}
	}
}

func TestSemanticPromotionsAreIntentionalPathChanges(t *testing.T) {
	// Frozen v0.2 compatibility covers matcher identities, not every real-world
	// path. A new more-specific matcher may promote a path that previously lost
	// to a generic extension or fallback. That is an intentional classification
	// change, documented in the changelog, not a mutation of the 256 frozen matchers.
	tests := []struct {
		name, justification string
		previous, current   Classification
	}{
		{
			name:          "requirements.txt",
			justification: "Python dependency manifest is more specific than generic text",
			previous:      Classification{Kind: "document.text", Roles: []Role{RoleDocument}, Source: SourceExtension, MatcherKey: ".txt"},
			current:       Classification{Kind: "manifest.python", Roles: []Role{RoleConfig}, Source: SourceFilename, MatcherKey: "requirements.txt"},
		},
		{
			name:          "Chart.yaml",
			justification: "Helm chart identity is more specific than generic YAML",
			previous:      Classification{Kind: "data.yaml", Roles: []Role{RoleConfig, RoleData}, Source: SourceExtension, MatcherKey: ".yaml"},
			current:       Classification{Kind: "manifest.helm", Roles: []Role{RoleContract, RoleInfra, RoleConfig}, Source: SourceFilename, MatcherKey: "chart.yaml"},
		},
		{
			name:          "go.work",
			justification: "Go workspace file was unclassified in v0.2; it is a Go module contract",
			previous:      Classification{Kind: "file", Roles: []Role{RoleGeneric}, Source: SourceFallback, MatcherKey: "file"},
			current:       Classification{Kind: "manifest.go", Roles: []Role{RoleContract, RoleConfig}, Source: SourceFilename, MatcherKey: "go.work"},
		},
		{
			name:          "terraform.tf",
			justification: "Terraform/OpenTofu sources were unclassified in v0.2; they are infra manifests",
			previous:      Classification{Kind: "file", Roles: []Role{RoleGeneric}, Source: SourceFallback, MatcherKey: "file"},
			current:       Classification{Kind: "manifest.terraform", Roles: []Role{RoleInfra, RoleSource}, Source: SourceExtension, MatcherKey: ".tf"},
		},
		{
			name:          "build.gradle.kts",
			justification: "Gradle Kotlin DSL project file is more specific than generic Kotlin script",
			previous:      Classification{Kind: "source.kotlin", Roles: []Role{RoleSource}, Source: SourceExtension, MatcherKey: ".kts"},
			current:       Classification{Kind: "manifest.java", Roles: []Role{RoleConfig, RoleTooling}, Source: SourceFilename, MatcherKey: "build.gradle.kts"},
		},
		{
			name:          "extra.gradle.kts",
			justification: "Compound Gradle suffix is more specific than generic Kotlin script",
			previous:      Classification{Kind: "source.kotlin", Roles: []Role{RoleSource}, Source: SourceExtension, MatcherKey: ".kts"},
			current:       Classification{Kind: "manifest.java", Roles: []Role{RoleConfig, RoleTooling}, Source: SourceSuffix, MatcherKey: ".gradle.kts"},
		},
		{
			name:          "flake.nix",
			justification: "Nix flake manifest is more specific than generic Nix source",
			previous:      Classification{Kind: "file", Roles: []Role{RoleGeneric}, Source: SourceFallback, MatcherKey: "file"},
			current:       Classification{Kind: "manifest.nix", Roles: []Role{RoleContract, RoleConfig}, Source: SourceFilename, MatcherKey: "flake.nix"},
		},
		{
			name:          ".env.local",
			justification: "Local env files are a structural security/config signal, not an unknown basename",
			previous:      Classification{Kind: "file", Roles: []Role{RoleGeneric}, Source: SourceFallback, MatcherKey: "file"},
			current:       Classification{Kind: "data.env", Roles: []Role{RoleSecurity, RoleConfig}, Source: SourceFilename, MatcherKey: ".env.local"},
		},
		{
			name:          "bun.lock",
			justification: "Bun lockfile was unclassified in v0.2; it is a lock document",
			previous:      Classification{Kind: "file", Roles: []Role{RoleGeneric}, Source: SourceFallback, MatcherKey: "file"},
			current:       Classification{Kind: "document.text", Roles: []Role{RoleLock, RoleConfig}, Source: SourceFilename, MatcherKey: "bun.lock"},
		},
		{
			name:          "next.config.ts",
			justification: "Next.js config is more specific than generic TypeScript source",
			previous:      Classification{Kind: "source.typescript", Roles: []Role{RoleSource}, Source: SourceExtension, MatcherKey: ".ts"},
			current:       Classification{Kind: "source.typescript", Roles: []Role{RoleConfig, RoleTooling}, Source: SourceFilename, MatcherKey: "next.config.ts"},
		},
	}
	for _, test := range tests {
		got := Classify(test.name, test.name, tree.NodeFile)
		if !reflect.DeepEqual(got, test.current) {
			t.Errorf("%s current = %#v, want %#v (%s)", test.name, got, test.current, test.justification)
		}
		if reflect.DeepEqual(got, test.previous) {
			t.Errorf("%s was not promoted away from %#v", test.name, test.previous)
		}
		if test.previous.Source != SourceFallback {
			generic := Classify("fixture"+test.previous.MatcherKey, "fixture/fixture"+test.previous.MatcherKey, tree.NodeFile)
			if generic.Kind != test.previous.Kind || generic.Source != test.previous.Source || generic.MatcherKey != test.previous.MatcherKey {
				t.Errorf("%s previous matcher %s changed: %#v", test.name, test.previous.MatcherKey, generic)
			}
		}
	}
}

func FuzzClassifyIsDeterministic(f *testing.F) {
	for _, seed := range []string{
		"README", ".env.local", "terraform.tf", "foo.spec.tsx", "foo.d.mts", "foo.blade.php",
		"Ω/名.go", "📄.md", ".gitignore", "a.b.c.d.ts", strings.Repeat("safe-basename-", 20),
		"README.md", "src/main.go", "types.d.ts", "Ω/名.go", "", "..",
		"flake.nix", "playwright.config.mts", "next.config.ts", "stack.tofu",
	} {
		f.Add(seed, seed)
	}
	f.Fuzz(func(t *testing.T, name, path string) {
		first := Classify(name, path, tree.NodeFile)
		second := Classify(name, path, tree.NodeFile)
		if !reflect.DeepEqual(first, second) || first.Kind == "" || len(first.Roles) == 0 || !utf8.ValidString(string(first.Kind)) {
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

func BenchmarkClassifyExtension(b *testing.B) {
	for range b.N {
		_ = Classify("main.go", "cmd/dirloom/main.go", tree.NodeFile)
	}
}

func BenchmarkClassifyLargeCatalogExact(b *testing.B) {
	for range b.N {
		_ = Classify("requirements.txt", "requirements.txt", tree.NodeFile)
		_ = Classify("Chart.yaml", "charts/app/Chart.yaml", tree.NodeFile)
		_ = Classify("terraform.tf", "infra/terraform.tf", tree.NodeFile)
	}
}

func TestExpandedCandidateKindsHaveMatchers(t *testing.T) {
	used := make(map[Kind]struct{}, KindCount)
	for _, entry := range Entries() {
		used[entry.Kind] = struct{}{}
	}
	for _, kind := range []Kind{
		"source.haskell", "source.ocaml", "source.nim", "source.d", "source.fortran",
		"source.gleam", "source.scheme", "source.racket", "source.elm", "source.v",
		"source.crystal", "source.nix", "source.hcl", "source.cue", "source.jsonnet",
		"data.properties", "data.plist", "data.certificate", "data.key", "data.localization",
		"manifest.terraform", "manifest.helm", "manifest.nix", "manifest.container",
	} {
		if _, ok := used[kind]; !ok {
			t.Errorf("kind %s is not referenced by any matcher", kind)
		}
	}
}

func writeClassificationV1Fixture() error {
	document := classificationFixtureDocument{CatalogVersion: Version, Cases: make([]classificationFixture, 0, EntryCount)}
	for index, entry := range Entries() {
		name, nodeType := catalogFixtureName(entry)
		document.Cases = append(document.Cases, classificationFixture{
			ID:           fmt.Sprintf("matcher-%03d", index+1),
			Name:         name,
			RelativePath: "fixture/" + name,
			Type:         nodeType,
			Kind:         entry.Kind,
			Roles:        append([]Role(nil), entry.Roles...),
			Source:       entry.Matcher.Source,
			MatcherKey:   entry.Matcher.Value,
		})
	}
	data, err := yaml.Marshal(&document)
	if err != nil {
		return err
	}
	return os.WriteFile("testdata/classification-v1.yaml", data, 0o644)
}

func catalogFixtureName(entry Entry) (string, tree.NodeType) {
	switch entry.Matcher.Source {
	case SourceDirectory:
		return entry.Matcher.Value, tree.NodeDirectory
	case SourceFilename:
		return entry.Matcher.Value, tree.NodeFile
	default:
		return "fixture" + entry.Matcher.Value, tree.NodeFile
	}
}
