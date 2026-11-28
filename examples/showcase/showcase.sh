#!/usr/bin/env bash
# Dirloom showcase: Structural Version Control from end to end, reproducibly.
# The walkthrough is docs/showcase.md; showcase.ps1 is the same scenario in
# PowerShell.
#
# Usage: showcase.sh [work-directory]
#
#   work-directory  where the project is created. It must be empty or not
#                   exist yet. Default: a new temporary directory, which is
#                   kept and named on stderr at the end, even on a failure.
#   DIRLOOM         environment variable: the dirloom executable to run.
#                   Default: dirloom on PATH.
#
# Requires Git 2.45 or newer. The script reads no configuration of the host:
# Dirloom runs with --no-config where a command takes it, and Git reads no
# configuration, ignore or attributes file and no template, with a fixed
# identity and fixed dates. Commit ids and outputs are therefore the same on
# every machine.
#
# stdout is the transcript: each dirloom command, its output and its exit
# code. The script exits 1 as soon as a dirloom command does not exit as
# documented in docs/reference/exit-codes.md, or writes to stderr.

set -euo pipefail

dirloom=${DIRLOOM:-dirloom}

if [ $# -gt 1 ]; then
  echo "usage: $0 [work-directory]" >&2
  exit 2
fi
# Paths, relative or not, must stay valid once the script changes directory.
case $dirloom in
  */*) dirloom=$(CDPATH='' cd -P -- "$(dirname -- "$dirloom")" && pwd)/$(basename -- "$dirloom") ;;
esac
if [ $# -eq 1 ]; then
  mkdir -p -- "$1"
  work=$(CDPATH='' cd -P -- "$1" && pwd)
  if [ -n "$(ls -A -- "$work")" ]; then
    echo "showcase: $1 is not empty" >&2
    exit 2
  fi
else
  work=$(mktemp -d "${TMPDIR:-/tmp}/dirloom-showcase.XXXXXX")
  trap 'echo "showcase: the project is in $work/shop" >&2' EXIT
fi
project=$work/shop

# Git reads no configuration of the host and records a fixed identity: with
# the fixed dates of commit_at, the commit ids are the same everywhere.
# XDG_CONFIG_HOME also hides the default core.excludesFile and
# core.attributesFile, and git init below copies no template.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY \
  GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR GIT_NAMESPACE \
  GIT_CONFIG GIT_CONFIG_PARAMETERS GIT_CONFIG_COUNT
export GIT_CONFIG_NOSYSTEM=1 GIT_ATTR_NOSYSTEM=1
export GIT_CONFIG_GLOBAL="$work/gitconfig"
export XDG_CONFIG_HOME="$work/xdg"
: >"$GIT_CONFIG_GLOBAL"
export GIT_AUTHOR_NAME="Dirloom Showcase" GIT_AUTHOR_EMAIL="showcase@dirloom.invalid"
export GIT_COMMITTER_NAME="Dirloom Showcase" GIT_COMMITTER_EMAIL="showcase@dirloom.invalid"

# commit_at <date> <message>: commit the index at a fixed date.
commit_at() {
  GIT_AUTHOR_DATE=$1 GIT_COMMITTER_DATE=$1 git commit --quiet --message "$2"
}

# write <path> <line>: write one line ending with LF, the same bytes on every
# system.
write() {
  mkdir -p "$(dirname "$1")"
  printf '%s\n' "$2" >"$1"
}

# section <name>: start a part of the transcript.
section() {
  printf '== %s ==\n' "$1"
}

# run <exit> <arguments...>: run dirloom and print the command, its output
# and its exit code. Stop unless it exits with <exit> and an empty stderr.
run() {
  local want=$1 code=0
  shift
  printf '$ dirloom %s\n' "$*"
  "$dirloom" "$@" 2>"$work/stderr" || code=$?
  printf '[exit %d]\n\n' "$code"
  if [ "$code" -ne "$want" ] || [ -s "$work/stderr" ]; then
    echo "showcase: 'dirloom $*' exited $code, expected $want" >&2
    cat "$work/stderr" >&2
    exit 1
  fi
}

# 1. A small reference project, committed on main.
mkdir "$project"
cd "$project"
write README.md '# shop'
write go.mod 'module example.com/shop'
write cmd/shop/main.go 'package main'
write docs/architecture.md '# Architecture'
write internal/billing/invoice.go 'package billing'
write internal/billing/tax.go 'package billing'
write internal/utils/strings.go 'package utils'
write payment/api/handler.go 'package api'
write payment/service.go 'package payment'
git init --quiet --template= --initial-branch=main --object-format=sha1
git add --all
commit_at 2026-01-05T09:00:00Z "Lay out the shop service"

section project
run 0 --no-config --style ascii --color never

# 2. Snapshot the structure and commit the snapshot with the code.
section snapshot
run 0 snapshot --no-config --output architecture.dlm.json
run 0 verify architecture.dlm.json
git add architecture.dlm.json
commit_at 2026-01-05T10:00:00Z "Record the architecture snapshot"

# 3. Refactor on a branch: move and rename files and directories.
git switch --quiet --create refactor/layout
# >>> refactor
git mv payment internal/payment
git mv internal/utils internal/textutil
mkdir internal/tax
git mv internal/billing/tax.go internal/tax/tax.go
git mv docs/architecture.md docs/design.md
# <<< refactor

# 4. The snapshot no longer matches: verify exits 1.
section verify
run 1 verify architecture.dlm.json

# 5. Diff names the changes and the moves: exit 1.
section diff
run 1 diff snapshot:architecture.dlm.json live:.

# 6. Commit the refactoring, then a content-only change, and compare the
# committed trees of the two branches.
commit_at 2026-01-05T11:00:00Z "Move payment under internal"
write README.md '# shop: invoices, taxes and payments'
git add README.md
commit_at 2026-01-05T12:00:00Z "Describe the service"

section git
run 1 diff git:main git:refactor/layout

# 7. The structural history of the branch.
section history
run 0 history
