# Security policy

## Supported versions

Security fixes are provided for the latest released Dirloom version.

## Reporting a vulnerability

Please use the repository's private GitHub Security Advisory reporting flow. Do not open a public issue for a vulnerability that could expose filesystem data, overwrite files or escape the explicitly selected root.

Include the affected version and operating system, minimal reproduction steps, expected impact and any suggested mitigation. Maintainers should acknowledge a report within seven days and coordinate disclosure after a fix is available.

## Security model

Dirloom is local-only and performs no telemetry or network requests. A normal inspection is read-only. The sole write operation is an explicit `--output`, which refuses symlink destinations and uses same-directory transactional replacement without an unsafe delete-and-rename fallback.

Dirloom never executes inspected files, parses project code, follows descendant symlinks or loads scripts from the inspected project.
