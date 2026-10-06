#Requires -Version 5.1
#Requires -RunAsAdministrator
[CmdletBinding()]
param(
    [string]$InstallDir,
    [switch]$OpenFirewall,
    [switch]$NoStart
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ([string]::IsNullOrWhiteSpace($InstallDir)) { $InstallDir = $PSScriptRoot }
$serviceName = 'CookieKill'
$firewallName = 'CookieKill-HTTP-HTTPS'
$installPath = (Resolve-Path -LiteralPath $InstallDir).ProviderPath.TrimEnd('\')
$executable = Join-Path $installPath 'cookiekill.exe'
$configPath = Join-Path $installPath 'config.json'
if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) { throw "Missing $executable. Extract the complete deployment ZIP first." }
if (-not (Test-Path -LiteralPath $configPath -PathType Leaf)) { throw "Missing $configPath. Restore config.json from the deployment ZIP and configure it first." }
if ([IO.Path]::GetPathRoot($installPath).TrimEnd('\') -eq $installPath) { throw 'Install into a dedicated local directory, not a drive root.' }
if ($installPath.StartsWith('\\')) { throw 'The service must be installed on a local disk.' }
if ($installPath.Contains('"')) { throw 'The installation path cannot contain a quote.' }

# All paths are resolved against the executable, never the shell working directory.
$config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$dataValue = 'data'
$logValue = 'logs/cookiekill.log'
if ($config.PSObject.Properties['dataDir']) { $dataValue = [string]$config.dataDir }
if ($config.PSObject.Properties['logFile']) { $logValue = [string]$config.logFile }
if ([string]::IsNullOrWhiteSpace($dataValue) -or [string]::IsNullOrWhiteSpace($logValue)) { throw 'dataDir and logFile must not be empty.' }
function Resolve-PackagePath([string]$Value) {
    $candidate = $Value
    if (-not [IO.Path]::IsPathRooted($candidate)) { $candidate = Join-Path $installPath $candidate }
    $candidate = [IO.Path]::GetFullPath($candidate).TrimEnd('\')
    if (-not $candidate.StartsWith($installPath + '\', [StringComparison]::OrdinalIgnoreCase)) {
        throw 'The installer requires dataDir and logFile inside the deployment directory. Use dedicated subdirectories such as data and logs.'
    }
    return $candidate
}
$dataPath = Resolve-PackagePath $dataValue
$logPath = Resolve-PackagePath $logValue
$logDirectory = Split-Path -Parent $logPath
if ($logDirectory -eq $installPath) { throw 'Place logFile in a dedicated subdirectory, such as logs/cookiekill.log.' }
if ($dataPath.Equals($logPath, [StringComparison]::OrdinalIgnoreCase) -or $dataPath.StartsWith($logPath + '\', [StringComparison]::OrdinalIgnoreCase)) {
    throw 'logFile cannot be the data directory or one of its parent directories.'
}
foreach ($writablePath in @($dataPath, $logDirectory)) {
    foreach ($protectedPath in @($executable, $configPath, (Join-Path $installPath 'Install-Service.ps1'), (Join-Path $installPath 'Uninstall-Service.ps1'))) {
        if ($protectedPath.Equals($writablePath, [StringComparison]::OrdinalIgnoreCase) -or $protectedPath.StartsWith($writablePath + '\', [StringComparison]::OrdinalIgnoreCase)) {
            throw 'dataDir and the log directory must not contain the executable, configuration, or installer scripts.'
        }
    }
}
$isDev = $false
if ($config.PSObject.Properties['dev']) { $isDev = $config.dev -eq $true }
if (-not $isDev) {
    # email is an optional certificate contact, unrelated to SMTP authentication.
    foreach ($name in @('domain', 'smtp')) {
        if (-not $config.PSObject.Properties[$name] -or -not $config.$name) { throw "Configure '$name' in config.json before installation." }
    }
    foreach ($name in @('host', 'from')) {
        if (-not $config.smtp.PSObject.Properties[$name] -or [string]::IsNullOrWhiteSpace([string]$config.smtp.$name)) { throw "Configure 'smtp.$name' in config.json before installation." }
    }
}

# Reject links before changing any permissions, so ACL operations stay in this package.
$packageItems = @((Get-Item -LiteralPath $installPath -Force)) + @(Get-ChildItem -LiteralPath $installPath -Recurse -Force)
foreach ($item in $packageItems) {
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Remove symbolic links/junctions before installation: $($item.FullName)" }
}

function Invoke-ServiceControl([string[]]$Arguments) {
    & "$env:SystemRoot\System32\sc.exe" @Arguments | Write-Host
    if ($LASTEXITCODE -ne 0) { throw "sc.exe $($Arguments[0]) failed with exit code $LASTEXITCODE." }
}
$existing = Get-CimInstance Win32_Service -Filter "Name='$serviceName'"
$binaryPath = '"' + $executable + '"'
if ($existing) {
    if ($existing.PathName.Trim() -ne $binaryPath) {
        throw "CookieKill is already registered at '$($existing.PathName)'. Run this installer from that deployment directory, or uninstall that service first."
    }
    $controller = Get-Service -Name $serviceName
    if ($controller.Status -ne 'Stopped') {
        Stop-Service -Name $serviceName
        $controller.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(45))
    }
    Invoke-ServiceControl @('config', $serviceName, 'start=', 'auto', 'obj=', 'NT AUTHORITY\LocalService')
} else {
    # New-Service preserves the quotes in ImagePath on Windows PowerShell 5.1.
    $account = New-Object Management.Automation.PSCredential('NT AUTHORITY\LocalService', (New-Object Security.SecureString))
    New-Service -Name $serviceName -DisplayName 'CookieKill' -BinaryPathName $binaryPath -StartupType Automatic -Credential $account | Out-Null
}
Invoke-ServiceControl @('description', $serviceName, 'CookieKill multiplayer game server with automatic HTTPS.')
Invoke-ServiceControl @('sidtype', $serviceName, 'unrestricted')
Invoke-ServiceControl @('failure', $serviceName, 'reset=', '86400', 'actions=', 'restart/5000/restart/15000/restart/60000')
Invoke-ServiceControl @('failureflag', $serviceName, '1')

# Restrict the package to administrators, SYSTEM, and this specific service SID.
# LocalService cannot replace its executable/configuration; only data/logs are writable.
$serviceSid = (New-Object Security.Principal.NTAccount("NT SERVICE\$serviceName")).Translate([Security.Principal.SecurityIdentifier])
$adminSid = New-Object Security.Principal.SecurityIdentifier('S-1-5-32-544')
$systemSid = New-Object Security.Principal.SecurityIdentifier('S-1-5-18')
$inheritance = [Security.AccessControl.InheritanceFlags]'ContainerInherit, ObjectInherit'
$propagation = [Security.AccessControl.PropagationFlags]::None
$allow = [Security.AccessControl.AccessControlType]::Allow
$rootAcl = New-Object Security.AccessControl.DirectorySecurity
$rootAcl.SetAccessRuleProtection($true, $false)
foreach ($sid in @($adminSid, $systemSid)) {
    $rootAcl.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($sid, 'FullControl', $inheritance, $propagation, $allow)))
}
$rootAcl.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($serviceSid, 'ReadAndExecute', $inheritance, $propagation, $allow)))
Set-Acl -LiteralPath $installPath -AclObject $rootAcl
foreach ($item in ($packageItems | Select-Object -Skip 1)) {
    if ($item.PSIsContainer) { $acl = New-Object Security.AccessControl.DirectorySecurity }
    else { $acl = New-Object Security.AccessControl.FileSecurity }
    $acl.SetAccessRuleProtection($false, $false)
    Set-Acl -LiteralPath $item.FullName -AclObject $acl
}
foreach ($writablePath in @($dataPath, $logDirectory) | Select-Object -Unique) {
    New-Item -ItemType Directory -Path $writablePath -Force | Out-Null
    $acl = Get-Acl -LiteralPath $writablePath
    $acl.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($serviceSid, 'Modify', $inheritance, $propagation, $allow)))
    Set-Acl -LiteralPath $writablePath -AclObject $acl
}
if ([Diagnostics.EventLog]::SourceExists($serviceName)) {
    if ([Diagnostics.EventLog]::LogNameFromSourceName($serviceName, '.') -ne 'Application') { throw 'The CookieKill event source already belongs to another log.' }
} else {
    [Diagnostics.EventLog]::CreateEventSource($serviceName, 'Application')
}
if ($OpenFirewall) {
    Get-NetFirewallRule -Name $firewallName -ErrorAction SilentlyContinue | Remove-NetFirewallRule
    New-NetFirewallRule -Name $firewallName -DisplayName 'CookieKill HTTP and HTTPS' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 80,443 -Program $executable -Profile Any | Out-Null
}
if (-not $NoStart) {
    try {
        Start-Service -Name $serviceName
        (Get-Service -Name $serviceName).WaitForStatus('Running', [TimeSpan]::FromSeconds(45))
    } catch {
        throw "CookieKill could not start: $($_.Exception.Message) Check '$logPath' and Event Viewer > Windows Logs > Application > CookieKill."
    }
}
Write-Host "CookieKill installed in $installPath. Configuration and existing progress were preserved."
Get-Service -Name $serviceName
