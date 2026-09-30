# ADR 0004 — Structural diff

> **Status:** accepted for `v0.4.0-a4`
>
> **Date:** 2026-09-30

## Context

`v0.4.0-a1` froze structural identity, `v0.4.0-a2` froze durable snapshots, and `v0.4.0-a3` answered whether a live tree still matches a snapshot. The remaining CHANGE question is what exactly changed, without turning the answer into a second identity engine.

## Decision

### Comparison operates on Identity Projection v1 records

`dirloom diff` never compares JSON documents, observation trees, or presentation. Each side is observed exactly once through the existing `source.Source` pipeline into a Canonical Structural Artifact, then flattened once with `identity.ProjectV1`. The comparison is a single two-way merge over the two path-sorted record lists: O(N+M), no nested loops, no matrix, no second scanner, no second fingerprint engine.

### Three operations, anchored to identity

The operation vocabulary is closed: `ADDED`, `REMOVED`, `CHANGED`. A path present only in B is `ADDED`; present only in A is `REMOVED`; present in both with a different Identity Projection v1 record is `CHANGED`. `CHANGED` therefore covers exactly a kind change or a target change on a target-carrying kind. File contents are never hashed, so a content-only edit is not a change. The synthetic root `.` is never emitted as a change.

### No move or rename detection

A rename produces one `REMOVED` plus one `ADDED`. There is no `MOVED` or `RENAMED` operation and no pairing heuristic. Move detection is a later increment with its own contract, if it ever happens.

### Sources are snapshots or capture-driven live views

The source grammar is `snapshot:<path>` and `live:<directory>`, split on the first colon only, with lowercase case-sensitive prefixes. A snapshot side is loaded only through the shared self-verifying `snapshot.Load` and wrapped in a `source.Snapshot` adapter that never parses Snapshot Schema itself. A `live:` side is observed with the Capture Semantics v1 stored in the opposite snapshot, through the same shared capture helper and conditional reference-file self-exclusion that verify uses; current project and user configuration are never consulted. `live:` against `live:` has no stored capture semantics to arbitrate and is a usage failure (exit 2).

### Deterministic, side-preserving failures

Snapshot sides are loaded in user order (A then B) and the first invalid side is reported with its `a`/`b` side preserved by typed errors, never by message parsing. Snapshot classification reuses the verify taxonomy: invalid is exit 3, unsupported is exit 4, operational is exit 5, internal is exit 6. `DIFFERENCES` is exit 1 with an empty stderr: a normal result, not an error.

### Presentation is a renderer, not the engine

The `internal/comparison` package produces a validated `StructuralDiff` model whose summary invariants are checked (`total = added + removed + changed`, sorted unique paths, `before != after` on `CHANGED`). The human text renderer and the Diff Result Schema v1 JSON encoder are separate projections of that model. `<none>` for an absent target is presentation only and never appears in the model. No ANSI, color, icon, or theme enters either renderer.

## Consequences

- `dirloom diff <source-a> <source-b>` is a reusable `app.Diff` service plus a CLI renderer, registered next to `verify`.
- Machine output is Diff Result Schema v1 (`--format json`), independent from Snapshot Schema v1 and Verify Result Schema v1.
- Fingerprint v1, Identity Projection v1, Canonical Identity Encoding v1, Snapshot Schema v1, Snapshot Artifact Projection v1, Capture Semantics v1, Verify Result Schema v1, and verify exit codes stay unchanged.
- A changed `.gitignore` can change a later diff when the snapshot stored `useGitignore: true`, exactly as with verify.
- Move detection, Git sources, content hashing, and live↔live comparison remain explicitly out of scope for A4.
