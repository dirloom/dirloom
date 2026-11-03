# ADR 0008 — Experimental structural watch

> **Status:** proposed for `v0.4.0-a8`, revised after the CTO framing review of 2026-11-01 and accepted the same day; revised again after the LOT 0 review of 2026-11-01 (platform facts, backend choice); contracts written on 2026-11-02 ([Structural Watch v1](../contracts/structural-watch-v1.md), [Watch Event Schema v1](../contracts/watch-event-v1.md)); engine implemented on fakes (LOT 2), `app.Watch` with polling (LOT 3), the Linux backend (LOT 4), 2026-11-02, the Windows (LOT 5) and macOS (LOT 6) backends and the CLI with its NDJSON encoder and goldens (LOT 7), 2026-11-03; the rest of the implementation awaiting an explicit GO
>
> **Date:** 2026-11-01

## Context

`v0.4.0-a7` is released and closed; its user acceptance campaign passed 28 of 28 tests on 2026-11-01. The v0.4 internal roadmap (§11, "Experimental Watch") asks for the last functional increment: expose local structural changes as an event stream, `dirloom watch --format ndjson`, explicitly `EXPERIMENTAL`. Its exit gate asks for an NDJSON structural event stream, a shared canonical model, debounce and coalescing, overflow recovery, filter consistency, graceful shutdown, bounded resource usage, real OS integration tests, and a clearly marked experimental status.

Seven constraints shape the design:

- **One structural truth.** Native notifications are signals, never truth. Every event must come from comparing two Canonical Structural Artifacts observed by the scanner, Capture Semantics v1, Identity Projection v1, Fingerprint v1, the comparison engine and Move Detection `structural-v1` that `fingerprint` and `diff` already use.
- **Notifications are lossy and platform-specific.** inotify watches one directory at a time, can overflow its queue, can run out of watches, and does not report changes made remotely on network filesystems. `ReadDirectoryChangesW` can overflow its buffer and, per its documentation, may report either the short or the long name of an entry. kqueue reports per file descriptor, without names. FSEvents needs cgo, and Dirloom releases are built with `CGO_ENABLED=0`. A notification can be missed without any overflow being reported.
- **No silently wrong state.** Lost notifications must be reported and followed by a full re-observation (roadmap A8.6), and a missed notification must not leave the published state stale indefinitely.
- **Bounded resources.** A fast producer or a slow consumer must not cause unbounded memory, a deadlock, or a corrupt state (A8.9).
- **Few dependencies, but not at any cost.** `golang.org/x/sys` is already a direct dependency; any other module needs a reason (`docs/dependencies.md`), and so does the lasting cost of maintaining three native backends.
- **Experimental, yet a contract.** The stream is versioned and documented, but it must not hold back the freeze of the other v0.4 contracts (A8.1).
- **What the operating system does not guarantee.** Notification delivery is imperfect, writes to stdout are not atomic, and Windows may end a process before its cleanup completes. The contract must separate what Dirloom guarantees (structural transitions, verifiable synchronization, bounded resources) from these limits.

## Decision

### `dirloom watch`, filesystem only

```text
dirloom watch [<directory>] [--root <directory>] [--preset <name>] [--depth <n>] [--dirs-only]
              [--hidden] [--ignore <pattern>]... [--no-default-ignore] [--no-gitignore]
              [--no-moves] [--debounce <duration>] [--poll <interval>] [--format text|ndjson]
```

- `watch` observes the live filesystem with Capture Semantics v1, resolved **once at startup** exactly as `dirloom fingerprint` resolves it (flags, preset, configuration file). A configuration change needs a restart. The `.gitignore` files below the root are part of the observed state: every observation re-reads them, as Capture Semantics v1 already says ("the `.gitignore` files that exist now").
- Git sources, snapshots, presentation flags, `--output` and `--copy` are rejected. `watch` never runs Git and never writes a file.
- The help text, the text output and every `watch.started` line mark the command as experimental.

### Signals trigger observations; observations are the truth

```text
native notifications ─► backend ─► relevance filter ─► dirty flag ─► debounce ─┐
reconciliation timer ──────────────────────────────────────────────────────────┤
                                                                               ▼
     last emitted state ◄── events ◄── comparison ◄────────────── full re-observation
```

Each cycle observes the **whole root** through the same path as `dirloom fingerprint`, projects it, fingerprints it and, when the fingerprint differs from the last emitted state, compares the two with `comparison.CompareRecords` under `structural-v1` (or `none` with `--no-moves`). Equal fingerprints produce no output. A difference produces one **generation**: its node events, then `state.changed`. A full observation of 10,000 entries took 84 ms on the A1 Windows benchmark machine, which a debounced loop can afford. Re-observing only the changed subtrees is out of scope for v1; if the A8 benchmarks require it later, it must give exactly the results of the full observation, proven by an equivalence test, as A7 required for its optional optimization.

### Periodic reconciliation in native mode

Because a backend can miss a notification without reporting an overflow, native mode also runs a full observation whenever no observation has succeeded for the **reconciliation interval** (60 s initially, a measurement base that the A8 benchmarks will adjust). A reconciliation that finds the same fingerprint writes nothing. A reconciliation that finds a difference while no relevant signal arrived from the start of its observation until one debounce after its end reveals a missed notification: the difference is written as an ordinary generation inside a rescan with reason `reconciliation`. A difference that a signal does explain is an ordinary generation. Polling mode needs no reconciliation: each poll is a full observation.

### Relevance: over-signal, never under-signal

A backend may drop a notification only when it can prove the notification cannot change the artifact:

- its parent directory is not in the structural watch set (below); or
- it reports a content or metadata change of an entry that is not a `.gitignore` (when `.gitignore` files apply) and, on Windows with hidden entries excluded, that does not change the entry's hidden attribute.

Anything uncertain is relevant: an unknown path, an unresolved rename, a Windows short name, an undecodable name, a check that fails. Over-signalling costs one observation; under-signalling would break correctness. **The contract is that content-only edits produce no structural event.** Avoiding the observation itself is an optimization that depends on what the backend reports; LOT 0 showed that all three backends allow it (kqueue gives no entry names, but its directory `NOTE_WRITE` stays silent for content edits), so it is tested as such, not promised by the contract.

### Debounce and coalescing

Relevant signals set one dirty flag. A cycle starts after `--debounce` of quiet (default 200 ms), or at the latest 10 × debounce after the first pending signal when activity never pauses. A signal that arrives during a cycle sets the flag again, so another cycle follows. Intermediate states between two observations are never reported: `watch` reports **transitions between observed states, not filesystem operations**. After stabilization, an editor save through a temporary file yields no net structural change; whatever intermediate events a given editor's sequence causes depends on that editor and on timing. A rename split across two cycles is reported as `node.removed` then `node.added`.

### Two watch sets

- The **structural watch set** is what Capture Semantics makes observable: the root and every captured directory the last observation entered. It never includes `.git`, `node_modules`, ignored subtrees, links, or directories at or beyond the depth limit; with `--depth 0` it is empty.
- The **control watch set** is what detecting the loss of the root needs, at every depth, `--depth 0` included: a registration that reports the root's removal, rename or replacement. It produces no structural event, only the fatal root condition. A rename of an ancestor of the root is detected at the next observation (at the latest one reconciliation interval later), when the root's identity is checked; on Windows such a rename is refused while a native watch runs (below).
- The root's identity is captured at startup: on Windows the volume serial number and file index read through a handle; on macOS the device, inode and birth time; on Linux the device, inode, birth time and file handle, which carries the inode generation (inode numbers and coarse birth times repeat when a directory is recreated at once). It is never a stored `FileInfo` compared later with `os.SameFile`, which on Windows reads the id by path at comparison time (LOT 0, F16).

### Native backends on `golang.org/x/sys`

| Platform | Mechanism | Structural registrations | Control registrations |
| --- | --- | --- | --- |
| Windows | `ReadDirectoryChangesW` | one recursive handle on the root, shared for read, write and delete | a non-recursive handle on the root's parent, filtered on the root's name (a drive root cannot be renamed or removed) |
| Linux | inotify | one watch per structural directory: entry creation, deletion and moves, self deletion and move, close-after-write for `.gitignore`; never follows links | the root's watch, with its self events, at every depth |
| macOS | kqueue | one `O_EVTONLY` descriptor per structural directory (`NOTE_WRITE`, `NOTE_DELETE`, `NOTE_RENAME`, `NOTE_REVOKE`), plus one per `.gitignore` when `.gitignore` files apply | the root's descriptor, at every depth |

Per-directory registrations are keyed by path and by the (device, inode) they had when registered. A registration is dropped on the backend's own removal notice, because inode numbers can be reused at once (LOT 0: 200 reuses in 200 fast recreations on Linux); and after each observation when its path is no longer a structural directory (a moved directory keeps its inotify watch, with no removal notice) or when the directory now at that path has another (device, inode) (a filesystem mounted over a registered directory hides its changes). Any change to the registrations is followed by one verification observation, which closes the window between the creation of a directory and its registration (LOT 0: a burst below a new directory is not notified before it is registered). Notification names are never trusted: a registration follows its directory when it moves, and the Windows root handle then names entries under the old root path.

The backends are written directly on `golang.org/x/sys/unix` and `golang.org/x/sys/windows` v0.47.0: no new module. LOT 0 compared them with `fsnotify` v1.10.1 on the three operating systems and the LOT 0 review chose them on every platform. On Windows, one recursive handle replaces one per directory (2 against 10,114 handles for 10,101 directories), and `fsnotify`'s `Add` blocks while events are undelivered. On macOS, one descriptor per directory replaces one per file (`fsnotify` failed with `EMFILE` after 3,697 of 10,101 directories). On Linux both are equivalent.

Two platform limits of native mode are documented rather than worked around:

- **Windows:** any open directory handle, this backend's or `fsnotify`'s, prevents renaming the root's **parent and ancestors** (`Access is denied`). The root and everything inside it can still be renamed and removed. `--poll` holds no handle.
- **macOS:** a native watch holds one descriptor per structural directory, within `kern.maxfilesperproc` (10,240 on the CI runner), so about 10,200 structural directories. Beyond, `watch_limit_reached` names the setting and suggests `--poll` or a narrower capture.

`--poll <interval>` replaces notifications by one observation per interval and produces the same structural results. It serves network and virtual filesystems, and is the test oracle for the native backends. It is never selected automatically: a silent fallback would change latency and cost without the user knowing.

### Filesystem detection: an explicit, testable policy

In native mode, `watch` checks the type of the filesystem that holds the root against a **documented list of recognized network and virtual types** and refuses those with `watch_filesystem_unsupported`, suggesting `--poll`:

| Platform | Probe | Recognized |
| --- | --- | --- |
| Linux | `statfs` type | NFS, SMB, SMB2, CIFS, 9p, AFS, Ceph, Coda |
| macOS | `statfs` `f_fstypename` | `nfs`, `smbfs`, `afpfs`, `webdav` |
| Windows | `GetDriveType` of the root's volume | `DRIVE_REMOTE` |

LOT 0 verified NFS on Linux, where a change made on the server side was never notified while a re-observation listed it, and `DRIVE_REMOTE` on a Windows UNC path; the reference marks the verified entries. This is a classification of known cases, not an exhaustive one:

- an unrecognized type is accepted, and so is FUSE, whose notifications depend on each implementation (virtiofs reports the FUSE type);
- a probe that fails (for example on a `subst` drive) means "unrecognized": accepted, never fatal;
- mount points below the root are not checked;
- `GetDriveType` does not characterize every Windows storage arrangement.

In all of these cases, periodic reconciliation bounds how long a missed change can stay unreported. The reference documents the recognized types and these limits.

### Overflow, errors and rescans

- **Lost notifications** (inotify queue overflow, `ReadDirectoryChangesW` buffer overflow, a registration that fails transiently): `rescan.started` with reason `overflow`, a full re-observation, any difference as an ordinary generation, then `rescan.completed` with the current fingerprint. Because every cycle observes the whole root, recovery needs nothing beyond one cycle.
- **Missed notifications** found by a reconciliation: reason `reconciliation`, as above.
- **A failed observation** (an entry vanished during the scan, permission denied below the root, a canonical collision): not fatal once the first state exists. `watch.error` with `fatal: false`, `rescan.started` with reason `error` unless a rescan is already open, the last good state is kept, and the observation is retried on the next relevant signal and on a backoff that doubles from the debounce up to 5 s. The first success closes the rescan with `rescan.completed`.
- Between `rescan.started` and `rescan.completed`, the last emitted state may be stale; outside a rescan, it is the state of the last successful observation.
- **Fatal** conditions: the root was removed, renamed or replaced (control registrations, and an identity check at every observation); a resource the backend requires cannot be allocated (`watch_limit_reached`); the backend failed; native mode was requested on a recognized network or virtual filesystem; stdout cannot be written; an invariant broke. A fatal condition ends the stream with `watch.error` (`fatal: true`) and `watch.stopped`. There is no automatic fallback.

### Generations are validated transactions, not atomic writes

A generation is computed and validated in full before any of it is written: no unvalidated change is ever published. Writing it to stdout, however, is **not atomic**: a large generation exceeds `PIPE_BUF` and may take several writes, and a write may fail after transferring part of the data. A consumer therefore treats a generation as committed only when it has read its `state.changed`, and discards a generation cut short by the end of the stream.

### Termination

- Startup resolves the configuration and the root, records the root's identity, checks the filesystem type, starts the backend with its control registrations, observes, registers the structural watch set, then writes `watch.started` (generation 0, metadata, Capture Semantics, fingerprint, node count). Changes found by the verification observation are an ordinary generation. The initial state is not emitted as `node.added` events: `dirloom snapshot` already serves that need.
- **Controlled interruption** (Ctrl+C everywhere, `CTRL_BREAK_EVENT` on Windows, `SIGINT` and `SIGTERM` on Unix, through a handler scoped to `watch` so that the other commands keep today's behavior): an observation in progress is abandoned (never a partial generation), the backend handles are closed, `watch.stopped` with reason `interrupted` is written, exit 0.
- **Blocked stdout.** Writes go through one writer goroutine, with at most one encoded generation in flight; the engine never observes again before that write completes. A blocking write to a pipe cannot be interrupted portably. When an interruption or a fatal condition arrives while a write is blocked, `watch` waits at most an **output grace period** (2 s) for the write to complete; if it does not, it exits 5 without `watch.stopped`, after a best-effort stderr diagnostic. The process always ends; the stream it leaves is visibly incomplete.
- **Windows console close, logoff and shutdown** events are handled like an interruption on a **best-effort** basis only: Windows may end the process before `watch.stopped` is written.
- When stdout is closed by the consumer, nothing more is written: on Unix the default `SIGPIPE` behavior applies; elsewhere `watch` exits 5 with a stderr diagnostic, as `diff` does for a failed write.

### Backpressure

Backend goroutines never block on the engine: they only set the dirty or overflow flag and wake it. While the consumer is slow, signals coalesce into the flag, kernel queues keep being drained, and the next generation covers everything since the last emitted state. Memory holds at most two artifacts and their projections, the two watch sets, and one encoded generation in flight: it grows with the tree, never with the event rate or the consumer's speed.

### Watch Event Schema v1 (NDJSON)

`--format ndjson` writes one compact JSON object per line: UTF-8, a line feed on every platform, no HTML escaping, `schemaVersion: 1` on every line, field order fixed by the encoder. Ten events:

| `event` | Meaning |
| --- | --- |
| `watch.started` | First line: metadata (`stability: experimental`, versions, `backend: native \| poll`), Capture Semantics, initial fingerprint and node count, generation 0 |
| `node.added`, `node.removed`, `node.changed`, `node.moved` | One Diff Result Schema v3 change of generation *g*: the same fields with `event` in place of `operation`, in the same order (ascending path) |
| `state.changed` | Closes and commits generation *g*: fingerprint, node count, summary |
| `rescan.started` | Reason `overflow`, `reconciliation` or `error`: the last state may be stale |
| `rescan.completed` | The stream is in sync again: generation, fingerprint, node count |
| `watch.error` | `diagnostic` (`code`, `message`) and `fatal` |
| `watch.stopped` | Last line of a complete stream: reason `interrupted` or `error`, last generation and fingerprint |

The roadmap's `overflow` becomes `rescan.started` with reason `overflow`, and its `error` becomes `watch.error`. `watch.started`, `state.changed` and `watch.stopped` are added so that a consumer can tell a committed generation, a resynchronized stream and a clean end from a truncated stream. No event carries a timestamp or a sequence number.

`--format text` (the default) is for people and is not a contract. `--format json` is refused: a stream is not one document.

Exit codes: `0` after a controlled interruption with `watch.stopped` written; `2` for usage errors (stderr, before any output, even under `--format ndjson`); `5` for fatal observation, backend and output errors, including an output grace period that expired; `6` for broken invariants. There is no exit 1, because `watch` is not a check, and no 3 or 4, because it reads no snapshot.

### Privacy

The stream never reveals the absolute path of the observed root, the host, the user or the wall-clock time. It does emit, as they are, the structural values of the artifact (for example an absolute link target that points outside the root, as Diff Result Schema v3 already does) and the options the user gave (`capture.ignore`). Diagnostic messages name paths relative to the root and must not disclose other local paths (the root, the home directory, temporary directories); tests check it.

### Guarantees and limits

- **Synchronization.** Outside a rescan, the last emitted state is the state of the last successful observation. Every rescan ends with `rescan.completed` or with the end of the stream.
- **Convergence.** If the tree stays unchanged and observations succeed, the last emitted fingerprint equals `dirloom fingerprint` with the same Capture Semantics at the latest one reconciliation interval plus one observation after the last change in native mode, and one poll interval plus one observation in polling mode; usually within the debounce, when the backend delivers the notification.
- **Diff equivalence.** The node events of each generation equal the `changes` of `dirloom diff` between snapshots of the two observed states, with the same Capture Semantics and move detection.
- **Structural equivalence across backends and platforms.** For the same scenario observed at quiescent checkpoints, every backend and platform reaches the same states (fingerprints, node counts) and the same net changes between checkpoints. Metadata that names the backend, and the grouping of changes into generations, may differ: grouping depends on timing.
- **Serialization determinism.** The same state and the same generation are serialized to the same bytes on every platform.
- **Not promised:** exactly-once delivery, a log of filesystem operations, the order of operations within a generation, atomic writes to stdout, a latency below the reconciliation interval when a backend misses a notification, `watch.stopped` after a Windows console close or a blocked stdout, the classification of every network or virtual filesystem, renaming the root's ancestors on Windows while a native watch runs, or native macOS watches beyond the descriptor limit.

### Experimental policy

Watch Event Schema v1 and Structural Watch v1 are excluded from the v0.4 RC contract freeze. Until `watch` is declared stable (v0.7 in the product roadmap), an incompatible change is allowed, but only with a new `schemaVersion` and a CHANGELOG entry. Additive changes follow the Diff Result Schema v3 evolution policy.

## Alternatives considered

- **`fsnotify`.** Mature and widely used, and three native backends have a lasting maintenance cost. LOT 0 measured v1.10.1 against the native prototypes: equivalent on Linux; on Windows one handle per directory (10,114 against 2), seconds of registration, and an `Add` that blocks while events are undelivered; on macOS one descriptor per file, failing with `EMFILE` after 3,697 of 10,101 directories. It also adds a module. Rejected at the LOT 0 review.
- **FSEvents on macOS.** Recursive and efficient, but it needs cgo, while releases are built with `CGO_ENABLED=0` and cross-compiled. Rejected for v1.
- **fanotify on Linux.** Recursive per filesystem, but it needs privileges or recent kernels depending on the mode. Rejected.
- **Polling only.** Simple and portable, but latency and CPU cost grow with the tree. Kept as `--poll`, not as the default.
- **Raw OS events** (create, write, rename as the OS reports them). Non-portable, noisy, and a second structural truth. Rejected.
- **Subtree re-observation.** Cheaper per cycle, but nested `.gitignore` scopes, the depth limit and moves across subtrees make it easy to get wrong. Deferred until measured.
- **Automatic fallback to polling.** Convenient, but it silently changes the guarantees. Rejected for an explicit `--poll`.
- **No periodic reconciliation in native mode** (the first framing). Cheaper when idle, but then convergence holds only when the backend delivers every notification it should, a weaker guarantee. Rejected at the framing review in favor of a low-frequency reconciliation.
- **Treating one `Write` call as an atomic publication.** Not true of stdout. Rejected for validated generations committed by `state.changed`.
- **Non-blocking stdout with write deadlines.** It would make blocked writes interruptible, but setting `O_NONBLOCK` on an inherited pipe affects every process sharing it, and Windows anonymous pipes have no equivalent. Rejected for a bounded output grace period.
- **The initial state as `node.added` events.** Self-contained, but very large for big trees, and `dirloom snapshot` exists. Rejected for v1; possible later as an additive option.
- **Timestamps and sequence numbers.** Convenient, but timestamps break serialization determinism and both duplicate what an ordered pipe and `generation` already give. Rejected for v1; sequence numbers stay possible as an additive field.
- **A JSON document (`--format json`).** Not streamable. Rejected.
- **`SIGTERM` in the global entry point.** Simpler, but it would change how every other command ends on `SIGTERM`. Rejected for a handler scoped to `watch`.
- **Waiting for a removed root to come back.** Friendlier for `rm -rf` and re-clone workflows, but the new directory is another object, possibly with another structure. Rejected: a removed or replaced root ends the stream.

## Consequences

- New contracts, both experimental: Structural Watch v1 (observation model, reconciliation, relevance, debounce, watch sets, filesystem policy, rescans, termination, bounds, guarantees, backends, diagnostics) and Watch Event Schema v1 (NDJSON). Unchanged: Capture Semantics v1, Identity Projection v1, Canonical Identity Encoding v1, Fingerprint v1, Snapshot Schema v1, Verify Result Schema v1, Diff Result Schema v3, Move Detection `structural-v1`, Git Tree Source v1, Structural History v1, History Result Schema v1, and every existing command's output and exit codes.
- New packages: `internal/watch` (engine, generations, rescans, writer, event model, validator) and `internal/watch/backend` (native backends and polling, behind build tags). `internal/app` gains `Watch`; `internal/cli` gains the `watch` command, its NDJSON encoder and its text renderer. Observation, projection, fingerprinting and comparison are reused unchanged.
- No new Go module: `golang.org/x/sys/unix` is used for the first time, its module (v0.47.0) already required; LOT 0 compiled and exercised every call on its OS.
- An idle native watch costs one full observation per reconciliation interval.
- Adding the command means `dirloom watch` no longer inspects a directory named `watch`; `dirloom ./watch` still does. The reference and the CHANGELOG say so, as A7 did for `history`.
- The backend details above rest on platform facts that LOT 0 checked on real systems on 2026-11-01, on the three CI runners and a local Windows machine (implementation plan §2.1, with evidence). Not verified there, and documented as such: real editors (save sequences were simulated), Windows console close, real SMB, 9p, `\\wsl$` and virtiofs mounts, Windows volume mount points, and the narrowing of Windows hidden-attribute notifications.
- Real OS tests become part of CI on Linux, Windows and macOS; a successful compilation is not a proof of behavior. Timing-dependent tests wait on conditions, never on fixed sleeps.
- Known limits are documented rather than hidden: missed notifications reported up to one reconciliation interval late, no operation log, rename pairs split across two cycles, non-atomic stdout, a root given through a link resolved once at startup, ancestor renames detected at the next observation (refused on Windows in native mode), case-only renames as the platform reports them, an incomplete filesystem classification, the macOS descriptor budget, and notification limits that depend on system settings (`fs.inotify.max_user_watches`, `fs.inotify.max_user_instances`, `kern.maxfilesperproc`).
- Framing review decisions (CTO, 2026-11-01): architecture approved; Q2–Q4, Q6, Q8–Q13, Q15–Q17 as recommended; Q1 conditional on the LOT 0 comparison with `fsnotify`; Q5 revised to the explicit detection policy above; Q7 accepted with the control watch set (`--depth 0`); Q14 revised to periodic reconciliation (60 s initially); D0 `go1.26.10` with a check for a newer patch before branching; D1 `x/sys` v0.47.0 if F11 holds on every OS; D2 approved. P1 corrections: convergence, separate equivalence and serialization tests, generations not atomic on stdout, root always watched, blocked consumer. P2 corrections: filesystem policy, best-effort console close, precise privacy. Gate: LOT 0 conditionally authorized once this revision is accepted; LOT 1–10 after the review of the LOT 0 findings (plan §3.2).
- LOT 0 review decisions (CTO, 2026-11-01): the F1–F20 conclusions and their evidence validated; Q1 decided for native backends on `golang.org/x/sys` on the three platforms; amendments A1–A10 accepted (plan §3.4): the Windows ancestor lock documented instead of the F1 fallback, registrations reconciled by path and (device, inode), names never trusted, the macOS descriptor budget, the root identity captured at startup, failed filesystem probes accepted and the recognized list above, both Windows overflow forms, 60 s kept as the initial reconciliation interval. LOT 0 closed; its throwaway spike was removed from the branch.
- LOT 1 (2026-11-02): the two experimental contracts are written from this ADR and the amended plan. They make precise, without changing a decision: timing values may be tuned within Structural Watch v1 with a CHANGELOG entry; `watch_internal_failure` is the fallback code for a broken invariant; a startup failure is exactly two lines.
- LOT 2 (2026-11-02): the engine (`internal/watch`) is implemented and tested on fakes. Two more contract precisions came from it: an interruption before the first state publishes only `watch.stopped` (`interrupted`), and startup retries are silent.
- LOT 3 (2026-11-02): `app.Watch` observes the live root and runs in polling mode; real-filesystem tests prove diff equivalence and convergence on the three platforms. Finding: on ext4 a recreated directory can get back both its inode number and its (coarse) birth time, so the Linux root identity also includes the `name_to_handle_at` file handle, which carries the inode generation.
- LOT 4 (2026-11-02): the Linux inotify backend is implemented; the random-mutation properties pass with native notifications only. A directory the system refuses to register for lack of permission is left to the reconciliation (contract precision).
- LOT 5 (2026-11-03): the Windows `ReadDirectoryChangesW` backend is implemented; the random-mutation properties pass with native notifications only on Linux and Windows. The ancestor lock (A1) is confirmed by a test; bursts of up to 20,000 creations lost no record while reads were re-issued at once (A7).
- LOT 6 (2026-11-03): the macOS kqueue backend is implemented; the random-mutation properties pass with native notifications only on the three platforms. The descriptor limit (A4) is reported as `watch_limit_reached`, tested through a lowered limit. Two conformance fixes to LOT 4–5: Windows handle and memory exhaustion is `watch_limit_reached`, and a directory that cannot be stat-ed for lack of permission is left to the reconciliation.
- LOT 7 (2026-11-03): `dirloom watch` is a command; the NDJSON encoder and the text renderer live in `internal/watch`, so that the Watch Event Schema v1 goldens come from the engine on fakes. Corrections: the engine checks the root before starting the backend, a missing root yields the contract's startup failure (exit 5) rather than the configuration loader's error, and a configuration that cannot be read is a usage error (contract precision). LOT 8–10 await an explicit GO.
