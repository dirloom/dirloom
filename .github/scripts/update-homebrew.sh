#!/usr/bin/env bash
set -euo pipefail

TAG="${TAG:?tag is required}"
VERSION="${TAG#v}"
TAP_REPO="${HOMEBREW_TAP_REPO:-dirloom/homebrew-tap}"
ROOT_REPO="${GITHUB_REPOSITORY:-dirloom/dirloom}"

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
branch="dirloom-${VERSION}"
count="$(gh pr list --repo "$TAP_REPO" --head "$branch" --state open --json number --jq 'length')"
if [[ "${count:-0}" -ge 1 ]]; then
  echo "Homebrew PR already open for ${VERSION}"
  exit 0
fi
git checkout -B "$branch"
mkdir -p Casks
cat > Casks/dirloom.rb <<EOF
cask "dirloom" do
  arch arm: "arm64", intel: "x86_64"

  version "${VERSION}"

  on_macos do
    sha256 arm:   "${darwin_arm}",
           intel: "${darwin_x64}"

    url "https://github.com/dirloom/dirloom/releases/download/v#{version}/dirloom_Darwin_#{arch}.tar.gz"
  end

  on_linux do
    sha256 arm:   "${linux_arm}",
           intel: "${linux_x64}"

    url "https://github.com/dirloom/dirloom/releases/download/v#{version}/dirloom_Linux_#{arch}.tar.gz"
  end

  name "Dirloom"
  desc "Clean project trees for humans and AI"
  homepage "https://github.com/dirloom/dirloom"

  livecheck do
    url "https://github.com/dirloom/dirloom/releases/latest"
    strategy :github_latest
  end

  binary "dirloom"

  postflight do
    executable = staged_path/"dirloom"
    prefix = Pathname.new(HOMEBREW_PREFIX)
    begin
      bash_dir = prefix/"etc/bash_completion.d"
      zsh_dir = prefix/"share/zsh/site-functions"
      fish_dir = prefix/"share/fish/vendor_completions.d"
      pwsh_dir = prefix/"share/pwsh/completions"
      bash_dir.mkpath
      zsh_dir.mkpath
      fish_dir.mkpath
      pwsh_dir.mkpath
      (bash_dir/"dirloom").write system_command(executable, args: ["completion", "bash"]).stdout
      (zsh_dir/"_dirloom").write system_command(executable, args: ["completion", "zsh"]).stdout
      (fish_dir/"dirloom.fish").write system_command(executable, args: ["completion", "fish"]).stdout
      (pwsh_dir/"dirloom.ps1").write system_command(executable, args: ["completion", "powershell"]).stdout
    rescue StandardError
      puts "Could not install generated shell completions (#{e.message}); run dirloom completion <shell> manually."
    end
  end

  caveats <<~EOS
    Generate shell completions from the installed binary if they were not
    installed automatically:

      dirloom completion bash
      dirloom completion zsh
      dirloom completion fish
      dirloom completion powershell
  EOS
end
EOF
git add Casks/dirloom.rb
if git diff --cached --quiet; then
  echo "Homebrew cask already at ${VERSION}"
  exit 0
fi
git -c user.name="dirloom-package-bot" -c user.email="41898282+github-actions[bot]@users.noreply.github.com" commit -m "chore(cask): update dirloom to ${VERSION}"
git push -u origin "$branch"
gh pr create --repo "$TAP_REPO" --head "$branch" --title "dirloom ${VERSION}" --body "Update the Dirloom cask to GitHub Release ${TAG}. Binaries are the official archives; Dirloom is not rebuilt. Hashes were recalculated independently from checksums.txt."
