# CI integration

> **Status:** user guide for Dirloom v0.4 (`v0.4.0-rc1`). It relies on the exit codes of `verify`, `diff` and `history`, candidates for the v0.4 freeze ([Exit codes and machine codes](reference/exit-codes.md)).

This guide keeps the structure of a repository under review. A snapshot of the structure is committed next to the code. Every pull request is checked against it, and a structural change passes only when the pull request also updates the snapshot, so that the change shows up in the review. The workflow also writes the structural changes of each pull request, and the recent structural history of `main`, to the job summary.

Read [Structural Version Control](structural-version-control.md) first for the commands themselves.

## 1. Commit a snapshot

Create the snapshot once, from a clean checkout of `main`, before anything is built:

```bash
dirloom snapshot --no-user-config --ignore architecture.dlm.json --output architecture.dlm.json
git add architecture.dlm.json
git commit -m "chore: record the project structure"
```

- `--no-user-config` keeps your personal configuration out of the snapshot. The project `.dirloom.yaml`, if there is one, still applies.
- `--ignore architecture.dlm.json` records that the snapshot is not part of the structure it describes. Without it, comparing the snapshot with a commit that contains it, as in `dirloom diff snapshot:architecture.dlm.json git:HEAD`, would report the file itself as `ADDED`.
- The snapshot stores the effective Capture Semantics: filters, depth, hidden entries, `.gitignore`. `verify` reapplies them in CI; the workflow passes no filter flag.

When a pull request changes the structure on purpose, its author runs the same `dirloom snapshot` command and commits the new `architecture.dlm.json`. The reviewer then sees the structural change twice: as the diff of the snapshot file, and in the job summary.

## 2. The workflow

Save this as `.github/workflows/structure.yml`:

````yaml
name: Structure

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  structure:
    runs-on: ubuntu-latest
    defaults:
      run:
        shell: bash
    env:
      DIRLOOM_VERSION: v0.4.0-rc1
    steps:
      - name: Check out the repository
        uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6
        with:
          fetch-depth: 0 # git: sources and history read committed objects

      - name: Install Dirloom
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          asset=dirloom_Linux_x86_64.tar.gz
          dir="$RUNNER_TEMP/dirloom"
          mkdir -p "$dir"
          cd "$dir"
          gh release download "$DIRLOOM_VERSION" --repo dirloom/dirloom --pattern "$asset" --pattern checksums.txt
          grep "  $asset\$" checksums.txt | sha256sum -c -
          gh attestation verify "$asset" --repo dirloom/dirloom \
            --signer-workflow dirloom/dirloom/.github/workflows/release.yml \
            --source-ref "refs/tags/$DIRLOOM_VERSION"
          tar -xzf "$asset" dirloom
          echo "$dir" >> "$GITHUB_PATH"
          ./dirloom --version

      - name: Summarize the structural changes of the pull request
        if: github.event_name == 'pull_request'
        env:
          BASE_REF: ${{ github.base_ref }}
        run: |
          status=0
          dirloom diff "git:origin/$BASE_REF" git:HEAD > "$RUNNER_TEMP/structure-diff.txt" || status=$?
          if [ "$status" -gt 1 ]; then exit "$status"; fi
          {
            echo '### Structural changes'
            echo '```text'
            cat "$RUNNER_TEMP/structure-diff.txt"
            echo '```'
          } >> "$GITHUB_STEP_SUMMARY"

      - name: Summarize the structural history
        if: github.event_name == 'push'
        run: |
          {
            echo '### Structural history'
            echo '```text'
            dirloom history --limit 20 --structural-only
            echo '```'
          } >> "$GITHUB_STEP_SUMMARY"

      - name: Check the structure against the committed snapshot
        run: |
          status=0
          dirloom verify architecture.dlm.json || status=$?
          if [ "$status" -eq 1 ]; then
            echo "::error file=architecture.dlm.json::The structure no longer matches the committed snapshot. Update it with dirloom snapshot if the change is intended."
            dirloom diff snapshot:architecture.dlm.json live:. || [ $? -eq 1 ]
          fi
          exit "$status"
````

Pin `DIRLOOM_VERSION` to the exact release you have validated. The stable package channels (Winget, Scoop, Homebrew) stay on `v0.3.3` during the v0.4 release candidate, and `v0.3.3` has none of these commands: install a v0.4 release from GitHub Releases, as above.

## 3. What each step relies on

The job sets `shell: bash`, so GitHub runs each `run:` block with `bash --noprofile --norc -eo pipefail`: any command or pipeline that exits non-zero ends the step, unless its status is captured with `|| status=$?`. Every file the steps write goes to `$RUNNER_TEMP` or to the job summary, never into the checked-out tree, which `verify` observes.

**Install Dirloom** downloads the Linux archive and `checksums.txt` of one release, checks the archive's SHA-256, and verifies its GitHub artifact attestation: the archive must have been built by the `release.yml` workflow of `dirloom/dirloom` from the tag of that version. The step fails on any mismatch. The [security policy](../SECURITY.md#verifying-a-release) explains what these checks prove, and what they do not. On a Windows or macOS runner, use `dirloom_Windows_x86_64.zip` or `dirloom_Darwin_arm64.tar.gz` and that platform's checksum tool.

**Summarize the structural changes** compares the committed tree of the target branch with the commit the pull request run checks out, GitHub's merge of the pull request into its target. Both sides are `git:` sources: Dirloom reads Git objects, not the working tree. They need Git 2.45 or newer on the runner; an older Git fails with `git_unavailable` (exit `5`).

| `diff` exit | Meaning | Step |
| --- | --- | --- |
| `0` | no structural differences | passes |
| `1` | structural differences, listed in the summary | passes |
| `2` | invalid usage | fails |
| `5` | a Git object or revision could not be read, for example `origin/main` missing from a shallow clone (`git_revision_not_found`) | fails |
| `6` | internal error | fails |

Two `git:` sources take no snapshot, so `diff` cannot exit `3` or `4` here.

**Summarize the structural history** lists, on `main`, the last 20 commits that changed the structure. `history` has no exit `1`: it exits `0`, or `2`, `5` or `6` on a failure, which fails the step through `-e`. In a shallow clone the walk stops at the shallow boundary and says so (`end: shallow`); `fetch-depth: 0` avoids that.

**Check the structure** is the gate. `verify` observes the checked-out directory with the Capture Semantics of the snapshot:

| `verify` exit | Meaning | Step |
| --- | --- | --- |
| `0` | the structure matches the snapshot | passes |
| `1` | the structure differs: the step prints an error annotation and the `diff` between the snapshot and the directory | fails |
| `2` | invalid usage | fails |
| `3` | the snapshot is invalid: not Snapshot Schema v1, or its embedded fingerprint does not match its structure | fails |
| `4` | the snapshot needs a newer Dirloom: unsupported schema, artifact version or required feature | fails |
| `5` | the snapshot or the directory could not be read: a missing file, a denied directory, a named pipe in the tree | fails |
| `6` | internal error | fails |

The gate runs last, so the summaries are written even when it fails, and before any build step, so that build outputs are not part of the observed structure.

## 4. Variations

- **Machine-readable results.** `dirloom verify architecture.dlm.json --format json` writes [Verify Result Schema v1](contracts/verify-result-v1.md) and `dirloom diff … --format json` writes [Diff Result Schema v3](contracts/diff-result-v3.md). Branch on the exit code first: with `--format json`, a usage error (exit `2`) or an interrupted write leaves stdout empty or incomplete.
- **Moves.** `diff` reports a moved subtree as one `MOVED` entry. Add `--no-moves` to list every added and removed path instead.
- **Several snapshots.** A monorepo can keep one snapshot per package, for example `dirloom snapshot packages/api --no-user-config --ignore architecture.dlm.json --output packages/api/architecture.dlm.json`, checked with `dirloom verify packages/api/architecture.dlm.json packages/api`. A literal name such as `architecture.dlm.json` is ignored at any depth, so the snapshot of the whole repository also leaves out those of its packages.
- **Other CI systems.** The commands and exit codes are the same in GitLab CI, Azure DevOps or Jenkins: stdout and stderr are separate, output is UTF-8 without ANSI colors, and no terminal is needed. The [verify](reference/verify.md#ci) and [diff](reference/diff.md) references show POSIX shell and PowerShell variants.
