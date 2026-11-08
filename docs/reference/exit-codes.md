# Exit codes and machine codes

> **Status:** candidate for the v0.4 freeze (`v0.4.0-rc1`). Consolidated from the contract of each command; on a conflict, the command's contract wins and this page is a defect.

Programs branch on three stable things:

- the **exit code**;
- the `status` of a JSON result;
- the machine `code` of a diagnostic.

Diagnostic messages, including localized operating-system text, are not stable ([compatibility policy](../compatibility-policy.md#2-what-is-stable-and-what-is-not)).

## Exit codes by command

| Command | 0 | 1 | 2 | 3 | 4 | 5 | 6 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| tree output (`dirloom [directory]`), `fingerprint`, `snapshot`, and the `config`, `preset`, `theme`, `completion` commands | success | runtime error, internal errors included | invalid usage | — | — | — | — |
| `verify` | `MATCH` | `MISMATCH` | invalid usage | `INVALID_SNAPSHOT` | `UNSUPPORTED_SNAPSHOT` | `SNAPSHOT_READ_ERROR`, `OBSERVATION_ERROR`, failed write of the result | `INTERNAL_ERROR` |
| `diff` | `NO_DIFFERENCES` | `DIFFERENCES` | invalid usage | `INVALID_SNAPSHOT` | `UNSUPPORTED_SNAPSHOT` | `SNAPSHOT_READ_ERROR`, `OBSERVATION_ERROR`, failed write of the result | `INTERNAL_ERROR` |
| `history` | `OK` | — | invalid usage | — | — | `OBSERVATION_ERROR`, failed write of the result | `INTERNAL_ERROR` |
| `watch` (experimental) | interrupted (Ctrl+C, `SIGTERM`) | — | invalid usage | — | — | fatal error, startup failure, failed write of events | broken internal invariant |

**Two families, kept for compatibility.** The first row is the v0.1–v0.3 family: `0`, `1`, `2` (decision D2 of the `v0.4.0-rc1` plan). The other commands use the `0`–`6` family of their contracts. The same failure can therefore exit `1` in the first family and `5` in the second, for example a missing directory, an unreadable tree, a failed write of stdout, or an internal error (`1` versus `6`).

**What reaches stdout.**

- In the first family, a failure writes nothing on stdout, even with `--format json`. The diagnostic is on stderr and carries no machine code.
- An invalid usage (exit `2`) writes nothing on stdout in any command.
- A failed write of the result is reported on stderr.

See [Q4](../compatibility-policy.md#2-what-is-stable-and-what-is-not).

## Unexpected internal failures

A panic is always a Dirloom bug: bad input and failed I/O are reported as errors. A panic does not crash the process; it is an internal error of the command:

- the exit code is the command's internal-error code: `1` in the first family, `6` in `verify`, `diff`, `history` and `watch`;
- stderr gets one line, `Error: internal error: unexpected panic: <value>`, without a stack trace;
- stdout keeps what the command wrote before the panic, and nothing is added. It can be empty, or hold a truncated document that is not valid JSON. With `--format json`, `verify`, `diff` and `history` write no failure document in this case. Programs check the exit code before they parse stdout.

In `watch`:

- a panic while events are encoded or written, or in the engine, ends the stream without `watch.stopped`. The stderr line is the one above; while writing, it reads `Error: internal error: write watch events: unexpected panic: <value>`;
- a panic in a native backend ends the run as a fatal condition: `watch.error` (`fatal: true`, `internal_invariant_failure`), then `watch.stopped` (`error`).

Both exit `6`.

Go *fatal errors* are not panics and cannot be recovered: out of memory, an exhausted goroutine stack (for example a recursion that is too deep), concurrent map writes, or a deadlock. The process then ends with Go's exit status `2` and a goroutine trace on stderr. In that case alone, `2` does not mean invalid usage.

## Categories

Every failure belongs to one category. The status and the exit code follow from the category, and the machine code names the cause.

| Category | Meaning | Status, exit (`verify`, `diff`) | `history` | `watch` | First family |
| --- | --- | --- | --- | --- | --- |
| usage | invalid arguments, flags or configuration file | exit `2`, no document | exit `2` | exit `2` | exit `2` |
| mismatch | a normal result: the structures differ | `MISMATCH` / `DIFFERENCES`, exit `1` | — | — | — |
| validation | a snapshot that is not valid Snapshot Schema v1 | `INVALID_SNAPSHOT`, exit `3` | — | — | — (no first-family command reads a snapshot) |
| corruption | a snapshot whose content does not match its own fingerprint | `INVALID_SNAPSHOT`, exit `3` | — | — | — |
| unsupported | a snapshot schema, artifact version or required feature this release does not accept | `UNSUPPORTED_SNAPSHOT`, exit `4` | — | — | — |
| source | the snapshot file, or the output, could not be read or written | `SNAPSHOT_READ_ERROR`, exit `5`; a failed write: exit `5`, no code | failed write: exit `5` | failed write: exit `5` | exit `1` |
| filesystem | the live tree could not be observed as a canonical artifact | `OBSERVATION_ERROR`, exit `5` | — | non-fatal once a state exists; fatal at startup or for a lost root, exit `5` | exit `1` |
| git | a Git repository, revision, object or history could not be read | `OBSERVATION_ERROR`, exit `5` | `OBSERVATION_ERROR`, exit `5` | — | — |
| internal | a broken Dirloom invariant | `INTERNAL_ERROR`, exit `6` | `INTERNAL_ERROR`, exit `6` | fatal, exit `6` | exit `1`, message prefixed `internal error:` |

## Machine codes

Every machine code that Dirloom reports, with its category.

| Code | Category | Reported when |
| --- | --- | --- |
| `invalid_snapshot_json` | validation | the snapshot is empty, not valid UTF-8, not valid JSON, has a duplicate key, or is not exactly one document |
| `missing_snapshot_field` | validation | a required snapshot field is absent |
| `invalid_snapshot_field` | validation | a snapshot field has the wrong type or an invalid value, including a malformed fingerprint string |
| `invalid_snapshot_artifact` | validation | the snapshot artifact breaks an artifact rule: a path outside the root, a duplicate node, a missing parent… |
| `snapshot_fingerprint_mismatch` | corruption | the embedded fingerprint is not the fingerprint of the embedded artifact |
| `snapshot_limit_exceeded` | validation | the snapshot exceeds a [reader limit](../contracts/snapshot-schema-v1.md#reader-limits): size, node count, nesting, ignore patterns or required features |
| `unsupported_snapshot_schema` | unsupported | `schemaVersion` is not `1` |
| `unsupported_snapshot_artifact_version` | unsupported | `artifactVersion` is not `1` |
| `unsupported_snapshot_feature` | unsupported | `requiredFeatures` names a feature this release does not know |
| `snapshot_read_failure` | source | the snapshot file could not be read |
| `source_read_failure` | filesystem | a directory or an entry could not be read: a missing root, a denied permission, an entry removed during the scan, a `.gitignore` above 1 MiB |
| `canonical_path_collision` | filesystem | two entries map to the same canonical path, for example NFC and NFD forms of one name |
| `invalid_canonical_path` | filesystem | an entry name is not a valid canonical name, for example a Windows name that is not valid UTF-16 |
| `unsupported_node_type` | filesystem | a named pipe, socket or device is in the observed view |
| `git_unavailable`, `git_repository_not_found`, `git_revision_not_found`, `git_object_unavailable`, `git_tree_invalid`, `git_tree_too_large` | git | see [Git Tree Source v1](../contracts/git-tree-source-v1.md#diagnostics) |
| `git_commit_invalid`, `git_shallow_invalid`, `history_from_not_reached` | git | see [Structural History v1](../contracts/structural-history-v1.md) |
| `watch_root_unavailable`, `watch_limit_reached`, `watch_backend_failure`, `watch_filesystem_unsupported` | filesystem | fatal `watch` conditions, see [Structural Watch v1](../contracts/structural-watch-v1.md#fatal-conditions) |
| `internal_invariant_failure` | internal | a broken invariant without a more specific code |
| `artifact_invariant_violation` | internal | an artifact that Dirloom built from an observation breaks an artifact rule. The same rule broken inside a snapshot is `invalid_snapshot_artifact` instead. |
| `canonical_encoding_failure` | internal | canonical identity encoding failed |
| `snapshot_internal_failure` | internal | the snapshot package failed without a cause |
| `verify_observation_failure`, `diff_observation_failure`, `history_observation_failure`, `watch_observation_failure` | filesystem or git | fallbacks when a failure has no typed code. In practice they appear when the command is interrupted during an observation. |
| `verify_internal_failure`, `diff_internal_failure`, `history_internal_failure`, `watch_internal_failure` | internal | fallbacks for a broken invariant without a typed code |

Two library codes are never reported by the CLI:

- `invalid_fingerprint_representation`: a malformed fingerprint in a snapshot is reported as `invalid_snapshot_field`.
- `snapshot_write_failure`: the CLI reports a failed write of its output without a machine code.

Each code is reported only under the status of its category. Adding a code under an existing status is compatible. Moving a code to another status or exit code needs a new contract version once the contracts are frozen ([policy §3](../compatibility-policy.md#3-changing-a-frozen-contract)).
