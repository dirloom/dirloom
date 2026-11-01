# Structural Watch v1

> **Status:** experimental normative contract for Dirloom `v0.4.0-a8`; excluded from the v0.4 RC contract freeze ([Versioning](#versioning))
>
> **Related:** [Watch Event Schema v1](watch-event-v1.md), [Snapshot Schema v1](snapshot-schema-v1.md) (Capture Semantics v1), [Identity Projection v1](identity-projection-v1.md), [Diff Result Schema v3](diff-result-v3.md), [Move Detection structural-v1](move-detection-structural-v1.md), [ADR 0008](../adr/0008-experimental-watch.md)

Structural Watch reports how the structure of a local directory changes while `dirloom watch` runs. Native filesystem notifications, a periodic reconciliation, or a polling timer only decide **when** to look again. What is reported always comes from a full observation of the root, compared with the last reported state through the same scanner, Capture Semantics v1, Identity Projection v1, Fingerprint v1, comparison engine and Move Detection `structural-v1` as `dirloom fingerprint` and `dirloom diff`. Watch reports **transitions between observed states, not filesystem operations**. It reads the filesystem only: it never runs Git, never opens a network connection, and never writes a file.

The machine output is [Watch Event Schema v1](watch-event-v1.md).

## Request

```text
dirloom watch [<directory>] [--root <directory>] [--preset <name>] [--depth <n>] [--dirs-only]
              [--hidden] [--ignore <pattern>]... [--no-default-ignore] [--no-gitignore]
              [--no-moves] [--debounce <duration>] [--poll <interval>] [--format text|ndjson]
```

| Operand or flag | Meaning |
| --- | --- |
| `<directory>`, `--root` | the root; one or the other, not both; default the current directory |
| `--preset`, `--depth`, `--dirs-only`, `--hidden`, `--ignore`, `--no-default-ignore`, `--no-gitignore` | Capture Semantics v1, with the configuration file, exactly as `dirloom fingerprint` resolves them |
| `--no-moves` | Move Detection `none` instead of `structural-v1` |
| `--debounce <duration>` | quiet time before an observation, native mode only; Go duration syntax, from 10 ms to 10 s; default 200 ms |
| `--poll <interval>` | the polling backend, one observation per interval; Go duration syntax, from 100 ms to 1 h |
| `--format` | `text` (default, for people, not a contract) or `ndjson` ([Watch Event Schema v1](watch-event-v1.md)) |

Usage errors (exit 2, stderr, before any output, whatever `--format`): two directory operands or an operand with `--root`; a duration outside its range or not in Go duration syntax; `--poll` together with `--debounce`; `--format json` (the message suggests `ndjson`) or any other format; `--color`, `--icons`, `--theme`, `--output`, `--copy`; a `git:`, `snapshot:` or `live:` operand; an invalid configuration, as for `dirloom fingerprint`. No filesystem handle is opened before the request is valid.

Capture Semantics v1 and the configuration are resolved **once, at startup**; a configuration change takes effect at the next start. The `.gitignore` files below the root belong to the observed state: when `.gitignore` files apply, every observation re-reads them. The reconciliation interval is not configurable in v1.

## Observation

An **observation** scans the whole root with the resolved Capture Semantics, exactly as `dirloom fingerprint` does, and yields its Identity Projection v1 records, its Fingerprint v1 and its node count (root included). It returns no partial result: a scan that fails fails the observation.

The **state** is the last observation reported. Its **generation** is 0 for the first observation and grows by one with each reported change.

When an observation's fingerprint equals the state's, nothing is reported. Otherwise the state and the observation are compared by the comparison engine of [Diff Result Schema v3](diff-result-v3.md) under the requested move detection, the result is validated, and it is reported as **generation** *g* + 1: its node events, then `state.changed` ([Watch Event Schema v1](watch-event-v1.md)). The observation becomes the state.

A notification never stands for a change: the content of a notification (an entry name, an operation) is never trusted and never reported. It only causes an observation.

## Startup

1. The request is validated and Capture Semantics v1 resolved.
2. The absolute root is resolved, following a root link as the scanner does, and its **identity** is captured at once: on Windows the volume serial number and file id read through a handle, on Unix (device, inode). A stored file description compared later by path (as `os.SameFile` does on Windows) is not an identity.
3. Native mode: the [filesystem policy](#filesystem-policy) is applied to the root.
4. The backend starts with its [control registrations](#watch-sets). On Windows it also opens the recursive root handle and issues its first read; on Linux and macOS it registers the root.
5. The first observation is made. A root that is missing, not a directory, or unreadable is fatal at once (`watch_root_unavailable`). Any other failure is retried on the [error backoff](#timing) for at most 5 s, then it is fatal with its own code.
6. Linux and macOS: the structural watch set of this state is registered, and one **verification observation** is scheduled.
7. `watch.started` is published with generation 0, and, in native mode, the reconciliation timer starts.

Registrations made before the first observation (Windows, and the root everywhere) and a verification observation after the per-directory registrations (Linux, macOS) leave no window in which a change is neither observed nor signalled by a working backend. Changes found by the verification observation are an ordinary generation. The initial state is never reported as `node.added` events.

## Cycle

A cycle is triggered by a relevant signal, an overflow, the retry timer, the reconciliation timer (native mode) or the poll timer (polling mode):

```text
wait for a trigger, a fatal signal, or cancellation
  cancellation                 → stop (interrupted)
  fatal signal                 → stop (error)
  overflow and no open rescan  → rescan.started (overflow)
  native, signal-triggered     → wait for the debounce (below)
clear the pending signal and overflow
the root's identity differs, or the root is gone → fatal watch_root_unavailable
observe the whole root
  failure → watch.error (fatal: false), at most once per code per open rescan;
            rescan.started (error) if no rescan is open;
            retry on the next relevant signal and on the error backoff; end of cycle
reset the error backoff; restart the reconciliation timer
fingerprint differs from the state's:
  compare and validate (a validation failure is fatal internal_invariant_failure)
  reconciliation-triggered, no open rescan, and no relevant signal from the start of
  the observation until one debounce after its end → rescan.started (reconciliation)
  publish generation g + 1; the observation becomes the state
per-directory backends: reconcile the registrations with the structural watch set
  a registration added or dropped, or refused transiently → schedule a verification observation
  a registration refused for a resource limit → fatal watch_limit_reached
an open rescan, and nothing pending → rescan.completed
```

A generation is published only after it is complete and validated. Cancellation abandons an observation in progress and publishes nothing for it.

## Reconciliation

In native mode, an observation is made whenever none has succeeded for the **reconciliation interval** (60 s in this version). The timer runs only while no cycle, retry or rescan is pending, and restarts after each successful observation. A reconciliation that finds the state's fingerprint publishes nothing. A reconciliation that finds a difference while no relevant signal arrived from the start of its observation until one debounce after its end reveals a missed notification: the difference is published as an ordinary generation inside a rescan with reason `reconciliation`. A difference that a signal explains is an ordinary generation.

Polling mode has no reconciliation: every poll is a full observation.

## Watch sets

The **structural watch set** of a state is the set of directories its observation entered: the root, unless the depth limit is 0, and every directory node of depth *d* (number of path components) below the depth limit, all of them when the depth is unlimited. Ignored subtrees, default-ignored directories, hidden directories when hidden entries are excluded, links and junctions are never in it. It is empty with `--depth 0`.

The **control watch set** detects the loss of the root at every depth, `--depth 0` included. It never produces a structural event, only the fatal root condition:

| Platform | Control registration |
| --- | --- |
| Windows | a non-recursive handle on the root's parent, keeping only notifications about the root's name; none for a drive root, which cannot be removed or renamed |
| Linux | the root's inotify watch with `IN_DELETE_SELF` and `IN_MOVE_SELF` (alone at `--depth 0`) |
| macOS | the root's descriptor with `NOTE_DELETE`, `NOTE_RENAME` and `NOTE_REVOKE` (alone at `--depth 0`) |

On Linux and macOS each member of the structural watch set is registered once, keyed by its canonical path and by the (device, inode) it had when registered. A registration is dropped, and a verification observation scheduled:

- on the backend's own removal notice (`IN_IGNORED`, `NOTE_DELETE`, `NOTE_RENAME`, `NOTE_REVOKE`). Inode numbers can be reused at once, so an identity check alone cannot see a directory removed and recreated under the same path;
- after an observation, when its path is no longer in the structural watch set (a moved directory keeps its inotify watch, now at its new path, with no removal notice), or when the directory now at that path has another (device, inode) (a filesystem mounted over a registered directory hides its changes).

The path is then registered again if it is in the structural watch set.

A rename of an ancestor of the root does not reach the control set. On Linux and macOS, the identity check of the next observation, at the latest one reconciliation interval later, makes it fatal. On Windows the rename is refused while a native watch runs ([Backends](#backends)).

## Relevance

A notification is relevant, and causes an observation, unless the backend proves that it cannot change the observed structure:

| Notification | Relevant when |
| --- | --- |
| entry created, deleted, renamed or moved | its parent directory is in the structural watch set, or the rename cannot be resolved |
| content written, size, timestamps | the entry is named `.gitignore` in a directory of the structural watch set and `.gitignore` files apply (case-insensitive on Windows and macOS) |
| attributes (Windows) | hidden entries are excluded and the entry's hidden attribute may disagree with its membership in the state; one `Lstat` of the entry may rule that out |
| a directory changed, without entry names (kqueue `NOTE_WRITE`) | always |
| removal or move of a registered directory | always |
| control registration: root removed, renamed or replaced | always (fatal) |
| overflow | always (rescan) |
| a path that cannot be decoded or placed, a Windows short name | always |

Dropping a relevant notification is a defect; an unneeded observation is only a cost. A content-only edit never produces a structural event; on all three native backends it normally causes no observation either.

## Debounce

Relevant signals set one pending flag. In native mode, a signal-triggered observation starts when no relevant signal has arrived for the **debounce** (`--debounce`, 200 ms by default), or at the latest 10 × debounce after the first pending signal when activity never pauses. A signal that arrives during an observation sets the flag again, so another cycle follows.

Intermediate states between two observations are never reported. After stabilization, an editor save through a temporary file leaves no net structural change; the intermediate generations it may cause depend on the editor and on timing. A rename split across two observations is reported as `node.removed`, then `node.added` in a later generation; move detection applies within one generation only.

## Timing

| Parameter | Value |
| --- | --- |
| debounce | `--debounce`, 200 ms by default, from 10 ms to 10 s |
| maximum delay under continuous activity | 10 × debounce |
| reconciliation interval (native) | 60 s |
| error backoff | the debounce (200 ms in polling mode), doubled after each consecutive failure, at most 5 s |
| startup retries | at most 5 s |
| output grace period | 2 s |
| poll interval | `--poll`, from 100 ms to 1 h |
| native read buffers | 64 KiB (Windows, Linux) |

## Rescans

A **rescan** marks a period in which the state may be stale:

| Reason | Opens when |
| --- | --- |
| `overflow` | the backend reports lost notifications (a full inotify queue, a `ReadDirectoryChangesW` overflow) or a registration fails transiently |
| `reconciliation` | a reconciliation finds a difference that no signal explains |
| `error` | an observation fails and no rescan is open |

A rescan closes with `rescan.completed` after the first cycle that observes successfully and leaves nothing pending. Rescans never nest: an overflow, a failure or a reconciliation difference during an open rescan keeps it open. Generations published during a rescan are ordinary generations.

Outside a rescan, the state is the last successful observation. Inside one, it may be stale.

After the first state, an observation failure (an entry vanished during the scan, permission denied below the root, a canonical collision) is never fatal: the state is kept, `watch.error` (`fatal: false`) is published once per code within the open rescan, and the observation is retried.

## Fatal conditions

| Code | Condition |
| --- | --- |
| `watch_root_unavailable` | the root is missing, not a directory or unreadable at startup; or it was removed, renamed or replaced during the run (control registrations, identity check at each observation) |
| `watch_limit_reached` | a resource the backend requires cannot be allocated: an inotify instance (`EMFILE`, `fs.inotify.max_user_instances`) or watch (`ENOSPC`, `fs.inotify.max_user_watches`); a kqueue or descriptor (`EMFILE`, `ENFILE`; on macOS one descriptor per structural directory within `kern.maxfilesperproc`); a Windows handle or buffer. The message names the setting and suggests `--poll` or a narrower capture |
| `watch_backend_failure` | any other backend failure (a handle invalidated, a read error) |
| `watch_filesystem_unsupported` | native mode on a recognized network or virtual filesystem ([Filesystem policy](#filesystem-policy)); the message suggests `--poll` |
| an observation code | the startup observation still fails after its retries |
| `internal_invariant_failure` | a generation fails validation, or the engine breaks one of its invariants (`watch_internal_failure` when no typed code exists) |

A fatal condition publishes `watch.error` (`fatal: true`), then `watch.stopped` (`error`), releases every handle and ends the process with exit 5, or 6 for an internal failure, within the output grace period ([Termination](#termination)). There is no automatic fallback to another backend.

## Termination

- **Controlled interruption.** Ctrl+C and `CTRL_BREAK_EVENT` (`os.Interrupt`), `SIGINT`, and `SIGTERM` on Unix through a handler installed by `watch` only. An observation in progress is abandoned, the backend is closed (descriptors, kqueue, Windows handles after their reads are cancelled), its goroutines end, `watch.stopped` (`interrupted`) is published, and the process exits 0.
- **Blocked stdout.** All output goes through one writer. A blocking write to a pipe cannot be interrupted portably: when an interruption or a fatal condition arrives while a write is blocked, `watch` waits at most the output grace period; then it writes a best-effort diagnostic to stderr and exits 5 without `watch.stopped`. The process always ends; the stream it leaves is incomplete.
- **Closed stdout.** Nothing more is written. On Unix the default `SIGPIPE` behavior applies; elsewhere `watch` exits 5 with a stderr diagnostic.
- **Windows console close, logoff and shutdown** (delivered as `SIGTERM`): handled like an interruption, on a best-effort basis only; Windows may end the process before `watch.stopped` is written.
- Stdin is never read.

## Publication

A generation is computed and validated in full before any of it is handed to the writer: an unvalidated change is never published. The writer flushes after each generation and each control event. Writing to stdout is **not atomic**: a generation larger than the pipe buffer may take several writes, and a write may fail after transferring part of its data. [Watch Event Schema v1](watch-event-v1.md#consuming-a-stream) states how a consumer commits generations.

## Bounds

- Backend goroutines never block on the engine: they only set the pending, overflow or fatal flag and wake it, and they keep draining the kernel's queues while the engine waits on the writer.
- The engine holds at most the state, the current observation, the two watch sets, the registrations and one encoded generation in flight. Memory grows with the size of the tree, never with the event rate or the consumer's speed.
- A slow consumer delays the next observation; the next generation covers everything that changed since the state. There is no queue of events or notifications.
- A generation has as many node events as the equivalent `dirloom diff` has changes (for example one `node.removed` per removed path).

## Backends

| Backend | Structural registrations | Signals | Overflow |
| --- | --- | --- | --- |
| Windows (`ReadDirectoryChangesW`) | one recursive handle on the root, opened with every share mode and backup semantics, overlapped reads issued again at once after each completion; filters: file name, directory name, last write, and attributes when hidden entries are excluded | relative paths, checked against the structural watch set | a 0-byte completion or `ERROR_NOTIFY_ENUM_DIR` |
| Linux (inotify) | one watch per structural directory: `IN_CREATE`, `IN_DELETE`, `IN_MOVED_FROM`, `IN_MOVED_TO`, `IN_DELETE_SELF`, `IN_MOVE_SELF`, `IN_CLOSE_WRITE`, with `IN_ONLYDIR`, `IN_DONT_FOLLOW`, `IN_EXCL_UNLINK` | watch → directory; names checked only for `.gitignore` | `IN_Q_OVERFLOW` |
| macOS (kqueue) | one `O_EVTONLY` descriptor per structural directory, `EVFILT_VNODE` with `NOTE_WRITE`, `NOTE_DELETE`, `NOTE_RENAME`, `NOTE_REVOKE`, `EV_CLEAR`; one per `.gitignore` when `.gitignore` files apply | descriptor → directory or `.gitignore` | none (events coalesce per descriptor) |
| polling (`--poll`) | none | one observation per interval | none |

No handle or descriptor opened below the root follows a link. Every notification record is parsed with bounds checks.

Native mode has two platform limits, documented rather than worked around:

- **Windows:** any open directory handle prevents renaming the root's **parent and ancestors** (`Access is denied`). The root and everything inside it can be renamed and removed. `--poll` holds no handle.
- **macOS:** one descriptor per structural directory, within `kern.maxfilesperproc` (10,240 on the macOS CI runner): there, about 10,200 structural directories, less the process's other descriptors. Beyond, `watch_limit_reached`.

## Filesystem policy

In native mode, the filesystem that holds the root is classified once, at startup. A **recognized** network or virtual type is refused with `watch_filesystem_unsupported`:

| Platform | Probe | Recognized |
| --- | --- | --- |
| Linux | `statfs` type | NFS, SMB, SMB2, CIFS, 9p, AFS, Ceph, Coda |
| macOS | `statfs` `f_fstypename` | `nfs`, `smbfs`, `afpfs`, `webdav` |
| Windows | `GetDriveType` of the root's volume | `DRIVE_REMOTE` |

This classifies known cases; it is not exhaustive:

- an unrecognized type is accepted, and so is FUSE (virtiofs reports the FUSE type);
- a probe that fails (for example on a `subst` drive) means unrecognized: accepted, never fatal;
- mount points below the root are not checked;
- `GetDriveType` does not describe every Windows storage arrangement.

In every one of these cases, the reconciliation bounds how long a missed change can stay unreported. Polling mode applies no policy.

## Guarantees

- **Synchronization.** Outside a rescan, the state is the last successful observation. Every rescan ends with `rescan.completed` or with the end of the stream.
- **Convergence.** When the tree stays unchanged and observations succeed, the state's fingerprint equals `dirloom fingerprint` of the root with the same Capture Semantics at the latest one reconciliation interval plus one observation after the last change in native mode, or one poll interval plus one observation in polling mode; usually within the debounce, when the backend delivers the notification.
- **Diff equivalence.** The node events of generation *g* are the `changes` of Diff Result Schema v3 between snapshots of states *g* − 1 and *g*, with the same Capture Semantics and move detection: the same values in the same order, with `event` in place of `operation`.
- **Structural equivalence.** For one scenario observed at quiescent checkpoints, every backend and platform reaches the same states (fingerprints, node counts) and the same net changes between consecutive checkpoints. `metadata.backend` and the grouping of changes into generations may differ.
- **Serialization determinism.** A given state and a given generation are serialized to the same bytes on every platform ([Watch Event Schema v1](watch-event-v1.md)).

Not guaranteed: exactly-once delivery; a log of filesystem operations; the order of operations inside a generation; a latency below the reconciliation interval when a backend misses a notification; move detection across two generations; atomic writes to stdout; `watch.stopped` after a blocked stdout or a Windows console close; the classification of every network or virtual filesystem; renaming the root's ancestors on Windows while a native watch runs; native macOS watches beyond the descriptor limit.

## Security and privacy

- Read-only: no file written, no process started, no network.
- Notification records are untrusted: bounds-checked record by record; a name that does not decode only causes an observation, whose paths go through the canonical path rules.
- No handle or descriptor opened below the root follows a link; a directory replaced by a link before its registration fails the registration, which schedules a verification observation.
- Bounded: at most one observation per debounce interval while signals arrive, one per reconciliation interval when idle, registrations limited by the system and refused cleanly, memory linear in the tree.
- Privacy: the stream never reveals the absolute root, the host, the user or the time. It carries the artifact's structural values as they are (an absolute link target pointing outside the root, as Diff Result Schema v3 does) and the options the user gave (`capture.ignore`). Diagnostic messages name paths relative to the root and never disclose another local path, such as the absolute root, the home directory or a temporary directory.

## Complexity

Let E be the entries the capture observes, D the directories of the structural watch set, and C the changes of one generation.

| Step | Cost |
| --- | --- |
| observation | linear in E (scan, projection, fingerprint), as `dirloom fingerprint` |
| generation | the comparison of two record lists (Move Detection `structural-v1` included), then C events |
| registrations | at most D, reconciled after each observation in linear time |
| idle | one observation per reconciliation interval (native) or per poll interval |
| memory | two record sets, the watch sets and one encoded generation |

## Reference outcomes

| Scenario | Expected |
| --- | --- |
| a file, a directory, a nested burst (`mkdir -p a/b/c/d` with files in each) created | `node.added` for every path, within one or more generations |
| a file or a directory tree removed | one `node.removed` per removed path, as `dirloom diff` |
| a file renamed in place; a directory of two or more nodes moved | `node.moved` (`name`, `subtree`); with `--no-moves`, `node.removed` and `node.added` |
| a file replaced by a directory of the same name; a link retargeted | `node.changed` (kind; target) |
| a directory removed and recreated under the same path, slowly or at once | later changes inside it reported |
| a structural directory moved inside the root; a filesystem mounted over a registered directory | the registration reconciled; later changes reported at the new path or inside the mount |
| content-only writes, `chmod`, `touch` | no event; normally no observation |
| editor saves (temporary file and rename, backup and rename, in place) | after stabilization, no net structural change |
| changes inside `.git`, `node_modules`, an ignored directory, or beyond `--depth`; a file added with `--dirs-only` | no event |
| a `.gitignore` gains or loses a pattern; with `--no-gitignore` | `node.removed`, `node.added`; with `--no-gitignore`, a content-only edit |
| hidden entries, with and without `--hidden`; a Windows hidden attribute toggled | added or removed as the capture says |
| `--ignore`, `--preset`, configuration file | the `watch.started` fingerprint equals `dirloom fingerprint` with the same options |
| `--depth 0`: children change; the root removed or renamed | no event; `watch_root_unavailable` |
| overflow | `rescan.started` (`overflow`), any generation, `rescan.completed` |
| a missed notification | the change reported by the next reconciliation inside `rescan.started` (`reconciliation`) … `rescan.completed` |
| a quiet tree for several reconciliation intervals | no output |
| an observation failure (an entry removed during the scan, an unreadable directory, a canonical collision) | one `watch.error` (`fatal: false`), `rescan.started` (`error`), then `rescan.completed` once the cause is gone |
| a resource limit reached | `watch_limit_reached`, exit 5 |
| the root removed, renamed or replaced; an ancestor renamed | `watch_root_unavailable`, exit 5 (an ancestor: at the next observation on Linux and macOS; refused on Windows in native mode) |
| the root missing, or a file, at startup | a startup-failure stream, exit 5 |
| a recognized network or virtual root; the same with `--poll`; an unrecognized type; a `subst` drive | `watch_filesystem_unsupported`; accepted; accepted; accepted |
| Ctrl+C, `CTRL_BREAK_EVENT`, `SIGINT`, `SIGTERM`, also during an observation | `watch.stopped` (`interrupted`), exit 0, no partial generation |
| a consumer that stops reading, then an interruption | exit 5 within the output grace period, no `watch.stopped` |
| a consumer that closes stdout | the process ends (Unix `SIGPIPE`; elsewhere exit 5) |
| a slow consumer during 10,000 changes | memory bounded by the tree; coalesced generations; the final state correct |
| one scenario, native and `--poll`, on Linux, Windows and macOS | the same states and net changes at every quiescent checkpoint |
| a usage error | exit 2, stderr, no stdout, no handle opened |

## Versioning

This contract is Structural Watch v1, **experimental**. It is excluded from the v0.4 RC contract freeze. Until `watch` is declared stable, any part of it may change, but never silently: a change to the observation model, the triggers, the watch sets, the relevance rules, the rescans, the fatal conditions, the termination, the guarantees or the filesystem policy comes with a new version of this contract, the matching [Watch Event Schema](watch-event-v1.md) version when its output changes, and a CHANGELOG entry. Timing values (the debounce default, the maximum delay, the reconciliation interval, the backoff, the grace period) may be tuned within v1, with a CHANGELOG entry. Every other contract is unchanged by this one.
