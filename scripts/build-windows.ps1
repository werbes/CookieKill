#Requires -Version 5.1
[CmdletBinding()]
param(
    [ValidateSet('amd64', 'arm64')][string]$Architecture = 'amd64',
    [string]$OutputDir
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repository = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($OutputDir)) { $OutputDir = Join-Path $repository 'dist' }
$goCommand = Get-Command go -ErrorAction SilentlyContinue
if ($goCommand) { $goExecutable = $goCommand.Source }
elseif (Test-Path -LiteralPath 'C:\Program Files\Go\bin\go.exe') { $goExecutable = 'C:\Program Files\Go\bin\go.exe' }
else { throw 'Install Go 1.26 or later, then run this script again.' }
$outputPath = [IO.Path]::GetFullPath($OutputDir)
New-Item -ItemType Directory -Path $outputPath -Force | Out-Null
$stageName = '.build-' + [Guid]::NewGuid().ToString('N')
$stagePath = Join-Path $outputPath $stageName
$packageName = "CookieKill-windows-$Architecture"
$packagePath = Join-Path $stagePath $packageName
$archivePath = Join-Path $outputPath ($packageName + '.zip')
New-Item -ItemType Directory -Path $packagePath -Force | Out-Null
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousCGO = $env:CGO_ENABLED
Push-Location $repository
try {
    $env:GOOS = 'windows'
    $env:GOARCH = $Architecture
    $env:CGO_ENABLED = '0'
    & $goExecutable build -trimpath '-ldflags=-s -w' -o (Join-Path $packagePath 'cookiekill.exe') .
    if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE." }
    Copy-Item -LiteralPath (Join-Path $repository 'config.example.json') -Destination (Join-Path $packagePath 'config.json')
    foreach ($file in @('Install-Service.ps1', 'Uninstall-Service.ps1', 'README.txt')) {
        Copy-Item -LiteralPath (Join-Path $repository "deployment\windows\$file") -Destination $packagePath
    }

    # Include complete notices from the exact dependencies used to build this archive.
    $notices = New-Object Text.StringBuilder
    [void]$notices.AppendLine('CookieKill third-party license notices')
    $goRoot = & $goExecutable env GOROOT
    if ($LASTEXITCODE -ne 0) { throw 'Could not find the Go distribution license.' }
    $licenseFiles = @(@{ Name = 'Go standard library/runtime'; Path = (Join-Path $goRoot 'LICENSE') }, @{ Name = 'Three.js'; Path = (Join-Path $repository 'web\vendor\THREE-LICENSE.txt') })
    $moduleLines = & $goExecutable list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}|{{.Dir}}{{end}}{{end}}' .
    if ($LASTEXITCODE -ne 0) { throw 'Could not resolve module licenses.' }
    foreach ($line in ($moduleLines | Sort-Object -Unique)) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        $parts = $line -split '\|', 2
        if ($parts.Count -ne 2 -or -not $parts[1]) { throw "Missing module directory: $line" }
        $licenses = @(Get-ChildItem -LiteralPath $parts[1] -File | Where-Object { $_.Name -match '^(LICENSE|COPYING|NOTICE)(\..*)?$' })
        if ($licenses.Count -eq 0) { throw "No license found for $($parts[0])." }
        foreach ($license in $licenses) { $licenseFiles += @{ Name = $parts[0]; Path = $license.FullName } }
    }
    foreach ($license in $licenseFiles) {
        [void]$notices.AppendLine()
        [void]$notices.AppendLine(('=' * 72))
        [void]$notices.AppendLine($license.Name)
        [void]$notices.AppendLine(('=' * 72))
        [void]$notices.AppendLine((Get-Content -LiteralPath $license.Path -Raw))
    }
    [IO.File]::WriteAllText((Join-Path $packagePath 'THIRD-PARTY-NOTICES.txt'), $notices.ToString(), (New-Object Text.UTF8Encoding($false)))
    Compress-Archive -LiteralPath $packagePath -DestinationPath $archivePath -CompressionLevel Optimal -Force
    $hash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText(($archivePath + '.sha256'), "$hash  $packageName.zip`n", (New-Object Text.UTF8Encoding($false)))
    Write-Host "Built $archivePath"
    Write-Host "SHA256 $hash"
} finally {
    Pop-Location
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:CGO_ENABLED = $previousCGO
    # Delete only the unique staging directory created by this invocation.
    $resolvedStage = [IO.Path]::GetFullPath($stagePath)
    if ((Split-Path -Parent $resolvedStage) -ne $outputPath.TrimEnd('\') -or (Split-Path -Leaf $resolvedStage) -ne $stageName) {
        throw "Refusing cleanup outside the build output directory: $resolvedStage"
    }
    if (Test-Path -LiteralPath $resolvedStage) { Remove-Item -LiteralPath $resolvedStage -Recurse -Force }
}
