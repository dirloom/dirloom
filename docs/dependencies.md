# Dependency decisions

Dirloom keeps its direct runtime dependencies small and encapsulated behind internal packages.

## Cobra v1.10.2

`github.com/spf13/cobra` is the framework required by the product specification. It provides stable POSIX-style flag parsing, generated help and version handling, and a path to future explicit subcommands. It is actively maintained and licensed under Apache-2.0.

Dirloom wraps Cobra in `internal/cli` and keeps application behavior independent from it.

## git-pkgs/gitignore v1.2.0

`github.com/git-pkgs/gitignore` was selected over the older matcher exposed by `go-git` because it uses a direct implementation modeled on Git's `wildmatch.c`, includes Git wildmatch conformance tests, handles standalone `**`, bracket expressions, escapes, directory-only patterns, negation and nested scopes, and is independent of OS path matching. It is MIT-licensed and has no transitive dependencies.

Dirloom creates the matcher with an empty root and feeds only `.gitignore` files encountered beneath the explicitly selected root. This intentionally prevents the library from reading global excludes or `.git/info/exclude`, which are outside the v0.1 contract. The dependency is encapsulated by `internal/filter.GitIgnore` and backed by Dirloom-specific conformance and integration tests.

## x/sys v0.47.0

`golang.org/x/sys/windows` provides the typed bindings needed for safe output replacement, temporary `ENABLE_VIRTUAL_TERMINAL_PROCESSING` setup, and native clipboard access (`CF_UNICODETEXT`) on Windows. The standard library cannot guarantee replacement through `os.Rename` there and does not expose the required console-mode or clipboard operations. Using the official Go extended-system package avoids maintaining unsafe local syscall wrappers. `x/sys` is maintained by the Go project and licensed under BSD-3-Clause. Dirloom does not add a third-party clipboard library.

## x/term v0.45.0

`golang.org/x/term` provides cross-platform terminal detection through `IsTerminal`. Dirloom uses it only at the presentation boundary: scanning, canonical Markdown, JSON, diagram sources, theme inspection, diagnostics, help, and errors do not depend on terminal state.

Version `v0.45.0` targets Go 1.25 and uses the already pinned `x/sys v0.47.0`. The dependency is maintained by the Go project and licensed under BSD-3-Clause.

## yaml v3.0.4

`go.yaml.in/yaml/v3` parses the independent public `.dirloom.yaml` and custom-theme YAML formats. Version 3 provides typed decoding, an inspectable node tree and strict known-field validation, which Dirloom combines with its own rejection of duplicate keys, multiple documents, anchors, aliases, merge keys and custom tags.

The dependency is isolated behind `internal/config` and `internal/presentation` loaders. Dirloom does not enable YAML-based execution, includes, templates or environment interpolation. Configuration and theme schema versions remain independent. The module is dual-licensed under MIT and Apache-2.0.

## Review policy

Dependency upgrades require:

1. reviewing upstream release and security notes;
2. confirming license compatibility;
3. running the full cross-platform test suite;
4. preserving the CLI and JSON contracts;
5. updating this document when the rationale changes.
