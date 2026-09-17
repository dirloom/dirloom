$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Pin: WingetCreate 1.12.13.0 windows amd64. Update this file atomically with the version.
$WingetCreateVersion = '1.12.13.0'
$WingetCreateSha256 = '24042BD37915805615E6CF969AC57C6439124C3FE85823327F5F3FB24BD9FFEA'

# Open an idempotent version PR against microsoft/winget-pkgs from the org fork
# dirloom/winget-pkgs. Requires GH_TOKEN (dirloom-package-mgr) with contents:write
# on that fork. WingetCreate only generates manifests; it does not --submit, so
# the PR head stays on the org fork rather than a personal machine-user fork.

$Tag = $env:TAG
if ([string]::IsNullOrWhiteSpace($Tag)) {
    throw 'TAG is required'
}
$Version = $Tag.TrimStart('v')
$RootRepo = if ($env:GITHUB_REPOSITORY) { $env:GITHUB_REPOSITORY } else { 'dirloom/dirloom' }
$ForkRepo = if ($env:WINGET_FORK_REPO) { $env:WINGET_FORK_REPO } else { 'dirloom/winget-pkgs' }
$UpstreamRepo = 'microsoft/winget-pkgs'
$PackageId = 'Dirloom.Dirloom'
$CommitName = 'dirloom-package-mgr'
$CommitEmail = '330109029+dirloom-package-mgr@users.noreply.github.com'
$Work = Join-Path ([System.IO.Path]::GetTempPath()) ("dirloom-winget-" + [guid]::NewGuid().ToString('n'))
New-Item -ItemType Directory -Path $Work | Out-Null

function Invoke-Gh {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$GhArgs)
    & gh @GhArgs
    if ($LASTEXITCODE -ne 0) {
        throw "gh $($GhArgs -join ' ') failed with exit $LASTEXITCODE"
    }
}

function Invoke-Git {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$GitArgs)
    & git @GitArgs
    if ($LASTEXITCODE -ne 0) {
        throw "git $($GitArgs -join ' ') failed with exit $LASTEXITCODE"
    }
}

function Set-LicenseUrl([string]$Root) {
    $wanted = "LicenseUrl: https://github.com/dirloom/dirloom/blob/$Tag/LICENSE"
    Get-ChildItem -LiteralPath $Root -Filter '*.yaml' -Recurse -File | ForEach-Object {
        $text = [System.IO.File]::ReadAllText($_.FullName)
        $updated = [regex]::Replace($text, '(?m)^LicenseUrl:\s*\S+\s*$', $wanted)
        if ($updated -ne $text) {
            [System.IO.File]::WriteAllText($_.FullName, $updated)
        }
    }
}

try {
    if ([string]::IsNullOrWhiteSpace($env:GH_TOKEN)) {
        if (-not [string]::IsNullOrWhiteSpace($env:PACKAGE_BOT_TOKEN)) {
            $env:GH_TOKEN = $env:PACKAGE_BOT_TOKEN
        } elseif (-not [string]::IsNullOrWhiteSpace($env:GITHUB_TOKEN)) {
            $env:GH_TOKEN = $env:GITHUB_TOKEN
        }
    }
    if ([string]::IsNullOrWhiteSpace($env:GH_TOKEN)) {
        throw 'GH_TOKEN is required'
    }

    Invoke-Gh auth setup-git

    # Deliberately do not sync the fork with upstream here.
    # Updating upstream workflow files would require the PAT `workflow` scope.
    # The publication branch is based on the fork's own master and contains
    # only the Dirloom manifests; GitHub computes the PR diff from the common
    # ancestor with microsoft/winget-pkgs.

    # Ignore open PRs from retired machine users (we cannot close those heads).
    # Only treat a version as already in flight when *our* org fork or current
    # publisher account already has an open PR.
    $oursOpen = & gh pr list --repo $UpstreamRepo --search "$PackageId $Version" --state open --json number,author,headRepositoryOwner --jq '[.[] | select(.author.login == "dirloom-package-mgr" or .headRepositoryOwner.login == "dirloom")] | length'
    if ($LASTEXITCODE -eq 0 -and $oursOpen -ne '0' -and $oursOpen -ne '') {
        Write-Host "Winget PR already open for $Version from dirloom/dirloom-package-mgr"
        return
    }

    $manifestPath = "manifests/d/Dirloom/Dirloom/$Version"
    & gh api "repos/$UpstreamRepo/contents/$manifestPath" 2>$null | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Winget already contains $PackageId $Version"
        return
    }

    Invoke-Gh release download $Tag --repo $RootRepo --pattern checksums.txt --dir $Work
    Invoke-Gh release download $Tag --repo $RootRepo --pattern dirloom_Windows_x86_64.zip --dir $Work
    Invoke-Gh release download $Tag --repo $RootRepo --pattern dirloom_Windows_arm64.zip --dir $Work

    $checksums = Get-Content -LiteralPath (Join-Path $Work 'checksums.txt')
    function Get-ListedHash([string]$Name) {
        foreach ($line in $checksums) {
            $parts = $line.Trim() -split '\s+', 2
            if ($parts.Length -eq 2 -and $parts[1] -eq $Name) {
                return $parts[0].ToUpperInvariant()
            }
        }
        throw "checksums.txt is missing $Name"
    }

    $x64Name = 'dirloom_Windows_x86_64.zip'
    $armName = 'dirloom_Windows_arm64.zip'
    $x64Listed = Get-ListedHash $x64Name
    $armListed = Get-ListedHash $armName
    $x64Actual = (Get-FileHash -LiteralPath (Join-Path $Work $x64Name) -Algorithm SHA256).Hash
    $armActual = (Get-FileHash -LiteralPath (Join-Path $Work $armName) -Algorithm SHA256).Hash
    if ($x64Actual -ne $x64Listed) { throw "x64 hash mismatch: actual=$x64Actual listed=$x64Listed" }
    if ($armActual -ne $armListed) { throw "arm64 hash mismatch: actual=$armActual listed=$armListed" }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    foreach ($zipName in @($x64Name, $armName)) {
        $zip = [System.IO.Compression.ZipFile]::OpenRead((Join-Path $Work $zipName))
        try {
            $hasBinary = $false
            foreach ($entry in $zip.Entries) {
                if ([System.IO.Path]::GetFileName($entry.FullName) -eq 'dirloom.exe') {
                    $hasBinary = $true
                    break
                }
            }
            if (-not $hasBinary) { throw "$zipName does not contain dirloom.exe" }
        }
        finally {
            $zip.Dispose()
        }
    }

    $exe = Join-Path $Work 'wingetcreate.exe'
    Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/microsoft/winget-create/releases/download/v${WingetCreateVersion}/wingetcreate.exe" -OutFile $exe
    $got = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash
    if ($got -ne $WingetCreateSha256) {
        throw "WingetCreate hash mismatch: actual=$got expected=$WingetCreateSha256"
    }

    $x64Url = "https://github.com/dirloom/dirloom/releases/download/$Tag/$x64Name"
    $armUrl = "https://github.com/dirloom/dirloom/releases/download/$Tag/$armName"
    $manifestDir = Join-Path $Work 'manifest'
    New-Item -ItemType Directory -Path $manifestDir | Out-Null

    & $exe update $PackageId --version $Version --urls $x64Url $armUrl --out $manifestDir --token $env:GH_TOKEN
    $yamlCount = @(Get-ChildItem -LiteralPath $manifestDir -Filter '*.yaml' -Recurse -File).Count
    if ($LASTEXITCODE -ne 0 -or $yamlCount -lt 3) {
        Write-Host "WingetCreate update failed; falling back to in-repo templates"
        Get-ChildItem -LiteralPath $manifestDir -Recurse -File -ErrorAction SilentlyContinue | Remove-Item -Force
        $repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
        $templateDir = Join-Path $repoRoot 'packaging\winget'
        Get-ChildItem -LiteralPath $templateDir -Filter 'Dirloom.Dirloom*.yaml' | ForEach-Object {
            $text = [System.IO.File]::ReadAllText($_.FullName)
            $text = $text.Replace('0.1.1', $Version)
            $text = $text.Replace('3BBD704956C9ADF2B41EFB7ABF88F86DDF476635BB366983762555526796A256', $x64Listed)
            $text = $text.Replace('2995A9DAF6ABA00724FAFC17C4DE9419A127AC5EC47F5D4C791F256BD803E6F8', $armListed)
            [System.IO.File]::WriteAllText((Join-Path $manifestDir $_.Name), $text)
        }
    }

    Set-LicenseUrl $manifestDir
    $generated = @(Get-ChildItem -LiteralPath $manifestDir -Filter '*.yaml' -Recurse -File)
    if ($generated.Count -lt 3) {
        throw "Expected at least 3 Winget YAML manifests, found $($generated.Count)"
    }

    $forkDir = Join-Path $Work 'fork'
    # Call git directly: PowerShell functions swallow `--`, which gh needs to
    # forward clone filters.
    & git clone --filter=blob:none --sparse --depth 1 "https://github.com/$ForkRepo.git" $forkDir
    if ($LASTEXITCODE -ne 0) {
        throw "git clone $ForkRepo failed with exit $LASTEXITCODE"
    }
    Push-Location $forkDir
    try {
        Invoke-Git sparse-checkout set manifests/d/Dirloom
        $branch = "Dirloom.Dirloom-$Version"
        Invoke-Git checkout -B $branch origin/master
        $dest = Join-Path $forkDir $manifestPath
        New-Item -ItemType Directory -Path $dest -Force | Out-Null
        foreach ($file in $generated) {
            Copy-Item -LiteralPath $file.FullName -Destination (Join-Path $dest $file.Name) -Force
        }
        Invoke-Git add -- $manifestPath
        git diff --cached --quiet
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Winget manifests already at $Version"
            return
        }
        & git -c "user.name=$CommitName" -c "user.email=$CommitEmail" commit -m "New version: $PackageId version $Version"
        if ($LASTEXITCODE -ne 0) {
            throw 'git commit failed'
        }
        & git push -u origin $branch
        if ($LASTEXITCODE -ne 0) {
            & gh api -X DELETE "repos/$ForkRepo/git/refs/heads/$branch" 2>$null | Out-Null
            Invoke-Git push -u origin $branch
        }
    }
    finally {
        Pop-Location
    }

    $forkOwner = ($ForkRepo -split '/')[0]
    Invoke-Gh pr create --repo $UpstreamRepo --head "${forkOwner}:${branch}" --base master --title "New version: $PackageId version $Version" --body @"
Update Dirloom.Dirloom to GitHub Release $Tag from the $ForkRepo fork. Installers are the official Windows zip archives; hashes were verified against checksums.txt. LicenseUrl points at $Tag.

Supersedes https://github.com/microsoft/winget-pkgs/pull/435891 : that PR was opened by the retired ``dirloom-package-bot`` account, which we can no longer access to close the PR or complete the CLA. Please close #435891 in favor of this one.
"@
}
finally {
    Remove-Item -LiteralPath $Work -Recurse -Force -ErrorAction SilentlyContinue
}
