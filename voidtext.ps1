<#
.SYNOPSIS
  VoidText - PowerShell Management Script
.DESCRIPTION
  Windows-native equivalent of Makefile. Supported commands:
    build, dev, run, stop, clean, rebuild, run-background, status, restart, logs, help
.EXAMPLE
  .\voidtext.ps1 build
  .\voidtext.ps1 dev
  .\voidtext.ps1 stop
  .\voidtext.ps1 status
#>

[CmdletBinding(SupportsShouldProcess)]
param(
  [Parameter(Position = 0)]
  [ValidateSet('build', 'dev', 'run', 'stop', 'clean', 'rebuild', 'run-background', 'status', 'restart', 'logs', 'help')]
  [string]$Command = 'help'
)

$ErrorActionPreference = 'Stop'

$BINARY_NAME = 'voidtext.exe'
$MAIN_PACKAGE = './cmd/voidtext/'
$SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Path
$BINARY_PATH = Join-Path $SCRIPT_DIR $BINARY_NAME
$LOG_DIR = Join-Path $SCRIPT_DIR 'logs'
$LOG_FILE = Join-Path $LOG_DIR 'voidtext.log'
$PID_FILE = Join-Path $LOG_DIR 'voidtext.pid'

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

function Ensure-LogDir {
  if (-not (Test-Path $LOG_DIR)) {
    New-Item -ItemType Directory -Path $LOG_DIR -Force | Out-Null
  }
}

function Get-RunningProcess {
  Get-Process -Name 'voidtext' -ErrorAction SilentlyContinue | Where-Object {
    $_.Path -and ($_.Path -eq $BINARY_PATH)
  }
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
  $procs = Get-RunningProcess
  if ($procs) {
    $procs | Stop-Process -Force
    Write-Host "  Stopped" -ForegroundColor Green
  } else {
    Write-Host "  No running process found" -ForegroundColor Yellow
  }
  # Clean up PID file
  if (Test-Path $PID_FILE) {
    Remove-Item -Force $PID_FILE -ErrorAction SilentlyContinue
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

  # Check if already running
  $existing = Get-RunningProcess
  if ($existing) {
    Write-Host "  Already running (PID: $($existing.Id))" -ForegroundColor Yellow
    Write-Host "  Use '.\voidtext.ps1 stop' to stop first" -ForegroundColor Yellow
    Write-Host ""
    exit 1
  }

  # Ensure log directory exists
  Ensure-LogDir

  Write-Host "  Config : .env" -ForegroundColor Gray
  Write-Host "  Data   : DATA_DIR (default: .\data)" -ForegroundColor Gray
  Write-Host "  Logs   : $LOG_FILE" -ForegroundColor Gray
  Write-Host ""

  # Start process with output redirected to log file
  $proc = Start-Process -FilePath $BINARY_PATH -WindowStyle Hidden -PassThru `
    -RedirectStandardOutput (Join-Path $LOG_DIR 'stdout.log') `
    -RedirectStandardError (Join-Path $LOG_DIR 'stderr.log')

  # Save PID to file
  $proc.Id | Out-File -FilePath $PID_FILE -Force

  # Wait a moment to check if process started successfully
  Start-Sleep -Milliseconds 500
  $checkProc = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
  if ($checkProc) {
    Write-Host "  Started in background" -ForegroundColor Green
    Write-Host "  PID    : $($proc.Id)" -ForegroundColor Cyan
    $port = Get-Port
    Write-Host "  URL    : http://localhost:$port" -ForegroundColor Cyan
    Write-Host "  Logs   : .\voidtext.ps1 logs" -ForegroundColor Yellow
    Write-Host "  Stop   : .\voidtext.ps1 stop" -ForegroundColor Yellow
  } else {
    Write-Host "  Failed to start! Check logs:" -ForegroundColor Red
    Write-Host "    $LOG_FILE" -ForegroundColor Red
    if (Test-Path (Join-Path $LOG_DIR 'stderr.log')) {
      Write-Host ""
      Write-Host "  Error output:" -ForegroundColor Red
      Get-Content (Join-Path $LOG_DIR 'stderr.log') -Tail 20 | ForEach-Object {
        Write-Host "    $_" -ForegroundColor Red
      }
    }
  }
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
}

function Invoke-Status {
  Write-Banner "Status"
  $procs = Get-RunningProcess
  if ($procs) {
    $proc = $procs | Select-Object -First 1
    Write-Host "  Status : Running" -ForegroundColor Green
    Write-Host "  PID    : $($proc.Id)" -ForegroundColor Cyan
    Write-Host "  Memory : $([math]::Round($proc.WorkingSet64 / 1MB, 2)) MB" -ForegroundColor Cyan
    Write-Host "  CPU    : $([math]::Round($proc.CPU, 2)) seconds" -ForegroundColor Cyan
    Write-Host "  Started: $($proc.StartTime.ToString('yyyy-MM-dd HH:mm:ss'))" -ForegroundColor Cyan
    $port = Get-Port
    Write-Host "  URL    : http://localhost:$port" -ForegroundColor Cyan

    # Check if port is actually listening
    $listening = Test-NetConnection -ComputerName localhost -Port $port -WarningAction SilentlyContinue -InformationLevel Quiet
    if ($listening) {
      Write-Host "  Port   : Listening" -ForegroundColor Green
    } else {
      Write-Host "  Port   : Not listening (may be starting up)" -ForegroundColor Yellow
    }
  } else {
    Write-Host "  Status : Not running" -ForegroundColor Yellow
    # Check if PID file exists (unexpected stop)
    if (Test-Path $PID_FILE) {
      $lastPid = Get-Content $PID_FILE -ErrorAction SilentlyContinue
      Write-Host "  Last PID: $lastPid (process may have crashed)" -ForegroundColor Red
      Remove-Item -Force $PID_FILE -ErrorAction SilentlyContinue
    }
  }
  Write-Host ""
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
}

function Invoke-Restart {
  Write-Banner "Restart"
  Write-Host "  Stopping..." -ForegroundColor Gray
  Invoke-Stop
  Write-Host "  Starting..." -ForegroundColor Gray
  Invoke-RunBackground
}

function Invoke-Logs {
  Write-Banner "Logs"
  Write-Host "  Log directory: $LOG_DIR" -ForegroundColor Gray
  Write-Host ""

  # Show main log file
  if (Test-Path $LOG_FILE) {
    Write-Host "=== voidtext.log (last 50 lines) ===" -ForegroundColor Cyan
    Get-Content $LOG_FILE -Tail 50 | ForEach-Object {
      Write-Host "  $_" -ForegroundColor White
    }
    Write-Host ""
  }

  # Show stderr if exists and has content
  $stderrFile = Join-Path $LOG_DIR 'stderr.log'
  if (Test-Path $stderrFile) {
    $stderrContent = Get-Content $stderrFile -ErrorAction SilentlyContinue
    if ($stderrContent) {
      Write-Host "=== stderr.log (last 20 lines) ===" -ForegroundColor Yellow
      Get-Content $stderrFile -Tail 20 | ForEach-Object {
        Write-Host "  $_" -ForegroundColor Yellow
      }
      Write-Host ""
    }
  }

  # Show stdout if exists
  $stdoutFile = Join-Path $LOG_DIR 'stdout.log'
  if (Test-Path $stdoutFile) {
    Write-Host "=== stdout.log (last 30 lines) ===" -ForegroundColor Gray
    Get-Content $stdoutFile -Tail 30 | ForEach-Object {
      Write-Host "  $_" -ForegroundColor Gray
    }
    Write-Host ""
  }

  if (-not (Test-Path $LOG_FILE) -and -not (Test-Path $stdoutFile)) {
    Write-Host "  No log files found" -ForegroundColor Yellow
    Write-Host "  Start the service first: .\voidtext.ps1 run-background" -ForegroundColor Yellow
  }
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
  Write-Host "  Build Commands:" -ForegroundColor Yellow
  Write-Host "    build           Compile binary" -ForegroundColor Green
  Write-Host "    clean           Remove binary (keep data)" -ForegroundColor Green
  Write-Host "    rebuild         Stop + clean + build" -ForegroundColor Green
  Write-Host ""
  Write-Host "  Runtime Commands:" -ForegroundColor Yellow
  Write-Host "    dev             Development mode (console logs)" -ForegroundColor Green
  Write-Host "    run             Run compiled binary (foreground)" -ForegroundColor Green
  Write-Host "    run-background  Run as background service" -ForegroundColor Green
  Write-Host "    stop            Stop running process" -ForegroundColor Green
  Write-Host "    restart         Restart background service" -ForegroundColor Green
  Write-Host ""
  Write-Host "  Monitoring Commands:" -ForegroundColor Yellow
  Write-Host "    status          Check service status" -ForegroundColor Green
  Write-Host "    logs            View service logs" -ForegroundColor Green
  Write-Host "    help            Show this help" -ForegroundColor Green
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
  'status'         { Invoke-Status }
  'restart'        { Invoke-Restart }
  'logs'           { Invoke-Logs }
  default          { Show-Help }
}
