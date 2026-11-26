<#
.SYNOPSIS
Dirloom showcase: Structural Version Control from end to end, reproducibly.

.DESCRIPTION
The walkthrough is docs/showcase.md; showcase.sh is the same scenario in
bash, and both print the same transcript.

Requires Git 2.45 or newer. The script reads no configuration of the host:
Dirloom runs with --no-config where a command takes it, and Git reads no
configuration, ignore or attributes file and no template, with a fixed
identity and fixed dates. Commit ids and outputs are therefore the same on
every machine. The environment variables and the location of the calling
session are restored at the end, even when the script fails.

The output is the transcript: each dirloom command, its output and its exit
code. The script stops with an error as soon as a dirloom command does not
exit as documented in docs/reference/exit-codes.md, or writes to stderr.

Runs on Windows PowerShell 5.1 and PowerShell 7.

.PARAMETER WorkDirectory
Where the project is created. It must be empty or not exist yet. Default: a
new temporary directory, which is kept and named on stderr at the end, even
on a failure.

.NOTES
The DIRLOOM environment variable names the dirloom executable to run.
Default: dirloom on PATH.
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$WorkDirectory
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = 'Stop'
# A nonzero exit status is a documented result here, never an error.
$PSNativeCommandUseErrorActionPreference = $false

$dirloom = if ($env:DIRLOOM) { $env:DIRLOOM } else { 'dirloom' }
# A path, relative or not, must stay valid once the script changes location.
if ($dirloom.IndexOfAny([char[]]'/\') -ge 0) {
    $dirloom = (Resolve-Path -LiteralPath $dirloom).ProviderPath
}

if ($WorkDirectory) {
    $null = New-Item -ItemType Directory -Force -Path $WorkDirectory
    if (Get-ChildItem -Force -LiteralPath $WorkDirectory | Select-Object -First 1) {
        [Console]::Error.WriteLine("showcase: $WorkDirectory is not empty")
        exit 2
    }
    $work = (Resolve-Path -LiteralPath $WorkDirectory).ProviderPath
    $announce = $false
} else {
    $work = Join-Path ([IO.Path]::GetTempPath()) ('dirloom-showcase.' + [Guid]::NewGuid().ToString('N').Substring(0, 8))
    $null = New-Item -ItemType Directory -Path $work
    $announce = $true
}
$project = Join-Path $work 'shop'
$stderrFile = Join-Path $work 'stderr'

# Git reads no configuration of the host and records a fixed identity: with
# the fixed dates of New-Commit, the commit ids are the same everywhere.
# XDG_CONFIG_HOME also hides the default core.excludesFile and
# core.attributesFile, and git init below copies no template.
$cleared = 'GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_OBJECT_DIRECTORY',
    'GIT_ALTERNATE_OBJECT_DIRECTORIES', 'GIT_COMMON_DIR', 'GIT_NAMESPACE',
    'GIT_CONFIG', 'GIT_CONFIG_PARAMETERS', 'GIT_CONFIG_COUNT'
$fixed = [ordered]@{
    GIT_CONFIG_NOSYSTEM = '1'
    GIT_ATTR_NOSYSTEM   = '1'
    GIT_CONFIG_GLOBAL   = Join-Path $work 'gitconfig'
    XDG_CONFIG_HOME     = Join-Path $work 'xdg'
    GIT_AUTHOR_NAME     = 'Dirloom Showcase'
    GIT_AUTHOR_EMAIL    = 'showcase@dirloom.invalid'
    GIT_COMMITTER_NAME  = 'Dirloom Showcase'
    GIT_COMMITTER_EMAIL = 'showcase@dirloom.invalid'
    GIT_AUTHOR_DATE     = $null
    GIT_COMMITTER_DATE  = $null
}
$saved = @{}
foreach ($name in @($cleared) + @($fixed.Keys)) {
    $saved[$name] = [Environment]::GetEnvironmentVariable($name)
}

# Set-ProcessVariable <name> <value>: set an environment variable of this
# process, or remove it when the value is null. An empty value is not a
# removal on every .NET version.
function Set-ProcessVariable([string]$Name, $Value) {
    if ($null -eq $Value) {
        Remove-Item -LiteralPath "Env:$Name" -ErrorAction SilentlyContinue
    } else {
        Set-Item -LiteralPath "Env:$Name" -Value $Value
    }
}

# Invoke-Git <arguments...>: run git, which prints nothing here, and stop
# when it fails.
function Invoke-Git {
    $null = & git @args
    if ($LASTEXITCODE -ne 0) { throw "git $args exited $LASTEXITCODE" }
}

# New-Commit <date> <message>: commit the index at a fixed date.
function New-Commit([string]$Date, [string]$Message) {
    $env:GIT_AUTHOR_DATE = $Date
    $env:GIT_COMMITTER_DATE = $Date
    Invoke-Git commit --quiet --message $Message
}

# Write-ProjectFile <path> <line>: write one line ending with LF, in UTF-8
# without a byte order mark: the same bytes as showcase.sh.
function Write-ProjectFile([string]$Path, [string]$Line) {
    $full = Join-Path (Get-Location).ProviderPath $Path
    $null = New-Item -ItemType Directory -Force -Path (Split-Path -Parent $full)
    [IO.File]::WriteAllText($full, "$Line`n")
}

# Write-Section <name>: start a part of the transcript.
function Write-Section([string]$Name) {
    "== $Name =="
}

# Invoke-Dirloom <exit> <arguments...>: run dirloom and print the command,
# its output and its exit code. Stop unless it exits with <exit> and an
# empty stderr.
function Invoke-Dirloom([int]$Expected) {
    '$ dirloom ' + ($args -join ' ')
    $output = & $dirloom @args 2> $stderrFile
    $code = $LASTEXITCODE
    $output
    "[exit $code]"
    ''
    $stderr = Get-Content -Raw -LiteralPath $stderrFile
    if ($code -ne $Expected -or $stderr) {
        throw "showcase: 'dirloom $args' exited $code, expected $Expected. $stderr"
    }
}

Push-Location -LiteralPath $work
try {
    foreach ($name in $cleared) { Set-ProcessVariable $name $null }
    foreach ($name in $fixed.Keys) { Set-ProcessVariable $name $fixed[$name] }
    [IO.File]::WriteAllText($env:GIT_CONFIG_GLOBAL, '')

    # 1. A small reference project, committed on main.
    $null = New-Item -ItemType Directory -Path $project
    Set-Location -LiteralPath $project
    Write-ProjectFile README.md '# shop'
    Write-ProjectFile go.mod 'module example.com/shop'
    Write-ProjectFile cmd/shop/main.go 'package main'
    Write-ProjectFile docs/architecture.md '# Architecture'
    Write-ProjectFile internal/billing/invoice.go 'package billing'
    Write-ProjectFile internal/billing/tax.go 'package billing'
    Write-ProjectFile internal/utils/strings.go 'package utils'
    Write-ProjectFile payment/api/handler.go 'package api'
    Write-ProjectFile payment/service.go 'package payment'
    Invoke-Git init --quiet --template= --initial-branch=main --object-format=sha1
    Invoke-Git add --all
    New-Commit 2026-01-05T09:00:00Z 'Lay out the shop service'

    Write-Section project
    Invoke-Dirloom 0 --no-config --style ascii --color never

    # 2. Snapshot the structure and commit the snapshot with the code.
    Write-Section snapshot
    Invoke-Dirloom 0 snapshot --no-config --output architecture.dlm.json
    Invoke-Dirloom 0 verify architecture.dlm.json
    Invoke-Git add architecture.dlm.json
    New-Commit 2026-01-05T10:00:00Z 'Record the architecture snapshot'

    # 3. Refactor on a branch: move and rename files and directories.
    Invoke-Git switch --quiet --create refactor/layout
    Invoke-Git mv payment internal/payment
    Invoke-Git mv internal/utils internal/textutil
    $null = New-Item -ItemType Directory -Path internal/tax
    Invoke-Git mv internal/billing/tax.go internal/tax/tax.go
    Invoke-Git mv docs/architecture.md docs/design.md

    # 4. The snapshot no longer matches: verify exits 1.
    Write-Section verify
    Invoke-Dirloom 1 verify architecture.dlm.json

    # 5. Diff names the changes and the moves: exit 1.
    Write-Section diff
    Invoke-Dirloom 1 diff snapshot:architecture.dlm.json live:.

    # 6. Commit the refactoring, then a content-only change, and compare the
    # committed trees of the two branches.
    New-Commit 2026-01-05T11:00:00Z 'Move payment under internal'
    Write-ProjectFile README.md '# shop: invoices, taxes and payments'
    Invoke-Git add README.md
    New-Commit 2026-01-05T12:00:00Z 'Describe the service'

    Write-Section git
    Invoke-Dirloom 1 diff git:main git:refactor/layout

    # 7. The structural history of the branch.
    Write-Section history
    Invoke-Dirloom 0 history
} finally {
    foreach ($name in $saved.Keys) { Set-ProcessVariable $name $saved[$name] }
    Pop-Location
    if ($announce) {
        [Console]::Error.WriteLine("showcase: the project is in $project")
    }
}
