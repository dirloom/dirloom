# ADR 0002 — Snapshot persistence and compatibility

> **Status:** accepted for `v0.4.0-a2`
>
> **Date:** 2026-09-24

## Context

`v0.4.0-a1` introduced Fingerprint v1 over the Canonical Structural Artifact.
`v0.4.0-a2` must persist that structural view as a durable reference that later
`verify` and `diff` can load without re-reading configuration or depending on
the absolute project root.

Identity Projection v1 and snapshot persistence serve different futures: the
fingerprint must stay frozen while snapshots may later carry structural
metadata that remains outside Fingerprint v1.

## Decision

### Separate Snapshot Artifact Projection from Identity Projection

Do not JSON-tag `artifact.Artifact` or reuse `identity.Record` as the
persistence contract. Persist a flat node projection (`path`, `kind`, optional
`target`) and reconstruct a validated `artifact.Artifact` on load.

### Capture Semantics record effective rules only

Persist depth, dirs-only, hidden, default ignores, gitignore and ordered custom
ignores. Exclude absolute roots, config provenance, presentation and the
operational `--output` path used for self-exclusion.

### Embedded fingerprint self-verifies structure, not the whole JSON

Recompute Fingerprint v1 from the reconstructed artifact. A mismatch means
corrupt/invalid snapshot, never “structure mismatch” against a live tree.

### Compatibility is additive, not silent about mandatory unknowns

Unknown schema/artifact versions and unknown `requiredFeatures` reject.
Unknown additive fields in an otherwise supported v1 document are ignored.
Duplicate JSON keys and trailing documents reject.

### No volatile provenance in v1 canonical output

Hostname, username, timestamps and absolute paths make byte-stable snapshots
impossible and leak host data. Omit them.

## Consequences

- `dirloom snapshot` reuses one structural observation, the A1 identity engine
  and the existing transactional output writer.
- Golden snapshot bytes are expected to match across Linux, Windows and macOS
  for portable logical fixtures.
- Future A3/A4 can load the shared validator without changing Fingerprint v1.
