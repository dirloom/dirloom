#!/usr/bin/env bash
# SemVer 2.0.0 precedence for the package-manager updaters.
#
# https://semver.org/spec/v2.0.0.html#spec-item-11. Versions carry no leading
# "v" (VERSION="${TAG#v}"). GNU `sort -V` is not SemVer: it puts 0.4.0 before
# 0.4.0-rc1, so a manifest at a release candidate refused the stable release
# (anomaly A6 of the v0.4.0-rc1 plan).
#
# Under SemVer, alphanumeric pre-release identifiers compare in ASCII order:
# 0.4.0-rc10 precedes 0.4.0-rc9, while 0.4.0-rc.10 follows 0.4.0-rc.9.
#
# Bash 3.2 compatible (macOS). Character classes are spelled out and
# identifiers compare by character code, so the result never depends on the
# locale.
#
# The block between the BEGIN and END lines is copied verbatim into
# packaging/scoop/update.yml; internal/packaging tests keep the copies equal.

# BEGIN semver.sh
_SEMVER_DIGITS='0123456789'
_SEMVER_LETTERS='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz'

# semver_valid VERSION: succeeds when VERSION is a SemVer 2.0.0 version.
semver_valid() {
  local d="$_SEMVER_DIGITS" l="$_SEMVER_LETTERS"
  local num="(0|[123456789][$d]*)"
  local pre="(0|[123456789][$d]*|[$d]*[$l-][$d$l-]*)"
  local build="[$d$l-]+"
  local re="^$num\\.$num\\.$num(-$pre(\\.$pre)*)?(\\+$build(\\.$build)*)?\$"
  [[ "$1" =~ $re ]]
}

# The helpers below set _semver_cmp to -1, 0 or 1 instead of printing it, so
# that a comparison starts no subshell.

# _semver_ascii_cmp A B: compares character codes.
_semver_ascii_cmp() {
  local a="$1" b="$2" i ca cb
  for ((i = 0; i < ${#a} && i < ${#b}; i++)); do
    if [[ "${a:i:1}" != "${b:i:1}" ]]; then
      printf -v ca '%d' "'${a:i:1}"
      printf -v cb '%d' "'${b:i:1}"
      if ((ca < cb)); then _semver_cmp=-1; else _semver_cmp=1; fi
      return 0
    fi
  done
  if ((${#a} < ${#b})); then
    _semver_cmp=-1
  elif ((${#a} > ${#b})); then
    _semver_cmp=1
  else
    _semver_cmp=0
  fi
}

# _semver_num_cmp A B: compares two numbers without leading zeros, of any
# length.
_semver_num_cmp() {
  if ((${#1} < ${#2})); then
    _semver_cmp=-1
  elif ((${#1} > ${#2})); then
    _semver_cmp=1
  else
    _semver_ascii_cmp "$1" "$2"
  fi
}

_semver_is_num() {
  [[ -n "$1" && "$1" != *[!0123456789]* ]]
}

# _semver_precedence A B: compares two valid versions.
_semver_precedence() {
  local a="${1%%+*}" b="${2%%+*}"
  local a_pre="" b_pre="" i n
  local -a a_ids b_ids
  if [[ "$a" == *-* ]]; then a_pre="${a#*-}"; fi
  if [[ "$b" == *-* ]]; then b_pre="${b#*-}"; fi
  IFS=. read -r -a a_ids <<<"${a%%-*}"
  IFS=. read -r -a b_ids <<<"${b%%-*}"
  for i in 0 1 2; do
    _semver_num_cmp "${a_ids[i]}" "${b_ids[i]}"
    if [[ "$_semver_cmp" != 0 ]]; then return 0; fi
  done
  # A version without a pre-release follows the same version with one.
  if [[ -z "$a_pre" && -z "$b_pre" ]]; then
    _semver_cmp=0
    return 0
  elif [[ -z "$a_pre" ]]; then
    _semver_cmp=1
    return 0
  elif [[ -z "$b_pre" ]]; then
    _semver_cmp=-1
    return 0
  fi
  IFS=. read -r -a a_ids <<<"$a_pre"
  IFS=. read -r -a b_ids <<<"$b_pre"
  n=${#a_ids[@]}
  if ((${#b_ids[@]} < n)); then n=${#b_ids[@]}; fi
  for ((i = 0; i < n; i++)); do
    if _semver_is_num "${a_ids[i]}" && _semver_is_num "${b_ids[i]}"; then
      _semver_num_cmp "${a_ids[i]}" "${b_ids[i]}"
    elif _semver_is_num "${a_ids[i]}"; then
      _semver_cmp=-1 # numeric identifiers precede alphanumeric ones
    elif _semver_is_num "${b_ids[i]}"; then
      _semver_cmp=1
    else
      _semver_ascii_cmp "${a_ids[i]}" "${b_ids[i]}"
    fi
    if [[ "$_semver_cmp" != 0 ]]; then return 0; fi
  done
  # Equal so far: the longer list of identifiers follows.
  _semver_num_cmp "${#a_ids[@]}" "${#b_ids[@]}"
}

# semver_compare A B: prints -1, 0 or 1 when A has lower, equal or higher
# precedence than B. Build metadata is ignored. Returns 2 when A or B is not a
# SemVer 2.0.0 version.
semver_compare() {
  local v
  for v in "$1" "$2"; do
    if ! semver_valid "$v"; then
      echo "not a SemVer 2.0.0 version: '${v}'" >&2
      return 2
    fi
  done
  _semver_precedence "$1" "$2"
  echo "$_semver_cmp"
}

# semver_update_decision CURRENT NEW: prints the decision of a package updater
# whose manifest is at CURRENT (empty when there is no manifest yet) and that
# is asked to move to NEW:
#   same       CURRENT and NEW are the same text: nothing to do;
#   newer      NEW has higher precedence than CURRENT, or there is no CURRENT;
#   not-newer  NEW has lower or equal precedence: the updater must refuse.
# Returns 2 when CURRENT or NEW is not a SemVer 2.0.0 version.
semver_update_decision() {
  if [[ -n "$1" ]] && ! semver_valid "$1"; then
    echo "not a SemVer 2.0.0 version: '${1}'" >&2
    return 2
  fi
  if ! semver_valid "$2"; then
    echo "not a SemVer 2.0.0 version: '${2}'" >&2
    return 2
  fi
  if [[ -z "$1" ]]; then
    echo newer
  elif [[ "$1" == "$2" ]]; then
    echo same
  else
    _semver_precedence "$1" "$2"
    if [[ "$_semver_cmp" == -1 ]]; then echo newer; else echo not-newer; fi
  fi
}
# END semver.sh

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  set -euo pipefail
  case "${1:-}" in
  valid)
    semver_valid "${2:-}"
    ;;
  compare)
    semver_compare "${2:-}" "${3:-}"
    ;;
  update-decision)
    semver_update_decision "${2:-}" "${3:-}"
    ;;
  *)
    echo "usage: semver.sh valid VERSION | compare A B | update-decision CURRENT NEW" >&2
    exit 2
    ;;
  esac
fi
