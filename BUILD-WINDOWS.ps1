[CmdletBinding()]
param(
    [ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version = '0.2.1',
    [string]$Revision = 'source',
    [switch]$Test
)
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    foreach ($tool in @('go', 'cargo', 'rustc', 'gcc', 'g++', 'windres')) {
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
    [void][System.IO.Directory]::CreateDirectory((Join-Path $PSScriptRoot 'dst'))
    $resourceText = [System.IO.File]::ReadAllText((Join-Path $PSScriptRoot 'distribution/StrongDMM.rc.in'))
    $resourceText = $resourceText.Replace('@VERSION@', $Version).Replace('@VERSION_NUMBERS@', (($Version -split '\.') -join ',') + ',0')
    [System.IO.File]::WriteAllText((Join-Path $PSScriptRoot 'dst/version.rc'), $resourceText)
    & windres -i dst/version.rc -o resource_windows_amd64.syso -O coff --target=pe-x86-64
    if ($LASTEXITCODE -ne 0) { throw 'Windows resource build failed.' }
    try {
        & go build "-mod=$moduleMode" -buildvcs=false -trimpath "-ldflags=$sharedFlags -H windowsgui" -o dst/StrongDMM.exe .
        if ($LASTEXITCODE -ne 0) { throw 'Editor build failed.' }
    } finally { Remove-Item -LiteralPath (Join-Path $PSScriptRoot 'resource_windows_amd64.syso') -ErrorAction SilentlyContinue }
    & go build "-mod=$moduleMode" -buildvcs=false -trimpath "-ldflags=$sharedFlags" -o dst/shipcheck.exe ./cmd/shipcheck
    if ($LASTEXITCODE -ne 0) { throw 'Checker build failed.' }
    Write-Host 'Built dst/StrongDMM.exe and dst/shipcheck.exe.'
} finally { Pop-Location }
