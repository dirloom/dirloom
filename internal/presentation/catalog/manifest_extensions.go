package catalog

func extensionSpecs() []entrySpec {
	return []entrySpec{
		// v0.2 compiled and systems languages
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

		// v0.3 additional source languages
		spec(".hs", "source.haskell", RoleSource), spec(".lhs", "source.haskell", RoleDocument, RoleSource),
		spec(".ml", "source.ocaml", RoleSource), spec(".mli", "source.ocaml", RoleContract, RoleSource),
		spec(".nim", "source.nim", RoleSource), spec(".nims", "source.nim", RoleSource),
		spec(".d", "source.d", RoleSource), spec(".f90", "source.fortran", RoleSource),
		spec(".f95", "source.fortran", RoleSource), spec(".f03", "source.fortran", RoleSource),
		spec(".for", "source.fortran", RoleSource), spec(".f", "source.fortran", RoleSource),
		spec(".gleam", "source.gleam", RoleSource), spec(".scm", "source.scheme", RoleSource),
		spec(".ss", "source.scheme", RoleSource), spec(".rkt", "source.racket", RoleSource),
		spec(".elm", "source.elm", RoleSource), spec(".v", "source.v", RoleSource),
		spec(".cr", "source.crystal", RoleSource), spec(".nix", "source.nix", RoleConfig, RoleSource),
		spec(".hcl", "source.hcl", RoleConfig, RoleSource), spec(".cue", "source.cue", RoleConfig, RoleSource),
		spec(".jsonnet", "source.jsonnet", RoleConfig, RoleSource), spec(".libsonnet", "source.jsonnet", RoleSource),
		spec(".tf", "manifest.terraform", RoleInfra, RoleSource), spec(".tfvars", "manifest.terraform", RoleInfra, RoleConfig),
		spec(".tofu", "manifest.terraform", RoleInfra, RoleSource), spec(".tfstate", "data.json", RoleInfra, RoleData),

		// v0.3 .NET project files
		spec(".sln", "manifest.dotnet", RoleConfig), spec(".csproj", "manifest.dotnet", RoleConfig, RoleSource),
		spec(".fsproj", "manifest.dotnet", RoleConfig, RoleSource), spec(".vbproj", "manifest.dotnet", RoleConfig),
		spec(".vcxproj", "manifest.dotnet", RoleConfig), spec(".gradle", "manifest.java", RoleConfig, RoleTooling),

		// v0.3 media, design, audio, and video
		spec(".gif", "media.image", RoleMedia), spec(".webp", "media.image", RoleMedia),
		spec(".avif", "media.image", RoleMedia), spec(".ico", "media.image", RoleMedia),
		spec(".bmp", "media.image", RoleMedia), spec(".tif", "media.image", RoleMedia),
		spec(".tiff", "media.image", RoleMedia), spec(".flac", "media.audio", RoleMedia),
		spec(".ogg", "media.audio", RoleMedia), spec(".m4a", "media.audio", RoleMedia),
		spec(".mov", "media.video", RoleMedia), spec(".mkv", "media.video", RoleMedia),
		spec(".avi", "media.video", RoleMedia), spec(".psd", "media.design", RoleMedia),
		spec(".ai", "media.design", RoleMedia), spec(".fig", "media.design", RoleMedia),

		// v0.3 archives and packages
		spec(".7z", "archive.compressed", RoleArchive), spec(".bz2", "archive.compressed", RoleArchive),
		spec(".xz", "archive.compressed", RoleArchive), spec(".zst", "archive.compressed", RoleArchive),
		spec(".rar", "archive.compressed", RoleArchive), spec(".apk", "archive.package", RoleArchive),
		spec(".aab", "archive.package", RoleArchive), spec(".ipa", "archive.package", RoleArchive),
		spec(".msi", "archive.package", RoleExecutable, RoleArchive), spec(".nupkg", "archive.package", RoleArchive),
		spec(".gem", "archive.package", RoleArchive),

		// v0.3 certificates and keys (structural security signal)
		spec(".pem", "data.certificate", RoleSecurity), spec(".crt", "data.certificate", RoleSecurity),
		spec(".cer", "data.certificate", RoleSecurity), spec(".key", "data.key", RoleSecurity),
		spec(".pub", "data.key", RoleSecurity), spec(".p12", "data.certificate", RoleSecurity),
		spec(".pfx", "data.certificate", RoleSecurity), spec(".asc", "data.certificate", RoleSecurity),
		spec(".sig", "data.certificate", RoleSecurity),

		// v0.3 structured data, localization, and fonts
		spec(".properties", "data.properties", RoleConfig, RoleData), spec(".plist", "data.plist", RoleConfig, RoleData),
		spec(".strings", "data.localization", RoleData), spec(".otf", "font", RoleMedia),
		spec(".eot", "font.web", RoleMedia),
	}
}
