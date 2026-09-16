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
       │    └─ internal/tree
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
- `internal/app`: root and output resolution plus the reusable `Inspect` application service.
- `internal/filter`: ordered filtering policies, explicit glob rules, hidden-file detection and the encapsulated Git-compatible matcher.
- `internal/tree`: filesystem traversal, symlink handling, renderer-independent nodes and deterministic sorting.
- `internal/diagram`: canonical graph projection (`Document`, `ContractVersion`, `structure` view) with a single `tree` adapter.
- `internal/outputformat`: public format catalog, aliases and capability flags shared by CLI, config, render and presentation.
- `internal/render`: canonical Unicode, ASCII, fenced Markdown, semantic Markdown, JSON schema v1 and diagram DSL contracts plus a presentation-neutral text decorator boundary.
- `internal/presentation`: immutable built-in themes, strict public theme-schema v1 loading, kind/role/rule compilation, terminal capability resolution, ANSI generation, icon fallback and versioned diagnostics.
- `internal/presentation/catalog`: pure immutable classification with 256 indexed matchers, 96 hierarchical technical kinds, 16 ordered structural roles and no filesystem, YAML, ANSI or Cobra dependency.
- `internal/output`: transactional same-directory temporary files and safe atomic replacement.
- `internal/clipboard`: injectable UTF-8 clipboard writer with native Windows, macOS, Linux and WSL backends. Tests never touch the real clipboard.
- `internal/buildinfo`: version metadata injected once at link time.
- `internal/releaseartifacts` and `cmd/release-artifacts`: 13-artifact inventory (6 archives, 6 SBOMs, `checksums.txt`), independent checksums, and archive payload checks for the release pipeline.

## Invariants

The scanner completes before rendering starts. Any traversal or metadata error therefore yields no output tree. A renderer never accesses the filesystem, and the scanner never knows about text or JSON formatting.

The root selected by the caller is resolved before configuration discovery and revalidated at the application boundary immediately before scanning. Links found after that point are represented as terminal `symlink` nodes and never traversed.

Configuration is resolved before the application service starts scanning. The resolver is independent from Cobra, distinguishes omitted and explicit zero values, selects at most one preset, expands it in its source layer, and returns a complete effective request. Project discovery is bounded by the nearest Git worktree; configuration and presets never select the inspected root or an output path.

The winning presentation reference is validated and compiled before `app.Inspect` runs. A configuration-backed theme path is confined to its configuration directory after symlink resolution; masked paths are never opened. Terminal capability evaluation is also complete before scanning, including `NO_COLOR` and Windows virtual-terminal preparation.

After the canonical scanner identifies a node type, presentation applies the pure semantic catalog to its name and normalized relative path. A winning user rule may replace the effective kind or visual role; the renderer then resolves base token, catalog glyph, parent-to-child kind bindings, role binding and direct rule fields. Icons and text use separate ANSI spans and resets. This projection cannot mutate node identity, membership or order.

`theme classify` is the only diagnostic adapter that accesses a target directly. It validates the theme first, confines the target to `--root`, performs one `Lstat`, does not follow the final symlink, and does not read contents or descendants.

`app.Inspect` receives only inspection settings and remains independent from Cobra, YAML, themes, ANSI and terminal state. It returns the same canonical tree for every theme. Text rendering shares one traversal: a neutral decorator preserves historical bytes, while the terminal decorator styles connector and node segments after escaping dangerous controls. Markdown always selects the neutral path, JSON serializes the tree model directly, and diagram formats project once through `diagram.ProjectStructure` before dialect-specific encoding.

Filter priority is encoded in `filter.Policy`; nested `.gitignore` state is loaded only when the scanner actually enters a directory. This preserves pruning and prevents ignored branches from influencing the scan.

The tree stores normalized relative paths only as private tie-break metadata. Public JSON deliberately projects to a separate type, preventing accidental leakage of absolute paths or future internal fields. The semantic Markdown renderer walks the same sorted model, creates only nested list items and escapes unsafe label characters without mutating node data.

Rendering finishes in memory before stdout, the clipboard, or the transactional file writer receives bytes. Theme, mode and terminal-preparation errors therefore leave stdout, the clipboard and existing output files untouched. A forced interactive Windows color session restores the previous console mode after writing.

The destination is exclusive: `--copy`, `--output`, or stdout. `--copy` and `--output` are rejected before configuration and scanning. Automatic color is disabled for the clipboard; automatic icons stay Unicode, like interactive text. The renderer validates UTF-8 once; the clipboard does not apply a stricter encoding policy.
