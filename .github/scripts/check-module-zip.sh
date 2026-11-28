#!/usr/bin/env bash
set -euo pipefail

# Check that the Go module proxy can build the module zip of the checked-out
# commit, so that a tag on it can be installed with `go install` (anomaly A14 of
# the v0.4.0-rc1 plan: seven tags hold file names that module zips refuse, and
# the proxy has no module for them).
#
# The go command builds the zip itself, with the code path of the proxy
# (`go mod download` in direct mode). url.insteadOf points the module's GitHub
# URL at this checkout, so nothing is fetched from GitHub; GONOSUMDB applies to
# this command only, whose pseudo-version the checksum database cannot know.
# Run it from anywhere inside the repository.

root="$(git rev-parse --show-toplevel)"
rev="$(git -C "$root" rev-parse HEAD)"
module="$(cd "$root" && GOENV=off GOFLAGS= GOWORK=off go list -m)"
url="file://${root}"
if [[ "$root" != /* ]]; then
  url="file:///${root}" # Windows: file:///C:/...
fi

tmp="$(mktemp -d)"
# cmd/go fetches refs/heads/* and refs/tags/*: make a detached HEAD, such as a
# pull request merge commit, reachable.
ref="refs/heads/module-zip-check-$$"
trap 'git -C "$root" update-ref -d "$ref" 2>/dev/null || true; rm -rf "$tmp"' EXIT
git -C "$root" update-ref "$ref" "$rev"

cd "$tmp"
GIT_CONFIG_COUNT=1 \
  GIT_CONFIG_KEY_0="url.${url}.insteadOf" \
  GIT_CONFIG_VALUE_0="https://${module}" \
  GIT_TERMINAL_PROMPT=0 \
  GOENV=off GOFLAGS=-modcacherw GOWORK=off \
  GOPROXY=direct GONOSUMDB="$module" GOPRIVATE= GONOPROXY= \
  GOPATH="$tmp/gopath" GOMODCACHE="$tmp/modcache" \
  go mod download -json "${module}@${rev}"
echo "The module zip of ${rev} builds."
