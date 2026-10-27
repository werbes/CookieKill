#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$OutputDir
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repository = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = Join-Path (Join-Path $repository 'dist') 'CookieKill-linux-amd64'
}

$goCommand = Get-Command go -ErrorAction SilentlyContinue
if ($goCommand) {
    $goExecutable = $goCommand.Source
}
elseif (Test-Path -LiteralPath 'C:\Program Files\Go\bin\go.exe') {
    $goExecutable = 'C:\Program Files\Go\bin\go.exe'
}
else {
    throw 'Install Go 1.26 or later, then run this script again.'
}

$outputPath = [IO.Path]::GetFullPath($OutputDir)
New-Item -ItemType Directory -Path $outputPath -Force | Out-Null
$binaryPath = Join-Path $outputPath 'cookiekill'
$configPath = Join-Path $outputPath 'config.json'

$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH
$previousCGO = $env:CGO_ENABLED
Push-Location $repository
try {
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    & $goExecutable build -trimpath '-ldflags=-s -w' -o $binaryPath .
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed with exit code $LASTEXITCODE."
    }
    Copy-Item -LiteralPath (Join-Path $repository 'config.example.json') -Destination $configPath -Force
    Write-Host "Built $binaryPath"
    Write-Host 'Target linux/amd64 with CGO disabled.'
}
finally {
    Pop-Location
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:CGO_ENABLED = $previousCGO
}
