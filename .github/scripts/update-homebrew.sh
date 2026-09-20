#!/usr/bin/env bash
set -euo pipefail

# Open an idempotent version PR against dirloom/homebrew-tap.
# Dirloom/dirloom is the unique Homebrew publisher. The tap owns Casks/dirloom.rb;
# this script patches only version and the four archive SHA-256 fields.
# Requires GH_TOKEN (bot) with contents:write and pull-requests:write on that repo.

TAG="${TAG:?tag is required}"
VERSION="${TAG#v}"
TAP_REPO="${HOMEBREW_TAP_REPO:-dirloom/homebrew-tap}"
ROOT_REPO="${GITHUB_REPOSITORY:-dirloom/dirloom}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/patch-homebrew-cask.sh
source "${SCRIPT_DIR}/lib/patch-homebrew-cask.sh"

BOT_NAME="dirloom-package-mgr"
BOT_EMAIL="330109029+dirloom-package-mgr@users.noreply.github.com"

if [[ -z "${GH_TOKEN:-}" ]]; then
  echo "GH_TOKEN is required" >&2
  exit 1
fi

gh auth setup-git

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
gh release download "$TAG" --repo "$ROOT_REPO" --pattern checksums.txt --dir "$work"

hash() { awk -v name="$1" '$2==name{print $1}' "$work/checksums.txt"; }

darwin_arm="$(hash dirloom_Darwin_arm64.tar.gz)"
darwin_x64="$(hash dirloom_Darwin_x86_64.tar.gz)"
linux_arm="$(hash dirloom_Linux_arm64.tar.gz)"
linux_x64="$(hash dirloom_Linux_x86_64.tar.gz)"
test -n "$darwin_arm" && test -n "$darwin_x64" && test -n "$linux_arm" && test -n "$linux_x64"

for archive in \
  dirloom_Darwin_arm64.tar.gz \
  dirloom_Darwin_x86_64.tar.gz \
  dirloom_Linux_arm64.tar.gz \
  dirloom_Linux_x86_64.tar.gz
do
  gh release download "$TAG" --repo "$ROOT_REPO" --pattern "$archive" --dir "$work"
  actual="$(sha256sum "$work/$archive" | awk '{print $1}')"
  expected="$(hash "$archive")"
  test "$actual" = "$expected"
done

if ! gh repo view "$TAP_REPO" >/dev/null 2>&1; then
  echo "Homebrew tap ${TAP_REPO} does not exist yet. Create it before publishing ${TAG}."
  exit 1
fi

gh repo clone "$TAP_REPO" "$work/tap"
cd "$work/tap"
git fetch origin main
git checkout --track origin/main 2>/dev/null || git checkout main

cask="Casks/dirloom.rb"
if [[ ! -f "$cask" ]]; then
  echo "Homebrew tap is missing ${cask}; the tap owns the cask and the publisher will not create it." >&2
  exit 1
fi

current="$(grep -E '^  version "' "$cask" | head -n1 | cut -d'"' -f2 || true)"
if [[ "$current" == "$VERSION" ]]; then
  echo "Homebrew cask already at ${VERSION}"
  exit 0
fi

branch="dirloom-${VERSION}"
count="$(gh pr list --repo "$TAP_REPO" --head "$branch" --state open --json number --jq 'length')"
if [[ "${count:-0}" -ge 1 ]]; then
  echo "Homebrew PR already open for ${VERSION}"
  exit 0
fi

if git ls-remote --exit-code --heads origin "$branch" >/dev/null 2>&1; then
  tip="$(git ls-remote origin "refs/heads/${branch}" | awk '{print $1}')"
  email="$(gh api "repos/${TAP_REPO}/commits/${tip}" --jq .commit.author.email)"
  message="$(gh api "repos/${TAP_REPO}/commits/${tip}" --jq .commit.message)"
  if [[ "$email" == "$BOT_EMAIL" || "$message" == chore\(cask\):\ update\ dirloom\ to\ * ]]; then
    echo "Deleting orphan automation branch ${branch}"
    git push origin --delete "$branch"
  else
    echo "Remote branch ${branch} exists without a PR and is not a package-bot cask bump; refusing to overwrite." >&2
    exit 1
  fi
fi

git checkout -B "$branch" origin/main
before="${work}/dirloom.rb.before"
cp "$cask" "$before"
patch_homebrew_cask "$cask" "$VERSION" "$darwin_arm" "$darwin_x64" "$linux_arm" "$linux_x64"
assert_homebrew_cask_release_fields_only "$before" "$cask"

git add -- "$cask"
changed_files="$(git diff --cached --name-only)"
if [[ "$changed_files" != "$cask" ]]; then
  echo "Homebrew publisher staged unexpected paths:" >&2
  printf '%s\n' "$changed_files" >&2
  exit 1
fi
if git diff --cached --quiet; then
  echo "Homebrew cask already at ${VERSION}"
  exit 0
fi
git -c user.name="$BOT_NAME" -c user.email="$BOT_EMAIL" commit -m "chore(cask): update dirloom to ${VERSION}"
git push -u origin "$branch"
gh pr create --repo "$TAP_REPO" --head "$branch" --title "dirloom ${VERSION}" --body "Update the Dirloom cask to GitHub Release ${TAG}. Binaries are the official archives; Dirloom is not rebuilt. Hashes were recalculated independently from checksums.txt. Packaging stanzas in Casks/dirloom.rb are owned by the tap and are not rewritten."
