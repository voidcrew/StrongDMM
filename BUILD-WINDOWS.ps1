[CmdletBinding()]
param(
    [string]$Version = 'voidcrew-workshop.2-test.2',
    [string]$Revision = 'source',
    [switch]$Test
)
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    foreach ($tool in @('go', 'cargo', 'rustc', 'gcc', 'g++')) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
            throw "Install $tool and add it to PATH; see BUILDING.txt."
        }
    }
    $env:CGO_ENABLED = '1'
    # Keep the source builder's machine path out of Rust panic locations.
    $env:CARGO_ENCODED_RUSTFLAGS = "--remap-path-prefix=$PSScriptRoot=source"
    if (-not $env:CC) { $env:CC = (Get-Command gcc).Source }
    if (-not $env:CXX) { $env:CXX = (Get-Command g++).Source }
    $moduleMode = 'mod'
    if (Test-Path -LiteralPath 'vendor/modules.txt') { $moduleMode = 'vendor' }
    Push-Location third_party/sdmmparser/src
    try {
        $cargoArgs = @('build', '--release', '--locked')
        if (Test-Path -LiteralPath 'vendor') { $cargoArgs += '--offline' }
        & cargo @cargoArgs
        if ($LASTEXITCODE -ne 0) { throw 'Rust parser build failed.' }
    } finally { Pop-Location }
    if ($Test) {
        & go test "-mod=$moduleMode" ./...
        if ($LASTEXITCODE -ne 0) { throw 'Tests failed.' }
    }
    $sharedFlags = "-s -w -X sdmm/internal/env.Version=$Version -X sdmm/internal/env.Revision=$Revision -extldflags=-static"
    & go build "-mod=$moduleMode" -buildvcs=false -trimpath "-ldflags=$sharedFlags -H windowsgui" -o dst/StrongDMM-Voidcrew.exe .
    if ($LASTEXITCODE -ne 0) { throw 'Editor build failed.' }
    & go build "-mod=$moduleMode" -buildvcs=false -trimpath "-ldflags=$sharedFlags" -o dst/shipcheck.exe ./cmd/shipcheck
    if ($LASTEXITCODE -ne 0) { throw 'Checker build failed.' }
    Write-Host 'Built dst/StrongDMM-Voidcrew.exe and dst/shipcheck.exe.'
} finally { Pop-Location }
