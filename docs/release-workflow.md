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

## Pins and approvals

Every GitHub Action in this repository is pinned by full commit SHA. Every
tool downloaded in CI or release workflows has a deterministic version (and a
checksum when the file is fetched as a binary):

| Tool | Pin |
| --- | --- |
| GoReleaser | `2.17.1` |
| Syft | `1.50.0` |
| WingetCreate | `1.12.13.0` |
| `actions/attest` | `v4.2.1` (SHA `508db95dd578ae2727ebd6217d5ba78e4fbda05d`) |
| golangci-lint | `v2.12.2` |
| govulncheck | `v1.6.0` |

Changes to `.github/workflows/release.yml`, `.github/workflows/update-packages.yml`,
or packaging automation require **two independent approvals**. Mechanical
version PRs in Scoop, Homebrew and Winget repositories require one maintainer
approval.

The Winget submission token lives in the GitHub Environment `package-publishing`.
It is not available to pull-request workflows.

## Release owner checklist

v0.2 uses a transitional human GO after the draft is attested. Later releases
should drop that extra gate once the pipeline has proven the inventory,
checksums, SBOMs and attestations on its own.

```text
snapshot → smoke → GO → merge → tag → draft → verification → human GO → publish
```

Target once the invariants are machine-proven:

```text
merge release → tag → build → attest → verify → publish
```

1. Keep the RC note and `[Unreleased]` changelog entries on `release/v0.2.0`.
2. Run the full validation matrix (CI including `release/**`, race, lint, vuln,
   completion syntax, GoReleaser check, snapshot, 13-artifact verify).
3. Record GO/NO-GO with commit SHA and artifact checksums.
4. Merge `release/v0.2.0` → `main` with a merge commit.
5. Tag annotated `v0.2.0` on `main` only after smoke tests on candidate archives.
   The binary reports `dirloom 0.2.0` (no leading `v`).
6. The tag workflow leaves a **draft**. Verify 13 artifacts, 12 checksum lines,
   SBOMs, licences inside archives, and `gh attestation verify` on all 13
   subjects. Do not publish from CI.
7. Human GO publishes the draft. Then delete `release/v0.2.0` after closure.
8. Publication opens Scoop, Homebrew and Winget PRs. **Release Done** does not
   wait for the Winget merge. Flip each channel to Distribution Verified after
   install/upgrade/uninstall smoke.

Bootstrap Homebrew and Winget against **v0.1.1** before tagging v0.2.0. See
[Distribution](distribution.md).

## Inventory

A published release is immutable. Expected artifacts:

```text
6 archives + 6 SBOM + checksums.txt = 13
checksums.txt has 12 hashes and excludes itself
```

After publication, never replace artifacts under the same tag. A binary defect
requires v0.2.1. A broken package recipe is fixed in its own repository and
does not undo Release Done.

Retain CI evidence for at least 90 days: commit SHA, tag, checksums, SBOMs,
attestations, CI results, approvals, distribution PR smokes, then per-channel
install smokes.

## Rollback

- Before publication: revert the relevant PRs on `release/v0.2.0`.
- After publication: no artifact replacement; ship a corrective version.
- A failing manager can remain on v0.1.1 while it is repaired.

## Process correction (2026-08-17)

Pull requests #8–#12 were merged into `main` before `release/v0.2.0` was
opened. The scope was moved onto `release/v0.2.0` and `main` was restored to
`v0.1.1` so the repository again matches the release-branch contract used for
`v0.1.1` (`release/v0.1.1` → `main`).
