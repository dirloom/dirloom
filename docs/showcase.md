# Showcase: a refactoring from end to end

This walkthrough takes a small project through a refactoring and shows the
Structural Version Control commands of Dirloom at work:

1. a reference project, committed on `main`;
2. a snapshot of its structure;
3. a refactoring, on a branch, that moves and renames files and directories;
4. `dirloom verify` fails: the structure no longer matches the snapshot;
5. `dirloom diff` names the changes, moves included;
6. the two Git branches compared;
7. the structural history of the branch.

Dirloom observes structure only: paths, kinds and link targets, never file
contents.

Every output on this page is real.
[`examples/showcase/showcase.sh`](../examples/showcase/showcase.sh) and
[`examples/showcase/showcase.ps1`](../examples/showcase/showcase.ps1) run the
scenario and print these transcripts. A test runs both scripts on Windows,
Linux and macOS and checks this page against them, byte for byte.

## Run it

You need `dirloom` and Git 2.45 or newer.

```bash
examples/showcase/showcase.sh
```

```powershell
pwsh -File examples/showcase/showcase.ps1
```

Each script builds the project in a new temporary directory and names that
directory on stderr at the end. To choose the directory, give it as the
first argument: it must be empty or not exist yet. To run another `dirloom`,
such as a local build, set the `DIRLOOM` environment variable to its path.
Windows PowerShell 5.1 runs the script too; its default policy refuses
scripts, so run it with
`powershell -ExecutionPolicy Bypass -File examples\showcase\showcase.ps1`.

The scripts read nothing from your configuration, so their output is the
same on every machine, commit ids included:

- Dirloom runs with `--no-config` wherever a command accepts it. `verify`,
  `diff` and `history` read no configuration at all.
- Git reads an empty configuration: `GIT_CONFIG_NOSYSTEM=1`, and
  `GIT_CONFIG_GLOBAL` names an empty file. It reads no global ignore or
  attributes file either (`XDG_CONFIG_HOME` names an empty directory, and
  `GIT_ATTR_NOSYSTEM=1`), and `git init --template=` copies no template, so
  no hook runs.
- Every commit has a fixed author, committer and date (`GIT_AUTHOR_DATE`,
  `GIT_COMMITTER_DATE`).
- Every file is written with the same bytes on every system: one line of
  UTF-8 ending with LF.

The PowerShell script restores the environment variables and the location of
your session when it ends.

In each transcript, `$ dirloom …` is the command and `[exit N]` its exit
code, as documented in
[Exit codes and machine codes](reference/exit-codes.md). A script stops with
an error as soon as a command exits otherwise or writes to stderr.

## 1. A reference project

`shop` is a small Go service, committed on `main`. The tree output shows its
structure; `--style ascii` draws it with ASCII characters only.

<!-- showcase:project -->
```text
$ dirloom --no-config --style ascii
shop/
|-- cmd/
|   `-- shop/
|       `-- main.go
|-- docs/
|   `-- architecture.md
|-- internal/
|   |-- billing/
|   |   |-- invoice.go
|   |   `-- tax.go
|   `-- utils/
|       `-- strings.go
|-- payment/
|   |-- api/
|   |   `-- handler.go
|   `-- service.go
|-- go.mod
`-- README.md
[exit 0]
```

## 2. Snapshot the structure

`dirloom snapshot` records the structure in a self-verifying file, and
`dirloom verify` confirms that the tree matches it. The snapshot is then
committed on `main` with the code, as a project keeps it.

<!-- showcase:snapshot -->
```text
$ dirloom snapshot --no-config --output architecture.dlm.json
[exit 0]

$ dirloom verify architecture.dlm.json
Structure matches snapshot.
[exit 0]
```

With `--output`, stdout stays empty. The snapshot leaves out its own file.
See [`dirloom snapshot`](reference/snapshot.md).

## 3. Refactor on a branch

On a new branch, `refactor/layout`, the refactoring moves the `payment`
package under `internal`, renames `internal/utils` to `internal/textutil`,
moves `tax.go` into a new `internal/tax` directory, and renames the
architecture document:

<!-- showcase.sh:refactor -->
```bash
git mv payment internal/payment
git mv internal/utils internal/textutil
mkdir internal/tax
git mv internal/billing/tax.go internal/tax/tax.go
git mv docs/architecture.md docs/design.md
```

`showcase.ps1` runs the same `git mv` commands, and
`New-Item -ItemType Directory -Path internal/tax` for `mkdir`. Nothing is
committed yet.

## 4. `verify` fails

The structure no longer matches the snapshot. `verify` says so with exit
code 1 and the two fingerprints. stderr stays empty: a mismatch is a result,
not an error. Which paths changed is the job of `diff`.

<!-- showcase:verify -->
```text
$ dirloom verify architecture.dlm.json
Structure differs from snapshot.
Expected: dlm:v1:sha256:f8faa972b86860438d21caf041f56f20dbba3eb784fe659187f8167312fa18d8
Actual:   dlm:v1:sha256:11b90a08ba7e5241d6d846ffbcaf13a1df42d3b952683d83c7334581f0644056
[exit 1]
```

In CI, that exit code is the check; `--format json` gives the same result as
a document. See [`dirloom verify`](reference/verify.md).

## 5. `diff` names the changes

`dirloom diff` compares the snapshot with the live tree:

<!-- showcase:diff -->
```text
$ dirloom diff snapshot:architecture.dlm.json live:.
Structural Diff

Summary: 2 added, 1 removed, 0 changed, 3 moved, 6 total

ADDED
  + docs/design.md
  + internal/tax

REMOVED
  - docs/architecture.md

MOVED
  > payment -> internal/payment (subtree, 4 nodes)
  > internal/billing/tax.go -> internal/tax/tax.go (name, 1 node)
  > internal/utils -> internal/textutil (subtree, 2 nodes)
[exit 1]
```

- `MOVED` lists each move once, at its destination, with its evidence and
  the number of records it carries. `payment` moved under `internal` with
  its 3 descendants, as one subtree; `internal/utils` was renamed in place;
  `tax.go` moved into a new directory and is matched by its name, which is
  unique on both sides: `name` is the weakest evidence.
- `internal/tax` is `ADDED`: the directory is new.
- `docs/architecture.md` renamed to `docs/design.md` is `REMOVED` plus
  `ADDED`. Move detection never pairs a file renamed in place: it is
  structural and conservative, without a similarity score, so it never
  presents a guess as a move.
- Exit 1 means differences; it is not an error either.

`--no-moves` lists every changed path instead of the moves, and
`--format json` writes the same result as a document. See
[`dirloom diff`](reference/diff.md) and its [moves](reference/diff.md#moves).

## 6. Compare two branches

The refactoring is committed on the branch, then a second commit rewrites
the README, a change of contents only. `git:` sources read committed trees,
never the working tree:

<!-- showcase:git -->
```text
$ dirloom diff git:main git:refactor/layout
Structural Diff

Summary: 2 added, 1 removed, 0 changed, 3 moved, 6 total

ADDED
  + docs/design.md
  + internal/tax

REMOVED
  - docs/architecture.md

MOVED
  > payment -> internal/payment (subtree, 4 nodes)
  > internal/billing/tax.go -> internal/tax/tax.go (name, 1 node)
  > internal/utils -> internal/textutil (subtree, 2 nodes)
[exit 1]
```

These are the changes found from the snapshot. The snapshot file is
committed on both branches, so it is no difference, and the README rewrite
changes contents only. A pull request compares its head with its base the
same way, once both are in the local repository:
`dirloom diff git:origin/main git:HEAD`.

## 7. Structural history

`dirloom history` walks the first-parent chain of `HEAD`, newest first, and
counts the structural changes of each commit, as `+added -removed ~changed
>moved`:

<!-- showcase:history -->
```text
$ dirloom history
Structural History (first parent, 4 commits examined, end: root)

8e28ae0e0552  2026-01-05T12:00:00Z  no structural change
0b5e4c23be2f  2026-01-05T11:00:00Z  +2 -1 ~0 >3
01aa10b461b3  2026-01-05T10:00:00Z  +1 -0 ~0 >0
4f026f9abad1  2026-01-05T09:00:00Z  +17 -0 ~0 >0  root

Changed paths of a commit: dirloom diff git:<commit>^ git:<commit>
[exit 0]
```

The refactoring has the counts of the diff above. The README rewrite is
`no structural change`, which `git log` cannot tell. The root commit added
17 paths: 9 files and their 8 directories. Commit messages, identities and
branch names are never shown. See [`dirloom history`](reference/history.md).

## Watch (experimental)

> **EXPERIMENTAL.** `dirloom watch` and its contracts are excluded from the
> v0.4 contract freeze and may change in any v0.x release. Its text output
> is for people; it is not a contract. See
> [`dirloom watch`](reference/watch.md).

`dirloom watch` reports structural changes as they happen. Start it in the
project that the script left, whose path the script names on stderr:

```bash
dirloom watch --no-config
```

In a second terminal, in the same directory, rename a directory, with bash:

```bash
mv internal/textutil internal/strutil
```

or with PowerShell:

```powershell
Move-Item internal/textutil internal/strutil
```

Then stop the watch with Ctrl+C:

<!-- showcase:watch -->
```text
$ dirloom watch --no-config
Watching structure (experimental, native): 20 nodes, dlm:v1:sha256:eaa4d0769c8e610272bbb9dd7bdfcf7733f72a37cf00fbb3ba34c6483f68fee8
#1  > internal/textutil -> internal/strutil (subtree, 2 nodes)
#1  = 20 nodes, dlm:v1:sha256:2e528d24d7566c6532b7816b70cff5a44971e8be2db3cdadc0f20dea977010bf  (+0 -0 ~0 >1)
Stopped at #1.
[exit 0]
```

The first line is the state at startup: 20 nodes, the snapshot file
included, since a watch has no snapshot to leave out. The rename is
generation `#1`: one `MOVED`, then the new node count, fingerprint and
counts. Ctrl+C is a controlled interruption, exit 0. A test replays this
demonstration on Windows, Linux and macOS, separately from the scenario.
