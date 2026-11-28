# Memory and resource limits

> **Status:** guidance for `v0.4.0-rc1`. The figures come from the [v0.4.0-rc1 performance and memory baselines](../benchmarks/v0.4-rc1.md), measured on one Windows 11 machine (24 cores, 16 GiB of RAM) with Go 1.26.10, on generated trees and snapshots. They describe what was measured, not a guarantee.

Snapshots and Git trees are untrusted input, so Dirloom bounds what it reads. The bounds keep a hostile input from running without end, but a workflow at the bounds still needs several GiB of memory. Dirloom does not cap its own memory.

The figures below are the **commit** of the process: the memory it has reserved, which a memory limit on Windows counts (Task Manager shows it as "Commit size"), measured together with the `git` processes Dirloom starts.

## The limits

| Input | Limit | Beyond it |
| --- | --- | --- |
| snapshot file | 256 MiB | `snapshot_limit_exceeded`, exit 3; `dirloom snapshot` refuses to write it, exit 1 |
| snapshot nodes, and elements of any JSON array | 2,000,000 | the same |
| snapshot JSON nesting | 64 levels | the same |
| snapshot `capture.ignore` patterns | 4,096 | the same |
| snapshot `requiredFeatures` entries | 64 | the same |
| Git observation (`diff git:`, `diff snapshot: git:`, `history`) | 2,000,000 nodes and 256 MiB of paths | `git_tree_too_large`, exit 5 |
| `.gitignore` | 1 MiB, and lines under 64 KiB (at most 65,535 bytes) | in a directory: `source_read_failure` (exit 1 for the tree output, `fingerprint` and `snapshot`; exit 5 for `verify` and `diff`); in a Git tree observed with a snapshot's capture that applies `.gitignore`: `git_tree_invalid`, exit 5 |

A live directory has no node limit: `verify` and `diff` observe it whole, and the [watch reference](watch.md) has the limits of `watch`. `dirloom snapshot` of a tree above 2,000,000 nodes fails, because it refuses to write a snapshot that it would refuse to read. The [security policy](../../SECURITY.md#untrusted-input) lists every bound on untrusted input, and [Exit codes and machine codes](exit-codes.md) every code.

## How much memory a workflow needs

Five runs of each workflow, at the limit named in the first column:

| Workflow | Commit, median | Commit, highest run |
| --- | ---: | ---: |
| writing a snapshot of a live tree, 2,000,000 nodes | 1.6 GiB | 1.7 GiB |
| loading one snapshot, 2,000,000 nodes | 1.8 GiB | 1.8 GiB |
| `verify` of a 2,000,000-node snapshot against a live tree | 1.9 GiB | 1.9 GiB |
| `diff snapshot: live:`, 2,000,000 nodes | 1.9 GiB | 2.2 GiB |
| `diff snapshot: git:`, 2,000,000 nodes | 1.8 GiB | 1.9 GiB |
| `diff git: git:`, 2,000,000 nodes | 1.6 GiB | 1.6 GiB |
| the same with `--no-moves` | 1.7 GiB | 1.8 GiB |
| two Git chains just under the 256 MiB path limit, compared | 1.7 GiB | 1.7 GiB |
| two snapshots of 2,000,000 nodes compared | 2.7 GiB | 2.8 GiB |
| two snapshots of 252 MiB with long file names compared, the highest valid workflow measured | 2.5 GiB | 3.4 GiB |
| a hostile snapshot, read then refused | 3.4 GiB | 4.1 GiB |
| the hostile snapshot compared with a valid 2,000,000-node snapshot | 3.8 GiB | 4.1 GiB |

The hostile snapshot is a 250 MiB document made of one JSON object with 17.5 million unique keys: it has no array, so the node limit never applies, and only the 256 MiB byte limit bounds it. In general, comparing two snapshots costs about 1.3 GiB per million nodes, and loading or verifying one about 0.9 GiB per million.

**4.1 GiB is the maximum observed, not a guaranteed ceiling.** It is the highest of the runs measured. The peak of a run depends on when the Go collector starts: the hostile snapshot alone committed 3.3 to 3.4 GiB in 13 of its 15 runs, and 4.0 and 4.1 GiB in the other 2. Another machine, another load or another document can go higher.

**The 8 GiB of the measurements is a budget, not a machine.** Every workflow ran inside an 8 GiB commit budget, a Windows job object around Dirloom and its `git` processes, and none came near it. That shows what fits in 8 GiB of commit. A machine with 8 GiB of RAM was not measured: its operating system and other programs take part of that memory, and it can page, moving memory to disk, before it refuses any.

**Smaller budgets,** at the 2,000,000-node limit, 5 runs each. The budgets are approximate: a run that reached its cap sometimes went up to 88 MiB past it.

| Commit available | Result |
| --- | --- |
| 4 GiB | the long-file-name comparison, the largest valid peak, passed 5 out of 5; the other valid workflows were not run at this budget, and are inferred to pass from their lower peaks; the hostile snapshot compared with a valid one failed 3 runs out of 5 |
| 3 GiB | two snapshots of 2,000,000 nodes compared passed 5 out of 5; the long-file-name comparison failed once; the hostile comparison always failed |
| 2 GiB | loading or writing one snapshot, `diff git: git:` and the two Git chains passed 5 out of 5; `verify` against a live tree and `diff snapshot: git:` 4 out of 5; `diff snapshot: live:` 2 out of 5; comparing two snapshots always failed |

The full tables are in the record: [where it holds and where it breaks](../benchmarks/v0.4-rc1.md#the-8-gib-environment-where-it-holds-and-where-it-breaks).

## When memory runs out

Dirloom has no machine code for running out of memory, and writes no diagnostic and no JSON error document when it happens.

- **Measured, on Windows under a job memory limit:** the Go runtime printed `fatal error: out of memory` and a goroutine trace on stderr. A process that ends this way exits with Go's status 2, which here does not mean invalid usage ([unexpected internal failures](exit-codes.md#unexpected-internal-failures)).
- **Also measured there: the process can hang instead of exiting.** The measured Go process printed the fatal error and was still running 60 seconds later, until something killed it. A plain Go program does the same: the cause is the Go runtime on Windows, not Dirloom. A Windows container with a memory limit may behave the same way; that was not measured. Give such runs a time limit, for example `timeout-minutes` in GitHub Actions.
- **Not measured:** Linux, macOS, and Windows without a memory limit, where the system pages before it refuses memory. There the operating system can also end the process itself, without the Go message.

## `GOMEMLIMIT`

`GOMEMLIMIT` is a **soft limit of the Go runtime, not a hard cap on the process**. As the heap nears it, the collector runs more often. It never refuses memory: a soft limit below the memory the workflow keeps in use, its live heap, cannot be honored, and Go goes past it; only the operating system or a job limit stops the process.

Measured with it, at the 2,000,000-node limit:

- in a 2 GiB budget, `GOMEMLIMIT=1792MiB` let both comparisons of two snapshots that were measured pass 5 out of 5: the 2,000,000-node one, which failed 5 out of 5 without it, and the long-file-name one;
- in a 3 GiB budget, `GOMEMLIMIT=2560MiB` let the hostile comparison pass 5 out of 5; without it, it failed 5 out of 5;
- the collector did more work: 29% to 54% more CPU time for the long-file-name comparison, 71% to 77% for the hostile one, and nothing measurable for the 2,000,000-node comparison. On 24 cores, the wall time stayed within the noise of these runs, about 20%. On a machine with few cores, the extra CPU time becomes wall time; that was not measured.

**Treat these values as a starting point for constrained environments, not as a universal setting.** They held for these workflows, these budgets and this machine, and only these three cases were measured. For them, the live heap was 1.3 to 2.4 GiB: a value below the live heap of a workflow does not help it.

To try it, set it in the environment of one command. The two values measured left 256 MiB and 512 MiB of the budget free, and stayed above the live heap; `1792MiB` is the value for a 2 GiB budget:

```bash
GOMEMLIMIT=1792MiB dirloom diff snapshot:base.dlm.json snapshot:head.dlm.json
```

```powershell
$env:GOMEMLIMIT = '1792MiB'
dirloom diff snapshot:base.dlm.json snapshot:head.dlm.json
Remove-Item Env:GOMEMLIMIT
```

## Limits that cost time

Two limits cost time rather than memory, because each entry is tested against every pattern:

- 4,096 `capture.ignore` patterns in a snapshot: `verify` of 100,000 live nodes took 35 s instead of 0.8 s;
- a `.gitignore` of 1 MiB made of 80,659 short patterns: fingerprinting 100,000 nodes took 212 s (median; 151 to 277 s), where Git took 88 to 126 s on the same tree. The same 1 MiB in 16 long lines took 0.22 s.

## Not measured

- Linux and macOS: every figure above comes from Windows;
- a machine with 8 GiB of RAM, paging, machines with few cores, and CI runners;
- real repositories: the trees and snapshots were generated;
- a live tree above 2,000,000 nodes, which `verify` and `diff` accept;
- `GOMEMLIMIT` beyond the three cases above.

Dirloom `v0.4.0-rc1` keeps these limits as they are and does not set a memory limit itself. The release candidate's soak, its use on real repositories before `v0.4.0`, is meant to show how they behave there.
