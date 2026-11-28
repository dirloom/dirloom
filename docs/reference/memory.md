# Memory and resource limits

> **Status:** guidance for `v0.4.0-rc1`. The figures come from the [v0.4.0-rc1 performance and memory baselines](../benchmarks/v0.4-rc1.md), measured on one Windows machine with Go 1.26.10. They describe what was measured, not a guarantee.

Snapshots and Git trees are untrusted input, so Dirloom bounds what it reads. The bounds keep a hostile input from running without end, but a workflow at the bounds still needs several GiB of memory. Dirloom does not cap its own memory: when the machine cannot provide what a workflow needs, the Go runtime stops the process.

## The limits

| Input | Limit | Beyond it |
| --- | --- | --- |
| snapshot file | 256 MiB | `snapshot_limit_exceeded`, exit 3; `dirloom snapshot` refuses to write it, exit 1 |
| snapshot nodes, and elements of any JSON array | 2,000,000 | the same |
| snapshot JSON nesting | 64 levels | the same |
| snapshot `capture.ignore` patterns | 4,096 | the same |
| snapshot `requiredFeatures` entries | 64 | the same |
| Git observation (`diff git:`, `diff snapshot: git:`, `history`) | 2,000,000 nodes and 256 MiB of paths | `git_tree_too_large`, exit 5 |
| `.gitignore` | 1 MiB, and lines of at most 65,535 bytes | `source_read_failure` for a directory, `git_tree_invalid` for a Git tree |

A live directory has no node limit: `verify` and `diff` observe it whole, and the [watch reference](watch.md) has the limits of `watch`. `dirloom snapshot` of a tree above 2,000,000 nodes fails, because it refuses to write a snapshot that it would refuse to read. The [security policy](../../SECURITY.md#untrusted-input) lists every bound on untrusted input, and [Exit codes and machine codes](exit-codes.md) every code.

## How much memory a workflow needs

The memory of a process, its commit, measured with its `git` processes at the 2,000,000-node limit:

| Workflow | Commit (median; highest run) |
| --- | --- |
| loading or verifying one snapshot | about 0.9 GiB per million nodes; 1.8 GiB at the limit |
| comparing two snapshots (`diff snapshot: snapshot:`) | about 1.3 GiB per million nodes; 2.7 to 2.8 GiB at the limit |
| comparing two snapshots of 252 MiB with long names, the highest valid workflow measured | 2.5 GiB; 3.4 GiB |
| a hostile 250 MiB snapshot of 17.5 million unique keys, read then refused | 3.4 GiB; 4.1 GiB |
| `diff git: git:` at 2,000,000 nodes | 1.6 GiB |
| two Git chains just under the 256 MiB path limit, compared | 1.7 GiB |

**4.1 GiB is the maximum observed, not a guaranteed ceiling.** It is the highest of the runs measured. The peak of one run depends on when the Go collector starts; the same hostile document committed 3.4 GiB in 13 of its 15 runs and 4.0 or 4.1 GiB in the other 2. Another machine, another load or another document can go higher.

**The 8 GiB of the measurements is a budget, not a machine.** Every workflow ran inside an 8 GiB commit budget, a Windows job object around Dirloom and its `git` processes, and none came near it. That shows what fits in 8 GiB of commit. A machine with 8 GiB of RAM was not measured: its operating system and other programs take part of that memory, and it pages before it refuses any.

**Smaller budgets,** at the 2,000,000-node limit, 5 runs each:

| Commit available | Result |
| --- | --- |
| 4 GiB | the long-named pair, the largest valid peak, passed 5 out of 5; the hostile document after a 2,000,000-node snapshot failed 3 runs out of 5 |
| 3 GiB | two wide snapshots compared passed 5 out of 5; the long-named pair failed once; the hostile case after a snapshot always failed |
| 2 GiB | loading or writing one snapshot, `diff git: git:` and the two Git chains passed 5 out of 5; `verify` against a live tree and `diff snapshot: git:` 4 out of 5; `diff snapshot: live:` 2 out of 5; comparing two snapshots always failed |

The full tables are in the record: [where it holds and where it breaks](../benchmarks/v0.4-rc1.md#the-8-gib-environment-where-it-holds-and-where-it-breaks).

## When memory runs out

Dirloom has no machine code for running out of memory. The Go runtime stops the process with `fatal error: out of memory`, a goroutine trace on stderr and exit status 2, which otherwise means invalid usage. Dirloom writes no diagnostic and no JSON error document ([unexpected internal failures](exit-codes.md#unexpected-internal-failures)).

**On Windows, under a commit limit, the process can hang.** In a Windows job object with a memory limit, or a Windows container with one, the measured Go process printed `fatal error: out of memory` and was still running 60 seconds later, until something killed it. A plain Go program does the same: the cause is the Go runtime on Windows, not Dirloom. Give such runs a time limit, for example `timeout-minutes` in GitHub Actions. Without a commit limit, Windows pages before it refuses memory; that was not measured.

## `GOMEMLIMIT`

`GOMEMLIMIT` is a **soft limit of the Go runtime, not a hard cap on the process**. As the heap nears it, the collector runs more often. It does not refuse memory: when the live data needs more, Go goes past it, and only the operating system or a job limit stops the process.

Measured, at the 2,000,000-node limit:

- in a 2 GiB budget, `GOMEMLIMIT=1792MiB` let both comparisons of two snapshots pass 5 out of 5; without it, the wide comparison failed 5 out of 5;
- in a 3 GiB budget, `GOMEMLIMIT=2560MiB` let the hostile case pass 5 out of 5; without it, it failed 5 out of 5;
- the cost was CPU time, not wall time, on 24 cores: up to 54% more CPU time for the long-named comparison, 71% to 77% more for the hostile case. On a machine with few cores, that CPU time becomes wall time; this was not measured.

**Treat these values as a lead for constrained environments, not as a universal setting.** They held for these workflows, these budgets and this machine. The soft limit cannot go below the live heap of the workflow, 1.3 to 2.4 GiB at the limits: below it, Go exceeds its soft limit, and only a hard limit stops the process. Only three cases were measured with it.

To try it, set it in the environment of one command, a little below the memory the process may use; the measured values were 256 MiB and 512 MiB below the budget:

```bash
GOMEMLIMIT=1792MiB dirloom diff snapshot:base.json snapshot:head.json
```

```powershell
$env:GOMEMLIMIT = '1792MiB'
dirloom diff snapshot:base.json snapshot:head.json
Remove-Item Env:GOMEMLIMIT
```

## Limits that cost time

Two limits cost time rather than memory, because each entry is tested against every pattern:

- 4,096 `capture.ignore` patterns in a snapshot: `verify` of 100,000 live nodes took 35 s instead of 0.8 s;
- a `.gitignore` at the 1 MiB limit: fingerprinting 100,000 nodes took 212 s, where Git itself took 88 to 126 s.

## Not measured

- Linux and macOS under a memory limit, and a machine with 8 GiB of RAM;
- paging, and machines with few cores;
- `GOMEMLIMIT` beyond the three cases above.

Dirloom `v0.4.0-rc1` keeps these limits as they are and does not set a memory limit itself. The release candidate's soak is meant to show how they behave on real repositories.
