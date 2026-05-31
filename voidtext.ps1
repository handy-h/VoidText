<#
.SYNOPSIS
  VoidText - PowerShell Management Script
.DESCRIPTION
  Windows-native equivalent of Makefile. Supported commands:
    build, dev, run, stop, clean, rebuild, run-background, help
.EXAMPLE
  .\voidtext.ps1 build
  .\voidtext.ps1 dev
  .\voidtext.ps1 stop
#>

[CmdletBinding(SupportsShouldProcess)]
param(
  [Parameter(Position = 0)]
  [ValidateSet('build', 'dev', 'run', 'stop', 'clean', 'rebuild', 'run-background', 'help')]
  [string]$Command = 'help'
)

$ErrorActionPreference = 'Stop'

$BINARY_NAME = 'voidtext.exe'
$MAIN_PACKAGE = './cmd/voidtext/'
$SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Path
$BINARY_PATH = Join-Path $SCRIPT_DIR $BINARY_NAME

function Write-Banner {
  param([string]$Title)
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host "  VoidText - $Title" -ForegroundColor Cyan
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
}

function Get-Port {
  if ($env:PORT) { return $env:PORT }
  return '8080'
}

function Invoke-Build {
  Write-Banner "Build"
  Write-Host "[1/2] Checking Go dependencies..."
  go mod tidy
  if ($LASTEXITCODE -ne 0) { Write-Host "  go mod tidy failed" -ForegroundColor Red; exit 1 }
  go mod verify
  if ($LASTEXITCODE -ne 0) { Write-Host "  go mod verify failed" -ForegroundColor Red; exit 1 }
  Write-Host ""
  Write-Host "[2/2] Compiling..."
  go build -ldflags "-s -w" -o $BINARY_NAME $MAIN_PACKAGE
  if ($LASTEXITCODE -ne 0) { Write-Host "  Build failed" -ForegroundColor Red; exit 1 }
  Write-Host ""
  Write-Host "  Done." -ForegroundColor Green
  if (Test-Path $BINARY_PATH) {
    $file = Get-Item $BINARY_PATH
    $size = if ($file.Length -lt 1MB) { "{0:N0} KB" -f ($file.Length / 1KB) } else { "{0:N2} MB" -f ($file.Length / 1MB) }
    Write-Host "  Output: $BINARY_NAME" -ForegroundColor Green
    Write-Host "  Size  : $size" -ForegroundColor Green
  }
  Write-Host ""
  Write-Host "  Next steps:" -ForegroundColor Gray
  Write-Host "    .\voidtext.ps1 dev    - Development mode" -ForegroundColor Gray
  Write-Host "    .\voidtext.ps1 run    - Production mode" -ForegroundColor Gray
  Write-Host "========================================" -ForegroundColor Cyan
}

function Invoke-Dev {
  Write-Banner "Development Mode"
  Write-Host "  Features:" -ForegroundColor Gray
  Write-Host "    - Console logging for real-time debugging" -ForegroundColor Gray
  Write-Host "    - Gin debug mode (routes visible)" -ForegroundColor Gray
  Write-Host "    - go run (no compile needed)" -ForegroundColor Gray
  Write-Host ""
  Write-Host "  Stop: Ctrl+C" -ForegroundColor Yellow
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""

  $origLOG = $env:LOG_TO_CONSOLE
  $origGIN = $env:GIN_MODE
  try {
    $env:LOG_TO_CONSOLE = 'true'
    $env:GIN_MODE = 'debug'
    go run $MAIN_PACKAGE
  } finally {
    $env:LOG_TO_CONSOLE = $origLOG
    $env:GIN_MODE = $origGIN
  }
}

function Invoke-Run {
  Write-Banner "Production Mode"
  Write-Host ""
  if (Test-Path $BINARY_PATH) {
    Write-Host "  Found: $BINARY_NAME" -ForegroundColor Green
  } else {
    Write-Host "  Binary not found. Run: .\voidtext.ps1 build" -ForegroundColor Red
    Write-Host ""
    exit 1
  }
  Write-Host ""
  Write-Host "  Config : .env" -ForegroundColor Gray
  Write-Host "  Data   : DATA_DIR (default: .\data)" -ForegroundColor Gray
  Write-Host ""
  $port = Get-Port
  Write-Host "  URL: http://localhost:$port" -ForegroundColor Cyan
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
  & $BINARY_PATH
}

function Invoke-Stop {
  Write-Banner "Stop"
  # Match by process name and verify path to avoid killing unrelated processes
  $procs = Get-Process -Name 'voidtext' -ErrorAction SilentlyContinue | Where-Object {
    $_.Path -and ($_.Path -eq $BINARY_PATH)
  }
  if ($procs) {
    $procs | Stop-Process -Force
    Write-Host "  Stopped" -ForegroundColor Green
  } else {
    Write-Host "  No running process found" -ForegroundColor Yellow
  }
  Write-Host ""
}

function Invoke-Clean {
  Invoke-Stop
  Write-Banner "Clean"
  Write-Host ""
  if (Test-Path $BINARY_PATH) {
    Remove-Item -Force $BINARY_PATH
    Write-Host "  Removed: $BINARY_NAME" -ForegroundColor Green
  } else {
    Write-Host "  No binary to remove" -ForegroundColor Yellow
  }
  Write-Host ""
  Write-Host "  Kept: source, .env, data\" -ForegroundColor Gray
  Write-Host "    - database (cleaning.db)" -ForegroundColor Gray
  Write-Host "    - uploads\" -ForegroundColor Gray
  Write-Host "    - backups\" -ForegroundColor Gray
  Write-Host "    - temp\" -ForegroundColor Gray
  Write-Host "========================================" -ForegroundColor Cyan
}

function Invoke-Rebuild {
  Write-Banner "Rebuild"
  Invoke-Stop
  Invoke-Clean
  Invoke-Build
}

function Invoke-RunBackground {
  Write-Banner "Background Mode"
  Write-Host ""
  if (Test-Path $BINARY_PATH) {
    Write-Host "  Found: $BINARY_NAME" -ForegroundColor Green
  } else {
    Write-Host "  Binary not found. Run: .\voidtext.ps1 build" -ForegroundColor Red
    Write-Host ""
    exit 1
  }
  Write-Host "  Config : .env" -ForegroundColor Gray
  Write-Host "  Data   : DATA_DIR (default: .\data)" -ForegroundColor Gray
  Write-Host "  Logs   : Written to log files" -ForegroundColor Gray
  Write-Host ""

  $proc = Start-Process -FilePath $BINARY_PATH -WindowStyle Hidden -PassThru
  Write-Host "  Started in background" -ForegroundColor Green
  Write-Host "  PID    : $($proc.Id)" -ForegroundColor Cyan
  $port = Get-Port
  Write-Host "  URL    : http://localhost:$port" -ForegroundColor Cyan
  Write-Host "  Stop   : .\voidtext.ps1 stop" -ForegroundColor Yellow
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
}

function Show-Help {
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host "  VoidText - PowerShell Commands" -ForegroundColor Cyan
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
  Write-Host "  Usage: .\voidtext.ps1 [-Command] <cmd>" -ForegroundColor White
  Write-Host ""
  Write-Host "  build           Compile binary" -ForegroundColor Green
  Write-Host "  dev             Development mode (console logs)" -ForegroundColor Green
  Write-Host "  run             Run compiled binary" -ForegroundColor Green
  Write-Host "  stop            Stop running process" -ForegroundColor Green
  Write-Host "  clean           Remove binary (keep data)" -ForegroundColor Green
  Write-Host "  rebuild         Stop + clean + build" -ForegroundColor Green
  Write-Host "  run-background  Run in background (hidden window)" -ForegroundColor Green
  Write-Host "  help            Show this help" -ForegroundColor Green
  Write-Host ""
  Write-Host "========================================" -ForegroundColor Cyan
}

# Dispatch
switch ($Command) {
  'build'          { Invoke-Build }
  'dev'            { Invoke-Dev }
  'run'            { Invoke-Run }
  'stop'           { Invoke-Stop }
  'clean'          { Invoke-Clean }
  'rebuild'        { Invoke-Rebuild }
  'run-background' { Invoke-RunBackground }
  default          { Show-Help }
}
