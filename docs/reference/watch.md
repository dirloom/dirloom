# `dirloom watch`

> **EXPERIMENTAL.** [Structural Watch v1](../contracts/structural-watch-v1.md)
> and [Watch Event Schema v1](../contracts/watch-event-v1.md) are excluded
> from the v0.4 RC contract freeze. They may change in any v0.x release,
> always with a new schema version and a CHANGELOG entry, never silently.

Watch answers one question: while you work, how does the structure of a
directory change?

It observes a directory, and each time the structure Dirloom sees changes,
it reports the changes between the last reported state and the new one as
one numbered **generation**: the same changes as `dirloom diff` between the
two states. It does not hash file contents, so editing a file reports
nothing. The design decisions are in
[ADR 0008](../adr/0008-experimental-watch.md).

## Syntax

```text
dirloom watch [directory] [--root <directory>] [--format text|ndjson]
              [--no-moves] [--debounce <duration>] [--poll <interval>]
              [--preset <name>] [--depth <n>] [--dirs-only] [--hidden]
              [--ignore <pattern>]... [--no-default-ignore] [--no-gitignore]
              [--config <path>] [--no-user-config] [--no-config]
```

```bash
dirloom watch
dirloom watch ./src --depth 3
dirloom watch --format ndjson
dirloom watch --no-moves
dirloom watch --debounce 500ms
dirloom watch --poll 2s
dirloom watch --preset monorepo --ignore dist --no-gitignore
```

| Operand or flag | Meaning |
| --- | --- |
| `[directory]`, `--root` | the directory to watch, not both; default the current directory |
| `--format` | `text` (default) or `ndjson`: one JSON event per line |
| `--no-moves` | disable move detection: report each move as `REMOVED` plus `ADDED` (move detection `none`) |
| `--debounce <duration>` | quiet time after a notification before observing, 10 ms to 10 s, default 200 ms; native mode only |
| `--poll <interval>` | observe every interval, 100 ms to 1 h, instead of using notifications |
| capture flags, `--preset`, configuration | exactly as [`dirloom fingerprint`](fingerprint.md), resolved once at startup |

Durations use Go syntax: `250ms`, `2s`, `1m30s`. The reconciliation interval
is not a flag.

### What is not accepted

These are usage errors, with exit 2, before any output and whatever
`--format`:

- `--format json`: the message lists the valid values and suggests
  `ndjson`, since a watch streams events; any other format;
- `--color`, `--icons`, `--theme`, `--style`: a watch has no presentation
  options;
- `--output`, `--copy`: a watch streams to stdout only;
- a `git:`, `snapshot:` or `live:` operand, two directories, or a directory
  with `--root`;
- a duration out of its range or zero, and `--poll` with `--debounce`;
- an invalid configuration, or one that cannot be read.

A refused flag is named with its reason, for example `--color is not
supported by watch: its output has no presentation options`, and is not
listed in `dirloom watch --help`.

### A command, not a directory

`watch` is a command name. `dirloom watch` runs this command; it no longer
inspects a directory named `watch` in the current directory. To inspect
such a directory, give it a path:

```bash
dirloom ./watch
```

## What a watch reports

A watch reports **structural transitions between observed states**, not
filesystem operations. Each observation scans the whole tree with the
capture rules of `dirloom fingerprint`; filesystem notifications only
decide *when* to observe, and their content (an entry name, an operation)
is never reported. A generation lists the changes between the last
reported state and the new observation, in the vocabulary of
[Diff Result Schema v3](../contracts/diff-result-v3.md): `ADDED`, `REMOVED`,
`CHANGED` (kind or link target) and `MOVED` (with move detection
`structural-v1`, a subtree or a uniquely named entry relocated). A file
renamed in place is a removal and an addition, as `dirloom diff` reports it.

A watch does not report:

- file contents, sizes, timestamps or permissions: a content edit, `chmod`
  or `touch` reports nothing;
- the operations themselves, their order or their number: a directory
  created and removed between two observations may report nothing, and
  several operations are reported as their net effect;
- what the capture excludes: ignored, default-ignored, hidden entries
  (unless `--hidden`), files with `--dirs-only`, entries beyond `--depth`;
- the content of a link or a Windows junction: they are never entered.

The first state is not reported as additions. `dirloom snapshot` with the
same options gives it, and the `watch.started` fingerprint tells whether
they match.

A named pipe, a socket or a device in the observed view cannot be part of
a state. Dirloom refuses it with `unsupported_node_type`, as `dirloom
fingerprint` does.

- **Present at startup:** the startup observation fails, and the watch stops
  with a fatal `watch.error` (exit 5).
- **Appearing later:** each observation reports a non-fatal `watch.error`,
  and no new state is published. The last state stays current until the
  entry is removed or excluded with `--ignore`.

Up to `v0.4.0-a8`, such an entry was observed as a file.

### Native notifications and reconciliation

By default a watch uses the platform's notifications: inotify on Linux,
`ReadDirectoryChangesW` on Windows, kqueue on macOS. After a notification,
it waits for the debounce (200 ms of quiet, at most 10 debounces under
continuous activity) and observes. A notification that is not structural
(a content write) does not cause an observation.

Notifications can be lost (a kernel queue overflow, a registration that
fails, a filesystem that does not deliver them). A native watch therefore
also observes when none has succeeded for **60 s**: the reconciliation. It
publishes nothing when the state is unchanged; a difference that no
notification explains is reported inside a rescan with reason
`reconciliation`. So a change is reported within the debounce when the
backend delivers its notification, and at the latest one reconciliation
interval later when it does not.

### Polling

`--poll <interval>` uses no notification: it observes every interval. It
holds no handle on the tree, works on every filesystem, and costs more: on
an idle 10,000-entry tree, `--poll 2s` used 1.5% to 3.8% of one core, a
native watch about 0.1% (see the
[v0.4-a8 benchmarks](../benchmarks/v0.4-a8.md)). Use it where native mode
refuses the filesystem or cannot keep up with its limits.

## Text output

The text output is for people; it is not a contract. A watch of a
directory where a link is retargeted, a file removed, files added and a
directory moved, then one more removal, and Ctrl+C:

```text
Watching structure (experimental, native): 9 nodes, dlm:v1:sha256:f89b3f9ff72bf58dec9d2e7db07f7334dce0309b230131ee51ceca746c0e0362
#1  ~ current (target: docs -> src)
#1  - docs/guide.md
#1  + notes\tdraft.md
#1  > shared/auth -> src/auth (subtree, 3 nodes)
#1  + src/main.go
#1  = 10 nodes, dlm:v1:sha256:bdc3cbdbeee5198e6ffc825c4b50d324d987d951e17bd42669f18fdbefa4fd78  (+2 -1 ~1 >1)
#2  - src/main.go
#2  = 9 nodes, dlm:v1:sha256:d3a101da4c4b19c34a692f433692819e2d3ad9cde2df794ed033d39eea1917d9  (+0 -1 ~0 >0)
Stopped at #2.
```

Each change is prefixed by its generation and uses the symbols of
`dirloom diff`: `+` added, `-` removed, `~` changed (with what changed),
`>` moved (`from -> path (evidence, nodes)`). The `=` line closes the
generation with the new node count, fingerprint and counts. A rescan shows
as `rescan (overflow)`, `rescan (reconciliation)` or `rescan (error)`, then
`rescan completed`. Diagnostics go to stderr: `Warning: <message> (<code>)`
for an error the watch recovers from, `Error: <message> (<code>)` for the
one that ends it. Control characters in paths and messages are escaped
(`notes\tdraft.md` above is a name with a tab), and the output never
contains ANSI.

## NDJSON output

`--format ndjson` writes [Watch Event Schema v1](../contracts/watch-event-v1.md)
to stdout: one compact JSON object per line, UTF-8, terminated by a line
feed on every platform. Every line starts with `schemaVersion` and `event`.

| `event` | Meaning |
| --- | --- |
| `watch.started` | first line: contracts, Capture Semantics, initial fingerprint and node count |
| `node.added`, `node.removed`, `node.changed`, `node.moved` | one change of a generation |
| `state.changed` | closes and commits a generation: new fingerprint, node count and summary |
| `rescan.started`, `rescan.completed` | the state may be stale in between |
| `watch.error` | a diagnostic, fatal or not |
| `watch.stopped` | last line of a complete stream |

```json
{"schemaVersion":1,"event":"node.moved","generation":1,"path":"src/auth","from":"shared/auth","before":{"kind":"directory"},"after":{"kind":"directory"},"evidence":"subtree","nodes":3}
{"schemaVersion":1,"event":"state.changed","generation":1,"fingerprint":"dlm:v1:sha256:<64 hex>","nodeCount":10,"summary":{"added":2,"removed":1,"changed":1,"moved":1,"total":5}}
```

Under `--format ndjson`, stderr stays empty except for a usage error, a
failed stdout write and an expired output grace period.

With `jq` (use `--unbuffered` so that each event is printed as it arrives):

```bash
# one line per committed generation
dirloom watch --format ndjson | jq --unbuffered -c 'select(.event == "state.changed") | {generation, fingerprint, total: .summary.total}'

# the paths added, as they are committed with their generation
dirloom watch --format ndjson | jq --unbuffered -r 'select(.event == "node.added") | .path'

# rescans and diagnostics only
dirloom watch --format ndjson | jq --unbuffered -c 'select(.event | startswith("rescan.") or . == "watch.error")'
```

### Consuming a stream

Dirloom validates each generation in full before writing it, but writing to
stdout is not atomic: a large generation takes several writes, and the
stream can end in the middle of one. A consumer:

- applies a generation only when it has read its `state.changed`, and
  discards a generation cut short by the end of the stream, including a
  last line without its line feed;
- treats its state as possibly stale between `rescan.started` and
  `rescan.completed`; the reasons are `overflow` (notifications were lost),
  `reconciliation` (a change was found that no notification announced) and
  `error` (an observation failed; the watch retries with a backoff of up to
  5 s);
- treats a stream without `watch.stopped` as incomplete: the process was
  killed, its stdout closed, or its stdout blocked beyond the grace period;
- can check its copy of the state against any `fingerprint` of the stream.

It ignores fields it does not know, treats an unknown diagnostic code or
rescan reason as opaque, and skips lines whose `event` it does not know.

## Filesystems

Native mode classifies the filesystem of the root once, at startup, and
refuses a recognized network or virtual type with
`watch_filesystem_unsupported` (exit 5); the message suggests `--poll`,
which applies no policy:

| Platform | Recognized | Verified on a real system |
| --- | --- | --- |
| Linux | NFS, SMB, SMB2, CIFS, 9p, AFS, Ceph, Coda | NFS (a loopback mount) |
| macOS | `nfs`, `smbfs`, `afpfs`, `webdav` | none |
| Windows | remote drives (`DRIVE_REMOTE`) | `\\localhost\C$` |

The classification covers known cases only. An unrecognized type is
accepted, and so is FUSE (virtiofs reports the FUSE type); a probe that
fails, as on a `subst` drive, means unrecognized and is accepted; mount
points below the root are not checked. On an NFS mount, a change made on
the server side was never notified: only the reconciliation found it. In
every such case the reconciliation bounds how long a change can stay
unreported; `--poll` is the safer choice.

## Platform limits

Native mode has two limits that are documented rather than worked around:

- **Windows:** while a native watch runs, the root's **parent and
  ancestors cannot be renamed** (`Access is denied`): any open directory
  handle locks them. The root and everything inside it can still be renamed
  and removed. `--poll` holds no handle.
- **macOS:** a native watch holds one descriptor per watched directory
  (and per `.gitignore` when `.gitignore` files apply), within the
  process's descriptor limit: about 10,200 directories under a
  `kern.maxfilesperproc` of 10,240, as on the macOS CI runner. Beyond, the
  watch ends with
  `watch_limit_reached`.

### System limits

A resource the backend cannot allocate ends the watch with
`watch_limit_reached` (exit 5), with a message that names the setting:

| Platform | Limit | Raise it |
| --- | --- | --- |
| Linux | one inotify watch per watched directory, `fs.inotify.max_user_watches` | `sudo sysctl fs.inotify.max_user_watches=524288`, persisted in `/etc/sysctl.d/` |
| Linux | one inotify instance per watch, `fs.inotify.max_user_instances` | `sudo sysctl fs.inotify.max_user_instances=1024` |
| macOS | one descriptor per watched directory, `kern.maxfilesperproc` and `ulimit -n` | `sudo sysctl kern.maxfilesperproc=<n>` and a larger `ulimit -n` |
| Windows | handles and memory of the process | rarely reached: one recursive handle serves the whole tree |

A narrower capture (`--depth`, `--ignore`; not `--dirs-only`, which keeps
every directory) or `--poll` avoids the limit.

### Editor saves

Editors often save through a temporary file and a rename, or through a
backup. Such a save may be observed half done, so a watch can publish
intermediate generations (a temporary file added, then removed); once the
save is over, the net change is none and the fingerprint is the one before.
This was tested with scripted saves (temporary file, backup, in place), not
with real editors.

### Not verified

These were not verified on a real system: real editors (only scripted
saves), a Windows console close, logoff or shutdown, real SMB, 9p,
`\\wsl$` and virtiofs mounts, and Windows volume mount points. The Windows
hidden-attribute rules were verified after the platform study: a change of
the hidden attribute is reported when hidden entries are excluded, and
content writes cause no observation on any backend.

## Termination and exit codes

Ctrl+C, `CTRL_BREAK_EVENT` on Windows, `SIGINT`, and `SIGTERM` (which only
`watch` handles) end the watch: the observation in progress is abandoned,
the backend is closed, `watch.stopped` (`interrupted`) is written, and the
exit status is 0. A Windows console close, logoff or shutdown is handled
the same way when Windows leaves the time to.

| Outcome | stdout (`ndjson`) | stderr | Exit |
| --- | --- | --- | ---: |
| controlled interruption | a complete stream ending with `watch.stopped` (`interrupted`) | empty | 0 |
| usage error | empty | `Error: …` | 2 |
| fatal error (root lost, backend, limit, filesystem refused); startup failure | a complete stream ending with a fatal `watch.error` and `watch.stopped` (`error`) | empty | 5 |
| interruption or fatal error while stdout is blocked beyond 2 s | an incomplete stream, without `watch.stopped` | best-effort `Error: …` | 5 |
| stdout write failure | an incomplete stream | `Error: …` | 5, or `SIGPIPE` on Unix |
| broken internal invariant | a complete stream ending with a fatal `watch.error` and `watch.stopped` | empty | 6 |

A root that is missing, not a directory or unreadable at startup is a
startup failure: two lines, `watch.error` then `watch.stopped`, without
`generation`. A root removed, renamed or replaced during the run ends it
with `watch_root_unavailable`. On Linux and macOS, a renamed ancestor of
the root is found at the next observation, at the latest one
reconciliation interval later; on Windows it is refused (see
[Platform limits](#platform-limits)). There is no exit 1, because a watch is
not a check, and no exit 3 or 4, because it reads no snapshot. In text
mode, the exit statuses are the same and every error goes to stderr.

When a consumer stops reading, stdout fills up and the watch waits: it does
not queue events, it observes again once the consumer reads, and the next
generation covers everything that changed meanwhile. An interruption that
arrives while stdout is blocked waits at most 2 s (the output grace
period), then the process exits 5 without `watch.stopped`.

### Scripts

POSIX:

```bash
set +e
dirloom watch --format ndjson > events.ndjson
code=$?
set -e

case "$code" in
  0) echo "interrupted" ;;
  2) echo "usage error" ;;
  5) echo "the watch ended on an error" ;;
  6) echo "internal error" ;;
esac
```

PowerShell 7:

```powershell
$watch = Start-Process dirloom -ArgumentList 'watch --format ndjson' -NoNewWindow -PassThru -RedirectStandardOutput events.ndjson
try {
    $watch.WaitForExit()
} finally {
    $watch.WaitForExit()
    switch ($watch.ExitCode) {
        0 { Write-Host "interrupted" }
        2 { Write-Host "usage error" }
        5 { Write-Host "the watch ended on an error" }
        6 { Write-Host "internal error" }
    }
}
```

`$watch.ExitCode` uses the same numbers, with two PowerShell rules:

- Let the watch write the stream itself, through `Start-Process
  -RedirectStandardOutput`. When PowerShell carries the stream, through `>`
  or a pipeline, Ctrl+C makes PowerShell terminate the watch: the exit
  status it reports (`-1`, or the previous `$LASTEXITCODE`) is not the
  watch's, and the stream can end without `watch.stopped`. Written by the
  watch, the file can also be read while the watch runs.
- Ctrl+C also stops the script that runs the watch: only its `finally`
  blocks still run, and what they write to the pipeline is discarded. Wait
  for the watch to end there, read `$watch.ExitCode`, and report with
  `Write-Host`.

## Privacy

The stream carries paths relative to the root and the Capture Semantics,
including the `--ignore` patterns you gave. It never carries the root's
absolute path, and diagnostics never name the root, the home directory or
a temporary directory. The fingerprint describes the observed structure,
not file contents.

## Performance

On a 10,000-entry tree, a directory created is reported 22 to 68 ms after
the debounce; a full observation of 100,000 entries takes 0.2 to 0.4 s; an
idle native watch uses about 0.1% of one core, mostly for the
reconciliation. Details, and the other platforms' figures, are in the
[v0.4-a8 benchmarks](../benchmarks/v0.4-a8.md).
