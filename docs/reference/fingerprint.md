# `dirloom fingerprint`

Fingerprint v1 identifies the **structural view** Dirloom produces after applying the active filters. It does not hash file contents.

## Purpose

```bash
dirloom fingerprint
dirloom fingerprint --root .
dirloom fingerprint --format json
```

The command prints a portable identity:

```text
dlm:v1:sha256:<64-character lowercase digest>
```

Two directories that Dirloom sees as the same structure — same relative paths, kinds and symlink targets after NFC canonicalization — receive the same fingerprint even if they live at different absolute locations or have different folder names.

Changing `main.go` in place does **not** change the fingerprint. Adding, removing, renaming or moving a node does.

## Syntax

```text
dirloom fingerprint [directory] [flags]
```

`directory` defaults to the current directory. `--root` is an alternative to the positional argument; they cannot be combined.

Structural flags are the same as inspection: `--depth`, `--dirs-only`, `--hidden`, `--ignore`, `--no-default-ignore`, `--no-gitignore`, `--preset`, plus `--config` / `--no-user-config` / `--no-config`.

`--color`, `--icons` and `--theme` are accepted and ignored. They never change the fingerprint and never inject ANSI.

## Text

Default `--format text` writes exactly one fingerprint line and a final newline. There is no label. This is intended for:

```bash
FP=$(dirloom fingerprint)
```

Stdout is the result. Diagnostics go to stderr. On success stderr is empty.

## JSON

`--format json` writes schema version 1:

```json
{
  "schemaVersion": 1,
  "fingerprint": "dlm:v1:sha256:...",
  "identityVersion": 1,
  "algorithm": "sha256",
  "nodeCount": 123
}
```

Stdout remains parseable JSON. `nodeCount` is metadata; it is not hashed.

## What enters identity

- Canonical relative paths (`/` separators, Unicode NFC, synthetic root `.`)
- Node kinds: directory, file, symlink (Windows junctions follow the scanner's observation type)
- Symlink/junction target strings, rewritten to root-relative form when they point inside the inspected root

## What does not enter identity

- File contents and file size
- mtime, owner, permissions, executable bit
- Absolute root, hostname, username, capture time
- Theme, color, icons, TTY, `NO_COLOR`, indentation
- Which configuration layer supplied a filter, when the resulting view is identical

## Filters

Fingerprint hashes the view Dirloom actually observes, not every physical object below the root. Ignore rules, `.gitignore`, hidden visibility, depth and `--dirs-only` therefore change the fingerprint when they change membership.

Different configuration mechanisms that produce the same view produce the same fingerprint.

## Exit status

- `0` — success
- `1` — runtime error (missing directory, unreadable tree, canonical collision, unsupported node)
- `2` — invalid arguments (unknown `--format`, `--root` combined with a positional path)

A canonical Unicode collision (two raw names that NFC to the same path) is an error, never a silent merge.

## Contracts

- [Canonical Structural Artifact v1](../canonical-structural-artifact-v1.md)
- [Identity Projection v1](../contracts/identity-projection-v1.md)
- [Canonical Identity Encoding v1](../contracts/canonical-identity-encoding-v1.md)
