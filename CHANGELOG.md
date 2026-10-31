# Changelog

All notable changes to Dirloom are documented here. The project follows Semantic Versioning.

## [Unreleased]

## [0.4.0-a7] - 2026-10-31

CHANGE alpha: structural history (`dirloom history`), one Fingerprint v1
and one structural change summary per commit along the first-parent chain
of a Git revision, and History Result Schema v1. This increment does not
add a full commit graph, per-path history, a persistent cache, Time
Machine views, or Git sources for other commands.

### Added

- Add `dirloom history [<revision>]`, a structural history of a Git repository. It walks the first-parent chain of a revision (default `HEAD`) and lists one entry per commit, newest first: its tree, its parents, its committer date in UTC, the Fingerprint v1 of its whole committed tree, its node count, and a summary of the added, removed, changed and moved paths against its first parent (the empty tree for a root commit), equal to the summary of `dirloom diff git:<commit>^ git:<commit>`. `structural` is true exactly when that summary is not empty: a commit that changes only file contents is `no structural change`. The walk stops after `--limit` commits (default 20), before `--from` (exclusive), at a root commit or at a shallow boundary, and the output names the end. A merge is listed once and compared with its first parent. `--structural-only` hides the commits without a structural change, and `--no-moves` counts moves as removals plus additions. Only committed Git objects are read, through one Git session per run, never the working tree, the index or the network; commit messages and identities are never shown. There is no exit 1. Git 2.45 or newer is required.
- Add History Result Schema v1 for `dirloom history --format json`, with an evolution policy from its first version: `metadata`, `range` (`to`, `from`, `limit`, `end`) and `entries`; at a shallow end the last entry has no `structural` or `summary`. Output is transactional: a failure at any commit yields one failure document, never a partial history.
- Add three diagnostics, all `OBSERVATION_ERROR` (exit 5): `git_commit_invalid` (a malformed commit object, or one above 1 MiB), `git_shallow_invalid` (an unreadable, oversized or malformed shallow boundary list) and `history_from_not_reached` (`--from` is not on the first-parent chain).
- Publish the Structural History v1 and History Result Schema v1 contracts, ADR 0007, the history reference and history benchmarks.

### Changed

- `history` is now a command name: `dirloom history` runs the structural history instead of inspecting a directory named `history`. Use `dirloom ./history` to inspect such a directory.

### Security

- Update `golang.org/x/text` from v0.39.0 to v0.41.0 for GO-2026-6629, which affects versions before v0.41.0. Dirloom imports only `golang.org/x/text/unicode/norm`, which govulncheck reported as not calling the vulnerable code, and that package, its Unicode tables included, is byte-identical between the two versions: NFC canonicalization, Fingerprint v1 and every structural identity are unchanged.

## [0.4.0-a6] - 2026-10-30

CHANGE alpha: Git sources in `dirloom diff` (`git:<revision>`), committed
trees observed through the same scanner as directories, and Diff Result
Schema v3. This increment does not add working-tree or index sources,
network fetches, submodule recursion, Git sources for other commands, or
history.

### Added

- Add Git sources to `dirloom diff`. `git:<revision>` observes the committed tree of a branch, a tag, a commit, `HEAD~n` or `<commit>:<path>` in the repository of the current directory, through the same scanner as a directory. `git:` against `git:` compares the whole trees; `git:` against `snapshot:` applies the snapshot's Capture Semantics; `git:` against `live:` is a usage error. Only committed Git objects are read, never the working tree, the index or the network; a submodule is an empty directory and link targets are kept as recorded on every platform. The comparison stays structural: a change of file contents alone is no difference. Git 2.45 or newer is required for `git:` sources only.
- Refuse, before Git runs, a `git:` revision that is empty, starts with `-`, contains a NUL or a newline, or names an index entry (`:<path>`): exit 2.
- Add five Git diagnostics, all `OBSERVATION_ERROR` (exit 5): `git_unavailable`, `git_repository_not_found`, `git_revision_not_found`, `git_object_unavailable` and `git_tree_invalid`.
- Publish the Git Tree Source v1 contract, ADR 0006, the diff reference update and Git observation benchmarks.

### Changed

- Change `dirloom diff --format json` to Diff Result Schema v3: `schemaVersion: 3`, the `git` source kind with `tree` and `commit`, and an evolution policy for additive changes. For `snapshot:` and `live:` sources a v3 document is the v2 document with `schemaVersion: 3`. Diff Result Schema v2 is no longer emitted; a consumer that checks `schemaVersion` fails instead of misreading the document. Statuses and exit codes are unchanged.

## [0.4.0-a5] - 2026-10-29

CHANGE alpha: conservative structural move detection in `dirloom diff`
(`structural-v1`), with the MOVED operation and Diff Result Schema v2. This
increment does not add content-based detection, Git sources or history.

### Added

- Add move detection (`structural-v1`) to `dirloom diff`. A removed and an added node are reported as one `MOVED` change when their name and structure occur exactly once in each source, or when a non-empty directory with a unique structure is renamed in place. A moved subtree is a single entry with `from`, `evidence` (`subtree` or `name`) and `nodes`. Ambiguous cases stay `REMOVED` plus `ADDED`. A `MOVED` is a structural correspondence, not proof of a move, and file contents are still never read.
- Add `dirloom diff --no-moves` to list every changed path as `v0.4.0-a4` did.
- Publish the Move Detection structural-v1 contract, ADR 0005, the diff reference update and move detection benchmarks.

### Changed

- Change `dirloom diff --format json` to Diff Result Schema v2: `schemaVersion: 2`, `metadata.moveDetection`, `summary.moved` counted in `total`, and the `MOVED` operation. Diff Result Schema v1 is no longer emitted; a consumer that checks `schemaVersion` fails instead of misreading the document. With `--no-moves`, the `changes` array and the added, removed and changed counts are exactly those of `v0.4.0-a4`. Statuses, exit codes and diagnostic codes are unchanged.
- Change the human `dirloom diff` output: the summary line includes the moved count, and a `MOVED` section lists `from -> path (evidence, nodes)`.

### Security

- Build Dirloom with Go 1.26.10 instead of Go 1.26.2, picking up the upstream Go security fixes released from Go 1.26.3 through Go 1.26.10.

## [0.4.0-a4] - 2026-09-30

CHANGE alpha: deterministic structural diff over Canonical Structural
Artifacts using Identity Projection v1, with ADDED, REMOVED and CHANGED
operations and Diff Result Schema v1. This increment does not add move
detection or Git sources.

### Added

- Add `dirloom diff <source-a> <source-b>` to list added, removed and changed canonical paths between `snapshot:<path>` and `live:<directory>` sources. A live side is observed once with the opposite snapshot's Capture Semantics; current configuration is never read.
- Add the structural comparison engine over Identity Projection v1: one projection per artifact, one O(N+M) merge, frozen ADDED/REMOVED/CHANGED vocabulary, path-first canonical ordering, and a validated StructuralDiff model. A rename or move is one REMOVED plus one ADDED; there is no move detection.
- Add Diff Result Schema v1 (`--format json`) and diff exits 0 through 6. Structural differences exit 1 without an `Error:` diagnostic; usage failures stay exit 2 with a human stderr diagnostic even under `--format json`.
- Publish diff contract/ADR/reference docs plus the comparison benchmark baseline.

## [0.4.0-a3] - 2026-09-30

CHANGE alpha: live structural verification against Snapshot Schema v1
using Fingerprint v1 equality. This increment does not add structural diff.

### Added

- Add `dirloom verify` to compare one live tree with a Snapshot Schema v1 file using Fingerprint v1 equality.
- Add Verify Result Schema v1 (`--format json`) and verify exits 0 through 6. A structural mismatch exits 1 without an `Error:` diagnostic.
- Capture Semantics stored in the snapshot select the live scan. Current project and user configuration are not reread. The reference file is excluded only when it is inside the root and absent from the expected artifact.

## [0.4.0-a2] - 2026-09-24

CHANGE alpha: persistent self-verifying structural snapshots on top of Fingerprint
v1. This increment does not add verify or diff.

### Added

- Add `dirloom snapshot` to persist Snapshot Schema v1 JSON (stdout or transactional `--output`).
- Add Snapshot Artifact Projection v1, Capture Semantics v1, deterministic JSON encode/decode and a shared self-verifying validator.
- Publish snapshot schema/ADR/reference docs plus large-snapshot benchmark baseline.

### Changed

- Document latest published `v0.4.0-a1` and the frozen prerelease candidate `v0.4.0-a2` on `release/v0.4.0-a2`.

## [0.4.0-a1] - 2026-09-20

CHANGE alpha: structural artifact identity and `dirloom fingerprint`, plus
Homebrew publisher uniqueness and GitHub Pre-release package-manager guards.
This increment does not add snapshot, verify or diff.

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

[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.4.0-a7...HEAD
[0.4.0-a7]: https://github.com/dirloom/dirloom/compare/v0.4.0-a6...v0.4.0-a7
[0.4.0-a6]: https://github.com/dirloom/dirloom/compare/v0.4.0-a5...v0.4.0-a6
[0.4.0-a5]: https://github.com/dirloom/dirloom/compare/v0.4.0-a4...v0.4.0-a5
[0.4.0-a4]: https://github.com/dirloom/dirloom/compare/v0.4.0-a3...v0.4.0-a4
[0.4.0-a3]: https://github.com/dirloom/dirloom/compare/v0.4.0-a2...v0.4.0-a3
[0.4.0-a2]: https://github.com/dirloom/dirloom/compare/v0.4.0-a1...v0.4.0-a2
[0.4.0-a1]: https://github.com/dirloom/dirloom/compare/v0.3.3...v0.4.0-a1
[0.3.3]: https://github.com/dirloom/dirloom/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/dirloom/dirloom/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/dirloom/dirloom/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/dirloom/dirloom/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/dirloom/dirloom/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/dirloom/dirloom/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/dirloom/dirloom/releases/tag/v0.1.0
