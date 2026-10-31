# History Result Schema v1

> **Status:** normative contract for Dirloom `v0.4.0-a7`
>
> **Related:** [Structural History v1](structural-history-v1.md), [Diff Result Schema v3](diff-result-v3.md), [Git Tree Source v1](git-tree-source-v1.md), [ADR 0007](../adr/0007-structural-history.md)

Machine-readable output of `dirloom history --format json`. It reports a [Structural History v1](structural-history-v1.md) result: the resolved range, why the walk ended, and one entry per listed commit. It has an explicit [evolution policy](#evolution-policy) from its first version. It does not modify any other contract. The JSON bytes are not fingerprinted.

## Document

```text
schemaVersion: 1
encoding: UTF-8
indentation: 2 spaces
html escaping: disabled
terminator: one final newline
```

Top-level field order is fixed by the encoder:

| Field | Presence |
| --- | --- |
| `schemaVersion` | always `1` |
| `status` | always |
| `metadata` | status `OK` |
| `range` | status `OK` |
| `entries` | status `OK`; `[]` when empty, never null |
| `diagnostic` | failure statuses |

`metadata` describes the contracts the result follows, never the environment. Field order:

| Field | Value |
| --- | --- |
| `historyVersion` | always `1` (Structural History v1) |
| `traversal` | always `first-parent` |
| `comparisonVersion` | always `1` |
| `identityProjectionVersion` | always `1` |
| `moveDetection` | `structural-v1` (default) or `none` (`--no-moves`) |

`range` describes the walk. Field order:

| Field | Presence and value |
| --- | --- |
| `to` | always: the newest commit id, resolved from the revision |
| `from` | only with `--from`: the resolved commit id of the exclusive bound |
| `limit` | always: the number of commits the walk could examine |
| `end` | always: `from`, `limit`, `root` or `shallow` ([Structural History v1](structural-history-v1.md#walk)) |

Ids are lowercase hex: 40 characters for SHA-1, 64 for SHA-256. Revision text, repository paths, remotes, branch names, identities and commit messages are never emitted.

## Entries

`entries` lists the listed commits, newest first, in walk order. With `--structural-only`, only entries with `structural: true` remain; `range` is unchanged. Entry fields, in encoder order:

| Field | Presence |
| --- | --- |
| `commit` | always: the commit id |
| `tree` | always: its tree id |
| `parents` | always: every recorded parent id, in order; `[]` for a root commit |
| `committedAt` | always: the committer timestamp, UTC, RFC 3339 with a `Z` suffix and whole seconds |
| `fingerprint` | always: Fingerprint v1 of the commit's whole tree, `dlm:v1:sha256:<64 hex>` |
| `nodeCount` | always: Identity Projection v1 records of that tree, root included |
| `structural` | when the entry has a base: `true` or `false` |
| `summary` | when the entry has a base |

`structural` and `summary` are both present or both absent. They are absent only for the last entry of a walk that ended at `shallow`: its structure against its base is unknown, never `false`.

`summary` is `{ "added": n, "removed": n, "changed": n, "moved": n, "total": n }` in that order, with `total = added + removed + changed + moved`. The counts are those of `dirloom diff git:<base> git:<commit>` under the same `moveDetection` (Diff Result Schema v3 `summary`); the base of a root commit is the empty tree. `structural` is `true` exactly when `total` is greater than zero.

Invariants a consumer may rely on:

- the first listed commit is `range.to`, unless `entries` is empty;
- each entry's first parent is the next entry's `commit`, except for entries hidden by `--structural-only`;
- `range.from`, when present, is never an entry's `commit`;
- the number of commits examined is at most `range.limit`; with `end: limit` it equals `range.limit`.

## Status vocabulary

| `status` | Exit | Meaning |
| --- | ---: | --- |
| `OK` | 0 | The walk completed; `range.end` says why it stopped |
| `OBSERVATION_ERROR` | 5 | A Git object, the repository, or the walk could not yield a trustworthy history |
| `INTERNAL_ERROR` | 6 | Broken Dirloom invariant |

An empty history (for example `--from` equal to the revision) is `OK` with `entries: []`. There is no exit 1: a history is not a pass/fail check. Exit `2` is reserved for invalid CLI usage and is not a history JSON document: the human diagnostic stays on stderr even when `--format json` was requested. A failed write of the JSON document itself is an operational exit `5` on stderr.

## Transactional output

The walk completes and every entry is validated before anything is written. A failure or a cancellation at any commit yields exactly one failure document: `schemaVersion`, `status` and `diagnostic`, with no `metadata`, `range` or `entries`. A partial history is never written as a success.

`diagnostic` is `{ "code": string, "message": string }`.

## Diagnostic codes

Git Tree Source v1 codes, all `OBSERVATION_ERROR`:

```text
git_unavailable
git_repository_not_found
git_revision_not_found
git_object_unavailable
git_tree_invalid
```

Structural History v1 codes, all `OBSERVATION_ERROR`:

```text
git_commit_invalid
git_shallow_invalid
history_from_not_reached
```

Artifact codes reused when they apply, such as `canonical_path_collision`. Fallback codes, used only when no typed code exists:

```text
history_observation_failure
history_internal_failure
```

## Stream rules

When `--format json` is accepted:

- stdout is exactly one JSON document plus a final newline;
- stderr is empty for that classified outcome;
- no ANSI sequence is emitted;
- no human sentence is appended.

Usage errors and a stdout write failure are stderr diagnostics instead.

## Evolution policy

Consumers must:

- ignore object fields they do not know;
- treat a diagnostic `code` they do not know as opaque, and still read `status`.

Within v1, these changes are additive and keep `schemaVersion: 1`:

- a new optional field on an existing object;
- a new diagnostic code under an existing status.

These changes require a new schema version:

- a new `status`, `end`, `traversal` or `moveDetection` value;
- a new exit code;
- removing or renaming a field, or changing the meaning, type, or presence rule of an existing field;
- changing the order of fields or of entries.

## Examples

Illustrative only; whitespace is condensed here, and the History Result Schema v1 golden fixtures are normative for bytes.

A history that ends at the requested bound:

```json
{
  "schemaVersion": 1,
  "status": "OK",
  "metadata": { "historyVersion": 1, "traversal": "first-parent", "comparisonVersion": 1, "identityProjectionVersion": 1, "moveDetection": "structural-v1" },
  "range": { "to": "<40 hex>", "from": "<40 hex>", "limit": 20, "end": "from" },
  "entries": [
    {
      "commit": "<40 hex>", "tree": "<40 hex>", "parents": ["<40 hex>"],
      "committedAt": "2026-10-30T19:40:09Z", "fingerprint": "dlm:v1:sha256:<64 hex>", "nodeCount": 697,
      "structural": true,
      "summary": { "added": 2, "removed": 1, "changed": 0, "moved": 1, "total": 4 }
    },
    {
      "commit": "<40 hex>", "tree": "<40 hex>", "parents": ["<40 hex>", "<40 hex>"],
      "committedAt": "2026-10-30T18:02:41Z", "fingerprint": "dlm:v1:sha256:<64 hex>", "nodeCount": 695,
      "structural": false,
      "summary": { "added": 0, "removed": 0, "changed": 0, "moved": 0, "total": 0 }
    }
  ]
}
```

A shallow clone, where the last listed commit has no base:

```json
{
  "schemaVersion": 1,
  "status": "OK",
  "metadata": { "historyVersion": 1, "traversal": "first-parent", "comparisonVersion": 1, "identityProjectionVersion": 1, "moveDetection": "structural-v1" },
  "range": { "to": "<40 hex>", "limit": 20, "end": "shallow" },
  "entries": [
    {
      "commit": "<40 hex>", "tree": "<40 hex>", "parents": ["<40 hex>"],
      "committedAt": "2026-10-30T19:40:09Z", "fingerprint": "dlm:v1:sha256:<64 hex>", "nodeCount": 697
    }
  ]
}
```

A failure:

```json
{
  "schemaVersion": 1,
  "status": "OBSERVATION_ERROR",
  "diagnostic": { "code": "history_from_not_reached", "message": "the walk reached a root commit without meeting --from" }
}
```
