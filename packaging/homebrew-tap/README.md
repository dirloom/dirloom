# Dirloom Homebrew tap

Official Homebrew cask for [Dirloom](https://github.com/dirloom/dirloom).

GitHub Releases is the only artifact source. This tap never rebuilds Dirloom.

## Install

```bash
brew install --cask dirloom/tap/dirloom
```

Works on macOS and Linux (Homebrew) for amd64 and arm64.

## Completions

The cask generates Bash, Zsh, Fish and PowerShell completion scripts from the installed binary. You can also generate them later:

```bash
dirloom completion bash
dirloom completion zsh
dirloom completion fish
dirloom completion powershell
```

## Updates

Version bumps are opened as pull requests by the Dirloom package bot. Direct pushes to `main` are not used. Mechanical version PRs need one maintainer approval; workflow changes need two independent approvals.

Bootstrap the tap with the last published stable release (`v0.1.1`) before promoting a new product version.
