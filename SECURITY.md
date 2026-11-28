# Security policy

## Supported versions

Security fixes are provided for the latest released Dirloom version.

## Reporting a vulnerability

Please use the repository's private GitHub Security Advisory reporting flow. Do not open a public issue for a vulnerability that could expose filesystem data, overwrite files or escape the explicitly selected root.

Include the affected version and operating system, minimal reproduction steps, expected impact and any suggested mitigation. Maintainers should acknowledge a report within seven days and coordinate disclosure after a fix is available.

## Security model

Dirloom is a local command-line tool. It sends no telemetry and makes no network request. A normal run is read-only: the only writes are an explicit `--output`, which refuses symlink destinations and replaces the destination transactionally in its own directory without an unsafe delete-and-rename fallback, and `--copy`, which writes to the clipboard.

Dirloom starts two kinds of external programs, never through a shell: `git`, for the commands that read Git objects (below), and, for `--copy` on Linux and macOS, the system clipboard utility (`pbcopy`, `wl-copy`, `xclip`, `xsel`, or `clip.exe` under WSL). On Windows, `--copy` uses the clipboard API directly.

### What Dirloom reads

- **The selected tree.** Names, entry kinds and link targets under the selected root. Apart from the files listed next, Dirloom never reads the contents of project files. It never executes them, never parses project code, never follows a symlink or a junction while it scans, and never loads a script from the project.
- **A few files, by design:** the `.gitignore` files of the tree (unless `--no-gitignore`; a `.gitignore` that is a link is not read), the project and user configuration files (unless `--no-config`), a custom theme file selected with `--theme` or by configuration, and the snapshot files given to `verify` and `diff`. A configuration file is read through a link when the link leads to a regular file.
- **Git data,** for a `git:` source of `diff` and for `history`: committed objects, through the `git` executable as described below, and, for `history`, the shallow boundary list of the repository, read directly from the path Git reports for it.

Configuration and theme files cannot execute commands, include other files, interpolate environment variables, select another root or choose an output destination ([configuration security](docs/configuration.md#security-and-trust)). On a tree you do not trust, `--no-config` disables the project and user configuration files.

### How Dirloom runs Git

Only `dirloom diff` with a `git:` source and `dirloom history` run Git, and only to read committed objects ([Git Tree Source v1](docs/contracts/git-tree-source-v1.md#git-invocation)):

- the `git` executable comes from `PATH`, never from the current directory, and is started with an argument list, never through a shell;
- every invocation runs as `git -c protocol.allow=never …` with `GIT_NO_LAZY_FETCH=1`, `GIT_NO_REPLACE_OBJECTS=1`, `GIT_TERMINAL_PROMPT=0` and `GIT_OPTIONAL_LOCKS=0`: no transport, no lazy fetch of a missing object, no replacement object, no credential prompt;
- the only commands are `git version`, `git rev-parse` and `git cat-file`. None of them reads the index or the working tree, so no hook, fsmonitor or filter runs;
- a revision that is empty, starts with `-`, contains a NUL or a newline, or names an index entry is refused before Git runs.

Git 2.45 or newer is required, the first version with `GIT_NO_LAZY_FETCH`.

### Untrusted input

Dirloom treats snapshots, Git data, `.gitignore`, configuration and theme files, and file names as untrusted. Each is validated before use, and its size is bounded:

| Input | Limits | Over the limit |
| --- | --- | --- |
| snapshot file | 256 MiB, 2,000,000 nodes or elements of any JSON array, 64 levels of nesting, 4,096 ignore patterns, 64 required features; strict UTF-8 and JSON, no duplicate key; embedded fingerprint recomputed | `snapshot_limit_exceeded` or another `INVALID_SNAPSHOT` code, exit 3 |
| Git objects | tree object 32 MiB, `.gitignore` blob 1 MiB, link target 4,096 bytes, commit object 1 MiB, shallow list 16 MiB; an observation keeps at most 2,000,000 nodes and 256 MiB of paths | a `git_*` code, exit 5 |
| `.gitignore` in a directory | 1 MiB, lines under 64 KiB | `source_read_failure` in the structural commands; exit 1 for the tree output |
| configuration and theme files | regular files only, 1 MiB | above 1 MiB: usage error, exit 2; not a regular file: exit 1 for a configuration file, exit 2 for a theme file |
| file names | valid UTF-8 on Linux and macOS, valid UTF-16 on Windows; never repaired | `invalid_canonical_path` in the structural commands; the tree outputs refuse the name too (exit 1) |

These bounds limit the input, not the memory: a workflow at the limits needs several GiB, and Dirloom does not cap its own memory ([memory and resource limits](docs/reference/memory.md)). Named pipes, sockets and devices are refused by the structural commands, never opened. The [exit codes reference](docs/reference/exit-codes.md) lists every machine code. An unexpected panic is reported as an internal error, never as a success and without a stack trace ([unexpected internal failures](docs/reference/exit-codes.md#unexpected-internal-failures)). The untrusted-input parsers ran through a recorded fuzz campaign before the v0.4 release candidate ([campaign record](docs/release/v0.4.0-rc1-fuzz-campaign.md)).

## Release trust model

GitHub Releases of `dirloom/dirloom` is the only source of Dirloom binaries. Winget, Scoop and Homebrew install the same archives and check the same SHA-256 sums; they never rebuild them ([distribution](docs/distribution.md)).

Each release holds 13 files:

- six archives, one per platform: `dirloom_<OS>_<arch>.zip` for Windows, `.tar.gz` for Linux and macOS;
- six SBOMs, `<archive>.spdx.json`, one per archive: SPDX 2.3 JSON, generated by Syft, listing the Go modules and the Go standard library version built into the binary;
- `checksums.txt`, the SHA-256 of the 12 other files.

The release workflow, `.github/workflows/release.yml`, builds them on a GitHub-hosted runner when a version tag is pushed, records one SLSA build provenance attestation for all 13 files, verifies it, and leaves the release as a draft. A maintainer publishes it after an explicit review ([release workflow](docs/release-workflow.md)).

### Verifying a release

Download the files of one version, for example with `gh release download <version> --repo dirloom/dirloom`, then:

**1. Checksums.** Check the archive against `checksums.txt`:

```bash
grep "  dirloom_Linux_x86_64.tar.gz\$" checksums.txt | sha256sum -c -
```

On macOS, use `shasum -a 256 -c -` instead of `sha256sum -c -`. In PowerShell:

```powershell
$Asset = "dirloom_Windows_x86_64.zip"
$Expected = ((Get-Content checksums.txt) -like "*  $Asset" -split " ")[0]
(Get-FileHash $Asset -Algorithm SHA256).Hash -eq $Expected
```

A match proves that the archive is the one `checksums.txt` describes. It does not prove where either file came from: someone able to replace one could replace both.

**2. Attestation.** With the GitHub CLI, check that the file was built by the Dirloom release workflow, from the tag of that version, on a GitHub-hosted runner:

```bash
gh attestation verify dirloom_Linux_x86_64.tar.gz --repo dirloom/dirloom \
  --signer-workflow dirloom/dirloom/.github/workflows/release.yml \
  --source-ref refs/tags/<version> --deny-self-hosted-runners
```

The same command verifies an SBOM or `checksums.txt`: all 13 files are subjects of the attestation. A success means that the file's SHA-256 is listed in a SLSA provenance statement signed during the release run with a short-lived certificate from the public Sigstore instance, issued to the `release.yml` workflow of `dirloom/dirloom` running at that tag, and that the signature is recorded in Sigstore's public transparency log. `--deny-self-hosted-runners` also requires a GitHub-hosted runner. A file built anywhere else, or from another tag, fails.

**3. SBOM.** Read `<archive>.spdx.json` to see which dependency versions a binary contains, for example to check it against a vulnerability advisory. Verify the SBOM itself with steps 1 and 2 first.

### What this is not

**These checks are not an independent cryptographic signature of the binaries.** No key held by a Dirloom maintainer signs the archives or the binaries: there is no GPG, minisign or cosign key signature, no Windows Authenticode signature and no Apple notarization. The attestation is signed with a short-lived certificate that Sigstore issues to the GitHub Actions workflow identity. It proves which workflow, repository and tag built a file, on GitHub's infrastructure. It does not protect against a compromise of GitHub, of the `dirloom/dirloom` repository, or of an account able to push a version tag and run its release workflow: such a compromise could produce files that verify. The checksums and the SBOMs add integrity and transparency, not authenticity.
