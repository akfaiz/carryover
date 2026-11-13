<#
.SYNOPSIS
  Run the Go tests in Docker (Linux) and check the Windows build. See test-docker.sh.

.EXAMPLE
  .\scripts\test-docker.ps1                       # linux + windows, ./...
  .\scripts\test-docker.ps1 linux ./services/...  # only Linux, one package
#>
param(
    [Parameter(Position = 0)][ValidateSet('linux', 'windows', 'all')][string]$Target = 'all',
    [Parameter(Position = 1, ValueFromRemainingArguments)][string[]]$GoArgs
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$image = 'carryover-test'
if (-not $GoArgs) { $GoArgs = @('./...') }

docker build -q --platform linux/amd64 --target build -t $image $root | Out-Null
if ($LASTEXITCODE) { throw 'docker build failed' }

function Invoke-In([string]$script, [string[]]$more) {
    docker run --rm --platform linux/amd64 -v "${root}:/src" -w /src `
        -v carryover-gomod:/go/pkg/mod -v carryover-gobuild:/root/.cache/go-build `
        $image bash -c $script _ @more
    if ($LASTEXITCODE) { throw 'failed' }
}

if ($Target -in 'linux', 'all') {
    Write-Host "== linux: go test $GoArgs"
    Invoke-In 'CGO_ENABLED=1 go test "$@"' $GoArgs
}
if ($Target -in 'windows', 'all') {
    Write-Host '== windows: cross-compile + vet (not executed)'
    Invoke-In 'GOOS=windows CGO_ENABLED=0 go vet ./internal/... ./services/...' @()
}
