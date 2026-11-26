# Contract Register

> **Status:** candidate register for the v0.4 freeze, prepared in `v0.4.0-rc1` LOT 1. The definitive register is published in the freeze commit (gate G2), where candidate status lines become "frozen for v0.4".
>
> **Related:** [Compatibility and freeze policy](../compatibility-policy.md), [Exit codes and machine codes](../reference/exit-codes.md), [Structural Version Control](../structural-version-control.md)

Every public input and output of Dirloom, with its status, its owner and what pins it. **Owner** is the package that defines the contract in code; a change to the contract starts there, with its tests and fixtures. Statuses are defined in the [policy](../compatibility-policy.md#1-what-is-a-public-contract).

## Machine outputs by command

Which command and flag write which machine-readable document, and how a program recognizes its version. Programs check that version field before they read the rest, and branch on the exit code first ([Exit codes and machine codes](../reference/exit-codes.md)).

| Command | Output | Contract | Version field | Status |
| --- | --- | --- | --- | --- |
| `dirloom [directory] --format json` | tree | [Tree JSON Schema v1](tree-json-v1.md) | `"schemaVersion": 1` | candidate |
| `dirloom [directory] --format markdown-tree` | Markdown list | [Markdown tree](../markdown-tree.md) | none: the format is the v0.2 output | candidate |
| `dirloom [directory] --format mermaid`, `graphviz`, `d2` | graph source | [Graph exports](../graph-exports.md) | header `dirloom-diagram-contract: 1` | candidate |
| `dirloom fingerprint` | one line `dlm:v1:sha256:<hex>` | [Identity Projection v1](identity-projection-v1.md) | the `v1` token | candidate |
| `dirloom fingerprint --format json` | fingerprint | [Fingerprint JSON v1](fingerprint-json-v1.md) | `"schemaVersion": 1` | candidate |
| `dirloom snapshot`, to stdout or `--output` | snapshot | [Snapshot Schema v1](snapshot-schema-v1.md) | `"schemaVersion": 1`, `"artifactVersion": 1`, `requiredFeatures` | candidate |
| `dirloom verify --format json` | verify result | [Verify Result Schema v1](verify-result-v1.md) | `"schemaVersion": 1` | candidate |
| `dirloom diff --format json` | diff result | [Diff Result Schema v3](diff-result-v3.md) | `"schemaVersion": 3` | candidate |
| `dirloom history --format json` | history result | [History Result Schema v1](history-result-v1.md) | `"schemaVersion": 1` | candidate |
| `dirloom watch --format ndjson` | one event per line | [Watch Event Schema v1](watch-event-v1.md) | `"schemaVersion": 1` on every line | experimental |
| `config explain`, `preset explain`, `theme list`, `theme explain`, `theme validate`, `theme classify`, each with `--as json` | diagnostics | [policy §5](../compatibility-policy.md#5-diagnostic-outputs-d1) | `schemaVersion` | diagnostic, not a contract |

Inputs that Dirloom reads have contracts too: a snapshot passed to `verify` or `diff` (Snapshot Schema v1), a Git tree named by `git:` or walked by `history` ([Git Tree Source v1](git-tree-source-v1.md), [Structural History v1](structural-history-v1.md)), and the YAML configuration and theme files (below).

## Candidates for the v0.4 freeze

| Contract | Version | Owner | Definition | Pinned by | Change rules |
| --- | --- | --- | --- | --- | --- |
| Canonical Structural Artifact | 1 | `internal/artifact` | [canonical-structural-artifact-v1.md](../canonical-structural-artifact-v1.md) | artifact validation tests, fuzz targets, junction tests, special-type refusal tests | [Invariants](../canonical-structural-artifact-v1.md#invariants), [policy §4](../compatibility-policy.md#4-identity-and-fingerprints-d13) |
| Canonical Identity Encoding | 1 | `internal/identity` | [canonical-identity-encoding-v1.md](canonical-identity-encoding-v1.md) | byte-vector golden, fuzz, property tests | [Versioning](canonical-identity-encoding-v1.md#versioning) |
| Identity Projection / Fingerprint | 1 (`dlm:v1:sha256`) | `internal/identity` | [identity-projection-v1.md](identity-projection-v1.md) | `testdata/identity/golden/*.fingerprint`, fuzz | [Versioning](identity-projection-v1.md#versioning), [policy §4](../compatibility-policy.md#4-identity-and-fingerprints-d13) |
| Fingerprint JSON | `schemaVersion` 1 | `internal/cli` | [fingerprint-json-v1.md](fingerprint-json-v1.md) | `testdata/identity/golden/*.fingerprint.json` | [policy §3](../compatibility-policy.md#3-changing-a-frozen-contract) |
| Snapshot Schema, with Capture Semantics | 1 | `internal/snapshot` | [snapshot-schema-v1.md](snapshot-schema-v1.md) | `testdata/snapshots/v1/valid`, `testdata/snapshots/v1/invalid` with the exact code of each fixture, fuzz | [Compatibility policy](snapshot-schema-v1.md#compatibility-policy), [ADR 0002](../adr/0002-snapshot-persistence-and-compatibility.md) |
| Verify Result Schema | 1 | `internal/cli` (`verify`) | [verify-result-v1.md](verify-result-v1.md) | `testdata/verify/v1/golden`, exits 0 to 6 | [Evolution policy](verify-result-v1.md#evolution-policy) |
| Diff Result Schema | 3 | `internal/cli` (`diff`), `internal/comparison` | [diff-result-v3.md](diff-result-v3.md) | `testdata/diff/v3/golden`, fuzz | [Evolution policy](diff-result-v3.md#evolution-policy) |
| Move Detection | `structural-v1` | `internal/comparison` | [move-detection-structural-v1.md](move-detection-structural-v1.md) | move tests, fuzz, diff goldens | [Versioning](move-detection-structural-v1.md#versioning) |
| Git Tree Source | 1 | `internal/gitsource` | [git-tree-source-v1.md](git-tree-source-v1.md) | protocol fuzz, Git matrix tests, diff `git-*` goldens | [Versioning](git-tree-source-v1.md#versioning) |
| Structural History | 1 | `internal/history` | [structural-history-v1.md](structural-history-v1.md) | history matrix tests, fuzz | [Versioning](structural-history-v1.md#versioning) |
| History Result Schema | 1 | `internal/cli` (`history`) | [history-result-v1.md](history-result-v1.md) | `testdata/history/v1/golden` | [Evolution policy](history-result-v1.md#evolution-policy) |
| Tree JSON Schema | 1 | `internal/render` | [tree-json-v1.md](tree-json-v1.md) | `internal/render/testdata/json*.golden`, `testdata/identity/golden/*.tree.json` | [Versioning](tree-json-v1.md#versioning) |
| Markdown tree (`markdown-tree`) | v0.2 output | `internal/render` | [markdown-tree.md](../markdown-tree.md) | `internal/render/testdata/markdown-tree.golden` | [Compatibility](../markdown-tree.md#compatibility) |
| Graph exports (Mermaid, Graphviz, D2) | diagram contract 1 | `internal/diagram`, `internal/render` | [graph-exports.md](../graph-exports.md) | `internal/render/testdata/{mermaid,graphviz,d2}.golden`, CI parsers | [Contract version and views](../graph-exports.md#contract-version-and-views) |
| Exit codes | per command | `internal/cli` | each command reference and result contract; [policy §7](../compatibility-policy.md#7-exit-codes) | per-command exit tests | [policy §3](../compatibility-policy.md#3-changing-a-frozen-contract) |
| CLI commands and flags | — | `internal/cli` | [README](../../README.md), command references in [reference](../reference/) | help and documentation tests | [policy §3](../compatibility-policy.md#3-changing-a-frozen-contract) |

The text and fenced Markdown tree renderings (`text`, `markdown`) are v0.1 outputs kept byte for byte by `internal/render/testdata/{unicode,ascii,markdown}.golden`; they are covered by the same freeze as the other tree outputs.

## Published, outside the v0.4 frozen set

| Contract | Version | Owner | Definition | Notes |
| --- | --- | --- | --- | --- |
| YAML configuration schema | 1 | `internal/config` | [configuration.md](../configuration.md#schema-reference) | strict reading: unknown fields, duplicate keys, several documents, anchors, aliases, merge keys and custom tags are refused; 1 MiB limit; [its compatibility rules](../configuration.md#compatibility) hold |
| Theme file schema | 1 (`catalogVersion` 1) | `internal/presentation` | [themes.md](../themes.md) | kept by its own rules since v0.2 |

## Experimental, excluded from the freeze

| Contract | Version | Owner | Definition |
| --- | --- | --- | --- |
| Structural Watch | 1 | `internal/watch` | [structural-watch-v1.md](structural-watch-v1.md) |
| Watch Event Schema | 1 | `internal/watch`, `internal/cli` (`watch`) | [watch-event-v1.md](watch-event-v1.md) |

## Superseded

| Contract | Emitted by | Definition |
| --- | --- | --- |
| Diff Result Schema v1 | `v0.4.0-a4` | [diff-result-v1.md](diff-result-v1.md), frozen fixtures in `testdata/diff/v1/golden` |
| Diff Result Schema v2 | `v0.4.0-a5` | [diff-result-v2.md](diff-result-v2.md), frozen fixtures in `testdata/diff/v2/golden` |

## Diagnostic outputs, not contracts

`config explain --as json`, `preset explain --as json`, and `theme list`, `theme explain`, `theme validate` and `theme classify` with `--as json`. See [policy §5](../compatibility-policy.md#5-diagnostic-outputs-d1).

## JSON Schema files (D12)

Dirloom ships no JSON Schema file for v0.4. Each result contract is defined by its Markdown document together with byte-exact golden fixtures and fuzz targets, which also pin field order, indentation and exact codes. A JSON Schema could not express those properties, and a second normative description would have to be kept in step with the first. The Markdown contracts, the golden fixtures and the tests are the normative reference. A JSON Schema can be published after v0.4 as an informative companion without changing any contract version. The CTO confirmed this deferral on 2026-11-26.
