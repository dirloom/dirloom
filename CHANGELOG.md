# Changelog

All notable changes to Dirloom are documented here. The project follows Semantic Versioning.

## [Unreleased]

### Added

- Publish the v0.4 [compatibility and freeze policy](docs/compatibility-policy.md) and the [contract register](docs/contracts/README.md): every public input and output, with its status, its owning package, what pins it and its change rules. The policy states what stays stable after the v0.4 freeze (fields, enumeration values, machine codes, exit codes, golden bytes, fingerprints) and what does not (diagnostic messages, including localized operating-system text, and stderr), and that an incompatible change to identity semantics, observation included, needs a new fingerprint version.
- Publish the Tree JSON Schema v1 and Fingerprint JSON Schema v1 contracts, which describe the existing `--format json` outputs of the tree and of `dirloom fingerprint`, and an evolution policy for Verify Result Schema v1.
- Publish [Exit codes and machine codes](docs/reference/exit-codes.md): the exit codes of every command, in their two compatible families (`0`/`1`/`2` for the tree output, `fingerprint` and `snapshot`; `0`–`6` for `verify`, `diff`, `history` and `watch`), and every machine code with its category (usage, mismatch, validation, corruption, unsupported, source, filesystem, git, internal), status and exit code. Snapshot Schema v1 now lists all of its diagnostic codes, and Verify Result v1, Diff Result v3 and History Result v1 list every artifact code they report.
- Add the help topics `structural`, `sources` and `moves`: the Structural Version Control commands (`fingerprint`, `snapshot`, `verify`, `diff`, `history`, `watch`) and what each one observes, the `snapshot:`, `live:` and `git:` operands of `diff`, and move detection with `--no-moves`. `dirloom help concepts` lists them, and `dirloom help examples` adds a snapshot-and-verify recipe.

### Changed

- The v0.4 contracts are marked as candidates for the v0.4 freeze, which takes effect at the release candidate's freeze commit; Structural Watch v1 and Watch Event Schema v1 stay experimental.
- The JSON outputs of `config explain`, `preset explain`, `theme list`, `theme explain`, `theme validate` and `theme classify` are documented as diagnostic outputs, not contracts: they keep their `schemaVersion`, but may change in any release, with a CHANGELOG entry. Their content is unchanged.
- Bound the resources spent on a snapshot, which is untrusted input:
  - `verify` and `diff` refuse a snapshot larger than 256 MiB without reading past that size;
  - they also refuse a snapshot with more than 2 000 000 nodes (or elements in any JSON array), JSON nested deeper than 64 levels, more than 4 096 `capture.ignore` patterns or more than 64 `requiredFeatures` entries;
  - the new code `snapshot_limit_exceeded` reports it as `INVALID_SNAPSHOT` (exit 3), distinct from a read failure.

  `dirloom snapshot` honors the same limits: it refuses to write a snapshot beyond them (exit 1), so every snapshot Dirloom writes can be read back. Loading a valid snapshot at the node limit was measured at 1.5 to 2.1 GiB of peak heap (see [docs/benchmarks/v0.4-rc1-snapshot-limits.md](docs/benchmarks/v0.4-rc1-snapshot-limits.md)).
- Refuse named pipes, sockets and devices in structural observations. A FIFO, a Unix or Windows AF_UNIX socket, or a device in the observed view was recorded as a plain `file`, against the Canonical Structural Artifact contract. It is now refused with `unsupported_node_type`, naming its path and type:
  - `fingerprint` and `snapshot` exit 1;
  - `verify` and `diff` report `OBSERVATION_ERROR` (exit 5);
  - `watch` stops at startup with a fatal error, or, when the entry appears later, reports non-fatal errors and publishes no new state until the entry is gone.

  Exclude such an entry with `--ignore`; `.gitignore`, default ignores, hidden entries, `--dirs-only` and depth exclude it as well. Tree outputs (text, Markdown, JSON, graph exports) still show it as a file, and Windows files behind a reparse point (app execution aliases, cloud placeholders) are still files. A snapshot taken with `v0.4.0-a8` or earlier that recorded such an entry still loads, but observing the same tree now fails instead of matching.
- Name a refused flag with its reason. `verify`, `diff` and `history` answered `unknown flag` to the capture, presentation and destination flags that other commands take; they now say, for example, `--depth is not supported by verify: snapshot capture semantics are authoritative`, still with exit 2. `diff`, `history` and `watch` refuse `--style` the same way, and the configuration flags that `theme` and `preset` refuse use the same wording. Messages remain outside the contracts: scripts branch on the exit code.
- Report an invalid `--format` the same way in every command. `fingerprint`, `verify`, `diff` and `history` now list their valid values and the help to read, as the tree output and `watch` do, and an empty value gives `--format requires a non-empty value`. The exit status is unchanged (2).
- Describe `--depth`, `--dirs-only` and `--no-moves` with the same words in every command's help. The help and the references of `fingerprint` and `snapshot` now say that `--color`, `--icons` and `--theme` are validated, then ignored: an invalid value has always been a usage error (exit 2), contrary to the former "accepted and ignored". Behavior is unchanged.

### Fixed

- Fix out-of-memory crashes on hostile repositories, found by the audit of panic sources:
  - **Git trees.** Git shares subtrees, so a repository of a few kilobytes can describe billions of nodes: a 3 KiB repository already made `history --limit 1` hold 4 million nodes and 2 GiB. A Git observation (`history`, `diff git:`, `diff snapshot: git:`) now stops at a snapshot's limits, 2,000,000 nodes and 256 MiB of paths, with the new code `git_tree_too_large` (`OBSERVATION_ERROR`, exit 5).
  - **`.gitignore`.** A live `.gitignore` above 1 MiB now fails the scan (`source_read_failure`), as a `.gitignore` blob above 1 MiB already failed a Git source. Every entry is tested against all accumulated patterns, so a huge or repeated `.gitignore` used to cost memory and time without bound.

  Compatibility: a Git tree within a snapshot's limits, and a `.gitignore` of at most 1 MiB, observe as before. A Git tree beyond them could not have been snapshotted anyway.
- Refuse a `.gitignore` with a line of 64 KiB or more instead of silently dropping that line and every rule after it. The pattern matcher stops reading at such a line without an error, so the files those rules ignore appeared in tree outputs, fingerprints and snapshots, where Git, which has no such limit, ignores them. Such a `.gitignore` now fails the scan, naming the file and the line, in a live source (`source_read_failure`) as in a Git source (`git_tree_invalid`): the tree output, `fingerprint` and `snapshot` exit 1, `verify` and `diff` exit 5. A `.gitignore` whose lines are all shorter observes as before.
- Fix Windows junctions observed as files. Since Go 1.23 reports a junction with an irregular file mode, Dirloom recorded it with kind `file` and no target, so retargeting a junction was invisible to `fingerprint`, `snapshot`, `verify`, `diff` and `watch`. A junction, or a folder where a volume is mounted, is now recognized by its reparse tag, whatever `GODEBUG=winsymlink` says, and recorded with kind `junction` and its target, rewritten relative to the root when it points inside it. It is still never entered. Tree outputs (text, Markdown, JSON, Mermaid, Graphviz, D2) show it as a symlink, `name -> target`, instead of a plain file, with no new `type` value; its graph node identifier changes accordingly. `dirloom theme classify` classifies it as a symlink instead of refusing it. Git sources are unchanged: Git never records a junction.
- Compatibility: no format changes, but the fingerprint of a Windows tree that contains a junction changes. A snapshot taken with `v0.4.0-a8` or earlier still loads and validates; `dirloom verify` against the unchanged tree reports a mismatch (exit 1), and `dirloom diff` shows the junction as `CHANGED`, kind `file` → `junction`. Check such snapshots, then capture them again. Trees without junctions, and every tree on Linux and macOS, keep their fingerprints.
- Report broken Dirloom invariants as internal errors, by context:
  - an artifact that Dirloom builds from an observation and that breaks an artifact rule (`artifact_invariant_violation`) is now `INTERNAL_ERROR` (exit 6) in `verify`, `diff` and `history`, and fatal in `watch`, instead of `OBSERVATION_ERROR` (exit 5); a snapshot that breaks the same rule is still `INVALID_SNAPSHOT` (exit 3);
  - a failed canonical encoding (`canonical_encoding_failure`) is now internal too;
  - a snapshot failure without a cause carries `snapshot_internal_failure` instead of a code documented for invalid input or I/O;
  - `verify` reports an unknown result status as an internal error (exit 6) before writing anything, instead of a failed write (exit 5).

  None of these cases is known to occur; they are reported correctly if one does.
- Report an unexpected panic as an internal error instead of a crash. A panic ended Dirloom with Go's exit status 2, which also means invalid usage, and a stack trace on stderr. It now exits with the command's internal-error code, with one stderr line, `Error: internal error: unexpected panic: …`, and no stack trace:
  - `1` for the tree output, `fingerprint`, `snapshot` and the `config`, `preset`, `theme` and `completion` commands;
  - `6` for `verify`, `diff`, `history` and `watch`.

  Output already written is not completed: stdout can be empty or hold a truncated document, and the exit code is never a success. In `watch`, a panic while writing events ends the stream without `watch.stopped`, and a panic in a native backend ends it with a fatal `internal_invariant_failure`; both exit 6. Go fatal errors, such as running out of memory or exhausting the stack, still exit 2 with a trace. No panic is known to occur ([exit codes](docs/reference/exit-codes.md#unexpected-internal-failures)).
- Fix crashes and hangs on hostile input, found by the audit of panic sources:
  - an ignore pattern made of many `**` segments, whether from `--ignore`, a configuration `ignore:` entry, a theme `match.glob` or a snapshot's capture. A few thousand made matching run for hours, and the millions a snapshot can hold overflowed the stack, a crash no error handling can catch. Matching is now linear in the pattern and the path, with the same results.
  - a project `.dirloom.yaml` or a theme file that is a device, a named pipe or a socket, for example a cloned link to `/dev/zero`, was read without end or blocked. It is now refused as not a regular file, and a configuration or theme file is never read beyond its 1 MiB limit.
- Fix scans on Windows blocking, and being blocked by, renames and removals. Dirloom listed each directory, and read each `.gitignore`, through a handle that did not share delete access. While the tree output, `fingerprint`, `snapshot`, `verify`, `diff` or `watch` read an entry, renaming or removing that entry failed with a sharing violation; a scan that reached a directory being renamed failed, and `watch` reported a `source_read_failure`. Every handle now shares delete access; entries, metadata and errors are unchanged, with two exceptions:
  - a directory that another program holds without sharing is retried for up to 50 ms before the scan fails;
  - a directory being deleted is reported as no longer existing, instead of `permission denied`. A real permission denial is still reported, and never retried.

  One Windows limit stays, documented in the [watch reference](docs/reference/watch.md#platform-limits): for the instant Dirloom reads an entry, Windows refuses to rename any directory above it (`Access is denied`); renaming again succeeds. Linux and macOS are not affected.
- Fix tree JSON (`dirloom [directory] --format json`) publishing names that do not exist. A name or a link target that is not valid UTF-8, such as a Windows name holding an unpaired UTF-16 surrogate or a Linux name holding arbitrary bytes, was written with U+FFFD replacement characters and exit 0, while the text output, `fingerprint`, `snapshot`, `verify` and `diff` refuse it. Tree JSON now refuses it too, with exit 1 and a diagnostic naming the entry: every string is checked before any byte is written, so stdout, the `--output` file and the clipboard are left untouched. Compatibility: this changes a v0.1 behavior that used to succeed; exclude such an entry with `--ignore`, or rename it. Tree JSON Schema v1 is unchanged for every valid tree.
- Fix the help of `verify`, `diff`, `history`, `preset` and `theme`, which listed `--config`, `--no-user-config` and `--no-config` under "Global Flags", and offered them in completion, although these commands refuse them. A command's help and completion now offer only the flags it accepts. Fix an example of the root help indented with a TAB.
- Correct the PowerShell example of the `dirloom watch` reference. On Ctrl+C, PowerShell terminates a native command whose output it carries, through a pipeline or a `>` redirection: the example then reported `-1` or the previous `$LASTEXITCODE` instead of exit status 0, the stream could end without `watch.stopped`, and in a script the line after the watch never ran. The example now starts the watch with `Start-Process -NoNewWindow -RedirectStandardOutput`, so that the watch writes the file and ends by itself, and reads its exit status in a `finally` block.

## [0.4.0-a8] - 2026-11-03

CHANGE alpha, experimental: a structural watch (`dirloom watch`), which
reports the structural changes of a directory as numbered generations while
they happen, with Structural Watch v1 and Watch Event Schema v1, both
excluded from the v0.4 RC contract freeze. This increment does not add an
operation log, content monitoring, FSEvents on macOS, subtree
re-observation, or a stable watch contract.

### Added

- Add `dirloom watch [directory]`, an **experimental** structural watch. Each time the observed structure of a directory changes, it reports the changes against the last reported state as one numbered generation: the same added, removed, changed and moved paths as `dirloom diff` between the two states. Every observation scans the whole tree with the capture rules of `dirloom fingerprint` (filters, presets, configuration); file contents, timestamps and the filesystem operations themselves are not reported, so editing a file reports nothing. Native notifications (inotify on Linux, `ReadDirectoryChangesW` on Windows, kqueue on macOS) only decide when to observe, after a debounce (`--debounce`, default 200 ms), and a reconciliation every 60 s reports, inside a rescan, what they missed; `--poll <interval>` observes at a fixed interval instead. `--no-moves` reports moves as removals and additions. Ctrl+C or `SIGTERM` ends the watch with `watch.stopped` and exit 0; a lost root, a backend failure, a resource limit or a refused filesystem exits 5, with the error in the stream; there is no exit 1. Recognized network filesystems are refused in native mode, with a message that suggests `--poll`.
- Add Watch Event Schema v1 for `dirloom watch --format ndjson`: one compact JSON object per line (`watch.started`, `node.added`, `node.removed`, `node.changed`, `node.moved`, `state.changed`, `rescan.started`, `rescan.completed`, `watch.error`, `watch.stopped`). Each generation is validated before it is written, and a consumer commits it on its `state.changed`. A blocked stdout delays observations instead of queueing events, and an interruption waits for it at most 2 s.
- Add the watch diagnostics `watch_root_unavailable`, `watch_limit_reached`, `watch_backend_failure`, `watch_filesystem_unsupported`, `watch_observation_failure` and `watch_internal_failure`.
- Publish the Structural Watch v1 and Watch Event Schema v1 contracts, both experimental and excluded from the v0.4 RC contract freeze, ADR 0008, the watch reference and watch benchmarks.

### Changed

- `watch` is now a command name: `dirloom watch` runs the structural watch instead of inspecting a directory named `watch`. Use `dirloom ./watch` to inspect such a directory.

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

[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.4.0-a8...HEAD
[0.4.0-a8]: https://github.com/dirloom/dirloom/compare/v0.4.0-a7...v0.4.0-a8
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
