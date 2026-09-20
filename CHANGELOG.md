# Changelog

All notable changes to Dirloom are documented here. The project follows Semantic Versioning.

## [Unreleased]

### Added

- Add `dirloom fingerprint` to identify the observed structural view as `dlm:v1:sha256:<digest>` without hashing file contents.
- Add Canonical Structural Artifact v1, Identity Projection v1 and Canonical Identity Encoding v1, reused by a single existing scanner traversal.
- Publish fingerprint text and JSON (`schemaVersion` 1) contracts, plus architecture, encoding and command reference docs.

### Changed

- Make `dirloom/dirloom` the unique Homebrew publisher: patch cask version and
  SHA-256 fields only, delete orphan `dirloom-<version>` branches, and keep the
  tap `Update cask` workflow as manual recovery (`workflow_dispatch`) rather
  than a scheduled writer.
- Mark SemVer prerelease tags as GitHub Pre-releases (`prerelease: auto`) and
  skip Scoop, Homebrew and Winget on those publications unless a maintainer
  uses `workflow_dispatch`.

## [0.3.3] - 2026-09-18

PRESENTATION refinement: Nerd catalog fidelity, provenance, and classification
refinement, without changing `catalogVersion`, matcher counts, ASCII, Unicode,
the theme schema, or the default `icons: never`.

### Fixed

- Correct Nerd glyph mappings for JPEG, SVG, LICENSE and CHANGELOG.
- Replace misleading technology pseudo-logos with verified Nerd Fonts glyphs
  or conservative semantic fallbacks.

### Changed

- Govern technology-specific Nerd glyphs using pinned, documented upstream
  collections and provenance (Nerd Fonts v3.5.1).
- Classify Docker Compose and Docker Bake files as `manifest.container`.
- Classify `.terraform.lock.hcl` as `manifest.terraform` while preserving its
  lock and infrastructure roles.

`catalogVersion` remains 1 with 506 matchers, 119 kinds, and 16 roles. ASCII
and Unicode catalogs are unchanged.

Manual terminal validation
- Windows Terminal / Nerd Font: PASS
- style=unicode × icons=nerd: PASS
- style=ascii × icons=nerd: PASS
- vivid × nerd: PASS

Optional extended terminal matrix
- WezTerm / Mono: NOT RUN — terminal unavailable
- WezTerm / non-Mono: NOT RUN — terminal unavailable
- Alacritty / Mono: NOT RUN — terminal unavailable
- Alacritty / non-Mono: NOT RUN — terminal unavailable
- Release blocker: NO

WezTerm and Alacritty were not available. That absence does not block v0.3.3:
Nerd glyph/codepoint validity is covered automatically, Nerd Fonts v3.5.1
provenance is pinned, ASCII and Unicode channels are frozen by tests, spacing
behavior is tested, Dirloom makes no terminal-width assumption, and the full
CI matrix is green.

## [0.3.2] - 2026-09-18

PRESENTATION refinement: portable ASCII icons and a declarative Nerd Font
capability, without redefining the v0.3 visual contract or the default
`icons: never`.

### Added

- Add strict ASCII semantic icons through `--icons ascii`.
- Add a declarative Nerd Font capability via `DIRLOOM_NERD_FONT` and user-only
  `terminal.capabilities.nerdFont`.

### Changed

- Resolve `--icons auto` conservatively: Nerd only when that capability is
  declared, otherwise portable Unicode. Auto no longer follows TTY, CI, pipe,
  or `--output` heuristics.

## [0.3.1] - 2026-09-18

PRESENTATION refinement: contextual CLI help and guided usage, without
redefining the v0.3 visual contract or the default `icons: never`.

### Added

- Contextual CLI help through `dirloom help <topic>`.
- `dirloom help topics`, `examples`, and `concepts`.
- Shell completion for command and contextual help targets.
- Actionable guidance and suggestions for invalid CLI usage.

### Changed

- `--icons` without an explicit value now behaves as `--icons=auto`.
- `--color` without an explicit value now behaves as `--color=auto`.

### Fixed

- `--help` and `--version` are no longer consumed as the value of `--icons` or `--color`.

## [0.3.0] - 2026-09-17

Presentation richness: additive semantic catalog (506 matchers, 119 kinds) and
richer built-in theme identity, without changing canonical formats, the theme
schema, or the default `icons: never`.

### Added

- Extend semantic catalog v1 additively to 506 matchers and 119 technical kinds while keeping `catalogVersion: 1` and the 16 structural roles.
- Recognize additional languages, manifests, lockfiles, CI/CD names, generated suffixes, project directories, media/archive formats, and certificate/key extensions.
- Classify Terraform/OpenTofu sources as `manifest.terraform` and well-known Nix files as `manifest.nix`, while generic `.hcl` and `.nix` stay source kinds.
- Add exact filenames for Playwright, Cypress, Next.js, and Nuxt config variants instead of prefix matchers.
- Add more specific Nerd glyphs for language, manifest, data, and media kinds, with the existing Unicode fallbacks unchanged.
- Materialize a synthetic `testdata/showcase` corpus covering Go, TypeScript, Node, Flutter, Python, Rust, .NET, JVM, Terraform/Kubernetes, and mixed-platform trees.

### Changed

- Promote selected real-world paths that previously lost to a generic v0.2 extension or fallback. The 256 v0.2 matcher identities are unchanged; `requirements.txt` becomes `manifest.python`, `Chart.yaml` becomes `manifest.helm`, `go.work` becomes `manifest.go`, Terraform `.tf` becomes `manifest.terraform`, and `build.gradle.kts` becomes `manifest.java`.
- Built-in themes inherit the expanded catalog. `vivid` adds `icon-security` and `icon-infra` so certificate/key and Helm/Terraform identities stay two-tone without changing the public theme schema.
- Public catalog, theme, README, and roadmap docs now describe the additive v0.3 coverage, the 64/40/32/120 v0.2 matcher groups, and the distinction between frozen matcher identities and intentional path promotions.

## [0.2.0] - 2026-09-16

### Added

- Add `--copy` to place the rendered tree on the native clipboard with silent success, exclusive destination, and OS backends for Windows, macOS, Linux and WSL.
- Add `dirloom completion` for Bash, Zsh, Fish and PowerShell with semantic flag values and no profile mutation.
- Publish clipboard, completion and distribution guides, including Release Done versus Distribution Verified.
- Add SPDX SBOMs, a 13-artifact GitHub Release inventory, checksums covering archives and SBOMs, and GitHub Artifact Attestations.
- Add GitOps package templates and published-release automation for Scoop, Homebrew (`dirloom/tap/dirloom`) and Winget (`Dirloom.Dirloom`).
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
- Add immutable `default`, `midnight`, `daylight`, and `vivid` themes plus strict, confined custom YAML themes.
- Add the semantic catalog v1 with 256 matchers, 96 hierarchical technical kinds, 16 ordered structural roles, and portable Unicode/Nerd glyph fallbacks.
- Add `dirloom theme list`, `theme explain`, `theme validate`, and bounded filesystem-aware `theme classify` text and JSON contracts.
- Preserve canonical text, Markdown, and tree JSON through a presentation-only renderer projection.
- Publish and automatically validate the terminal themes guide, icon provenance, and pipeline guarantees.
- Add deterministic `mermaid`, `graphviz` (alias `dot`) and `d2` structure-graph sources from a canonical `diagram.Document`.
- Publish and automatically validate the graphical export guide, including official parser checks in CI.

### Changed

- Replace the pre-release theme prototype with the definitive public theme schema v1; `catalogVersion: 1` is now required and no legacy loader is retained.
- Change the built-in icon mode from `auto` to `never`; Unicode and Nerd glyphs are now explicitly activated while automatic color remains unchanged.
- Resolve icon color separately from text color and emit independent ANSI spans and resets.
- Redesign `vivid` as an independent two-tone neon theme with role-driven text, kind-driven icon colors, stronger semantic emphasis, and WCAG AA contrast for every built-in color.
- Change the GitHub Release checksum file so it covers the six archives and six SBOMs and never hashes itself.
- Convert Scoop updates from direct commits on `main` to idempotent version pull requests.

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

[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.3.3...HEAD
[0.3.3]: https://github.com/dirloom/dirloom/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/dirloom/dirloom/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/dirloom/dirloom/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/dirloom/dirloom/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/dirloom/dirloom/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/dirloom/dirloom/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/dirloom/dirloom/releases/tag/v0.1.0
