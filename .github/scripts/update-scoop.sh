#!/usr/bin/env bash
set -euo pipefail

# Open an idempotent version PR against dirloom/scoop-bucket.
# Requires GH_TOKEN (bot) with contents:write and pull-requests:write on that repo.

TAG="${TAG:?tag is required}"
VERSION="${TAG#v}"
BUCKET_REPO="${SCOOP_BUCKET_REPO:-dirloom/scoop-bucket}"
ROOT_REPO="${GITHUB_REPOSITORY:-dirloom/dirloom}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

gh release download "$TAG" --repo "$ROOT_REPO" --pattern checksums.txt --dir "$work"
gh release download "$TAG" --repo "$ROOT_REPO" --pattern 'dirloom_Windows_x86_64.zip' --dir "$work"
gh release download "$TAG" --repo "$ROOT_REPO" --pattern 'dirloom_Windows_arm64.zip' --dir "$work"

checksum_hash() {
  awk -v name="$1" '$2==name {print $1}' "$work/checksums.txt"
}

win_x64="$(checksum_hash dirloom_Windows_x86_64.zip)"
win_arm="$(checksum_hash dirloom_Windows_arm64.zip)"
test -n "$win_x64"
test -n "$win_arm"

actual_x64="$(sha256sum "$work/dirloom_Windows_x86_64.zip" | awk '{print $1}')"
actual_arm="$(sha256sum "$work/dirloom_Windows_arm64.zip" | awk '{print $1}')"
test "$actual_x64" = "$win_x64"
test "$actual_arm" = "$win_arm"

unzip -l "$work/dirloom_Windows_x86_64.zip" | grep -q 'dirloom.exe'
unzip -l "$work/dirloom_Windows_arm64.zip" | grep -q 'dirloom.exe'

gh repo clone "$BUCKET_REPO" "$work/bucket"
cd "$work/bucket"
branch="dirloom-${VERSION}"
count="$(gh pr list --repo "$BUCKET_REPO" --head "$branch" --state open --json number --jq 'length')"
if [[ "${count:-0}" -ge 1 ]]; then
  echo "Scoop PR already open for ${VERSION}"
  exit 0
fi

git checkout -B "$branch"
current=""
if [[ -f dirloom.json ]]; then
  current="$(python3 -c 'import json; print(json.load(open("dirloom.json")).get("version",""))')"
fi
if [[ "$current" == "$VERSION" ]]; then
  echo "Scoop manifest already at ${VERSION}"
  exit 0
fi
if [[ -n "$current" ]]; then
  latest="$(printf '%s\n%s\n' "$current" "$VERSION" | sort -V | tail -n1)"
  if [[ "$latest" != "$VERSION" ]]; then
    echo "Refusing to regress Scoop from ${current} to ${VERSION}"
    exit 1
  fi
fi

python3 - <<PY
from pathlib import Path
import json
manifest = {
  "version": "${VERSION}",
  "description": "Clean project trees for humans and AI",
  "homepage": "https://github.com/dirloom/dirloom",
  "license": "MIT",
  "architecture": {
    "64bit": {
      "url": "https://github.com/dirloom/dirloom/releases/download/${TAG}/dirloom_Windows_x86_64.zip",
      "hash": "${win_x64}"
    },
    "arm64": {
      "url": "https://github.com/dirloom/dirloom/releases/download/${TAG}/dirloom_Windows_arm64.zip",
      "hash": "${win_arm}"
    }
  },
  "bin": "dirloom.exe",
  "notes": [
    "Install completions with: dirloom completion powershell | Out-String | Invoke-Expression"
  ],
  "checkver": "github",
  "autoupdate": {
    "architecture": {
      "64bit": {
        "url": "https://github.com/dirloom/dirloom/releases/download/v\$version/dirloom_Windows_x86_64.zip"
      },
      "arm64": {
        "url": "https://github.com/dirloom/dirloom/releases/download/v\$version/dirloom_Windows_arm64.zip"
      }
    }
  }
}
Path("dirloom.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
json.loads(Path("dirloom.json").read_text(encoding="utf-8"))
PY

git add dirloom.json
if git diff --cached --quiet; then
  echo "Scoop manifest already at ${VERSION}"
  exit 0
fi
git -c user.name="dirloom-package-bot" -c user.email="41898282+github-actions[bot]@users.noreply.github.com" commit -m "chore(scoop): update dirloom to ${VERSION}"
git push -u origin "$branch"
gh pr create --repo "$BUCKET_REPO" --head "$branch" --title "dirloom ${VERSION}" --body "Update Scoop to GitHub Release ${TAG}. Artefacts are the official immutable archives; hashes were recalculated independently from checksums.txt."
