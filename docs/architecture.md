# Architecture

Dirloom uses a small, one-way dependency graph:

```text
cmd/dirloom
  └─ internal/cli
       ├─ internal/config
       │    ├─ internal/filter
       │    └─ internal/presentation
       ├─ internal/app
       │    ├─ internal/filter
       │    └─ internal/tree
       ├─ internal/render
       ├─ internal/presentation
       └─ internal/output
```

## Package responsibilities

- `internal/cli`: Cobra flags, validation, stable exit-code mapping and stream routing.
- `internal/config`: strict YAML parsing, the immutable built-in preset catalog, project and user discovery, layered inspection and presentation resolution, provenance and diagnostics.
- `internal/app`: root and output resolution plus the reusable `Inspect` application service.
- `internal/filter`: ordered filtering policies, explicit glob rules, hidden-file detection and the encapsulated Git-compatible matcher.
- `internal/tree`: filesystem traversal, symlink handling, renderer-independent nodes and deterministic sorting.
- `internal/render`: canonical Unicode, ASCII, Markdown and JSON schema v1 contracts plus a presentation-neutral text decorator boundary.
- `internal/presentation`: immutable built-in themes, strict custom-theme loading, rule compilation, terminal capability resolution, ANSI generation, icon fallback and theme diagnostics.
- `internal/output`: transactional same-directory temporary files and safe atomic replacement.
- `internal/buildinfo`: version metadata injected once at link time.

## Invariants

The scanner completes before rendering starts. Any traversal or metadata error therefore yields no output tree. A renderer never accesses the filesystem, and the scanner never knows about text or JSON formatting.

The root selected by the caller is resolved before configuration discovery and revalidated at the application boundary immediately before scanning. Links found after that point are represented as terminal `symlink` nodes and never traversed.

Configuration is resolved before the application service starts scanning. The resolver is independent from Cobra, distinguishes omitted and explicit zero values, selects at most one preset, expands it in its source layer, and returns a complete effective request. Project discovery is bounded by the nearest Git worktree; configuration and presets never select the inspected root or an output path.

The winning presentation reference is validated and compiled before `app.Inspect` runs. A configuration-backed theme path is confined to its configuration directory after symlink resolution; masked paths are never opened. Terminal capability evaluation is also complete before scanning, including `NO_COLOR` and Windows virtual-terminal preparation.

`app.Inspect` receives only inspection settings and remains independent from Cobra, YAML, themes, ANSI and terminal state. It returns the same canonical tree for every theme. Text rendering shares one traversal: a neutral decorator preserves historical bytes, while the terminal decorator styles connector and node segments after escaping dangerous controls. Markdown always selects the neutral path, and JSON serializes the tree model directly.

Filter priority is encoded in `filter.Policy`; nested `.gitignore` state is loaded only when the scanner actually enters a directory. This preserves pruning and prevents ignored branches from influencing the scan.

The tree stores normalized relative paths only as private tie-break metadata. Public JSON deliberately projects to a separate type, preventing accidental leakage of absolute paths or future internal fields.

Rendering finishes in memory before stdout or the transactional file writer receives bytes. Theme, mode and terminal-preparation errors therefore leave stdout and existing output files untouched. A forced interactive Windows color session restores the previous console mode after writing.
