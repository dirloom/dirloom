package catalog

func directorySpecs() []entrySpec {
	return []entrySpec{
		// v0.2 source layout
		spec("src", "directory", RoleSource), spec("source", "directory", RoleSource),
		spec("lib", "directory", RoleSource), spec("internal", "directory", RoleSource),
		spec("pkg", "directory", RoleSource), spec("cmd", "directory", RoleSource, RoleExecutable),
		spec("app", "directory", RoleSource), spec("apps", "directory", RoleSource),

		// v0.2 tests
		spec("test", "directory", RoleTest), spec("tests", "directory", RoleTest),
		spec("__tests__", "directory", RoleTest), spec("spec", "directory", RoleTest),
		spec("specs", "directory", RoleTest), spec("fixtures", "directory", RoleTest, RoleData),
		spec("mocks", "directory", RoleTest), spec("snapshots", "directory", RoleTest, RoleGenerated),

		// v0.2 documentation and media
		spec("docs", "directory", RoleDocument), spec("doc", "directory", RoleDocument),
		spec("examples", "directory", RoleDocument), spec("samples", "directory", RoleDocument),
		spec("assets", "directory", RoleMedia), spec("public", "directory", RoleMedia),
		spec("static", "directory", RoleMedia), spec("media", "directory", RoleMedia),

		// v0.2 vendor and generated
		spec("node_modules", "directory", RoleVendor), spec("vendor", "directory", RoleVendor),
		spec("third_party", "directory", RoleVendor), spec("dist", "directory", RoleGenerated),
		spec("build", "directory", RoleGenerated), spec("out", "directory", RoleGenerated),
		spec("target", "directory", RoleGenerated), spec("coverage", "directory", RoleGenerated, RoleTest),

		// v0.2 tooling
		spec(".git", "directory", RoleVendor, RoleTooling), spec(".github", "directory", RoleInfra, RoleTooling),
		spec(".gitlab", "directory", RoleInfra, RoleTooling), spec(".vscode", "directory", RoleConfig, RoleTooling),
		spec(".idea", "directory", RoleConfig, RoleTooling), spec(".cache", "directory", RoleGenerated),
		spec("tmp", "directory", RoleGenerated), spec("migrations", "directory", RoleData, RoleTooling),

		// v0.3 JavaScript / TypeScript project directories
		spec(".storybook", "directory", RoleDocument, RoleTooling), spec(".changeset", "directory", RoleTooling, RoleDocument),
		spec(".husky", "directory", RoleTooling), spec(".next", "directory", RoleGenerated),
		spec(".nuxt", "directory", RoleGenerated), spec(".svelte-kit", "directory", RoleGenerated),
		spec(".output", "directory", RoleGenerated), spec(".nx", "directory", RoleGenerated, RoleTooling),
		spec(".turbo", "directory", RoleGenerated, RoleTooling),

		// v0.3 language tool caches and environments
		spec(".dart_tool", "directory", RoleGenerated), spec(".venv", "directory", RoleVendor),
		spec("venv", "directory", RoleVendor), spec("__pycache__", "directory", RoleGenerated),
		spec(".pytest_cache", "directory", RoleGenerated, RoleTest), spec(".mypy_cache", "directory", RoleGenerated),
		spec(".ruff_cache", "directory", RoleGenerated), spec(".tox", "directory", RoleGenerated, RoleTest),
		spec(".cargo", "directory", RoleConfig, RoleTooling), spec(".gradle", "directory", RoleGenerated, RoleTooling),

		// v0.3 build outputs
		spec("obj", "directory", RoleGenerated), spec("bin", "directory", RoleGenerated, RoleExecutable),

		// v0.3 infrastructure directories
		spec("charts", "directory", RoleInfra), spec("helm", "directory", RoleInfra),
		spec("k8s", "directory", RoleInfra), spec("kubernetes", "directory", RoleInfra),
		spec("manifests", "directory", RoleInfra), spec("deploy", "directory", RoleInfra),
		spec("deployments", "directory", RoleInfra), spec(".terraform", "directory", RoleGenerated, RoleInfra),
		spec(".devcontainer", "directory", RoleConfig, RoleTooling), spec(".circleci", "directory", RoleInfra, RoleTooling),

		// v0.3 additional project directories
		spec("testdata", "directory", RoleTest, RoleData), spec("scripts", "directory", RoleTooling, RoleExecutable),
		spec(".direnv", "directory", RoleConfig, RoleTooling),
	}
}
