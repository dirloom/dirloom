# Release workflow

Dirloom follows the Ginov **release branch** model documented in the
[Release workflow & Git-Ops hub](https://knowledge.floxio.ai/doc/guide-release-workflow-git-ops-hub-6ERj1DbE2s).

Release notes policy lives in
[CHANGELOG policy](release/changelog-policy.md) and
[Release notes template](release/release-notes-template.md).
`CHANGELOG.md` is the unique source of truth. GoReleaser does not generate a
changelog from Git history (`changelog.disable: true`).

> When preparing a release, the agent never writes an independent GitHub
> Release description. It updates `CHANGELOG.md` according to
> `docs/release/changelog-policy.md`. The workflow then extracts the section
> that matches the tag. Any divergence between `CHANGELOG.md` and the GitHub
> Release is a release defect.

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
- `v0.3.3` — PRESENTATION refinement / Nerd Catalog Fidelity, Provenance & Classification Refinement
- `v0.4.0-a1` — CHANGE alpha / Artifact Identity & Fingerprint
- `v0.4.0` — CHANGE / Fingerprint, Snapshot, Verify, Diff

`v0.4.0` stays reserved for the CHANGE milestone. Contextual help and the icon
capability contract belong to the PRESENTATION refinement line and must not be
requalified as v0.4.

## Release branch integration

This rule is normative.

> Every change that belongs to `vX.Y.Z` is integrated by pull request into
> `release/vX.Y.Z`. Only `release/vX.Y.Z`, once complete, frozen, and
> validated, opens the final pull request to `main`. The annotated tag
> `vX.Y.Z` is created on `main` after that merge.

```text
main
  │
  └── create release/vX.Y.Z
          │
          └── create feat/… from release/vX.Y.Z
                    │
                    └── PR → release/vX.Y.Z
                              │
                              └── … other vX.Y.Z work
                                      │
                                      ▼
                               freeze and validate
                                      │
                                      └── PR finale → main
                                                   │
                                                   ▼
                                                tag vX.Y.Z
```

Do not open versioned feature or fix pull requests against `main`. Do not
develop on `main` or on a previous `release/v*` branch for the current
version. Do not tag from a feature branch.

Open `release/vX.Y.Z` from the intended `main` SHA **before** the first
feature branch of that version:

```bash
git fetch --prune origin
git switch main
git pull --ff-only origin main
git switch -c release/vX.Y.Z
git push -u origin release/vX.Y.Z
```

Then create work from that release branch:

```bash
git fetch --prune origin
git switch release/vX.Y.Z
git pull --ff-only origin release/vX.Y.Z
git switch -c feat/vX.Y.Z-<topic>
# … commit, push, open PR → release/vX.Y.Z
```

If a feature branch was started from `main` by mistake, repair it without
retargeting to `main`. Create `release/vX.Y.Z` from the **same** base SHA as
the feature, then retarget the pull request to `release/vX.Y.Z`. Rebase onto
the release branch only when that branch already contains commits the feature
does not have.

## Publishable increments

Each GitHub-publishable version is its own release-branch unit, including
alphas and release candidates:

```text
v0.4.0-a1  →  release/v0.4.0-a1
v0.4.0-a2  →  release/v0.4.0-a2
v0.4.0-a3  →  release/v0.4.0-a3
v0.4.0-rc1 →  release/v0.4.0-rc1
v0.4.0     →  release/v0.4.0
```

Do not land several publishable increments on one shared `release/v0.4.0`
branch. After freeze and validation, that increment's branch opens the final
pull request to `main`. The annotated tag is created on the real merge
commit in `main`.

## GitHub Pre-releases and package managers

GoReleaser leaves every tag as a **draft** and sets `prerelease: auto`. A
SemVer prerelease identifier (`-a1`, `-rc1`, …) becomes a GitHub
Pre-release; a stable tag such as `v0.4.0` does not.

```text
v0.4.0-a1 published
  → GitHub Pre-release
  → 13 artifacts / SBOM / attestations
  → Scoop / Homebrew / Winget stable channels unchanged

v0.4.0 published
  → GitHub Release
  → Scoop / Homebrew / Winget update PRs
```

The `Update package managers` jobs run on `release: published` only when
`github.event.release.prerelease` is false, or on `workflow_dispatch`.
`workflow_dispatch` remains the escape hatch to test a prerelease tag in a
package repository without making that the default path.

## Active release

| Field | Value |
| --- | --- |
| Version | `v0.4.0-a1` |
| Latest published release | `v0.3.3` |
| Release branch | `release/v0.4.0-a1` |
| Integration branch | `main` |
| Profile | prerelease / CLI package |

`v0.3.3` is the latest published GitHub tag. `release/v0.4.0-a1` is the
scope freeze: changelog, product status, snapshot and smoke only. No new
features. Freeze-only commits land directly on this branch. Do not tag,
draft or publish until this freeze merges to `main` and the release
ceremony completes. Each publishable increment (`a1`, `a2`, `a3`, `rc1`,
stable) keeps its own `release/v…` branch.

```text
feature → release/v0.4.0-a1
release/v0.4.0-a1 → main
main → annotated tag v0.4.0-a1
tag workflow → draft GitHub Pre-release
human GO → publish Pre-release
stable package managers stay silent
```

## Developer workflow

```bash
git fetch --prune origin
git switch release/v0.4.0-a1
git pull --ff-only origin release/v0.4.0-a1
# freeze-only commits: changelog, product status, snapshot, smoke
# land directly on release/v0.4.0-a1 — no extra feature or chore branch
```

Do not add v0.4.0-a1 product scope after the freeze. Do not open a pull
request to `main` until the Release Owner starts the final
`release/v0.4.0-a1` → `main` merge. Do not create `chore/v0.4.0-a1-freeze`.

## v0.4.0-a1 freeze checklist

The freeze is open on `release/v0.4.0-a1`.

```text
snapshot → smoke → freeze reviewed → CI green → RELEASE READY
  → merge release → main → tag v0.4.0-a1 → draft Pre-release
  → verification → human GO → publish Pre-release
```

1. Keep the freeze changelog (`[0.4.0-a1] - 2026-09-20`, empty
   `[Unreleased]`) on `release/v0.4.0-a1`. Do not tag or publish from this
   step.
2. Run the full validation matrix (CI including `release/**`, race, lint,
   vuln, completion syntax, GoReleaser check, snapshot, 13-artifact
   verify).
3. Record GO/NO-GO with commit SHA and artifact checksums.
4. Merge `release/v0.4.0-a1` → `main` with a merge commit.
5. Tag annotated `v0.4.0-a1` on `main` only after smoke tests on candidate
   archives. The binary reports `dirloom 0.4.0-a1` (no leading `v`).
6. The tag workflow leaves a **draft** GitHub Pre-release (`prerelease:
   auto`). Verify 13 artifacts, 12 checksum lines, SBOMs, licences inside
   archives, and `gh attestation verify` on all 13 subjects. Do not
   publish from CI.
7. Human GO publishes the draft Pre-release. Then delete
   `release/v0.4.0-a1` after closure.
8. Publication must **not** open Scoop, Homebrew or Winget PRs. Stable
   package channels stay on `v0.3.3` until a non-prerelease tag.

Do not create `chore/v0.4.0-a1-freeze`.

## v0.3.3 release record

v0.3.3 is published. The checklist below is the completed ceremony, kept
for audit. It is not the current active release state.

The **scope freeze** lived on `release/v0.3.3`.

```text
snapshot → smoke → freeze reviewed → CI green → RELEASE READY
  → merge release → main → tag v0.3.3 → draft → verification → human GO → publish
```

1. Keep the freeze changelog (`[0.3.3] - 2026-09-18`, empty `[Unreleased]`) on
   `release/v0.3.3`. Do not tag or publish from this step.
2. Run the full validation matrix (CI including `release/**`, race, lint, vuln,
   completion syntax, GoReleaser check, snapshot, 13-artifact verify).
3. Record GO/NO-GO with commit SHA and artifact checksums.
4. Merge `release/v0.3.3` → `main` with a merge commit.
5. Tag annotated `v0.3.3` on `main` only after smoke tests on candidate archives.
   The binary reports `dirloom 0.3.3` (no leading `v`).
6. The tag workflow leaves a **draft**. Verify 13 artifacts, 12 checksum lines,
   SBOMs, licences inside archives, and `gh attestation verify` on all 13
   subjects. Do not publish from CI.
7. Human GO publishes the draft. Then delete `release/v0.3.3` after closure.
8. Publication opens Scoop, Homebrew and Winget PRs. **Release Done** does not
   wait for the Winget merge.

Required visual gate was Windows Terminal with a compatible Nerd Font. The
extended WezTerm/Alacritty matrix is optional and is not a release blocker.

```text
Manual terminal validation
- Windows Terminal / Nerd Font: PASS
- style=unicode × icons=nerd: PASS
- style=ascii × icons=nerd: PASS
- vivid × nerd: PASS

Optional extended terminal matrix
- WezTerm / Mono: NOT RUN — terminal unavailable
- WezTerm / non-Mono: NOT RUN — terminal unavailable
- Alacritty / Mono: NOT RUN — terminal unavailable
- Alacritty / non-Mono: NOT RUN — terminal unavailable
- Release blocker: NO
```

WezTerm and Alacritty were not available. Absence did not block v0.3.3
because Nerd glyph/codepoint validity is covered automatically, Nerd Fonts
v3.5.1 provenance is pinned, ASCII and Unicode channels are frozen by tests,
spacing behavior is tested, Dirloom makes no terminal-width assumption, and
the full CI matrix is green.

Do not create `chore/v0.3.3-freeze`. That freeze is closed.

## v0.3.2 release record

v0.3.2 is published. The checklist below is the completed ceremony, kept for
audit. It is not the current active release state.

The **scope freeze** lived on `release/v0.3.2`.

```text
snapshot → smoke → freeze reviewed → CI green → RELEASE READY
  → merge release → main → tag v0.3.2 → draft → verification → human GO → publish
```

1. Keep the freeze changelog (`[0.3.2] - 2026-09-18`, empty `[Unreleased]`) on
   `release/v0.3.2`. Do not tag or publish from this step.
2. Run the full validation matrix (CI including `release/**`, race, lint, vuln,
   completion syntax, GoReleaser check, snapshot, 13-artifact verify).
3. Record GO/NO-GO with commit SHA and artifact checksums.
4. Merge `release/v0.3.2` → `main` with a merge commit.
5. Tag annotated `v0.3.2` on `main` only after smoke tests on candidate archives.
   The binary reports `dirloom 0.3.2` (no leading `v`).
6. The tag workflow leaves a **draft**. Verify 13 artifacts, 12 checksum lines,
   SBOMs, licences inside archives, and `gh attestation verify` on all 13
   subjects. Do not publish from CI.
7. Human GO publishes the draft. Then delete `release/v0.3.2` after closure.
8. Publication opens Scoop, Homebrew and Winget PRs. **Release Done** does not
   wait for the Winget merge.

Do not create `chore/v0.3.2-freeze`. That freeze is closed.

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

## v0.3.1 release record

v0.3.1 is published. The checklist below is the completed ceremony, kept for
audit. It is not the current active release state.

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

See [Distribution](distribution.md). The latest published GitHub tag is `v0.3.3`.

## Inventory

A published release is immutable. Expected artifacts:

```text
6 archives + 6 SBOM + checksums.txt = 13
checksums.txt has 12 hashes and excludes itself
```

After publication, never replace artifacts under the same tag. A binary defect
requires a new version such as v0.3.2. A broken package recipe is fixed in its
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

## Process correction (2026-09-18)

Pull request #29 (`feat/v0.3.2-icon-capabilities`) targeted `main` before
`release/v0.3.2` existed. `release/v0.3.2` was opened from `main` at
`a1dcf800906cb97dc7d012fa6e0dbe26a5349e58` and #29 was retargeted to
`release/v0.3.2`. Versioned work must not be merged to `main` except through
that release branch.

## Process correction (2026-08-17)

Pull requests #8–#12 were merged into `main` before `release/v0.2.0` was
opened. The scope was moved onto `release/v0.2.0` and `main` was restored to
`v0.1.1` so the repository again matches the release-branch contract used for
`v0.1.1` (`release/v0.1.1` → `main`).
