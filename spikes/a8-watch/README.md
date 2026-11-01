# v0.4.0-a8 LOT 0 spike (temporary)

Throwaway experiments for the platform facts F1–F20 of
`.plans/v0.4.0-a8-implementation-plan.md` §2.1. This is a separate Go module,
so Dirloom's `go.mod` never depends on `fsnotify`. It is deleted, with
`.github/workflows/a8-lot0.yml`, in the commit that closes LOT 0; only the
findings recorded in the plan remain.

- `notify/`: prototype backends on `golang.org/x/sys` v0.47.0 (inotify,
  kqueue, `ReadDirectoryChangesW`), an `fsnotify` v1.10.1 adapter, filesystem
  classification and handle counting.
- `facts/`: one test per fact. Tests fail only on harness errors; each finding
  is logged as `EVIDENCE <fact> <os> <verdict>: <detail>` and appended to
  `$A8_EVIDENCE` (JSON lines).
- `cmd/helper`: child process for F9 (signals) and F15 (blocked stdout).
- `cmd/summary`: turns the evidence file into a Markdown table.

```bash
cd spikes/a8-watch
GOTOOLCHAIN=go1.26.10 A8_EVIDENCE=/tmp/evidence.jsonl go test -v -count=1 -timeout 70m ./facts/
go run ./cmd/summary /tmp/evidence.jsonl
```

`A8_F19_DIRS` shrinks the F19/F20 tree (default 10,000 directories × 10
files); `A8_SUDO=1` enables the Linux sub-mount fact; the workflow sets
`A8_NFS_MOUNT`, `A8_NFS_EXPORT` and `A8_LOW_WATCHES`.
