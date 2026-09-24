# Release notes template

Use this template when drafting the GitHub Release body that must match the
`CHANGELOG.md` entry for the tag. Empty sections are omitted. Do not write
`None.` Do not invent filler. When the CHANGELOG entry uses `### Added`,
`### Changed`, `### Fixed`, and `### Security`, keep that fixed order and
omit any of those four that have no bullets. Editorial sections below
(`Highlights`, `Compatibility`, `Scope`, `Release artifacts`) are optional
prose; omit each entirely when unused.

```markdown
## Highlights

<One or two sentences. For aN/rcN, keep the boundary sentence
"This increment does not add …". For a stable milestone, summarize the
completed milestone and drop that boundary sentence.>

## Added

- <user-visible addition>

## Changed

- <user-visible change>

## Fixed

- <user-visible fix>

## Security

- <security-relevant change>

## Compatibility

<Optional contract or upgrade notes. Omit this heading when unused.>

## Scope

<Optional boundary of what this tag includes or deliberately excludes.
Omit this heading when unused.>

## Release artifacts

<Optional pointer to the 13-artifact inventory, checksums, SBOMs, and
attestations. Omit this heading when unused.>

Full release notes: see the `[X.Y.Z]` entry in
[`CHANGELOG.md`](https://github.com/dirloom/dirloom/blob/vX.Y.Z/CHANGELOG.md).
```

The release workflow extracts the matching `CHANGELOG.md` section and sets
that markdown as the draft GitHub Release body. Prefer writing the
CHANGELOG entry so that extraction already yields the notes reviewers will
publish.
