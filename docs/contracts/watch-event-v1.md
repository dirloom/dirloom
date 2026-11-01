# Watch Event Schema v1

> **Status:** experimental normative contract for Dirloom `v0.4.0-a8`; excluded from the v0.4 RC contract freeze ([Evolution and experimental policy](#evolution-and-experimental-policy))
>
> **Related:** [Structural Watch v1](structural-watch-v1.md), [Diff Result Schema v3](diff-result-v3.md), [Snapshot Schema v1](snapshot-schema-v1.md), [ADR 0008](../adr/0008-experimental-watch.md)

Machine-readable output of `dirloom watch --format ndjson`: a stream of events, one JSON object per line, that reports a [Structural Watch v1](structural-watch-v1.md) run. It does not modify any other contract. The bytes are not fingerprinted.

## Encoding

```text
schemaVersion: 1 on every line
encoding: UTF-8, no byte order mark
layout: one compact JSON object per line, no indentation, no blank line
html escaping: disabled
line terminator: one line feed (\n), on every platform
```

Every line starts with `schemaVersion`, then `event`, then `generation`, except the lines of a [startup failure](#startup-failure), which have no `generation`. The other fields follow in the order given below, fixed by the encoder.

## Events

| `event` | Meaning |
| --- | --- |
| `watch.started` | first line of a run: contracts, Capture Semantics, initial state |
| `node.added`, `node.removed`, `node.changed`, `node.moved` | one structural change of a generation |
| `state.changed` | closes and commits a generation |
| `rescan.started` | the state may be stale from here on |
| `rescan.completed` | the stream is in sync again |
| `watch.error` | a diagnostic, fatal or not |
| `watch.stopped` | last line of a complete stream |

### `watch.started`

| Field | Value |
| --- | --- |
| `generation` | `0` |
| `metadata` | the contracts the run follows (below) |
| `capture` | Capture Semantics v1, as in [Snapshot Schema v1](snapshot-schema-v1.md#capture-semantics-v1), same fields in the same order: `depth`, `dirsOnly`, `hidden`, `useDefaultIgnores`, `useGitignore`, `ignore` |
| `fingerprint` | Fingerprint v1 of the initial state, `dlm:v1:sha256:<64 hex>` |
| `nodeCount` | Identity Projection v1 records of the initial state, root included |

`metadata`, in this order:

| Field | Value |
| --- | --- |
| `watchVersion` | `1` (Structural Watch v1) |
| `stability` | `experimental` |
| `comparisonVersion` | `1` |
| `identityProjectionVersion` | `1` |
| `moveDetection` | `structural-v1` (default) or `none` (`--no-moves`) |
| `backend` | `native` or `poll` (`--poll`); never the name of the platform mechanism |

### Node events

A node event is one change of [Diff Result Schema v3](diff-result-v3.md#change-vocabulary) between the previous state and the new one. It has exactly the fields, values, presence rules and node-state objects (`kind`, plus `target` for `symlink` and `junction`) of a v3 change, with `event` in place of `operation`:

| `event` | v3 `operation` | Fields after `generation`, in order |
| --- | --- | --- |
| `node.added` | `ADDED` | `path`, `after` |
| `node.removed` | `REMOVED` | `path`, `before` |
| `node.changed` | `CHANGED` | `path`, `before`, `after` |
| `node.moved` | `MOVED` | `path`, `from`, `before`, `after`, `evidence`, `nodes` |

`generation` is the generation the change belongs to. `path` and `from` are canonical paths relative to the root, never the root `.` itself. A `node.moved` covers its whole subtree, as a v3 `MOVED` does: its descendants are not reported separately.

### `state.changed`

| Field | Value |
| --- | --- |
| `generation` | the generation it closes |
| `fingerprint` | Fingerprint v1 of the new state |
| `nodeCount` | Identity Projection v1 records of the new state, root included |
| `summary` | `{ "added", "removed", "changed", "moved", "total" }` in that order, counting the generation's node events, with `total` their sum |

### `rescan.started`

| Field | Value |
| --- | --- |
| `generation` | the last committed generation |
| `reason` | `overflow`, `reconciliation` or `error` ([Structural Watch v1](structural-watch-v1.md#rescans)) |

### `rescan.completed`

| Field | Value |
| --- | --- |
| `generation` | the last committed generation |
| `fingerprint` | the state's fingerprint |
| `nodeCount` | the state's node count |

### `watch.error`

| Field | Value |
| --- | --- |
| `generation` | the last committed generation; absent in a startup failure |
| `fatal` | `true` when the run ends with it, else `false` |
| `diagnostic` | `{ "code": string, "message": string }` |

### `watch.stopped`

| Field | Value |
| --- | --- |
| `generation` | the last committed generation; absent in a startup failure |
| `reason` | `interrupted` or `error` |
| `fingerprint` | the state's fingerprint; absent in a startup failure |

## Stream grammar

```text
stream     = started { item } [ fatal ] stopped
           | fatal stopped                                 (startup failure)
item       = generation | rescan-started | rescan-completed | error
generation = node-event { node-event } state-changed
error      = watch.error (fatal: false) [ rescan-started (reason: error) ]
fatal      = watch.error (fatal: true)
```

A stream may also end anywhere without `watch.stopped`: the process was killed, its stdout was closed, or a blocked stdout outlasted the output grace period. Such a stream is **incomplete**.

### Startup failure

When the run fails before its first state, the stream is exactly two lines: a fatal `watch.error` and `watch.stopped` (`error`), neither with `generation`, and `watch.stopped` without `fingerprint`.

## Invariants

A complete stream satisfies all of these; Dirloom's stream validator checks them in tests.

1. Every line is one JSON object with `schemaVersion: 1` and a known `event`.
2. The first line is `watch.started` with generation 0, or the first line of a startup failure.
3. Generations increase by exactly 1. The node events of a generation are contiguous, carry its number, are sorted by `path` as UTF-8 bytes, obey every Diff Result Schema v3 change rule, and are followed at once by its `state.changed`, whose `summary` counts them; `total` is at least 1.
4. Each `state.changed` fingerprint differs from the previous state's.
5. Any other line carries the last committed generation.
6. Rescans never nest. `rescan.started` (`error`) directly follows a non-fatal `watch.error`. `rescan.started` (`reconciliation`) is directly followed by a generation. `rescan.completed` carries the state's fingerprint and node count.
7. There is at most one fatal `watch.error`, directly followed by `watch.stopped` (`error`). `watch.stopped` is the last line; with `interrupted`, no fatal error precedes it; its fingerprint is the state's.

## Consuming a stream

Dirloom validates each generation in full before writing it, but writing to stdout is not atomic ([Structural Watch v1](structural-watch-v1.md#publication)). A consumer therefore:

- commits a generation only when it has read its `state.changed`;
- discards a generation cut short by the end of the stream, including a last line without its line feed;
- treats the state as possibly stale between `rescan.started` and `rescan.completed`;
- treats a stream without `watch.stopped` as incomplete;
- can check its own copy of the state against any `fingerprint` the stream carries.

A consumer that starts from `watch.started` and applies every committed generation holds the stream's state. The initial state itself is not in the stream: `dirloom snapshot` with the same Capture Semantics provides it, and `watch.started.fingerprint` tells whether the two match.

## Exit codes and stream rules

| Outcome | stdout | stderr | Exit |
| --- | --- | --- | ---: |
| controlled interruption | a complete stream ending with `watch.stopped` (`interrupted`) | empty | 0 |
| usage error | empty | `Error: …` | 2 |
| fatal observation, backend or filesystem error; startup failure | a complete stream ending with `watch.error` (fatal) and `watch.stopped` (`error`) | empty | 5 |
| interruption or fatal condition with stdout blocked beyond the output grace period | an incomplete stream | a best-effort `Error: …` | 5 |
| stdout write failure | an incomplete stream | `Error: …` | 5, or `SIGPIPE` on Unix |
| broken invariant | a complete stream ending with `watch.error` (fatal, `internal_invariant_failure` or `watch_internal_failure`) and `watch.stopped` (`error`) | empty | 6 |
| Windows console close, logoff or shutdown | best effort: `watch.stopped` (`interrupted`) when Windows allows it | — | — |

Under `--format ndjson`, stdout carries only events: no ANSI sequence, no human sentence. Stderr stays empty except in the rows above that say otherwise. There is no exit 1, because a watch is not a check, and no exit 3 or 4, because it reads no snapshot.

## Diagnostic codes

Fatal codes ([Structural Watch v1](structural-watch-v1.md#fatal-conditions)):

```text
watch_root_unavailable
watch_limit_reached
watch_backend_failure
watch_filesystem_unsupported
internal_invariant_failure
```

Observation codes, non-fatal once the first state exists, fatal when the startup observation still fails after its retries: the artifact codes, such as `source_read_failure`, `canonical_path_collision` and `unsupported_node_type`.

Fallback codes, used only when no typed code exists: `watch_observation_failure` for an observation, `watch_internal_failure` (exit 6) for a broken invariant.

```text
watch_observation_failure
watch_internal_failure
```

Messages are informative, not stable. They name paths relative to the root and never disclose another local path.

## Evolution and experimental policy

Consumers must:

- ignore object fields they do not know;
- treat a diagnostic `code` or a rescan `reason` they do not know as opaque;
- skip a line whose `event` they do not know.

Within v1, these changes are additive and keep `schemaVersion: 1`:

- a new optional field on an existing object;
- a new diagnostic code;
- a new rescan reason;
- a new event that a consumer may skip without losing track of the state.

These changes require a new schema version:

- a new node event, a new `watch.stopped` reason, or a new `metadata.backend` or `moveDetection` value;
- a change of exit codes;
- removing or renaming a field, or changing the meaning, type, presence rule or order of an existing field or line.

Watch Event Schema v1 is **experimental**: it is excluded from the v0.4 RC contract freeze, and a change that requires a new schema version may ship in any v0.x release before `watch` is declared stable. It always comes with the new `schemaVersion` and a CHANGELOG entry; it is never silent.

## Examples

Illustrative only; long fingerprints and some fields are abridged here, and the Watch Event Schema v1 golden fixtures are normative for bytes.

A run that observes a directory created and another moved, then is interrupted:

```json
{"schemaVersion":1,"event":"watch.started","generation":0,"metadata":{"watchVersion":1,"stability":"experimental","comparisonVersion":1,"identityProjectionVersion":1,"moveDetection":"structural-v1","backend":"native"},"capture":{"depth":null,"dirsOnly":false,"hidden":false,"useDefaultIgnores":true,"useGitignore":true,"ignore":[]},"fingerprint":"dlm:v1:sha256:<64 hex>","nodeCount":1204}
{"schemaVersion":1,"event":"node.added","generation":1,"path":"src/features","after":{"kind":"directory"}}
{"schemaVersion":1,"event":"node.moved","generation":1,"path":"src/features/auth","from":"src/shared/auth","before":{"kind":"directory"},"after":{"kind":"directory"},"evidence":"subtree","nodes":4}
{"schemaVersion":1,"event":"state.changed","generation":1,"fingerprint":"dlm:v1:sha256:<64 hex>","nodeCount":1205,"summary":{"added":1,"removed":0,"changed":0,"moved":1,"total":2}}
{"schemaVersion":1,"event":"watch.stopped","generation":1,"reason":"interrupted","fingerprint":"dlm:v1:sha256:<64 hex>"}
```

An overflow whose rescan finds a removed file:

```json
{"schemaVersion":1,"event":"rescan.started","generation":4,"reason":"overflow"}
{"schemaVersion":1,"event":"node.removed","generation":5,"path":"build/out.log","before":{"kind":"file"}}
{"schemaVersion":1,"event":"state.changed","generation":5,"fingerprint":"dlm:v1:sha256:<64 hex>","nodeCount":1203,"summary":{"added":0,"removed":1,"changed":0,"moved":0,"total":1}}
{"schemaVersion":1,"event":"rescan.completed","generation":5,"fingerprint":"dlm:v1:sha256:<64 hex>","nodeCount":1203}
```

A missed notification found by a reconciliation:

```json
{"schemaVersion":1,"event":"rescan.started","generation":5,"reason":"reconciliation"}
{"schemaVersion":1,"event":"node.added","generation":6,"path":"docs/notes.md","after":{"kind":"file"}}
{"schemaVersion":1,"event":"state.changed","generation":6,"fingerprint":"dlm:v1:sha256:<64 hex>","nodeCount":1204,"summary":{"added":1,"removed":0,"changed":0,"moved":0,"total":1}}
{"schemaVersion":1,"event":"rescan.completed","generation":6,"fingerprint":"dlm:v1:sha256:<64 hex>","nodeCount":1204}
```

An observation failure, then recovery with no structural change:

```json
{"schemaVersion":1,"event":"watch.error","generation":6,"fatal":false,"diagnostic":{"code":"source_read_failure","message":"read metadata for \"tmp/x\": entry no longer exists"}}
{"schemaVersion":1,"event":"rescan.started","generation":6,"reason":"error"}
{"schemaVersion":1,"event":"rescan.completed","generation":6,"fingerprint":"dlm:v1:sha256:<64 hex>","nodeCount":1204}
```

The root removed during the run:

```json
{"schemaVersion":1,"event":"watch.error","generation":6,"fatal":true,"diagnostic":{"code":"watch_root_unavailable","message":"the watched root was removed"}}
{"schemaVersion":1,"event":"watch.stopped","generation":6,"reason":"error","fingerprint":"dlm:v1:sha256:<64 hex>"}
```

A startup failure:

```json
{"schemaVersion":1,"event":"watch.error","fatal":true,"diagnostic":{"code":"watch_filesystem_unsupported","message":"the root is on a network filesystem (nfs); use --poll"}}
{"schemaVersion":1,"event":"watch.stopped","reason":"error"}
```
