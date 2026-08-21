# Distribution and trusted releases

GitHub Releases is the only artifact source for Dirloom. Scoop, Homebrew and Winget never rebuild the binary: they reference the same immutable archive URLs and SHA-256 sums.

<!-- dirloom-distribution-command:install-matrix -->
```text
Windows
  winget install Dirloom.Dirloom
  scoop install dirloom

macOS
  brew install --cask dirloom/tap/dirloom

Linux
  brew install --cask dirloom/tap/dirloom
  GitHub archive
```

## Two independent statuses

**Release Done** is a product milestone. It is complete when v0.2.0 is published on GitHub with verified archives, checksums, SBOMs and attestations, `--copy` and `completion` are shipped, and the Scoop, Homebrew and Winget pull requests have been opened. A waiting Microsoft Winget merge does **not** reopen the milestone.

**Distribution Verified** is operational and tracked per channel after a clean install, version check, upgrade and uninstall on that manager.

<!-- dirloom-distribution-status -->
```text
RELEASE STATUS
Released at GitHub tag v0.1.1; v0.2.0 composing

DISTRIBUTION STATUS
GitHub     ✅ v0.1.1
Scoop      ✅ v0.1.1 (PR-based updates from this increment)
Homebrew   ⏳ tap bootstrap with v0.1.1
Winget     ⏳ package bootstrap with v0.1.1
```

## Identifiers

| Channel | Identifier | Architectures | Notes |
| --- | --- | --- | --- |
| GitHub Releases | [dirloom/dirloom](https://github.com/dirloom/dirloom/releases) | Windows, Linux, macOS × amd64, arm64 | Canonical artifacts |
| Scoop | `dirloom` in [dirloom/scoop-bucket](https://github.com/dirloom/scoop-bucket) | Windows amd64, arm64 | `scoop bucket add dirloom https://github.com/dirloom/scoop-bucket` |
| Homebrew | `dirloom/tap/dirloom` | macOS and Linux, amd64 and arm64 | `brew install --cask dirloom/tap/dirloom` |
| Winget | `Dirloom.Dirloom` | Windows amd64, arm64 | Zip + nested portable `dirloom.exe` |

No additional package manager is in v0.2.0. A later candidate must have demonstrated user demand, an official or Dirloom-maintained recipe, GitHub artifact consumption, SHA-256 verification, automated PR updates, install and upgrade tests, a named maintainer, and a documented retirement plan.

## Release inventory

A published GitHub Release is an immutable tuple: tag, commit, artifact, SHA-256, SBOM, attestation.

The inventory is exactly **13** subjects:

```text
checksums.txt covers:
- the 6 archives
- the 6 SBOMs
- and excludes checksums.txt itself

6 archives
6 SBOM
1 checksums.txt
──────────────
13 release artifacts
```

`checksums.txt` contains exactly 12 hash lines. Archives keep LICENSE, README, CHANGELOG, CONTRIBUTING, SECURITY, THIRD_PARTY_NOTICES and the platform binary (`dirloom` or `dirloom.exe`).

SBOMs are SPDX JSON, one per archive, produced by Syft 1.50.0. GitHub Artifact Attestations (actions/attest v4.2.1) cover all 13 subjects. Verify with:

```bash
gh release download v0.2.0 --pattern checksums.txt
sha256sum -c checksums.txt
gh attestation verify dirloom_Linux_x86_64.tar.gz --repo dirloom/dirloom
```

Do not rebuild or replace artifacts under an already published tag. A bad binary requires a new version such as v0.2.1.

## Bootstrap before a product tag

Homebrew and Winget must be exercised with **v0.1.1 before v0.2.0**. That separates “the distribution pipeline works” from “the product tag is correct”. Do not tag v0.2.0, publish a draft, or replace artifacts under an existing tag until this bootstrap is done.

### Runbook

1. **Environment.** Create GitHub Environment `package-publishing` on `dirloom/dirloom` (reviewers: org maintainers). Add environment secret `PACKAGE_BOT_TOKEN` (never on pull-request workflows). The token must write to `dirloom/scoop-bucket`, `dirloom/homebrew-tap`, and the `dirloom/winget-pkgs` fork.
2. **Homebrew tap.** Create public `dirloom/homebrew-tap` from `packaging/homebrew-tap/` (MIT, README, CODEOWNERS, cask pinned to v0.1.1). First commit may land on `main` because the repository is empty; afterwards protect `main` (required `brew audit`, no force-push). Version bumps are pull requests only.
3. **Scoop.** Stop direct commits to `dirloom/scoop-bucket` `main`. Convert the updater to open `dirloom-$version` pull requests. Mechanical version PRs need one maintainer approval; workflow changes need two.
4. **Winget.** Identity is `Dirloom.Dirloom`. Bootstrap with zip + nested portable `dirloom.exe`. If a v0.1.0 new-package PR is already open and validated, do not open a competing PR; submit v0.1.1 after it merges. Use WingetCreate 1.12.13.0.
5. **Product tag.** Tag v0.2.0 only after Homebrew is bootstrapped with v0.1.1 and Winget has an opened (not necessarily merged) v0.1.1 or validated v0.1.0 identity PR. Release workflow leaves a **draft**. Human GO publishes it. Never replace artifacts under that tag.

The `Update package managers` workflow runs only on **published** GitHub Releases (and manual `workflow_dispatch`). It uses the protected `package-publishing` environment. The bot token is never available to pull-request workflows.

## Maintainer procedure

1. Snapshot GoReleaser, smoke archives, record GO.
2. Merge `release/vX.Y.Z` → `main` with a merge commit.
3. Tag `vX.Y.Z` on `main`.
4. The Release workflow builds a **draft**, attaches SBOMs, rewrites `checksums.txt`, attests 13 subjects, and verifies attestations. It does not publish.
5. Human GO publishes the draft.
6. The package workflow opens Scoop, Homebrew and Winget PRs. Jobs are idempotent if a matching PR or version already exists.
7. Release Done closes the milestone. Distribution Verified flips per channel after install smoke.

Rollback: before publication, revert on the release branch. After publication, never replace artifacts. A broken manager recipe stays on the previous version while it is fixed in its own repository; that does not undo Release Done.

Workflow and packaging-automation changes require two independent approvals. Mechanical version PRs in package repositories require one maintainer approval.

Pinned tools: GoReleaser 2.17.1, Syft 1.50.0, WingetCreate 1.12.13.0, actions/attest v4.2.1 (SHA), and SHA-pinned GitHub Actions. Every downloaded release tool must carry a version and a checksum in-tree.

CI proofs are retained for at least 90 days. Minimum evidence for a release: commit SHA, tag, checksums, SBOMs, attestations, CI results, approvals, distribution PR smokes, then per-channel install smokes when the channel is Verified.
