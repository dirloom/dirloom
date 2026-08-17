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
	filenames := []entrySpec{
		spec("readme.md", "document.markdown", RoleContract, RoleDocument), spec("license", "document.license", RoleContract, RoleDocument),
		spec("licence", "document.license", RoleContract, RoleDocument), spec("copying", "document.license", RoleContract, RoleDocument),
		spec("notice", "document.license", RoleContract, RoleDocument), spec("changelog.md", "document.changelog", RoleContract, RoleDocument),
		spec("contributing.md", "document.markdown", RoleContract, RoleDocument), spec("code_of_conduct.md", "document.markdown", RoleContract, RoleDocument),

		spec("security.md", "document.markdown", RoleSecurity, RoleContract, RoleDocument), spec("authors", "document.text", RoleContract, RoleDocument),
		spec("maintainers", "document.text", RoleContract, RoleDocument), spec("governance.md", "document.markdown", RoleContract, RoleDocument),
		spec("support.md", "document.markdown", RoleContract, RoleDocument), spec("citation.cff", "data.yaml", RoleContract, RoleData),
		spec("codeowners", "document.text", RoleSecurity, RoleContract), spec(".editorconfig", "data.ini", RoleConfig, RoleTooling),

		spec("package.json", "manifest.node", RoleConfig, RoleData), spec("go.mod", "manifest.go", RoleContract, RoleConfig),
		spec("cargo.toml", "manifest.rust", RoleContract, RoleConfig), spec("pyproject.toml", "manifest.python", RoleContract, RoleConfig),
		spec("pom.xml", "manifest.java", RoleContract, RoleConfig), spec("build.gradle", "manifest.java", RoleConfig, RoleTooling),
		spec("composer.json", "manifest.php", RoleContract, RoleConfig), spec("pubspec.yaml", "manifest.dart", RoleContract, RoleConfig),

		spec("package-lock.json", "data.json", RoleLock, RoleConfig, RoleData), spec("yarn.lock", "document.text", RoleLock, RoleConfig),
		spec("pnpm-lock.yaml", "data.yaml", RoleLock, RoleConfig), spec("bun.lockb", "data.binary", RoleLock, RoleConfig),
		spec("go.sum", "document.text", RoleLock, RoleConfig), spec("cargo.lock", "manifest.rust", RoleLock, RoleConfig),
		spec("poetry.lock", "data.toml", RoleLock, RoleConfig), spec("pipfile.lock", "data.json", RoleLock, RoleConfig),

		spec("dockerfile", "manifest.container", RoleInfra, RoleExecutable), spec("containerfile", "manifest.container", RoleInfra, RoleExecutable),
		spec("docker-compose.yml", "data.yaml", RoleInfra, RoleConfig), spec("compose.yaml", "data.yaml", RoleInfra, RoleConfig),
		spec("makefile", "manifest.generic", RoleExecutable, RoleTooling), spec("cmakelists.txt", "manifest.generic", RoleConfig, RoleTooling),
		spec("justfile", "manifest.generic", RoleExecutable, RoleTooling), spec("taskfile.yml", "data.yaml", RoleExecutable, RoleTooling),

		spec(".gitlab-ci.yml", "data.yaml", RoleInfra, RoleConfig), spec("azure-pipelines.yml", "data.yaml", RoleInfra, RoleConfig),
		spec("jenkinsfile", "manifest.generic", RoleInfra, RoleExecutable), spec("vagrantfile", "source.ruby", RoleInfra, RoleExecutable),
		spec("ansible.cfg", "data.ini", RoleInfra, RoleConfig), spec(".terraform.lock.hcl", "document.text", RoleLock, RoleInfra),
		spec(".bazeliskrc", "data.ini", RoleConfig, RoleTooling), spec(".pre-commit-config.yaml", "data.yaml", RoleConfig, RoleTooling),

		spec("tsconfig.json", "data.json", RoleConfig, RoleTooling), spec("jsconfig.json", "data.json", RoleConfig, RoleTooling),
		spec("eslint.config.js", "source.javascript", RoleConfig, RoleTooling), spec("prettier.config.js", "source.javascript", RoleConfig, RoleTooling),
		spec("vitest.config.ts", "source.typescript", RoleTest, RoleConfig), spec("jest.config.js", "source.javascript", RoleTest, RoleConfig),
		spec("tailwind.config.js", "source.javascript", RoleConfig, RoleTooling), spec("vite.config.ts", "source.typescript", RoleConfig, RoleTooling),

		spec("graphql.schema", "source.graphql", RoleContract, RoleData), spec("buf.yaml", "data.yaml", RoleConfig, RoleTooling),
		spec("buf.gen.yaml", "data.yaml", RoleGenerated, RoleConfig), spec("renovate.json", "data.json", RoleConfig, RoleTooling),
		spec("dependabot.yml", "data.yaml", RoleSecurity, RoleConfig), spec("mkdocs.yml", "data.yaml", RoleDocument, RoleConfig),
		spec("book.toml", "data.toml", RoleDocument, RoleConfig), spec("docker-bake.hcl", "document.text", RoleInfra, RoleConfig),
	}

	directories := []entrySpec{
		spec("src", "directory", RoleSource), spec("source", "directory", RoleSource),
		spec("lib", "directory", RoleSource), spec("internal", "directory", RoleSource),
		spec("pkg", "directory", RoleSource), spec("cmd", "directory", RoleSource, RoleExecutable),
		spec("app", "directory", RoleSource), spec("apps", "directory", RoleSource),

		spec("test", "directory", RoleTest), spec("tests", "directory", RoleTest),
		spec("__tests__", "directory", RoleTest), spec("spec", "directory", RoleTest),
		spec("specs", "directory", RoleTest), spec("fixtures", "directory", RoleTest, RoleData),
		spec("mocks", "directory", RoleTest), spec("snapshots", "directory", RoleTest, RoleGenerated),

		spec("docs", "directory", RoleDocument), spec("doc", "directory", RoleDocument),
		spec("examples", "directory", RoleDocument), spec("samples", "directory", RoleDocument),
		spec("assets", "directory", RoleMedia), spec("public", "directory", RoleMedia),
		spec("static", "directory", RoleMedia), spec("media", "directory", RoleMedia),

		spec("node_modules", "directory", RoleVendor), spec("vendor", "directory", RoleVendor),
		spec("third_party", "directory", RoleVendor), spec("dist", "directory", RoleGenerated),
		spec("build", "directory", RoleGenerated), spec("out", "directory", RoleGenerated),
		spec("target", "directory", RoleGenerated), spec("coverage", "directory", RoleGenerated, RoleTest),

		spec(".git", "directory", RoleVendor, RoleTooling), spec(".github", "directory", RoleInfra, RoleTooling),
		spec(".gitlab", "directory", RoleInfra, RoleTooling), spec(".vscode", "directory", RoleConfig, RoleTooling),
		spec(".idea", "directory", RoleConfig, RoleTooling), spec(".cache", "directory", RoleGenerated),
		spec("tmp", "directory", RoleGenerated), spec("migrations", "directory", RoleData, RoleTooling),
	}

	suffixes := []entrySpec{
		spec("_test.go", "source.go", RoleTest, RoleSource), spec(".pb.go", "source.go", RoleGenerated, RoleSource),
		spec(".g.dart", "source.dart", RoleGenerated, RoleSource), spec(".freezed.dart", "source.dart", RoleGenerated, RoleSource),
		spec(".generated.go", "source.go", RoleGenerated, RoleSource), spec(".gen.go", "source.go", RoleGenerated, RoleSource),
		spec(".d.ts", "source.typescript", RoleContract, RoleGenerated, RoleSource), spec(".d.mts", "source.typescript", RoleContract, RoleGenerated, RoleSource),

		spec(".d.cts", "source.typescript", RoleContract, RoleGenerated, RoleSource), spec(".test.ts", "source.typescript", RoleTest, RoleSource),
		spec(".test.tsx", "source.typescript", RoleTest, RoleSource), spec(".test.js", "source.javascript", RoleTest, RoleSource),
		spec(".test.jsx", "source.javascript", RoleTest, RoleSource), spec(".spec.ts", "source.typescript", RoleTest, RoleSource),
		spec(".spec.tsx", "source.typescript", RoleTest, RoleSource), spec(".spec.js", "source.javascript", RoleTest, RoleSource),

		spec(".spec.jsx", "source.javascript", RoleTest, RoleSource), spec(".stories.ts", "source.typescript", RoleDocument, RoleSource),
		spec(".stories.tsx", "source.typescript", RoleDocument, RoleSource), spec(".stories.js", "source.javascript", RoleDocument, RoleSource),
		spec(".stories.jsx", "source.javascript", RoleDocument, RoleSource), spec(".snap", "document.text", RoleTest, RoleGenerated),
		spec(".min.js", "source.javascript", RoleGenerated, RoleSource), spec(".min.css", "source.css", RoleGenerated, RoleSource),

		spec(".generated.ts", "source.typescript", RoleGenerated, RoleSource), spec(".generated.js", "source.javascript", RoleGenerated, RoleSource),
		spec(".gen.ts", "source.typescript", RoleGenerated, RoleSource), spec(".gen.js", "source.javascript", RoleGenerated, RoleSource),
		spec(".mock.ts", "source.typescript", RoleTest, RoleSource), spec(".mock.js", "source.javascript", RoleTest, RoleSource),
		spec(".designer.cs", "source.csharp", RoleGenerated, RoleSource), spec(".generated.cs", "source.csharp", RoleGenerated, RoleSource),
	}

	extensions := []entrySpec{
		spec(".c", "source.c", RoleSource), spec(".h", "source.c", RoleContract, RoleSource), spec(".cc", "source.cpp", RoleSource), spec(".cpp", "source.cpp", RoleSource), spec(".cxx", "source.cpp", RoleSource), spec(".hpp", "source.cpp", RoleContract, RoleSource), spec(".m", "source.objective-c", RoleSource), spec(".mm", "source.objective-c", RoleSource),
		spec(".swift", "source.swift", RoleSource), spec(".go", "source.go", RoleSource), spec(".rs", "source.rust", RoleSource), spec(".zig", "source.zig", RoleSource), spec(".java", "source.java", RoleSource), spec(".kt", "source.kotlin", RoleSource), spec(".kts", "source.kotlin", RoleSource), spec(".scala", "source.scala", RoleSource),
		spec(".cs", "source.csharp", RoleSource), spec(".fs", "source.fsharp", RoleSource), spec(".fsx", "source.fsharp", RoleSource), spec(".dart", "source.dart", RoleSource), spec(".sol", "source.solidity", RoleSource), spec(".vhd", "source.vhdl", RoleSource), spec(".vhdl", "source.vhdl", RoleSource), spec(".asm", "source.assembly", RoleSource),
		spec(".py", "source.python", RoleSource), spec(".pyi", "source.python", RoleContract, RoleSource), spec(".rb", "source.ruby", RoleSource), spec(".php", "source.php", RoleSource), spec(".lua", "source.lua", RoleSource), spec(".pl", "source.perl", RoleSource), spec(".pm", "source.perl", RoleSource), spec(".r", "source.r", RoleSource),
		spec(".jl", "source.julia", RoleSource), spec(".ex", "source.elixir", RoleSource), spec(".exs", "source.elixir", RoleSource), spec(".erl", "source.erlang", RoleSource), spec(".hrl", "source.erlang", RoleContract, RoleSource), spec(".clj", "source.clojure", RoleSource), spec(".cljs", "source.clojure", RoleSource), spec(".groovy", "source.groovy", RoleSource),
		spec(".sh", "source.shell", RoleExecutable, RoleSource), spec(".bash", "source.shell", RoleExecutable, RoleSource), spec(".zsh", "source.shell", RoleExecutable, RoleSource), spec(".fish", "source.shell", RoleExecutable, RoleSource), spec(".ps1", "source.powershell", RoleExecutable, RoleSource), spec(".bat", "source.batch", RoleExecutable, RoleSource), spec(".cmd", "source.batch", RoleExecutable, RoleSource), spec(".js", "source.javascript", RoleSource),
		spec(".mjs", "source.javascript", RoleSource), spec(".cjs", "source.javascript", RoleSource), spec(".jsx", "source.javascript", RoleSource), spec(".ts", "source.typescript", RoleSource), spec(".mts", "source.typescript", RoleSource), spec(".cts", "source.typescript", RoleSource), spec(".tsx", "source.typescript", RoleSource), spec(".html", "source.html", RoleSource),
		spec(".css", "source.css", RoleSource), spec(".scss", "source.css", RoleSource), spec(".sass", "source.css", RoleSource), spec(".less", "source.css", RoleSource), spec(".vue", "source.vue", RoleSource), spec(".svelte", "source.svelte", RoleSource), spec(".astro", "source.astro", RoleSource), spec(".wasm", "source.webassembly", RoleExecutable),
		spec(".json", "data.json", RoleData), spec(".jsonc", "data.json", RoleConfig, RoleData), spec(".yaml", "data.yaml", RoleConfig, RoleData), spec(".yml", "data.yaml", RoleConfig, RoleData), spec(".toml", "data.toml", RoleConfig, RoleData), spec(".xml", "data.xml", RoleData), spec(".ini", "data.ini", RoleConfig), spec(".env", "data.env", RoleSecurity, RoleConfig),
		spec(".cfg", "data.ini", RoleConfig), spec(".conf", "data.ini", RoleConfig), spec(".csv", "data.tabular", RoleData), spec(".tsv", "data.tabular", RoleData), spec(".sql", "data.sql", RoleData), spec(".graphql", "source.graphql", RoleContract, RoleData), spec(".gql", "source.graphql", RoleContract, RoleData), spec(".proto", "source.protobuf", RoleContract, RoleData),
		spec(".avsc", "data.schema", RoleContract, RoleData), spec(".parquet", "data.binary", RoleData), spec(".tex", "document.tex", RoleDocument), spec(".txt", "document.text", RoleDocument), spec(".ipynb", "data.notebook", RoleData, RoleDocument), spec(".odt", "document.office", RoleDocument), spec(".epub", "document.ebook", RoleDocument), spec(".db", "data.database", RoleData),
		spec(".md", "document.markdown", RoleDocument), spec(".mdx", "document.markdown", RoleDocument), spec(".rst", "document.rst", RoleDocument), spec(".adoc", "document.asciidoc", RoleDocument), spec(".pdf", "document.pdf", RoleDocument), spec(".docx", "document.office", RoleDocument), spec(".xlsx", "document.office", RoleData, RoleDocument), spec(".pptx", "document.office", RoleDocument),
		spec(".png", "media.image.png", RoleMedia), spec(".jpg", "media.image.jpeg", RoleMedia), spec(".jpeg", "media.image.jpeg", RoleMedia), spec(".svg", "media.image.svg", RoleMedia), spec(".mp3", "media.audio", RoleMedia), spec(".wav", "media.audio", RoleMedia), spec(".mp4", "media.video", RoleMedia), spec(".webm", "media.video", RoleMedia),
		spec(".zip", "archive.compressed", RoleArchive), spec(".tar", "archive.compressed", RoleArchive), spec(".gz", "archive.compressed", RoleArchive), spec(".tgz", "archive.compressed", RoleArchive), spec(".jar", "archive.package", RoleArchive), spec(".war", "archive.package", RoleArchive), spec(".whl", "archive.package", RoleArchive), spec(".deb", "archive.package", RoleArchive),
		spec(".rpm", "archive.package", RoleArchive), spec(".exe", "binary.executable", RoleExecutable), spec(".dll", "binary.library", RoleExecutable), spec(".so", "binary.library", RoleExecutable), spec(".dylib", "binary.library", RoleExecutable), spec(".ttf", "font", RoleMedia), spec(".woff", "font.web", RoleMedia), spec(".woff2", "font.web", RoleMedia),
	}

	if len(filenames) != 64 || len(directories) != 40 || len(suffixes) != 32 || len(extensions) != 120 {
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
