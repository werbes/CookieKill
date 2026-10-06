#Requires -Version 5.1
#Requires -RunAsAdministrator
[CmdletBinding()]
param([string]$InstallDir)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ([string]::IsNullOrWhiteSpace($InstallDir)) { $InstallDir = $PSScriptRoot }
$serviceName = 'CookieKill'
$installPath = (Resolve-Path -LiteralPath $InstallDir).ProviderPath.TrimEnd('\')
$binaryPath = '"' + (Join-Path $installPath 'cookiekill.exe') + '"'
$existing = Get-CimInstance Win32_Service -Filter "Name='$serviceName'"
if ($existing) {
    if ($existing.PathName.Trim() -ne $binaryPath) { throw "CookieKill is registered elsewhere: $($existing.PathName). Run its own uninstall script." }
    $controller = Get-Service -Name $serviceName
    if ($controller.Status -ne 'Stopped') {
        Stop-Service -Name $serviceName
        $controller.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(45))
    }
    & "$env:SystemRoot\System32\sc.exe" delete $serviceName
    if ($LASTEXITCODE -ne 0) { throw "sc.exe delete failed with exit code $LASTEXITCODE." }
}
Get-NetFirewallRule -Name 'CookieKill-HTTP-HTTPS' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
Write-Host 'CookieKill service and its optional firewall rule have been removed. All files, configuration, logs, saved progress, ACLs, and the Event Log source have been preserved.'
