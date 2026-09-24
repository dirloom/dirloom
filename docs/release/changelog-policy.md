# CHANGELOG.md policy

`CHANGELOG.md` is the unique source of truth for Dirloom release notes.
This document is normative for every publishable tag, including alphas and
release candidates.

## 1. Source of truth

Every user-visible release note for a tag lives in `CHANGELOG.md` under the
matching version heading. The GitHub Release body is derived from that
section. Agents and maintainers must not invent a parallel release narrative
in the GitHub UI, in GoReleaser commit logs, or in ad-hoc markdown files.

Any divergence between `CHANGELOG.md` and the published GitHub Release body
is a release defect.

## 2. Structure

Each version entry uses Keep a Changelog headings in this fixed order:

```text
## [X.Y.Z] - YYYY-MM-DD

<summary>

### Added
### Changed
### Fixed
### Security
```

Rules:

- The title must be `## [X.Y.Z] - YYYY-MM-DD` with a real calendar date.
- Never use the word `Unreleased` in a version title that is being frozen or
  tagged.
- Include only the sections that have content. Omit empty sections.
- Do not write placeholders such as `None.` or `N/A`.
- Section titles are restricted to `Added`, `Changed`, `Fixed`, and
  `Security`. Unknown `###` headings are invalid for an extractable release
  entry.
- Within an entry, present sections only in the order
  Added → Changed → Fixed → Security.

## 3. Content rules

- Write user-visible outcomes, not internal commit lists.
- Prefer imperative, present-tense bullets (`Add …`, `Fix …`, `Change …`).
- Group related work; do not duplicate the same fact across sections.
- Keep the summary short: one or two sentences that state the increment’s
  product intent and its boundary.

### Alphas and release candidates

For `aN` and `rcN` increments, the summary must keep an explicit boundary
sentence of the form:

```text
This increment does not add …
```

That sentence states what the rest of the milestone still withholds.

For a stable milestone tag such as `v0.4.0`, drop that boundary sentence and
summarize the completed milestone as a whole.

## 4. Language

Release notes are written in English. Code comments and policy docs follow
the same language rule. Product CLI output language is unchanged by this
policy.

## 5. Compatibility

Call out compatibility-affecting changes in `Changed` (or `Fixed` when the
change is a corrective contract repair). Do not bury breaking or
contract-visible changes only in prose outside the structured sections.

Optional compatibility narrative may appear in the summary. Do not invent a
`### Compatibility` heading inside `CHANGELOG.md`; that heading is reserved
for the GitHub release-notes template’s editorial sections, which are
composed from the CHANGELOG entry rather than stored as extra `###` titles.

## 6. Freeze

On a release branch `release/vX.Y.Z` at freeze:

- `[X.Y.Z]` exists with a real date (not `Unreleased`).
- `[Unreleased]` exists and is empty (heading only, no bullets or sections).
- The compare link for `[Unreleased]` points at `vX.Y.Z...HEAD`.
- The compare link for `[X.Y.Z]` points at `previous...vX.Y.Z`.
- The version entry is extractable and structurally valid under this policy.

Freeze commits may adjust only the changelog, product status, snapshot and
smoke evidence required by the active release workflow. They must not reopen
product scope.

## 7. GitHub Release

GoReleaser must not generate release notes from Git history. After the draft
release exists, CI extracts the matching `CHANGELOG.md` section with
`go run ./cmd/release-artifacts notes` and replaces the draft body with that
exact markdown. The draft remains unpublished until human GO.

## 8. Immutability

After a tag is published, do not rewrite that version’s CHANGELOG entry to
change meaning. Corrective user-visible fixes ship in a later version.
Historical older entries are not mass-reformatted when this policy evolves;
the strict validator applies to the entry extracted for a tag or release
branch, not to a full-file rewrite of past releases.

## Normative example (v0.4.0-a2 shape)

The block below is the normative shape for an alpha freeze entry. It is an
example for authoring and validation fixtures. It does not modify the frozen
file on `release/v0.4.0-a2`.

```markdown
## [0.4.0-a2] - 2026-09-24

CHANGE alpha: persistent self-verifying structural snapshots on top of Fingerprint
v1. This increment does not add verify or diff.

### Added

- Add `dirloom snapshot` to persist Snapshot Schema v1 JSON (stdout or transactional `--output`).
- Add Snapshot Artifact Projection v1, Capture Semantics v1, deterministic JSON encode/decode and a shared self-verifying validator.
- Publish snapshot schema/ADR/reference docs plus large-snapshot benchmark baseline.

### Changed

- Document latest published `v0.4.0-a1` and the frozen prerelease candidate `v0.4.0-a2` on `release/v0.4.0-a2`.
```

## Related documents

- [Release notes template](release-notes-template.md)
- [Release workflow](../release-workflow.md)
