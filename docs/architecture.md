# Architecture

Dirloom uses a small, one-way dependency graph:

```text
cmd/dirloom
  └─ internal/cli
       ├─ internal/config
       │    ├─ internal/filter
       │    ├─ internal/outputformat
       │    └─ internal/presentation
       ├─ internal/app
       │    ├─ internal/filter
       │    ├─ internal/tree
       │    ├─ internal/gitsource
       │    ├─ internal/source
       │    │    ├─ internal/tree
       │    │    ├─ internal/gitsource
       │    │    │    └─ internal/artifact
       │    │    └─ internal/artifact
       │    ├─ internal/identity
       │    │    └─ internal/artifact
       │    ├─ internal/snapshot
       │    │    ├─ internal/artifact
       │    │    └─ internal/identity
       │    └─ internal/comparison
       │         ├─ internal/source
       │         └─ internal/identity
       ├─ internal/render
       │    ├─ internal/diagram
       │    └─ internal/outputformat
       ├─ internal/diagram
       │    └─ internal/tree   # project_structure adapter only
       ├─ internal/presentation
       │    └─ internal/presentation/catalog
       ├─ internal/clipboard
       └─ internal/output
```

## Package responsibilities

- `internal/cli`: Cobra flags, validation, stable exit-code mapping and stream routing.
- `internal/config`: strict YAML parsing, the immutable built-in preset catalog, project and user discovery, layered inspection and presentation resolution, provenance and diagnostics.
- `internal/app`: root and output resolution plus the reusable `Inspect`, `Fingerprint`, `Snapshot`, `Verify`, and `Diff` application services.
- `internal/filter`: ordered filtering policies, explicit glob rules, hidden-file detection and the encapsulated Git-compatible matcher.
- `internal/tree`: one scanner over a read-only `Backend` (the operating-system filesystem, or a Git tree), symlink handling, renderer-independent nodes and deterministic sorting. Capture Semantics, traversal order and error wording live in the scanner; a backend only reads.
- `internal/artifact`: Canonical Structural Artifact v1, path canonicalization (NFC, `/`) and validation. No presentation or CLI dependency.
- `internal/source`: `StructuralSource` abstraction, the FilesystemSource adapter over the scanner, and the Git source, which runs the same scanner over the Git backend and records the observed tree and commit.
- `internal/gitsource`: Git Tree Source v1. A hardened runner for the `git` executable (fixed plumbing commands, no shell, no network, no replace objects, Git 2.45 or newer), revision resolution, one bounded `cat-file --batch` reader per observation, a strict tree parser, and the Git `tree.Backend`. Everything Git returns is untrusted input.
- `internal/identity`: Identity Projection v1, Canonical Identity Encoding v1, SHA-256 fingerprint type, parser and JSON/text formatters.
- `internal/snapshot`: Snapshot Schema v1, Capture Semantics v1, Artifact Projection v1, deterministic JSON encode/decode and shared self-verifying validator.
- `internal/comparison`: StructuralDiff model, model validator, the O(N+M) comparison engine over Identity Projection v1, and the Move Detection structural-v1 detector with its shape index. No filesystem, CLI or presentation dependency.
- `internal/diagram`: canonical graph projection (`Document`, `ContractVersion`, `structure` view) with a single `tree` adapter.
- `internal/outputformat`: public format catalog, aliases and capability flags shared by CLI, config, render and presentation.
- `internal/render`: canonical Unicode, ASCII, fenced Markdown, semantic Markdown, JSON schema v1 and diagram DSL contracts plus a presentation-neutral text decorator boundary.
- `internal/presentation`: immutable built-in themes, strict public theme-schema v1 loading, kind/role/rule compilation, terminal capability resolution, ANSI generation, icon fallback and versioned diagnostics.
- `internal/presentation/catalog`: pure immutable classification with 506 indexed matchers, 119 hierarchical technical kinds, 16 ordered structural roles and no filesystem, YAML, ANSI or Cobra dependency.
- `internal/output`: transactional same-directory temporary files and safe atomic replacement.
- `internal/clipboard`: injectable UTF-8 clipboard writer with native Windows, macOS, Linux and WSL backends. Tests never touch the real clipboard.
- `internal/buildinfo`: version metadata injected once at link time.
- `internal/releaseartifacts` and `cmd/release-artifacts`: 13-artifact inventory (6 archives, 6 SBOMs, `checksums.txt`), independent checksums, and archive payload checks for the release pipeline.

## Invariants

The scanner completes before rendering starts. Any traversal or metadata error therefore yields no output tree. A renderer never accesses the filesystem, and the scanner never knows about text or JSON formatting.

The root selected by the caller is resolved before configuration discovery and revalidated at the application boundary immediately before scanning. Links found after that point are represented as terminal `symlink` nodes and never traversed.

Configuration is resolved before the application service starts scanning. The resolver is independent from Cobra, distinguishes omitted and explicit zero values, selects at most one preset, expands it in its source layer, and returns a complete effective request. Project discovery is bounded by the nearest Git worktree; configuration and presets never select the inspected root or an output path.

The winning presentation reference is validated and compiled before `app.Inspect` runs. A configuration-backed theme path is confined to its configuration directory after symlink resolution; masked paths are never opened. Terminal capability evaluation is also complete before scanning, including `NO_COLOR` and Windows virtual-terminal preparation.

After the canonical scanner identifies a node type, presentation applies the pure semantic catalog to its name and normalized relative path. Classification is not a Nerd mapping: it yields a kind and roles, then a later projection selects ASCII, Unicode, or Nerd glyphs. Nerd projections carry compiled provenance metadata (official name, codepoint, collection, upstream, license) that is not resolved on the render hot path.

```text
classification
        ↓
semantic kind
        ↓
glyph projection
        ├── ASCII
        ├── Unicode
        └── Nerd + provenance metadata
```

A classification refinement therefore stays in the catalog. The renderer still only draws the tree. A winning user rule may replace the effective kind or visual role; the renderer then resolves base token, catalog glyph, parent-to-child kind bindings, role binding and direct rule fields. Icons and text use separate ANSI spans and resets. This projection cannot mutate node identity, membership or order.

`theme classify` is the only diagnostic adapter that accesses a target directly. It validates the theme first, confines the target to `--root`, performs one `Lstat`, does not follow the final symlink, and does not read contents or descendants.

`app.Inspect` receives only inspection settings and remains independent from Cobra, YAML, themes, ANSI and terminal state. It returns the same canonical tree for every theme. Text rendering shares one traversal: a neutral decorator preserves historical bytes, while the terminal decorator styles connector and node segments after escaping dangerous controls. Markdown always selects the neutral path, JSON serializes the tree model directly, and diagram formats project once through `diagram.ProjectStructure` before dialect-specific encoding.

Filter priority is encoded in `filter.Policy`; nested `.gitignore` state is loaded only when the scanner actually enters a directory. This preserves pruning and prevents ignored branches from influencing the scan.

The tree stores normalized relative paths only as private tie-break metadata. Public JSON deliberately projects to a separate type, preventing accidental leakage of absolute paths or future internal fields. The semantic Markdown renderer walks the same sorted model, creates only nested list items and escapes unsafe label characters without mutating node data.

Rendering finishes in memory before stdout, the clipboard, or the transactional file writer receives bytes. Theme, mode and terminal-preparation errors therefore leave stdout, the clipboard and existing output files untouched. A forced interactive Windows color session restores the previous console mode after writing.

The destination is exclusive: `--copy`, `--output`, or stdout. `--copy` and `--output` are rejected before configuration and scanning. Automatic color is disabled for the clipboard; automatic icons stay Unicode, like interactive text. The renderer validates UTF-8 once; the clipboard does not apply a stricter encoding policy.

`dirloom fingerprint` reuses `app.Inspect` for a single scan, adapts the observation into a Canonical Structural Artifact, projects Identity v1, encodes it without JSON, and hashes with SHA-256. Presentation flags are ignored. Contracts: [Canonical Structural Artifact v1](canonical-structural-artifact-v1.md), [Identity Projection v1](contracts/identity-projection-v1.md), [Canonical Identity Encoding v1](contracts/canonical-identity-encoding-v1.md), [fingerprint command](reference/fingerprint.md), [ADR 0001](adr/0001-structural-fingerprint-identity.md). Local performance snapshots: [v0.4-a1 benchmarks](benchmarks/v0.4-a1.md).

`dirloom snapshot` reuses the same single observation path, builds Snapshot Artifact Projection v1 plus Capture Semantics v1, embeds Fingerprint v1, and emits deterministic JSON (stdout or transactional `--output`). Snapshot persistence is separate from Identity Projection. Contracts: [Snapshot Schema v1](contracts/snapshot-schema-v1.md), [snapshot command](reference/snapshot.md), [ADR 0002](adr/0002-snapshot-persistence-and-compatibility.md). Local performance snapshots: [v0.4-a2 benchmarks](benchmarks/v0.4-a2.md).

`dirloom verify` loads that snapshot through the shared validator, observes the selected root once with the persisted Capture Semantics, and compares typed Fingerprint v1 values. Mismatch is a normal exit, not an error diagnostic. It does not re-read project or user configuration and it does not emit a structural diff. Contracts: [Verify Result Schema v1](contracts/verify-result-v1.md), [verify command](reference/verify.md), [ADR 0003](adr/0003-snapshot-verification.md). Local performance snapshots: [v0.4-a3 benchmarks](benchmarks/v0.4-a3.md).

`dirloom diff` compares two structural sources (`snapshot:<path>`, `live:<directory>`, or `git:<revision>`) through the comparison engine and lists added, removed, changed, and moved canonical paths. The data flow is one-directional:

```text
Source (snapshot adapter, filesystem observation, or Git tree observation)
    → Canonical Structural Artifact v1
    → Identity Projection v1 (once per artifact)
    → Comparison Engine (two-way merge over sorted records)
    → Move Detector (structural-v1; skipped with --no-moves)
    → StructuralDiff (validated model)
    → human renderer or JSON renderer
```

The comparison engine is neither the human renderer nor the JSON renderer: it produces the validated `StructuralDiff` model and never formats bytes. A live side is observed once with the opposite snapshot's Capture Semantics through the same shared helper verify uses, with conditional reference-file self-exclusion; current configuration is never consulted.

A Git side goes through the same scanner with a different backend:

```text
live:<dir> ──► filesystem backend ─┐
                                   ├─► tree.Scanner (Capture Semantics) ─► source.FromTree ─► artifact
git:<rev>  ──► Git backend ────────┘
                 ▲
                 └─ internal/gitsource: runner, resolver, batch reader, tree parser
```

The revision is resolved to a tree with a few `git rev-parse` and `git cat-file -t` calls. One `git cat-file --batch` process then serves the observation: the backend reads a tree object only when the scanner enters that directory, so a subtree excluded by depth or ignore rules is never read. Each response is checked against the requested id, type and per-object size limit, and the process is closed, killed after a grace period if needed, and always waited for. A Git source never reads the working tree, the index or the network, and keeps link targets as recorded so the same revision yields the same artifact on every platform. Against a snapshot it uses the snapshot's Capture Semantics; against another revision it uses none. Contracts: [Git Tree Source v1](contracts/git-tree-source-v1.md), [Diff Result Schema v3](contracts/diff-result-v3.md), [ADR 0006](adr/0006-git-sources.md). Local performance snapshots: [v0.4-a6 benchmarks](benchmarks/v0.4-a6.md).

The Move Detector is a pure post-pass over the two record lists and the merge result. It never observes a source and reads no contents. It interns rename-invariant subtree shapes, counts names and shapes over each whole tree, and relabels a REMOVED and an ADDED node as one MOVED only when the correspondence is unique. CHANGED entries and paths present on both sides are never touched, so expanding every MOVED gives back the merge result. Contracts: [Diff Result Schema v3](contracts/diff-result-v3.md), [Move Detection structural-v1](contracts/move-detection-structural-v1.md), [diff command](reference/diff.md), [ADR 0004](adr/0004-structural-diff.md), [ADR 0005](adr/0005-move-detection.md). Local performance snapshots: [v0.4-a4 benchmarks](benchmarks/v0.4-a4.md), [v0.4-a5 benchmarks](benchmarks/v0.4-a5.md).
