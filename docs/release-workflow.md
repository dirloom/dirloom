# Release workflow

Dirloom follows the Ginov **release branch** model documented in the
[Release workflow & Git-Ops hub](https://knowledge.floxio.ai/doc/guide-release-workflow-git-ops-hub-6ERj1DbE2s).

## Pre-1.0 versioning policy

While Dirloom remains in 0.x:

- `0.Y.0` marks a product milestone or capability boundary.
- `0.Y.Z` is a backward-compatible maintenance release within that milestone.

Patch releases may include bug fixes, UX refinements, CLI ergonomics,
documentation/discoverability improvements, and small additive capabilities
that do not redefine the milestone.

Examples:

- `v0.3.0` — PRESENTATION
- `v0.3.1` — PRESENTATION refinement / Contextual Help & CLI Guidance
- `v0.3.2` — PRESENTATION refinement / Icon Capability Contract & Portable ASCII Icons
- `v0.4.0` — CHANGE / Fingerprint, Snapshot, Verify, Diff

`v0.4.0` stays reserved for the CHANGE milestone. Contextual help and the icon
capability contract belong to the PRESENTATION refinement line and must not be
requalified as v0.4.

## Active release

| Field | Value |
| --- | --- |
| Version | `v0.3.1` |
| Latest published release | `v0.3.0` |
| Release branch | `release/v0.3.1` |
| Integration branch | `main` |
| Profile | CLI / package — build on tag after RC validation |

`v0.3.0` remains the latest published GitHub tag. `release/v0.3.1` is the
scope freeze: changelog, product status, snapshot and smoke only. No new
features. Do not tag, draft or publish until this freeze merges to `main`
and the release ceremony completes.

## Developer workflow

```bash
git fetch --prune origin
git switch release/v0.3.1
git pull --ff-only origin release/v0.3.1
git switch -c chore/v0.3.1-freeze-follow-up
# … freeze-only commit, push, open PR → release/v0.3.1
```

Use `main` as the base for work outside this freeze, with explicit Release
Owner approval. Do not add v0.3.1 product scope after the freeze.

## v0.3.1 freeze checklist

```text
snapshot → smoke → freeze reviewed → CI green → RELEASE READY
  → merge release → main → tag v0.3.1 → draft → verification → human GO → publish
```

1. Keep the freeze changelog (`[0.3.1] - 2026-09-18`, empty `[Unreleased]`) on
   `release/v0.3.1`. Do not tag or publish from this step.
2. Run the full validation matrix (CI including `release/**`, race, lint, vuln,
   completion syntax, GoReleaser check, snapshot, 13-artifact verify).
3. Record GO/NO-GO with commit SHA and artifact checksums.
4. Merge `release/v0.3.1` → `main` with a merge commit.
5. Tag annotated `v0.3.1` on `main` only after smoke tests on candidate archives.
   The binary reports `dirloom 0.3.1` (no leading `v`).
6. The tag workflow leaves a **draft**. Verify 13 artifacts, 12 checksum lines,
   SBOMs, licences inside archives, and `gh attestation verify` on all 13
   subjects. Do not publish from CI.
7. Human GO publishes the draft. Then delete `release/v0.3.1` after closure.
8. Publication opens Scoop, Homebrew and Winget PRs. **Release Done** does not
   wait for the Winget merge.

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

## v0.3.0 release record

v0.3.0 is published. The checklist below is the completed ceremony, kept for
audit. It is not the current active release state.

v0.3 kept the transitional human GO after the draft is attested.

```text
snapshot → smoke → freeze reviewed → CI green → RELEASE READY
  → merge release → main → tag v0.3.0 → draft → verification → human GO → publish
```

1. Keep the freeze changelog (`[0.3.0] - 2026-09-17`, empty `[Unreleased]`) on
   `release/v0.3.0`. Do not tag or publish from this step.
2. Run the full validation matrix (CI including `release/**`, race, lint, vuln,
   completion syntax, GoReleaser check, snapshot, 13-artifact verify).
3. Record GO/NO-GO with commit SHA and artifact checksums.
4. Merge `release/v0.3.0` → `main` with a merge commit.
5. Tag annotated `v0.3.0` on `main` only after smoke tests on candidate archives.
   The binary reports `dirloom 0.3.0` (no leading `v`).
6. The tag workflow leaves a **draft**. Verify 13 artifacts, 12 checksum lines,
   SBOMs, licences inside archives, and `gh attestation verify` on all 13
   subjects. Do not publish from CI.
7. Human GO publishes the draft. Then delete `release/v0.3.0` after closure.
8. Publication opens Scoop, Homebrew and Winget PRs. **Release Done** does not
   wait for the Winget merge. Flip each channel to Distribution Verified after
   install/upgrade/uninstall smoke.

See [Distribution](distribution.md). The latest published GitHub tag is `v0.3.0`.

## Inventory

A published release is immutable. Expected artifacts:

```text
6 archives + 6 SBOM + checksums.txt = 13
checksums.txt has 12 hashes and excludes itself
```

After publication, never replace artifacts under the same tag. A binary defect
requires a new version such as v0.3.1. A broken package recipe is fixed in its
own repository and does not undo Release Done.

Retain CI evidence for at least 90 days: commit SHA, tag, checksums, SBOMs,
attestations, CI results, approvals, distribution PR smokes, then per-channel
install smokes.

## Rollback

- Before publication: revert the relevant freeze commits on that version's
  release branch.
- After publication: no artifact replacement; ship a corrective version.
- A failing manager can remain on the last Distribution Verified version while
  it is repaired.

## Process correction (2026-08-17)

Pull requests #8–#12 were merged into `main` before `release/v0.2.0` was
opened. The scope was moved onto `release/v0.2.0` and `main` was restored to
`v0.1.1` so the repository again matches the release-branch contract used for
`v0.1.1` (`release/v0.1.1` → `main`).
