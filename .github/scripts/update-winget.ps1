$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Pin: WingetCreate 1.12.13.0 windows amd64. Update this file atomically with the version.
$WingetCreateVersion = '1.12.13.0'
$WingetCreateSha256 = '24042BD37915805615E6CF969AC57C6439124C3FE85823327F5F3FB24BD9FFEA'

$Tag = $env:TAG
if ([string]::IsNullOrWhiteSpace($Tag)) {
    throw 'TAG is required'
}
$Version = $Tag.TrimStart('v')
$RootRepo = if ($env:GITHUB_REPOSITORY) { $env:GITHUB_REPOSITORY } else { 'dirloom/dirloom' }
$PackageId = 'Dirloom.Dirloom'
$Work = Join-Path ([System.IO.Path]::GetTempPath()) ("dirloom-winget-" + [guid]::NewGuid().ToString('n'))
New-Item -ItemType Directory -Path $Work | Out-Null

function Invoke-Gh {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$GhArgs)
    & gh @GhArgs
    if ($LASTEXITCODE -ne 0) {
        throw "gh $($GhArgs -join ' ') failed with exit $LASTEXITCODE"
    }
}

try {
    $existingPr = & gh pr list --repo microsoft/winget-pkgs --search "$PackageId $Version" --state open --json number --jq 'length'
    if ($LASTEXITCODE -eq 0 -and $existingPr -ne '0' -and $existingPr -ne '') {
        Write-Host "Winget PR already open for $Version"
        return
    }

    $manifestPath = "manifests/d/Dirloom/Dirloom/$Version"
    & gh api "repos/microsoft/winget-pkgs/contents/$manifestPath" 2>$null | Out-Null
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
    $token = $env:PACKAGE_BOT_TOKEN
    if ([string]::IsNullOrWhiteSpace($token)) {
        $token = $env:GITHUB_TOKEN
    }
    if ([string]::IsNullOrWhiteSpace($token)) {
        throw 'PACKAGE_BOT_TOKEN or GITHUB_TOKEN is required'
    }

    & $exe update $PackageId --version $Version --urls $x64Url $armUrl --submit --token $token
    if ($LASTEXITCODE -eq 0) {
        return
    }

    $repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    $templateDir = Join-Path $repoRoot 'packaging\winget'
    $manifestDir = Join-Path $Work 'manifest'
    New-Item -ItemType Directory -Path $manifestDir | Out-Null
    Get-ChildItem -LiteralPath $templateDir -Filter 'Dirloom.Dirloom*.yaml' | ForEach-Object {
        $text = [System.IO.File]::ReadAllText($_.FullName)
        $text = $text.Replace('0.1.1', $Version)
        $text = $text.Replace('3BBD704956C9ADF2B41EFB7ABF88F86DDF476635BB366983762555526796A256', $x64Listed)
        $text = $text.Replace('2995A9DAF6ABA00724FAFC17C4DE9419A127AC5EC47F5D4C791F256BD803E6F8', $armListed)
        [System.IO.File]::WriteAllText((Join-Path $manifestDir $_.Name), $text)
    }
    & $exe submit $manifestDir --token $token
    if ($LASTEXITCODE -ne 0) {
        throw "WingetCreate failed to update or submit $PackageId $Version"
    }
}
finally {
    Remove-Item -LiteralPath $Work -Recurse -Force -ErrorAction SilentlyContinue
}
