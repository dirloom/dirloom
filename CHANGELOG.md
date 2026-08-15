# Changelog

All notable changes to Dirloom are documented here. The project follows Semantic Versioning.

## [Unreleased]

### Added

- Add official Scoop distribution for Windows x64 and ARM64 with automatic updates from published stable releases.
- Add strict layered configuration through `.dirloom.yaml`, the native user configuration directory and explicit CLI overrides.
- Add Git-bounded project discovery, additive ignore rules and `dirloom config explain` diagnostics with value provenance.
- Add `--config`, `--no-user-config`, `--no-config` and `--depth unlimited`.
- Publish and automatically validate the public persistent-configuration guide.
- Add the inspectable `docs`, `compact`, `monorepo` and `ai` presets with CLI and YAML activation.
- Add `dirloom preset explain` text and JSON contracts plus preset provenance in `config explain`.
- Publish and automatically validate the built-in preset guide.
- Add the deterministic `markdown-tree` format for native nested Markdown lists while preserving existing text, fenced Markdown and JSON contracts.
- Publish and automatically validate the semantic Markdown guide.
- Add terminal-safe `--color`, `--icons`, and `--theme` controls with TTY-aware defaults and `NO_COLOR` support.
- Add immutable `default`, `midnight`, and `daylight` themes plus strict, confined custom YAML themes.
- Add `dirloom theme list`, `theme explain`, and `theme validate` text and JSON contracts.
- Preserve canonical text, Markdown, and tree JSON through a presentation-only renderer projection.
- Publish and automatically validate the terminal themes guide, icon provenance, and pipeline guarantees.

## [0.1.1] - 2026-08-11

### Fixed

- Resolve the release version from Go module build metadata when Dirloom is installed with `go install module@version`, while preserving GoReleaser linker injection and the `dev` fallback for local builds.

## [0.1.0] - 2026-08-11

### Added

- Deterministic cross-platform directory scanning with depth, directory-only and hidden-entry controls.
- Ordered default, CLI and scoped `.gitignore` filtering with directory pruning.
- Terminal symlink and Windows junction representation without recursive traversal.
- Unicode, ASCII, Markdown and JSON schema v1 output contracts.
- Transactional `--output` with self-exclusion and safe atomic replacement.
- Stable CLI help, version behavior and exit codes.
- Unit, integration, contract, CLI and benchmark coverage.
- Windows, Linux and macOS CI plus GoReleaser archives for amd64 and arm64.

[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/dirloom/dirloom/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/dirloom/dirloom/releases/tag/v0.1.0
