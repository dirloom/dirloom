# Contributing to Dirloom

Thank you for helping improve Dirloom.

## Prerequisites

- Go 1.25.12 or newer;
- Git;
- optional: GoReleaser 2.17.1 for release snapshots;
- optional: golangci-lint 2.12.2 for the same lint pass as CI.

## Development workflow

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

## Pull requests

Explain the user-visible problem, the chosen behavior, tests added and cross-platform considerations. Keep unrelated refactors separate. CI must pass before merge.
