#!/usr/bin/env bash
# Patch Dirloom Homebrew cask release fields without rewriting packaging logic.
# The tap owns Casks/dirloom.rb. The publisher may change only:
#   - version
#   - sha256 Darwin arm64
#   - sha256 Darwin x86_64
#   - sha256 Linux arm64
#   - sha256 Linux x86_64
set -euo pipefail

is_sha256() {
  [[ "$1" =~ ^[a-fA-F0-9]{64}$ ]]
}

is_cask_version() {
  [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z]+)*$ ]]
}

patch_homebrew_cask() {
  local file="$1"
  local version="$2"
  local darwin_arm="$3"
  local darwin_x64="$4"
  local linux_arm="$5"
  local linux_x64="$6"
  local tmp

  if [[ ! -f "$file" ]]; then
    echo "Homebrew cask not found: ${file}" >&2
    return 1
  fi
  if ! is_cask_version "$version"; then
    echo "Invalid cask version: ${version}" >&2
    return 1
  fi
  for digest in "$darwin_arm" "$darwin_x64" "$linux_arm" "$linux_x64"; do
    if ! is_sha256 "$digest"; then
      echo "Invalid sha256: ${digest}" >&2
      return 1
    fi
  done

  tmp="$(mktemp)"
  if ! awk -v version="$version" \
    -v h1="$darwin_arm" \
    -v h2="$darwin_x64" \
    -v h3="$linux_arm" \
    -v h4="$linux_x64" '
    BEGIN { hashes[1]=h1; hashes[2]=h2; hashes[3]=h3; hashes[4]=h4; hi=1; ver=0 }
    /^  version "/ {
      print "  version \"" version "\""
      ver++
      next
    }
    /sha256 arm:/ || /^[[:space:]]+intel:[[:space:]]+"/ {
      if (hi <= 4 && match($0, /"[a-fA-F0-9]{64}"/)) {
        $0 = substr($0, 1, RSTART - 1) "\"" hashes[hi] "\"" substr($0, RSTART + RLENGTH)
        hi++
      }
    }
    { print }
    END {
      if (ver != 1) {
        print "expected exactly one version stanza" > "/dev/stderr"
        exit 1
      }
      if (hi != 5) {
        print "expected exactly four sha256 hashes in the cask" > "/dev/stderr"
        exit 1
      }
    }
  ' "$file" >"$tmp"; then
    rm -f "$tmp"
    return 1
  fi
  mv "$tmp" "$file"
}

is_allowed_cask_release_diff_line() {
  local line="$1"
  case "$line" in
  '--- '* | '+++ '* | '@@ '* | diff\ * | index\ *)
    return 0
    ;;
  esac
  [[ "$line" =~ ^[-+][[:space:]]+version[[:space:]]+\" ]] && return 0
  [[ "$line" =~ ^[-+][[:space:]]+sha256[[:space:]]+arm: ]] && return 0
  [[ "$line" =~ ^[-+][[:space:]]+intel:[[:space:]]+\" ]] && return 0
  return 1
}

assert_homebrew_cask_release_fields_only() {
  local before="$1"
  local after="$2"
  local diff_out line

  if [[ ! -f "$before" || ! -f "$after" ]]; then
    echo "cask snapshots missing for field-only assert" >&2
    return 1
  fi
  if cmp -s "$before" "$after"; then
    echo "Homebrew cask patch produced no changes" >&2
    return 1
  fi

  diff_out="$(diff -u "$before" "$after" || true)"
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    [[ "$line" == ---* || "$line" == +++* || "$line" == @@* ]] && continue
    if [[ "$line" == -* || "$line" == +* ]]; then
      if ! is_allowed_cask_release_diff_line "$line"; then
        echo "Homebrew cask patch changed non-release fields:" >&2
        echo "$line" >&2
        return 1
      fi
    fi
  done <<<"$diff_out"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  case "${1:-}" in
  --assert-fields-only)
    assert_homebrew_cask_release_fields_only "$2" "$3"
    ;;
  *)
    patch_homebrew_cask "$@"
    ;;
  esac
fi
