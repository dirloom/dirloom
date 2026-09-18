package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

type showcaseNode struct {
	name     string
	nodeType tree.NodeType
	known    bool
}

type showcaseScenario struct {
	name  string
	nodes []showcaseNode
}

func presentationShowcaseScenarios() []showcaseScenario {
	file := func(name string, known bool) showcaseNode {
		return showcaseNode{name: name, nodeType: tree.NodeFile, known: known}
	}
	dir := func(name string, known bool) showcaseNode {
		return showcaseNode{name: name, nodeType: tree.NodeDirectory, known: known}
	}
	return []showcaseScenario{
		{name: "go-service", nodes: []showcaseNode{
			file("go.mod", true), file("go.sum", true), file("go.work", true), file("main.go", true),
			file("user_test.go", true), file("user.pb.go", true), file("Dockerfile", true), file(".golangci.yml", true),
			dir("cmd", true), dir("internal", true), dir("testdata", true), file("notes.unknown", false),
		}},
		{name: "typescript-next", nodes: []showcaseNode{
			file("package.json", true), file("bun.lock", true), file("tsconfig.json", true), file("next.config.ts", true),
			file("playwright.config.ts", true), file("page.tsx", true), file("page.test.tsx", true), dir(".next", true),
			dir(".storybook", true), file("orphan.bin", false),
		}},
		{name: "node-monorepo", nodes: []showcaseNode{
			file("pnpm-workspace.yaml", true), file("turbo.json", true), file("nx.json", true), file("biome.json", true),
			dir(".husky", true), dir(".changeset", true), dir("apps", true), dir("node_modules", true),
		}},
		{name: "flutter-app", nodes: []showcaseNode{
			file("pubspec.yaml", true), file("pubspec.lock", true), file("analysis_options.yaml", true), file("main.dart", true),
			file("model.g.dart", true), dir(".dart_tool", true),
		}},
		{name: "python-service", nodes: []showcaseNode{
			file("pyproject.toml", true), file("requirements.txt", true), file("uv.lock", true), file("conftest.py", true),
			file("test_app.py", true), dir(".venv", true), dir("__pycache__", true), dir(".ruff_cache", true),
		}},
		{name: "rust-cli", nodes: []showcaseNode{
			file("Cargo.toml", true), file("Cargo.lock", true), file("rust-toolchain.toml", true), file("main.rs", true), dir(".cargo", true),
		}},
		{name: "dotnet-service", nodes: []showcaseNode{
			file("global.json", true), file("App.csproj", true), file("Program.cs", true), file("Directory.Build.props", true),
			dir("bin", true), dir("obj", true),
		}},
		{name: "jvm-service", nodes: []showcaseNode{
			file("pom.xml", true), file("build.gradle.kts", true), file("settings.gradle.kts", true), file("gradlew", true),
			file("Main.java", true), dir(".gradle", true),
		}},
		{name: "infra-terraform-k8s", nodes: []showcaseNode{
			file("main.tf", true), file("variables.tf", true), file("terraform.tfvars", true), file(".terraform.lock.hcl", true),
			file("terraform.tfstate", true), file("Chart.yaml", true), file("values.yaml", true),
			file("kustomization.yaml", true), file("compose.yaml", true), file("compose.yml", true),
			file("docker-compose.yml", true), file("docker-compose.yaml", true), file("docker-bake.hcl", true),
			dir(".terraform", true), dir("charts", true), dir("k8s", true),
		}},
		{name: "mixed-platform", nodes: []showcaseNode{
			file("README", true), file("README.md", true), file("LICENSE", true), file("LICENSE.md", true), file("CHANGELOG.md", true),
			file("flake.nix", true), file("flake.lock", true), file("site.hs", true),
			file("ca.pem", true), file("logo.webp", true), file("image.png", true), file("image.jpg", true),
			file("image.jpeg", true), file("image.svg", true), file("archive.7z", true), file(".env.local", true),
			file(".gitignore", true), dir(".github", true), dir(".devcontainer", true), file("mystery", false),
		}},
		{name: "catalog-fidelity", nodes: []showcaseNode{
			file("App.svelte", true), file("index.astro", true), file("main.dart", true), file("main.ml", true),
			file("main.nim", true), file("main.d", true), file("main.gleam", true), file("Main.elm", true),
			file("main.v", true), file("main.cr", true), file("policy.cue", true), file("Dockerfile", true),
		}},
	}
}

func TestPresentationShowcaseKnownEntriesAreClassified(t *testing.T) {
	for _, scenario := range presentationShowcaseScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			for _, node := range scenario.nodes {
				got := Classify(node.name, scenario.name+"/"+node.name, node.nodeType)
				if node.known {
					if got.Source == SourceFallback {
						t.Errorf("%s/%s fell back to %#v", scenario.name, node.name, got)
					}
					if node.nodeType == tree.NodeFile && got.Kind == "file" && containsRole(got.Roles, RoleGeneric) && len(got.Roles) == 1 {
						t.Errorf("%s/%s classified as generic file: %#v", scenario.name, node.name, got)
					}
					if node.nodeType == tree.NodeDirectory && got.Kind == "directory" && got.Source == SourceFallback {
						t.Errorf("%s/%s classified as generic directory: %#v", scenario.name, node.name, got)
					}
					continue
				}
				if got.Source != SourceFallback {
					t.Errorf("%s/%s should stay generic, got %#v", scenario.name, node.name, got)
				}
			}
		})
	}
}

func TestShowcaseCorpusIsMaterialized(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "showcase")
	if os.Getenv("DIRLOOM_WRITE_SHOWCASE") == "1" {
		if err := writeShowcaseCorpus(root); err != nil {
			t.Fatal(err)
		}
	}
	for _, scenario := range presentationShowcaseScenarios() {
		dir := filepath.Join(root, scenario.name)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("missing showcase %s: %v", scenario.name, err)
		}
		for _, node := range scenario.nodes {
			path := filepath.Join(dir, node.name)
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("%s/%s: %v", scenario.name, node.name, err)
				continue
			}
			if node.nodeType == tree.NodeDirectory && !info.IsDir() {
				t.Errorf("%s/%s is not a directory", scenario.name, node.name)
			}
			if node.nodeType == tree.NodeFile && info.IsDir() {
				t.Errorf("%s/%s is not a file", scenario.name, node.name)
			}
		}
	}
}

func writeShowcaseCorpus(root string) error {
	for _, scenario := range presentationShowcaseScenarios() {
		dir := filepath.Join(root, scenario.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		for _, node := range scenario.nodes {
			path := filepath.Join(dir, node.name)
			if node.nodeType == tree.NodeDirectory {
				if err := os.MkdirAll(path, 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(path, ".keep"), nil, 0o644); err != nil {
					return err
				}
				continue
			}
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsRole(roles []Role, want Role) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}
