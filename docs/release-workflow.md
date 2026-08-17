# Release workflow

Dirloom follows the Ginov **release branch** model documented in the
[Release workflow & Git-Ops hub](https://knowledge.floxio.ai/doc/guide-release-workflow-git-ops-hub-6ERj1DbE2s).

## Active release

| Field | Value |
| --- | --- |
| Version | `v0.2.0` |
| Release branch | `release/v0.2.0` |
| Integration branch | `main` (last published tag: `v0.1.1`) |
| Profile | CLI / package — build on tag after RC validation |

During `v0.2.0` composition, feature and fix pull requests MUST target
`release/v0.2.0`, not `main`. `main` receives the release only after the
release candidate passes GO/NO-GO and a `release/v0.2.0` → `main` pull request
is merged.

## Developer workflow

```bash
git fetch --prune origin
git switch release/v0.2.0
git pull --ff-only origin release/v0.2.0
git switch -c feat/my-change
# … commit, push, open PR → release/v0.2.0
```

Use `main` as the base only for documentation or tooling that must land
outside the current release scope, with explicit Release Owner approval.

## Release owner checklist

1. Keep the RC note and `[Unreleased]` changelog entries on `release/v0.2.0`.
2. Run the full validation matrix (CI, race, lint, vuln, GoReleaser snapshot).
3. Record GO/NO-GO with commit SHA and artifact checksums.
4. Merge `release/v0.2.0` → `main` with a merge commit.
5. Tag `v0.2.0` on `main` only after smoke tests on candidate archives.
6. Publish the GitHub draft release, then delete `release/v0.2.0` after closure.

## Process correction (2026-08-17)

Pull requests #8–#12 were merged into `main` before `release/v0.2.0` was
opened. The scope was moved onto `release/v0.2.0` and `main` was restored to
`v0.1.1` so the repository again matches the release-branch contract used for
`v0.1.1` (`release/v0.1.1` → `main`).
