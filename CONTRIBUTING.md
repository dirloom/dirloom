# Contributing to Dirloom

Thank you for helping improve Dirloom.

## Prerequisites

- Go 1.25.12 or newer;
- Git;
- optional: GoReleaser 2.17.1 for release snapshots;
- optional: golangci-lint 2.12.2 for the same lint pass as CI.

## Development workflow

While `release/v0.2.0` is open, base feature and fix branches on
`release/v0.2.0` and open pull requests against that branch. See
[Release workflow](docs/release-workflow.md).

1. Create a focused branch.
2. Add or update tests with every behavior change.
3. Keep filesystem scanning, filtering, the tree model and rendering separated.
4. Run the required checks:

```bash
gofmt -w ./cmd ./internal
go mod tidy
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/dirloom
```

If installed, also run:

```bash
golangci-lint run
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
goreleaser release --snapshot --clean
```

## Compatibility expectations

Windows, Linux and macOS are first-class targets. Do not introduce shell-specific product behavior. Preserve UTF-8 without BOM, LF-only output, stable ordering and exactly one final LF in all public formats.

Changes to JSON fields, CLI flags, default exclusions, filter priority or ordering are public-contract changes and require explicit discussion plus contract tests.

Changes to `.dirloom.yaml`, configuration discovery, precedence, diagnostic fields, source statuses, preset names, preset definitions, visual defaults, built-in themes, icon mappings, rule priority, theme schema, or diagram export contracts are also public-contract changes. Update implementation tests, CLI tests, the relevant public guide (`docs/configuration.md`, `docs/presets.md`, `docs/themes.md`, or `docs/graph-exports.md`), README and changelog together. Marked YAML, JSON and command examples in the public guides are checked by the test suite and must remain executable.

Presentation changes must prove that neutral text goldens, Markdown, and tree JSON remain unchanged. Diagram encoder changes must prove Mermaid, Graphviz and D2 goldens, hostile-name escaping, and that no encoder accepts `tree.Node` directly. Test `NO_COLOR`, TTY and non-TTY behavior, forced modes, custom-theme safety limits, and Windows terminal setup where applicable. Never make the scanner or `app.Inspect` depend on ANSI, glyphs, themes, or terminal state. Official Mermaid CLI, Graphviz `dot` and D2 parsers belong in CI only; do not add them to the Dirloom binary.

## Pull requests

Explain the user-visible problem, the chosen behavior, tests added and cross-platform considerations. Keep unrelated refactors separate. CI must pass before merge.
