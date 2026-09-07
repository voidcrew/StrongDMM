[CmdletBinding()]
param(
    [Parameter(Position = 0)][string]$Project,
    [switch]$Check,
    [switch]$ValidateOnly
)
$ErrorActionPreference = 'Stop'
$diagnosticPath = $null

function New-ReportDirectory {
    foreach ($folder in @((Join-Path $PSScriptRoot 'reports'), (Join-Path ([System.IO.Path]::GetTempPath()) 'Voidcrew-Ship-Workshop-reports'))) {
        try {
            [void][System.IO.Directory]::CreateDirectory($folder)
            $probe = Join-Path $folder ([System.IO.Path]::GetRandomFileName())
            [System.IO.File]::WriteAllText($probe, '')
            [System.IO.File]::Delete($probe)
            return $folder
        } catch { }
    }
    throw 'Unable to write diagnostic reports beside the launcher or in the Windows temporary folder.'
}

function Write-Diagnostic([string]$Message) {
    if ($script:diagnosticPath) {
        [System.IO.File]::AppendAllText($script:diagnosticPath, $Message + [Environment]::NewLine)
    }
}

try {
    if (-not $ValidateOnly) {
        $reportDir = New-ReportDirectory
        $stamp = (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '-' + $PID
        $diagnosticPath = Join-Path $reportDir ("startup-" + $stamp + '.txt')
        Write-Diagnostic 'Voidcrew Ship Workshop launcher diagnostics 1'
        Write-Diagnostic ("Started: " + (Get-Date -Format o))
        Write-Diagnostic ("Windows: " + [Environment]::OSVersion.VersionString)
        Write-Diagnostic ("64-bit OS: " + [Environment]::Is64BitOperatingSystem)
        Write-Diagnostic ("PowerShell: " + $PSVersionTable.PSVersion)
        Write-Diagnostic ("Launcher: " + $PSScriptRoot)
        $buildInfo = Join-Path $PSScriptRoot 'BUILD-INFO.json'
        if (Test-Path -LiteralPath $buildInfo) {
            Write-Diagnostic ([System.IO.File]::ReadAllText($buildInfo))
        }
        Write-Host ("Startup report: " + $diagnosticPath)
    }
    if ([string]::IsNullOrWhiteSpace($Project)) {
        if ($ValidateOnly) { throw 'Supply a project folder or .dme file.' }
        Add-Type -AssemblyName System.Windows.Forms
        $picker = New-Object System.Windows.Forms.OpenFileDialog
        $picker.Title = 'Choose tgstation.dme in your Voidcrew checkout'
        $picker.Filter = 'BYOND environment (*.dme)|*.dme'
        $picker.CheckFileExists = $true
        try {
            if ($picker.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) {
                Write-Diagnostic 'Project selection cancelled; editor was not started.'
                exit 0
            }
            $Project = $picker.FileName
        } finally {
            $picker.Dispose()
        }
    }
    $projectItem = Get-Item -LiteralPath $Project
    if ($projectItem.PSIsContainer) {
        $projectItem = Get-Item -LiteralPath (Join-Path $projectItem.FullName 'tgstation.dme')
    }
    if ($projectItem.Extension -ine '.dme') { throw 'Choose a .dme file or the folder containing tgstation.dme.' }
    $dmePath = $projectItem.FullName
    $projectRoot = $projectItem.DirectoryName
    $shipConfig = Join-Path $projectRoot 'voidcrew\modules\ship_upgrades\ship_upgrades.toml'
    if (-not (Test-Path -LiteralPath $shipConfig -PathType Leaf)) {
        throw 'This checkout does not contain the Voidcrew ship upgrade configuration.'
    }
    $executable = Join-Path $PSScriptRoot 'StrongDMM-Voidcrew.exe'
    if ($Check) { $executable = Join-Path $PSScriptRoot 'shipcheck.exe' }
    if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
        throw 'The executable is missing. Extract the entire ZIP before running the launcher.'
    }
    if ($ValidateOnly) {
        [pscustomobject]@{
            Executable = $executable
            Environment = $dmePath
            WorkingDirectory = $projectRoot
            Mode = $(if ($Check) { 'check' } else { 'edit' })
        } | ConvertTo-Json
        exit 0
    }
    $start = New-Object System.Diagnostics.ProcessStartInfo
    $start.FileName = $executable
    $start.WorkingDirectory = $projectRoot
    $start.UseShellExecute = $false
    Write-Diagnostic ("Project: " + $dmePath)
    Write-Diagnostic ("Executable: " + $executable)
    if (-not $Check) {
        $start.Arguments = '"' + $dmePath + '" --ship-workspace'
        $start.CreateNoWindow = $true
        $start.RedirectStandardOutput = $true
        $start.RedirectStandardError = $true
        $outputPath = Join-Path $reportDir ("editor-" + $stamp + '.log')
        $errorPath = Join-Path $reportDir ("crash-" + $stamp + '.log')
        Write-Diagnostic ("Editor output: " + $outputPath)
        Write-Diagnostic ("Crash output: " + $errorPath)
        Write-Host 'Starting Ship Workshop. Keep this window open while using the editor.'
        # Dedicated readers keep noisy native stdout and stderr from blocking
        # each other. C# workers do not require a PowerShell runspace.
        Add-Type -TypeDefinition @'
using System.IO;
using System.Threading;
using System.Threading.Tasks;
public static class ShipWorkshopStreams {
    public static Task Copy(Stream input, Stream output) {
        return Task.Factory.StartNew(() => {
            byte[] buffer = new byte[8192];
            int count;
            while ((count = input.Read(buffer, 0, buffer.Length)) > 0) {
                output.Write(buffer, 0, count);
                output.Flush();
            }
        }, CancellationToken.None, TaskCreationOptions.LongRunning, TaskScheduler.Default);
    }
}
'@
        $outputFile = $null
        $errorFile = $null
        $process = $null
        try {
            $outputFile = [System.IO.File]::Open($outputPath, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write, [System.IO.FileShare]::ReadWrite)
            $errorFile = [System.IO.File]::Open($errorPath, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write, [System.IO.FileShare]::ReadWrite)
            $process = [System.Diagnostics.Process]::Start($start)
            Write-Diagnostic ("Process ID: " + $process.Id)
            $stdout = [ShipWorkshopStreams]::Copy($process.StandardOutput.BaseStream, $outputFile)
            $stderr = [ShipWorkshopStreams]::Copy($process.StandardError.BaseStream, $errorFile)
            $process.WaitForExit()
            [void]$stdout.GetAwaiter().GetResult()
            [void]$stderr.GetAwaiter().GetResult()
            $resultCode = $process.ExitCode
        } finally {
            if ($outputFile) { $outputFile.Dispose() }
            if ($errorFile) { $errorFile.Dispose() }
            if ($process) { $process.Dispose() }
        }
        $unsignedCode = [BitConverter]::ToUInt32([BitConverter]::GetBytes([int]$resultCode), 0)
        $status = 'Editor exit code: {0} (0x{1:X8})' -f $resultCode, $unsignedCode
        Write-Diagnostic $status
        Write-Diagnostic ("Finished: " + (Get-Date -Format o))
        if ($resultCode -ne 0) {
            Write-Host $status -ForegroundColor Red
            foreach ($logPath in @($outputPath, $errorPath)) {
                if ((Get-Item -LiteralPath $logPath).Length -gt 0) {
                    Write-Host ("--- " + [System.IO.Path]::GetFileName($logPath) + ' ---')
                    Get-Content -LiteralPath $logPath -Tail 20
                }
            }
            Write-Host 'The editor stopped unexpectedly. Send the three matching report files to the person who supplied this build.'
            Write-Host ("Reports: " + $reportDir)
            exit 1
        }
        Write-Host 'Editor closed normally.'
        exit 0
    }
    $start.Arguments = '-dme "' + $dmePath + '" -all'
    $start.CreateNoWindow = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    $start.StandardOutputEncoding = New-Object System.Text.UTF8Encoding($false)
    $start.StandardErrorEncoding = New-Object System.Text.UTF8Encoding($false)
    Write-Host 'Checking saved ship loadouts...'
    $process = [System.Diagnostics.Process]::Start($start)
    $stdout = $process.StandardOutput.ReadToEndAsync()
    $stderr = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    $outputText = $stdout.GetAwaiter().GetResult()
    $errorText = $stderr.GetAwaiter().GetResult()
    $resultCode = $process.ExitCode
    $process.Dispose()
    $report = Join-Path $reportDir ("shipcheck-" + $stamp + '.json')
    [System.IO.File]::WriteAllText($report, $outputText, (New-Object System.Text.UTF8Encoding($false)))
    if ($errorText) {
        [System.IO.File]::WriteAllText((Join-Path $reportDir ("shipcheck-" + $stamp + '.log')), $errorText)
    }
    Write-Host ("Report: " + $report)
    if ($resultCode -eq 0) {
        Write-Host 'No assembly errors. Review any static warnings in the report.'
    } else {
        Write-Host 'The check found errors. See the report and its accompanying log.'
    }
    exit $resultCode
} catch {
    $failure = "Ship Workshop: " + $_.Exception.Message
    try { Write-Diagnostic $failure } catch { }
    Write-Host $failure -ForegroundColor Red
    if ($diagnosticPath) { Write-Host ("Startup report: " + $diagnosticPath) }
    exit 1
}
