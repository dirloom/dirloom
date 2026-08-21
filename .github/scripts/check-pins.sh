#!/usr/bin/env bash
set -euo pipefail

# Fail when a GitHub Action is not pinned to a full 40-character SHA, or when a
# downloaded release tool is missing its deterministic version and checksum.

root="$(cd "$(dirname "$0")/../.." && pwd)"
fail=0

while IFS= read -r line; do
  if [[ "$line" =~ uses:[[:space:]]*[^[:space:]#]+@(main|master|latest) ]]; then
    echo "Floating GitHub Action ref is not allowed: ${line}"
    fail=1
    continue
  fi
  if [[ "$line" =~ uses:[[:space:]]*[^[:space:]#]+@ ]]; then
    if [[ ! "$line" =~ @[0-9a-f]{40} ]]; then
      echo "GitHub Action is not SHA-pinned: ${line}"
      fail=1
    fi
  fi
done < <(grep -R --include='*.yml' --include='*.yaml' -n 'uses:' "${root}/.github/workflows" || true)

syft="${root}/.github/scripts/install-syft.sh"
if ! grep -q 'SYFT_VERSION="1.50.0"' "$syft" || ! grep -q 'SYFT_SHA256="bf7b29ff57f06da30918266a0e1c2885a8f99784798d1bdb1628886aa015d788"' "$syft"; then
  echo "Syft must stay pinned to v1.50.0 with a verified SHA-256 in install-syft.sh"
  fail=1
fi

winget="${root}/.github/scripts/update-winget.ps1"
if ! grep -q '1.12.13.0' "$winget" || ! grep -qi '24042BD37915805615E6CF969AC57C6439124C3FE85823327F5F3FB24BD9FFEA' "$winget"; then
  echo "WingetCreate must stay pinned to 1.12.13.0 with a verified SHA-256 in update-winget.ps1"
  fail=1
fi

if ! grep -q 'version: v2.17.1' "${root}/.github/workflows/ci.yml" || ! grep -q 'version: v2.17.1' "${root}/.github/workflows/release.yml"; then
  echo "GoReleaser must stay pinned to v2.17.1 in CI and release workflows"
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi
echo "Release-tool and GitHub Action pins are present."
