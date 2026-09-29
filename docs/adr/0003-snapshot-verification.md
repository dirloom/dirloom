# ADR 0003 — Snapshot verification

> **Status:** accepted for `v0.4.0-a3`
>
> **Date:** 2026-09-29

## Context

`v0.4.0-a1` froze Fingerprint v1. `v0.4.0-a2` froze Snapshot Schema v1, including Capture Semantics, so a later command can re-observe a tree without re-reading configuration. `v0.4.0-a3` is the first command that answers whether the live structure still matches a snapshot. It must not become a diff engine.

## Decision

### The validated snapshot is the reference

`dirloom verify` loads the reference only through the shared A2 loader (`snapshot.Load`). The expected fingerprint is `Validated.Fingerprint`. It is not recomputed by verify and it is not taken from untrusted JSON. An embedded fingerprint that does not match the reconstructed artifact is an invalid snapshot, never a live mismatch.

### Capture Semantics drive live observation

The live scan uses the snapshot's Capture Semantics v1: depth, directories-only, hidden, default ignores, gitignore, and the ordered custom ignore list. Current project configuration, user configuration, and CLI presets are not a second source of structural scope. Inherited `--config`, `--no-user-config`, and `--no-config` are rejected rather than ignored.

### Comparison is Fingerprint v1 equality

Verify observes the selected root once through the existing scanner and source pipeline, computes one live fingerprint with `identity.Compute`, and compares the typed fingerprints. It does not hash snapshot JSON, compare node arrays, or use node counts as equality.

### Mismatch is not an error

`MATCH` exits 0. `MISMATCH` exits 1, prints both fingerprints, and leaves stderr empty. Corrupt snapshots, unsupported contracts, operational failures, and internal failures use exits 3 through 6 and are not reported as structural mismatch.

### Reference-file self-exclusion is conditional

When the reference file lies inside the live root and its canonical relative path is absent from the expected artifact, verify passes that path through the existing output-exclusion filter. When the expected artifact already contains that path, the file is kept. This is operational plumbing, not a diff.

### A3 stops where A4 starts

Verify does not list added or removed paths, explain type or target changes, infer renames or moves, or advertise `dirloom diff`. Those belong to a later increment.

## Consequences

- `dirloom verify <snapshot> [directory]` is a reusable `app.Verify` service plus a CLI renderer.
- Machine output is Verify Result Schema v1 (`--format json`), independent from Snapshot Schema v1.
- Fingerprint v1, Identity Projection v1, Canonical Identity Encoding v1, and Snapshot Schema v1 stay unchanged.
- A changed `.gitignore` can change a later verify result when `useGitignore` was persisted as true, because the snapshot stores the flag rather than historical ignore contents.
