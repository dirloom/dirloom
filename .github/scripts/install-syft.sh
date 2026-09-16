#!/usr/bin/env bash
set -euo pipefail

# Pin: Syft v1.50.0 linux amd64. Update this file atomically with the version.
SYFT_VERSION="1.50.0"
SYFT_SHA256="bf7b29ff57f06da30918266a0e1c2885a8f99784798d1bdb1628886aa015d788"
DEST="${1:-/usr/local/bin}"

archive="$(mktemp)"
trap 'rm -f "$archive"' EXIT
curl -fsSL -o "$archive" "https://github.com/anchore/syft/releases/download/v${SYFT_VERSION}/syft_${SYFT_VERSION}_linux_amd64.tar.gz"
echo "${SYFT_SHA256}  ${archive}" | sha256sum -c -
tar -xzf "$archive" -C "$DEST" syft
chmod +x "${DEST}/syft"
"${DEST}/syft" version
