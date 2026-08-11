# Architecture

Dirloom uses a small, one-way dependency graph:

```text
cmd/dirloom
  └─ internal/cli
       ├─ internal/app
       │    ├─ internal/filter
       │    └─ internal/tree
       ├─ internal/render
       └─ internal/output
```

## Package responsibilities

- `internal/cli`: Cobra flags, validation, stable exit-code mapping and stream routing.
- `internal/app`: root and output resolution plus the reusable `Inspect` application service.
- `internal/filter`: ordered filtering policies, explicit glob rules, hidden-file detection and the encapsulated Git-compatible matcher.
- `internal/tree`: filesystem traversal, symlink handling, renderer-independent nodes and deterministic sorting.
- `internal/render`: Unicode, ASCII, Markdown and JSON schema v1 contracts writing to `io.Writer`.
- `internal/output`: transactional same-directory temporary files and safe atomic replacement.
- `internal/buildinfo`: version metadata injected once at link time.

## Invariants

The scanner completes before rendering starts. Any traversal or metadata error therefore yields no output tree. A renderer never accesses the filesystem, and the scanner never knows about text or JSON formatting.

The root selected by the caller is resolved and validated once. Links found after that point are represented as terminal `symlink` nodes and never traversed.

Filter priority is encoded in `filter.Policy`; nested `.gitignore` state is loaded only when the scanner actually enters a directory. This preserves pruning and prevents ignored branches from influencing the scan.

The tree stores normalized relative paths only as private tie-break metadata. Public JSON deliberately projects to a separate type, preventing accidental leakage of absolute paths or future internal fields.
