# Contributing to Dirloom

Thank you for helping improve Dirloom.

## Prerequisites

- Go 1.25.12 or newer;
- Git;
- optional: GoReleaser 2.17.1 for release snapshots;
- optional: golangci-lint 2.12.2 for the same lint pass as CI.

## Development workflow

Versioned product work lands by pull request on `release/vX.Y.Z`. The v0.4.0-a2 freeze lives on `release/v0.4.0-a2`; that branch accepts only freeze and release-candidate work. Do not open a feature PR to `main` for this version. See
[Release workflow](docs/release-workflow.md). Release notes are governed by
[CHANGELOG policy](docs/release/changelog-policy.md) and the
[release notes template](docs/release/release-notes-template.md): update
`CHANGELOG.md` only; never invent a separate GitHub Release description.

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
goreleaser check
goreleaser release --snapshot --clean --skip=publish
go run ./cmd/release-artifacts prepare --dist dist --syft syft
go run ./cmd/release-artifacts verify --dist dist
```

Catalog expansions must keep `internal/presentation/catalog/testdata/classification-v0.2.yaml` frozen. That fixture is never regenerated automatically; `DIRLOOM_WRITE_CATALOG_FIXTURE=1` rewrites only `classification-v1.yaml`. After matcher changes, regenerate the exhaustive v1 fixture with that variable and materialize `testdata/showcase` with `DIRLOOM_WRITE_SHOWCASE=1`. Do not introduce prefix matchers, `catalogVersion: 2`, new roles, or a default icon mode other than `never`. Path promotions such as `requirements.txt` or `Chart.yaml`, and v0.3.3 classification corrections such as `docker-compose.yml`, must be changelogued as intentional classification changes, distinct from the 256 frozen v0.2 matcher identities. Nerd glyph changes must keep ASCII and Unicode frozen and must record provenance against the pinned Nerd Fonts version.

## Compatibility expectations

Windows, Linux and macOS are first-class targets. Do not introduce shell-specific product behavior. Preserve UTF-8 without BOM, LF-only output, stable ordering and exactly one final LF in all public formats.

Changes to JSON fields, CLI flags, default exclusions, filter priority or ordering are public-contract changes and require explicit discussion plus contract tests.

Changes to `.dirloom.yaml`, configuration discovery, precedence, diagnostic fields, source statuses, preset names, preset definitions, visual defaults, built-in themes, icon mappings, rule priority, theme schema, diagram export contracts, clipboard behavior, or completion scripts are also public-contract changes. Update implementation tests, CLI tests, the relevant public guide (`docs/configuration.md`, `docs/presets.md`, `docs/themes.md`, `docs/graph-exports.md`, `docs/clipboard-and-completions.md`, or `docs/distribution.md`), README and changelog together. Marked YAML, JSON and command examples in the public guides are checked by the test suite and must remain executable.

Presentation changes must prove that neutral text goldens, Markdown, and tree JSON remain unchanged. Diagram encoder changes must prove Mermaid, Graphviz and D2 goldens, hostile-name escaping, and that no encoder accepts `tree.Node` directly. Test `NO_COLOR`, TTY and non-TTY behavior, forced modes, custom-theme safety limits, and Windows terminal setup where applicable. Clipboard tests must use the injectable writer and must not require a real OS clipboard. Never make the scanner or `app.Inspect` depend on ANSI, glyphs, themes, or terminal state. Official Mermaid CLI, Graphviz `dot` and D2 parsers belong in CI only; do not add them to the Dirloom binary.

Every GitHub Action used by CI or release must be pinned to a full commit SHA. Every tool downloaded in those workflows must have a deterministic version and, when fetched as a binary, a checksum in the repository. Changes to `.github/workflows/release.yml`, `.github/workflows/update-packages.yml`, or packaging automation require two independent approvals. Mechanical version PRs in Scoop, Homebrew and Winget repositories require one maintainer approval.

## Pull requests

Explain the user-visible problem, the chosen behavior, tests added and cross-platform considerations. Keep unrelated refactors separate. CI must pass before merge.
