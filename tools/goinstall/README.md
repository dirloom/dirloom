# `go install` experiment (A14)

This tool studies why `go install github.com/dirloom/dirloom/cmd/dirloom` fails, anomaly A14 of the
[v0.4.0-rc1 plan](../../.plans/v0.4.0-rc1-implementation-plan.md), and checks a published tag at gates G4/G5.
It is a separate Go module: it needs `golang.org/x/mod`, which Dirloom itself does not, and the Dirloom module
zip leaves it out.

## The problem

Seven tags, `v0.2.0`, `v0.3.0` to `v0.3.3`, `v0.4.0-a1` and `v0.4.0-a2`, hold plan files whose names contain
U+2014 or U+2019. A module zip refuses those characters, so proxy.golang.org has no module and sum.golang.org no
record for these versions. A tag cannot change.

`go install pkg@version` also reads the `go.mod` of the module's *latest* version, to look for a deprecation
notice, and `go list -m -versions` reads its retractions there. Latest is the highest release in the version
list, so `v0.3.3` when the list holds it; a pre-release is latest only when no release exists.

## Simulation

```bash
cd tools/goinstall
go run . simulate -go "$(GOTOOLCHAIN=go1.26.10 go env GOROOT)/bin/go" -out results/simulate.md -log results/simulate.log
```

It needs Git, takes about two minutes, and reaches no network. It clears every `GIT_*` variable it inherits,
and stops with status 3 if a version's zip fails for any reason other than the refused file names. On Windows,
keep `TMP` short: the module cache paths under it get long. It builds a throwaway repository with the
topology of the real tags (same names, same commit dates, the same refused file names), then serves it as:

- a module proxy on 127.0.0.1 that answers 404 for an unzippable version, like proxy.golang.org, with a version
  list that holds either every tag or the zippable tags only: proxy.golang.org listed the zippable tags only on
  2026-11-26 from 19:58Z, and every tag at 19:47Z that day and on 2026-11-28 from 02:35Z to 03:25Z at least;
- a checksum database on 127.0.0.1 that has no record for an unzippable version, like sum.golang.org;
- the repository itself for `GOPROXY=direct`, through a Git `url.insteadOf`.

Every command runs with a fresh `GOPATH`, module cache and `GOBIN`, `GOENV=off`, `GOTOOLCHAIN=local`, and
`HTTP(S)_PROXY` set to a closed local port. Six variants are compared: today's tags; `v0.4.0-rc1` tagged, its
`go.mod` without `retract`, retracting the seven unzippable versions, or retracting every earlier version; and
`v0.4.0` tagged after it, without and with `retract`.

### Fidelity

[`results/check-v0.4.0-a8.md`](results/check-v0.4.0-a8.md) runs the same commands against the real module on
2026-11-28 at 03:25Z, while proxy.golang.org listed every tag (15 versions at 03:25:32Z). Its two columns match
the "proxy listing every tag, then direct" and `GOPROXY=direct` columns of the simulation's first variant, command
for command: same exit statuses, same versions listed and selected, the same
`loading deprecation … v0.3.3: verifying go.mod … 404` failure. Only the server names and the text of their 404
responses differ.

## Results

[`results/simulate.md`](results/simulate.md), with Go 1.26.10 on Windows:

1. **A `retract` directive in the `go.mod` of `v0.4.0-rc1` changes nothing**, in any mode: the three `v0.4.0-rc1`
   variants give identical results. Go reads retractions in the `go.mod` of the latest version, `v0.3.3` or
   `v0.1.1` here, never in a pre-release while a release exists.
2. **`go install …@v0.4.0-rc1` installs only through a proxy that omits the unzippable tags.** Through a proxy
   that lists them, and with `GOPROXY=direct`, it fails: `loading deprecation for github.com/dirloom/dirloom:
   github.com/dirloom/dirloom@v0.3.3: verifying go.mod: … 404`. `go mod download …@v0.4.0-rc1` succeeds in every
   mode: the module of `v0.4.0-rc1` itself is sound.
3. **`@latest` fails, or installs `v0.1.1`.** It selects `v0.3.3` and fails to zip it, unless the list omits the
   unzippable tags; then it installs `v0.1.1`, from August 2026.
4. `go list -m -versions` fails like the install, because it loads the retractions of `v0.3.3`; with `-retracted`
   it lists the versions.
5. A proxy without the `direct` fallback fails as soon as it lists the unzippable tags.
6. **Once `v0.4.0` is tagged with a zippable module, everything installs in every mode**, `v0.4.0-rc1` included,
   and `@latest` installs `v0.4.0`. A `retract` block in the `go.mod` of `v0.4.0` then only hides the seven
   versions from `go list -m -versions`.

### What this means for RC1

Without rewriting a tag and without an intermediate stable version, no change to the RC1 tree makes
`go install …@v0.4.0-rc1` reliable: the outcome depends on the version list of proxy.golang.org, which Dirloom
does not control, and `GOPROXY=direct` always fails until a zippable stable release exists. The repair is
`v0.4.0` itself, provided its module zips. What RC1 can do:

- keep every commit zippable: CI runs [`check-module-zip.sh`](../../.github/scripts/check-module-zip.sh), which has
  `go` build the module zip of the commit with the proxy's code path, and the release workflow runs it again on
  the tag before it builds the draft;
- not rely on `retract`, which the simulation shows has no effect;
- check the published tag at G4/G5, below, and leave the `@latest` outcome to the CTO.

## Check of a published tag (G4/G5)

```bash
cd tools/goinstall
go run . check -version v0.4.0-rc1 -go "$(GOTOOLCHAIN=go1.26.10 go env GOROOT)/bin/go" \
  -out results/check-v0.4.0-rc1-<os>.md -log results/check-v0.4.0-rc1-<os>.log
```

On Windows PowerShell, pass `-go "$(go env GOROOT)\bin\go.exe"` from a shell where `$env:GOTOOLCHAIN='go1.26.10'`.

It runs, through `GOPROXY=https://proxy.golang.org,direct` and then with `GOPROXY=direct`, each command with a fresh
`GOPATH`, module cache and `GOBIN`, `GOENV=off`, `GOFLAGS=-modcacherw` only, no `GONOSUMDB`, `GOPRIVATE`,
`GONOPROXY` or `GOINSECURE`, and the Git global configuration ignored. The Git system configuration and the
network proxy settings of the host stay, so that the host can reach GitHub:

1. `go list -m -versions -retracted github.com/dirloom/dirloom`, the versions the source lists;
2. `go list -m -versions github.com/dirloom/dirloom`;
3. `go list -m github.com/dirloom/dirloom@latest`, the version `@latest` selects;
4. `go mod download -json github.com/dirloom/dirloom@v0.4.0-rc1`;
5. `go install github.com/dirloom/dirloom/cmd/dirloom@v0.4.0-rc1`, then `go version -m` and `dirloom --version` of
   the installed binary;
6. `go install github.com/dirloom/dirloom/cmd/dirloom@latest`, then the same.

It exits 1 when the named version does not install, with the expected module version and binary output, in
both modes, 2 on a usage error, and 3 when the tool itself fails, keeping its work directory and writing its
log. Each go command is stopped after 10 minutes. `@latest` does not have to install the release candidate: its outcome is recorded for the CTO.
The CTO accepted A14 for RC1 on 2026-11-28: an outcome that reproduces the simulation does not block the publication, and any other outcome goes to the CTO. Restoring `go install` is a condition of `v0.4.0`.

The tag is public from G3, and querying the proxy for it records `v0.4.0-rc1` in proxy.golang.org and
sum.golang.org, as any user can.
