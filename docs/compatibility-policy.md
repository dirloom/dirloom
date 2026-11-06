# Compatibility and Freeze Policy (v0.4)

> **Status:** candidate policy for the v0.4 freeze, prepared in `v0.4.0-rc1` LOT 1. It takes effect for the frozen contracts in the freeze commit of the release branch (gate G2); until then, every contract listed as a candidate may still be corrected before the freeze.
>
> **Related:** [Contract register](contracts/README.md), [Changelog policy](release/changelog-policy.md), [Release workflow](release-workflow.md)

This policy says what Dirloom promises to keep stable from v0.4 on, what it does not promise, and what a change must do when it touches a promise. The [contract register](contracts/README.md) lists each contract, its owner and its own change rules.

## 1. What is a public contract

A public contract is an input Dirloom reads or an output Dirloom writes that other programs or files may rely on, with a written definition and tests that pin it. Each one is in the register with one of these statuses:

| Status | Meaning |
| --- | --- |
| **candidate for the v0.4 freeze** | in the frozen set decided for v0.4 (D1); still correctable until G2 |
| **frozen for v0.4** | after G2 only; changed only under §3 |
| **published** | a v0.1–v0.3 public contract outside the v0.4 frozen set; it keeps its own compatibility rules |
| **experimental** | may change in any release, never silently (Structural Watch v1, Watch Event Schema v1) |
| **superseded** | no longer emitted; kept as the reference for frozen fixtures |
| **diagnostic** | a machine-readable output meant for inspection, not a contract (§5) |

No contract is frozen before G2.

## 2. What is stable, and what is not

Stable, for a frozen contract:

- the meaning, type and presence rule of every documented field;
- every documented enumeration value: statuses, kinds, change types, `end`, `traversal`, `moveDetection`, and similar;
- the machine diagnostic codes, and the status and exit code each one is reported with;
- the exit codes of each command;
- the bytes of each golden fixture, where the contract says the fixtures are normative for bytes;
- for identity contracts, the fingerprint of a given observed tree (§4).

Not stable, even after the freeze:

- **diagnostic messages.** Message text, including localized operating-system text such as `Accès refusé.`, may change in any release. Programs must branch on the machine `code`, the `status` and the exit code, never on a message (F6).
- **stderr.** Human diagnostics on stderr are not a format.
- **timing and performance,** within the documented limits.
- **help text and wording,** as long as every documented flag keeps its meaning.

Some failures may write nothing on stdout, even with `--format json`: usage errors (exit `2`), failures of the tree output and of `fingerprint`, and a failed write of the result itself. The diagnostic is on stderr and the exit code carries the outcome (Q4). A uniform JSON error document would be a later, explicitly versioned change.

## 3. Changing a frozen contract

A change is **compatible** when a consumer written against the contract keeps working. Unless the contract's own section says otherwise, these are compatible:

- adding an optional field to a JSON object, for consumers that ignore unknown fields, as the result contracts ask them to;
- adding a diagnostic code under an existing status;
- adding a command, a flag or a `--format` value that changes no existing output.

These are **incompatible** and need a new version of the contract (a new `schemaVersion`, contract version or strategy name) and a CHANGELOG entry:

- removing or renaming a field, or changing its meaning, type or presence rule;
- a new enumeration value where the contract lists a closed set (a new `status`, kind, or change type);
- a new exit code, or a different exit code for an existing outcome;
- moving an existing diagnostic code to another status or exit code;
- changing field order or entry order, where the bytes are normative.

A contract's own **Versioning**, **Evolution policy** or **Compatibility policy** section is more precise than this list and wins over it. Each change also follows the [changelog policy](release/changelog-policy.md); a CHANGELOG entry never replaces a version change.

## 4. Identity and fingerprints (D13)

Fingerprint v1 is a promise that the same observed tree gives the same fingerprint, on every supported platform and in every later v0.4-compatible release.

After the freeze, **an incompatible change to identity semantics needs a new fingerprint version**, even when no format changes. This includes the observation itself: if a change makes Dirloom record a different canonical artifact for the same tree on disk (another kind, another target, another path, an entry newly included or excluded by default), the fingerprints it produces would no longer be comparable with the old ones, so the change ships with a new identity projection version and a new `dlm:vN` prefix.

Two changes of this kind are made inside v1 in `v0.4.0-rc1`, before the freeze, and each CHANGELOG entry describes its effect:

- the junction correction (#70) moves the fingerprint of a Windows tree that holds a junction;
- the refusal of named pipes, sockets and devices (LOT 4, D3) leaves a tree that holds such an entry with no fingerprint at all: the observation fails with `unsupported_node_type` instead of recording the entry as a `file`.

## 5. Diagnostic outputs (D1)

These outputs have a `schemaVersion` so that scripts can detect a change, but they are **diagnostics, not contracts**, and are outside the v0.4 freeze:

- `dirloom config explain --as json`;
- `dirloom preset explain --as json`;
- `dirloom theme list --as json`, `theme explain --as json`, `theme validate --as json` and `theme classify --as json`.

They may change in any release. Every change is listed in the CHANGELOG, and their `schemaVersion` changes when an existing field is removed or changes meaning. Do not build automation that must keep working across releases on them.

## 6. Configuration and theme files

The YAML configuration schema v1 (`.dirloom.yaml`, `--config`, the user file) and the theme file schema v1 are **published** input contracts, kept by their own rules:

- [configuration](configuration.md#compatibility): schema version `1` keeps the documented meanings and types. A new optional field may appear with release notes; an older binary rejects a field it does not know, because unknown fields are always refused. A change to an existing field's meaning or type needs a new schema version.
- [themes](themes.md): theme file `schemaVersion: 1` with `catalogVersion: 1`.

## 7. Exit codes

Two families exist, and both are kept for compatibility (D2):

- tree output, `fingerprint`, `snapshot`: `0` success, `1` runtime error, `2` invalid usage, as since v0.1;
- `verify`, `diff`, `history` and `watch`: the `0`–`6` family their contracts define.

The same failure may therefore exit `1` in one family and `5` in the other. Dirloom does not renumber them. The consolidated table, with the category, status and exit code of every machine code, is [Exit codes and machine codes](reference/exit-codes.md) (D4).

## 8. Freeze procedure

1. **LOT 1** marks the candidates and pins them with golden fixtures. No contract is marked frozen.
2. **LOTs 2 to 4** may still correct candidate contracts. Each correction updates the contract, its fixtures and the CHANGELOG.
3. **G2, the freeze commit** of `release/v0.4.0-rc1`, first checks that the corrections left every candidate valid, then publishes the definitive register: candidate status lines become "frozen for v0.4".
4. After G2, §3 and §4 apply.
