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
       │    ├─ internal/source
       │    │    ├─ internal/tree
       │    │    └─ internal/artifact
       │    ├─ internal/identity
       │    │    └─ internal/artifact
       │    └─ internal/snapshot
       │         ├─ internal/artifact
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
- `internal/app`: root and output resolution plus the reusable `Inspect`, `Fingerprint` and `Snapshot` application services.
- `internal/filter`: ordered filtering policies, explicit glob rules, hidden-file detection and the encapsulated Git-compatible matcher.
- `internal/tree`: filesystem traversal, symlink handling, renderer-independent nodes and deterministic sorting.
- `internal/artifact`: Canonical Structural Artifact v1, path canonicalization (NFC, `/`) and validation. No presentation or CLI dependency.
- `internal/source`: `StructuralSource` abstraction and FilesystemSource adapter over the existing scanner.
- `internal/identity`: Identity Projection v1, Canonical Identity Encoding v1, SHA-256 fingerprint type, parser and JSON/text formatters.
- `internal/snapshot`: Snapshot Schema v1, Capture Semantics v1, Artifact Projection v1, deterministic JSON encode/decode and shared self-verifying validator.
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
