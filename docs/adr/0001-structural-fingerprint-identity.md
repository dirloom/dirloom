# ADR 0001 — Structural fingerprint identity

> **Status:** accepted for `v0.4.0-a1`  
> **Date:** 2026-09-17

## Context

Dirloom already produces deterministic trees, JSON, Markdown and diagrams from a single scanner. v0.4 needs a stable identifier for the **structural view** so later snapshot, verify and diff work can compare identities rather than presentation.

## Decision

### Fingerprint is based on Identity Projection v1

The fingerprint identifies the Canonical Structural Artifact after structural filters, not the raw filesystem and not the rendered tree. Two configuration mechanisms that produce the same artifact produce the same fingerprint. Changing ignore/depth/hidden rules can change the fingerprint because the observed structure changed.

### JSON bytes are not hashed

Public JSON v1 includes display names, optional pretty-printing and a schema that may evolve independently of identity. Hashing `json.Marshal` would couple identity to encoder details, map order and indentation. Identity uses a dedicated length-prefixed big-endian encoding instead.

### Root is synthetic

The physical directory basename and absolute location are not identity. The canonical root path is always `.`. Relocating or renaming the containing folder MUST NOT change the fingerprint when the relative structure is unchanged.

### Unicode is NFC

Visually identical names can be NFD on macOS and NFC on Linux or Git. Identity normalizes to NFC. Two distinct raw entries that NFC to the same path are a hard collision error; Dirloom never merges them silently.

### Identity is case-sensitive

`Auth` and `auth` are different nodes. Dirloom does not fold case for identity. If the local filesystem cannot store both, Dirloom does not pretend that the missing name exists.

### Presentation and config provenance are excluded

Theme, color, icons, TTY, `NO_COLOR`, indentation and which layer supplied an ignore rule are not hashed. Only the resulting structure is. File contents are not hashed in v1; changing `main.go` without renaming or moving it keeps the same structural fingerprint.

## Consequences

- `dirloom fingerprint` reuses `app.Inspect` / `internal/tree` rather than walking the filesystem twice.
- A future snapshot schema can grow beside the artifact without silently changing `dlm:v1`.
- Cross-platform golden fingerprints are expected to match for portable fixtures.
