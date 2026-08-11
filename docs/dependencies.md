# Dependency decisions

Dirloom v0.1 intentionally has three direct runtime dependencies.

## Cobra v1.10.2

`github.com/spf13/cobra` is the framework required by the product specification. It provides stable POSIX-style flag parsing, generated help and version handling, and a path to future explicit subcommands. It is actively maintained and licensed under Apache-2.0.

Dirloom wraps Cobra in `internal/cli` and keeps application behavior independent from it.

## git-pkgs/gitignore v1.2.0

`github.com/git-pkgs/gitignore` was selected over the older matcher exposed by `go-git` because it uses a direct implementation modeled on Git's `wildmatch.c`, includes Git wildmatch conformance tests, handles standalone `**`, bracket expressions, escapes, directory-only patterns, negation and nested scopes, and is independent of OS path matching. It is MIT-licensed and has no transitive dependencies.

Dirloom creates the matcher with an empty root and feeds only `.gitignore` files encountered beneath the explicitly selected root. This intentionally prevents the library from reading global excludes or `.git/info/exclude`, which are outside the v0.1 contract. The dependency is encapsulated by `internal/filter.GitIgnore` and backed by Dirloom-specific conformance and integration tests.

## x/sys v0.47.0

`golang.org/x/sys/windows` provides the typed `MoveFileEx` binding needed for safe replacement of an existing output file on Windows. The standard `os.Rename` contract cannot guarantee replacement there. Using the official Go extended-system package avoids maintaining an unsafe local syscall wrapper. `x/sys` is maintained by the Go project and licensed under BSD-3-Clause.

## Review policy

Dependency upgrades require:

1. reviewing upstream release and security notes;
2. confirming license compatibility;
3. running the full cross-platform test suite;
4. preserving the CLI and JSON contracts;
5. updating this document when the rationale changes.
