$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
$goCommand = Get-Command go -ErrorAction SilentlyContinue
if ($goCommand) {
    $goExecutable = $goCommand.Source
} elseif (Test-Path -LiteralPath 'C:\Program Files\Go\bin\go.exe') {
    $goExecutable = 'C:\Program Files\Go\bin\go.exe'
} else {
    throw 'Install Go 1.26 or later, then run this script again.'
}
& $goExecutable run . -dev @args
exit $LASTEXITCODE
