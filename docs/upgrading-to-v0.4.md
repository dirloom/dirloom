# Upgrading from v0.3.3 to v0.4

> **Status:** upgrade note for `v0.4.0-rc1`, the first release candidate of v0.4. `v0.3.3` stays the stable release until `v0.4.0`.

v0.4 adds Structural Version Control: six commands that record, check and compare the structure of a project ([overview](structural-version-control.md)). The tree outputs of v0.1 to v0.3 keep their formats and their exit codes, with the few corrections listed in [Changes to existing behavior](#changes-to-existing-behavior).

## Installing the release candidate

The stable channels stay on `v0.3.3` during the release candidate: `winget upgrade`, `scoop update` and `brew upgrade` do not install v0.4 until `v0.4.0` is published. To try the candidate:

- download a release archive from [GitHub Releases](https://github.com/dirloom/dirloom/releases) and verify it as the [security policy](../SECURITY.md#verifying-a-release) describes. The installation blocks of the [README](../README.md#release-archives) download one version, check its SHA-256 and keep the previous binary as `.bak`; do not point them at a directory that a package manager manages;
- or build it from a clone checked out at the release candidate's tag, with Go 1.26.10 or newer ([Build from source](../README.md#build-from-source)). `go install` does not work for Dirloom at the moment, for any version: the README's [Install with Go](../README.md#install-with-go) explains why.

`dirloom --version` prints the installed version.

## New commands

| Command | Purpose | Exit codes |
| --- | --- | --- |
| `dirloom fingerprint [directory]` | print the identity of the observed structure | `0`, `1`, `2` |
| `dirloom snapshot [directory]` | record it as a self-verifying JSON file | `0`, `1`, `2` |
| `dirloom verify <snapshot> [directory]` | check a directory against a snapshot | `0` to `6` |
| `dirloom diff <source-a> <source-b>` | list the paths added, removed, changed or moved between two snapshots, directories or Git revisions | `0` to `6` |
| `dirloom history [revision]` | summarize the structural changes along a Git history | `0`, `2`, `5`, `6` |
| `dirloom watch [directory]` | **experimental**: report structural changes as they happen | `0`, `2`, `5`, `6` |

`git:` sources of `diff`, and `history`, need Git 2.45 or newer. The help topics `structural`, `sources` and `moves` introduce them in the terminal: `dirloom help structural`.

**These names now run commands.** In `v0.3.3`, `dirloom diff` inspected a directory named `diff`. In v0.4, `fingerprint`, `snapshot`, `verify`, `diff`, `history` and `watch` are commands, as `config`, `preset`, `theme`, `completion` and `help` already were. To inspect a directory with one of these names, write a path: `dirloom ./diff`.

## Exit codes

- The tree output and the `config`, `preset`, `theme` and `completion` commands keep `0` (success), `1` (runtime error) and `2` (invalid usage). The new `fingerprint` and `snapshot` use the same three codes.
- `verify`, `diff`, `history` and `watch` use the `0`–`6` family of their contracts, where `1` means a mismatch or differences, not an error.
- The same failure can therefore exit `1` in the first family and `5` in the second. Dirloom keeps both families for compatibility; [Exit codes and machine codes](reference/exit-codes.md) has the table.
- **Changed:** an unexpected panic used to end Dirloom with Go's crash status `2`, the status of an invalid usage, and a stack trace. It now exits with the command's internal-error code, `1` for the tree output, with one `Error: internal error: …` line and no stack trace. No panic is known to occur.

Diagnostic messages, on stderr, are not stable: scripts branch on the exit code ([compatibility policy](compatibility-policy.md#2-what-is-stable-and-what-is-not)).

## Changes to existing behavior

### Windows junctions

`v0.3.3` showed a Windows junction, or a folder where a volume is mounted, as a plain file: `"type": "file"` in tree JSON, with no target. v0.4 recognizes it by its reparse tag and shows it as a link, never entered:

- text and Markdown outputs: `name -> target`;
- tree JSON: `"type": "symlink"` with its `"target"`; Tree JSON Schema v1 gains no new type;
- Mermaid, Graphviz and D2: a link node, so its node identifier changes;
- `dirloom theme classify`: a symlink, instead of a refusal.

The structural commands record it with the kind `junction` and its target, rewritten relative to the root when it points inside it. The v0.4 alphas, up to `v0.4.0-a8`, recorded a junction as a `file`. A snapshot they took of a tree holding a junction still loads, but `verify` reports a mismatch on the unchanged tree and `diff` shows the junction as `CHANGED`, from `file` to `junction`. Check such a snapshot with `diff`, then capture it again. Trees without junctions, and every tree on Linux and macOS, keep their fingerprints.

### Named pipes, sockets and devices

The tree outputs still show a named pipe, a Unix or Windows AF_UNIX socket, or a device as a file, as in `v0.3.3`. The structural commands refuse it with `unsupported_node_type`, naming its path and type: `fingerprint` and `snapshot` exit `1`, `verify` and `diff` exit `5`, and `watch` stops at startup or reports an error. Exclude such an entry with `--ignore`; `.gitignore`, the built-in exclusions, hidden entries, `--depth` and `--dirs-only` exclude it as well. Windows files behind a reparse point, such as app execution aliases and cloud placeholders, are still files.

### `.gitignore` files

A `.gitignore` above 1 MiB, or with a line of 64 KiB or more, now fails the scan with a diagnostic naming the file, and the line when one is too long: exit `1` for the tree output, `fingerprint` and `snapshot`, exit `5` for `verify` and `diff`.

In `v0.3.3`, the pattern matcher stopped at a line of 64 KiB or more without an error, so that line and every rule after it were dropped: the files those rules ignore appeared in the output, while Git ignores them. A large `.gitignore` was also read without limit. A `.gitignore` whose lines are all shorter, and of at most 1 MiB, applies as before.

### File names that are not valid text

A file name that is not valid UTF-8, or a Windows name holding an unpaired UTF-16 surrogate, used to appear in tree JSON with U+FFFD replacement characters and exit `0`, while the text output refused it. Tree JSON now refuses it too, with exit `1`, before writing anything: stdout, the `--output` file and the clipboard stay untouched. Exclude such an entry with `--ignore`, or rename it.

### Configuration and theme files

A `.dirloom.yaml`, a user configuration file or a theme file that is not a regular file, such as a link to `/dev/zero` or a named pipe, is refused before it is read, instead of being read without end or blocking. The 1 MiB limit of these files now holds whatever the file system reports as their size.

### Windows scans

A scan no longer blocks a rename or a removal of the entries it reads. Two outcomes change: a directory that another program holds without sharing is retried for up to 50 ms before the scan fails, and a directory being deleted is reported as missing instead of `permission denied`. A real denial is still reported, and never retried. One Windows limit stays: for the instant Dirloom reads an entry, Windows refuses to rename a directory above it; renaming again succeeds.

### Diagnostic JSON outputs

`config explain`, `preset explain`, `theme list`, `theme explain`, `theme validate` and `theme classify` with `--as json` are now documented as diagnostic outputs, not contracts. Their content is unchanged, and they keep their `schemaVersion`, but they may change in any release, with a CHANGELOG entry. Do not build automation that must keep working across releases on them ([compatibility policy](compatibility-policy.md#5-diagnostic-outputs-d1)).

### Unchanged, now documented

- Ignore patterns made of many `**` segments match with the same results, in time linear in the pattern and the path.
- On Windows, `--output` cannot replace a destination that another process has open: exit `1`, the existing file unchanged. This was already the case in `v0.3.3` ([safe filesystem behavior](../README.md#safe-filesystem-behavior)).

## Stability from v0.4 on

v0.4 publishes a [compatibility and freeze policy](compatibility-policy.md) and a [contract register](contracts/README.md). Every result format above is a candidate for the v0.4 freeze, which takes effect at the release candidate's freeze commit: after it, an incompatible change needs a new contract version, and an incompatible change of what Dirloom observes needs a new fingerprint version. Structural Watch v1 and Watch Event Schema v1 stay experimental. The [CHANGELOG](../CHANGELOG.md) lists every change since `v0.3.3`, alpha by alpha.
