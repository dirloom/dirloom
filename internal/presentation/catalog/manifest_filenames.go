package catalog

func filenameSpecs() []entrySpec {
	return []entrySpec{
		// v0.2 documentation and governance
		spec("readme.md", "document.markdown", RoleContract, RoleDocument), spec("license", "document.license", RoleContract, RoleDocument),
		spec("licence", "document.license", RoleContract, RoleDocument), spec("copying", "document.license", RoleContract, RoleDocument),
		spec("notice", "document.license", RoleContract, RoleDocument), spec("changelog.md", "document.changelog", RoleContract, RoleDocument),
		spec("contributing.md", "document.markdown", RoleContract, RoleDocument), spec("code_of_conduct.md", "document.markdown", RoleContract, RoleDocument),
		spec("security.md", "document.markdown", RoleSecurity, RoleContract, RoleDocument), spec("authors", "document.text", RoleContract, RoleDocument),
		spec("maintainers", "document.text", RoleContract, RoleDocument), spec("governance.md", "document.markdown", RoleContract, RoleDocument),
		spec("support.md", "document.markdown", RoleContract, RoleDocument), spec("citation.cff", "data.yaml", RoleContract, RoleData),
		spec("codeowners", "document.text", RoleSecurity, RoleContract), spec(".editorconfig", "data.ini", RoleConfig, RoleTooling),

		// v0.2 language manifests and lockfiles
		spec("package.json", "manifest.node", RoleConfig, RoleData), spec("go.mod", "manifest.go", RoleContract, RoleConfig),
		spec("cargo.toml", "manifest.rust", RoleContract, RoleConfig), spec("pyproject.toml", "manifest.python", RoleContract, RoleConfig),
		spec("pom.xml", "manifest.java", RoleContract, RoleConfig), spec("build.gradle", "manifest.java", RoleConfig, RoleTooling),
		spec("composer.json", "manifest.php", RoleContract, RoleConfig), spec("pubspec.yaml", "manifest.dart", RoleContract, RoleConfig),
		spec("package-lock.json", "data.json", RoleLock, RoleConfig, RoleData), spec("yarn.lock", "document.text", RoleLock, RoleConfig),
		spec("pnpm-lock.yaml", "data.yaml", RoleLock, RoleConfig), spec("bun.lockb", "data.binary", RoleLock, RoleConfig),
		spec("go.sum", "document.text", RoleLock, RoleConfig), spec("cargo.lock", "manifest.rust", RoleLock, RoleConfig),
		spec("poetry.lock", "data.toml", RoleLock, RoleConfig), spec("pipfile.lock", "data.json", RoleLock, RoleConfig),

		// v0.2 containers, task runners, and CI
		spec("dockerfile", "manifest.container", RoleInfra, RoleExecutable), spec("containerfile", "manifest.container", RoleInfra, RoleExecutable),
		spec("docker-compose.yml", "data.yaml", RoleInfra, RoleConfig), spec("compose.yaml", "data.yaml", RoleInfra, RoleConfig),
		spec("makefile", "manifest.generic", RoleExecutable, RoleTooling), spec("cmakelists.txt", "manifest.generic", RoleConfig, RoleTooling),
		spec("justfile", "manifest.generic", RoleExecutable, RoleTooling), spec("taskfile.yml", "data.yaml", RoleExecutable, RoleTooling),
		spec(".gitlab-ci.yml", "data.yaml", RoleInfra, RoleConfig), spec("azure-pipelines.yml", "data.yaml", RoleInfra, RoleConfig),
		spec("jenkinsfile", "manifest.generic", RoleInfra, RoleExecutable), spec("vagrantfile", "source.ruby", RoleInfra, RoleExecutable),
		spec("ansible.cfg", "data.ini", RoleInfra, RoleConfig), spec(".terraform.lock.hcl", "document.text", RoleLock, RoleInfra),
		spec(".bazeliskrc", "data.ini", RoleConfig, RoleTooling), spec(".pre-commit-config.yaml", "data.yaml", RoleConfig, RoleTooling),

		// v0.2 JavaScript tooling and contracts
		spec("tsconfig.json", "data.json", RoleConfig, RoleTooling), spec("jsconfig.json", "data.json", RoleConfig, RoleTooling),
		spec("eslint.config.js", "source.javascript", RoleConfig, RoleTooling), spec("prettier.config.js", "source.javascript", RoleConfig, RoleTooling),
		spec("vitest.config.ts", "source.typescript", RoleTest, RoleConfig), spec("jest.config.js", "source.javascript", RoleTest, RoleConfig),
		spec("tailwind.config.js", "source.javascript", RoleConfig, RoleTooling), spec("vite.config.ts", "source.typescript", RoleConfig, RoleTooling),
		spec("graphql.schema", "source.graphql", RoleContract, RoleData), spec("buf.yaml", "data.yaml", RoleConfig, RoleTooling),
		spec("buf.gen.yaml", "data.yaml", RoleGenerated, RoleConfig), spec("renovate.json", "data.json", RoleConfig, RoleTooling),
		spec("dependabot.yml", "data.yaml", RoleSecurity, RoleConfig), spec("mkdocs.yml", "data.yaml", RoleDocument, RoleConfig),
		spec("book.toml", "data.toml", RoleDocument, RoleConfig), spec("docker-bake.hcl", "document.text", RoleInfra, RoleConfig),

		// v0.3 JavaScript / TypeScript / Node ecosystem
		spec("bun.lock", "document.text", RoleLock, RoleConfig), spec("deno.json", "data.json", RoleConfig, RoleTooling),
		spec("deno.jsonc", "data.json", RoleConfig, RoleTooling), spec("biome.json", "data.json", RoleConfig, RoleTooling),
		spec("biome.jsonc", "data.json", RoleConfig, RoleTooling), spec("turbo.json", "data.json", RoleConfig, RoleTooling),
		spec("nx.json", "data.json", RoleConfig, RoleTooling), spec("lerna.json", "data.json", RoleConfig, RoleTooling),
		spec("playwright.config.ts", "source.typescript", RoleTest, RoleConfig), spec("playwright.config.js", "source.javascript", RoleTest, RoleConfig),
		spec("playwright.config.mts", "source.typescript", RoleTest, RoleConfig), spec("playwright.config.cts", "source.typescript", RoleTest, RoleConfig),
		spec("playwright.config.mjs", "source.javascript", RoleTest, RoleConfig), spec("playwright.config.cjs", "source.javascript", RoleTest, RoleConfig),
		spec("cypress.config.ts", "source.typescript", RoleTest, RoleConfig), spec("cypress.config.js", "source.javascript", RoleTest, RoleConfig),
		spec("cypress.config.mts", "source.typescript", RoleTest, RoleConfig), spec("cypress.config.mjs", "source.javascript", RoleTest, RoleConfig),
		spec("cypress.config.cjs", "source.javascript", RoleTest, RoleConfig),
		spec("next.config.ts", "source.typescript", RoleConfig, RoleTooling), spec("next.config.js", "source.javascript", RoleConfig, RoleTooling),
		spec("next.config.mjs", "source.javascript", RoleConfig, RoleTooling), spec("nuxt.config.ts", "source.typescript", RoleConfig, RoleTooling),
		spec("nuxt.config.js", "source.javascript", RoleConfig, RoleTooling), spec("nuxt.config.mjs", "source.javascript", RoleConfig, RoleTooling),
		spec("pnpm-workspace.yaml", "data.yaml", RoleConfig, RoleTooling), spec(".npmrc", "data.ini", RoleConfig, RoleTooling),
		spec(".yarnrc.yml", "data.yaml", RoleConfig, RoleTooling),

		// v0.3 Go
		spec("go.work", "manifest.go", RoleContract, RoleConfig), spec("go.work.sum", "document.text", RoleLock, RoleConfig),
		spec(".golangci.yml", "data.yaml", RoleConfig, RoleTooling), spec(".golangci.yaml", "data.yaml", RoleConfig, RoleTooling),
		spec(".goreleaser.yml", "data.yaml", RoleConfig, RoleTooling), spec(".goreleaser.yaml", "data.yaml", RoleConfig, RoleTooling),

		// v0.3 Dart / Flutter
		spec("pubspec.lock", "data.yaml", RoleLock, RoleConfig), spec("analysis_options.yaml", "data.yaml", RoleConfig, RoleTooling),
		spec("build.yaml", "data.yaml", RoleConfig, RoleTooling), spec(".flutter-plugins", "document.text", RoleGenerated, RoleConfig),
		spec(".flutter-plugins-dependencies", "document.text", RoleGenerated, RoleConfig),

		// v0.3 Python
		spec("requirements.txt", "manifest.python", RoleConfig), spec("requirements-dev.txt", "manifest.python", RoleTest, RoleConfig),
		spec("pipfile", "manifest.python", RoleContract, RoleConfig), spec("uv.lock", "data.toml", RoleLock, RoleConfig),
		spec("tox.ini", "data.ini", RoleTest, RoleConfig), spec("pytest.ini", "data.ini", RoleTest, RoleConfig),
		spec("ruff.toml", "data.toml", RoleConfig, RoleTooling), spec("mypy.ini", "data.ini", RoleConfig, RoleTooling),
		spec(".python-version", "document.text", RoleConfig, RoleTooling), spec("conftest.py", "source.python", RoleTest, RoleSource),

		// v0.3 Rust
		spec("rust-toolchain", "document.text", RoleConfig, RoleTooling), spec("rust-toolchain.toml", "data.toml", RoleConfig, RoleTooling),

		// v0.3 JVM / Gradle
		spec("build.gradle.kts", "manifest.java", RoleConfig, RoleTooling), spec("settings.gradle", "manifest.java", RoleConfig, RoleTooling),
		spec("settings.gradle.kts", "manifest.java", RoleConfig, RoleTooling), spec("gradle.properties", "data.properties", RoleConfig, RoleTooling),
		spec("gradlew", "source.shell", RoleExecutable, RoleTooling), spec("gradlew.bat", "source.batch", RoleExecutable, RoleTooling),

		// v0.3 .NET
		spec("directory.build.props", "manifest.dotnet", RoleConfig), spec("directory.build.targets", "manifest.dotnet", RoleConfig),
		spec("global.json", "data.json", RoleConfig, RoleTooling), spec("nuget.config", "data.xml", RoleConfig),
		spec("packages.lock.json", "data.json", RoleLock, RoleConfig, RoleData),

		// v0.3 PHP / Ruby / Elixir / Erlang
		spec("composer.lock", "data.json", RoleLock, RoleConfig, RoleData), spec("phpunit.xml", "data.xml", RoleTest, RoleConfig),
		spec("gemfile", "manifest.generic", RoleContract, RoleConfig), spec("gemfile.lock", "document.text", RoleLock, RoleConfig),
		spec("rakefile", "source.ruby", RoleExecutable, RoleTooling), spec("mix.exs", "source.elixir", RoleContract, RoleConfig),
		spec("mix.lock", "document.text", RoleLock, RoleConfig), spec("rebar.config", "source.erlang", RoleConfig, RoleTooling),

		// v0.3 Terraform state (HCL sources use manifest.terraform; generic .hcl stays source.hcl)
		spec("terraform.tfstate", "data.json", RoleInfra, RoleData), spec("terraform.tfstate.backup", "data.json", RoleGenerated, RoleInfra, RoleData),

		// v0.3 Nix manifests (generic .nix stays source.nix)
		spec("flake.nix", "manifest.nix", RoleContract, RoleConfig), spec("default.nix", "manifest.nix", RoleConfig),
		spec("shell.nix", "manifest.nix", RoleConfig), spec("flake.lock", "data.json", RoleLock, RoleConfig),

		// v0.3 Kubernetes / Helm / GitOps
		spec("chart.yaml", "manifest.helm", RoleContract, RoleInfra, RoleConfig), spec("chart.lock", "manifest.helm", RoleLock, RoleInfra),
		spec("values.yaml", "manifest.helm", RoleInfra, RoleConfig), spec("kustomization.yaml", "data.yaml", RoleInfra, RoleConfig),
		spec("helmfile.yaml", "manifest.helm", RoleInfra, RoleConfig), spec("skaffold.yaml", "data.yaml", RoleInfra, RoleConfig),

		// v0.3 containers
		spec(".dockerignore", "document.text", RoleInfra, RoleConfig), spec("compose.yml", "data.yaml", RoleInfra, RoleConfig),
		spec("docker-compose.yaml", "data.yaml", RoleInfra, RoleConfig), spec("devcontainer.json", "data.json", RoleConfig, RoleTooling),

		// v0.3 CI/CD
		spec("action.yml", "data.yaml", RoleInfra, RoleConfig), spec("action.yaml", "data.yaml", RoleInfra, RoleConfig),
		spec("cloudbuild.yaml", "data.yaml", RoleInfra, RoleConfig),

		// v0.3 editor and runtime pins
		spec(".tool-versions", "document.text", RoleConfig, RoleTooling), spec(".nvmrc", "document.text", RoleConfig, RoleTooling),
		spec(".node-version", "document.text", RoleConfig, RoleTooling),

		// v0.3 documentation variants
		spec("readme", "document.text", RoleContract, RoleDocument), spec("readme.txt", "document.text", RoleContract, RoleDocument),
		spec("readme.rst", "document.rst", RoleContract, RoleDocument), spec("license.md", "document.license", RoleContract, RoleDocument),
		spec("licence.md", "document.license", RoleContract, RoleDocument), spec("changelog", "document.changelog", RoleContract, RoleDocument),
		spec("changelog.txt", "document.changelog", RoleContract, RoleDocument), spec("contributors", "document.text", RoleContract, RoleDocument),

		// v0.3 environment files (structural security signal, not a secret verdict)
		spec(".env.local", "data.env", RoleSecurity, RoleConfig), spec(".env.development", "data.env", RoleSecurity, RoleConfig),
		spec(".env.production", "data.env", RoleSecurity, RoleConfig), spec(".env.test", "data.env", RoleSecurity, RoleConfig),
		spec(".env.example", "data.env", RoleConfig),

		// v0.3 Git and ignore files
		spec(".gitignore", "document.text", RoleConfig, RoleTooling), spec(".gitattributes", "document.text", RoleConfig, RoleTooling),
		spec(".gitmodules", "document.text", RoleConfig, RoleVendor), spec(".eslintignore", "document.text", RoleConfig, RoleTooling),
		spec(".prettierignore", "document.text", RoleConfig, RoleTooling),

		// v0.3 platform manifests
		spec("vercel.json", "data.json", RoleInfra, RoleConfig), spec("netlify.toml", "data.toml", RoleInfra, RoleConfig),
		spec("fly.toml", "data.toml", RoleInfra, RoleConfig), spec("procfile", "document.text", RoleInfra, RoleExecutable),
		spec("brewfile", "source.ruby", RoleConfig, RoleTooling),
	}
}
