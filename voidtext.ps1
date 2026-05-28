<#
.SYNOPSIS
  湮文 VoidText — PowerShell 管理脚本
.DESCRIPTION
  对应 Makefile 功能的 PowerShell 实现。支持的命令：
    build, dev, run, stop, clean, rebuild, run-background, help
.EXAMPLE
  .\voidtext.ps1 build
  .\voidtext.ps1 run-background
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

function Write-Section {
  param([string]$Title)
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host "  湮文 VoidText — $Title" -ForegroundColor Cyan
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
}

function Get-Port {
  if ($env:PORT) { return $env:PORT }
  return '8080'
}

function Invoke-Build {
  Write-Section "编译项目"
  Write-Host "[1/2] 检查 Go 依赖..."
  go mod tidy
  if ($LASTEXITCODE -ne 0) { Write-Host "  ✗ go mod tidy 失败" -ForegroundColor Red; exit 1 }
  go mod verify
  if ($LASTEXITCODE -ne 0) { Write-Host "  ✗ go mod verify 失败" -ForegroundColor Red; exit 1 }
  Write-Host ""
  Write-Host "[2/2] 编译二进制文件..."
  go build -ldflags "-s -w" -o $BINARY_NAME $MAIN_PACKAGE
  if ($LASTEXITCODE -ne 0) { Write-Host "  ✗ 编译失败" -ForegroundColor Red; exit 1 }
  Write-Host ""
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host "  ✓ 编译成功" -ForegroundColor Green

  if (Test-Path $BINARY_PATH) {
    $file = Get-Item $BINARY_PATH
    $size = if ($file.Length -lt 1MB) {
      "{0:N0} KB" -f ($file.Length / 1KB)
    } else {
      "{0:N2} MB" -f ($file.Length / 1MB)
    }
    Write-Host "  输出: $BINARY_NAME" -ForegroundColor Green
    Write-Host "  大小: $size" -ForegroundColor Green
  }
  Write-Host ""
  Write-Host "  运行方式:" -ForegroundColor Gray
  Write-Host "    .\voidtext.ps1 dev    — 开发者模式（控制台日志）" -ForegroundColor Gray
  Write-Host "    .\voidtext.ps1 run    — 生产环境运行" -ForegroundColor Gray
  Write-Host "========================================" -ForegroundColor Cyan
}

function Invoke-Dev {
  Write-Section "开发者模式"
  Write-Host "  功能说明:" -ForegroundColor Gray
  Write-Host "  • 控制台实时打印日志 — 便于排查问题" -ForegroundColor Gray
  Write-Host "  • Gin debug 模式 — 显示路由和请求详情" -ForegroundColor Gray
  Write-Host "  • go run 直接运行 — 无需提前编译" -ForegroundColor Gray
  Write-Host ""
  Write-Host "  停止方式: Ctrl+C" -ForegroundColor Yellow
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
  Write-Section "生产模式启动"
  Write-Host ""
  if (Test-Path $BINARY_PATH) {
    Write-Host "  ✓ 检测到编译产物: $BINARY_NAME" -ForegroundColor Green
  } else {
    Write-Host "  ✗ 未检测到编译产物，请先执行 build" -ForegroundColor Red
    Write-Host ""
    exit 1
  }
  Write-Host ""
  Write-Host "  启动说明:" -ForegroundColor Gray
  Write-Host "  • 配置文件: .env（从工作目录加载）" -ForegroundColor Gray
  Write-Host "  • 数据目录: DATA_DIR（默认 ./data）" -ForegroundColor Gray
  Write-Host ""

  $port = Get-Port
  Write-Host "  访问地址: http://localhost:$port" -ForegroundColor Cyan
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
  & $BINARY_PATH
}

function Invoke-Stop {
  Write-Section "停止进程"
  $procs = Get-Process -Name 'voidtext' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $BINARY_PATH }
  if ($procs) {
    $procs | Stop-Process -Force
    Write-Host "  ✓ 已停止进程" -ForegroundColor Green
  } else {
    Write-Host "  - 未发现运行中的进程" -ForegroundColor Yellow
  }
  Write-Host ""
}

function Invoke-Clean {
  Invoke-Stop
  Write-Section "清理编译产物"
  Write-Host ""
  if (Test-Path $BINARY_PATH) {
    Remove-Item -Force $BINARY_PATH
    Write-Host "  ✓ 已删除: $BINARY_NAME" -ForegroundColor Green
  } else {
    Write-Host "  - 未发现编译产物" -ForegroundColor Yellow
  }
  Write-Host ""
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host "  ✓ 清理完毕" -ForegroundColor Green
  Write-Host "  已删除: 编译产物" -ForegroundColor Green
  Write-Host "  保留: 源代码、配置文件 (.env)、数据目录 (data/):" -ForegroundColor Gray
  Write-Host "    • 数据库 (cleaning.db)" -ForegroundColor Gray
  Write-Host "    • 上传文件 (uploads/)" -ForegroundColor Gray
  Write-Host "    • 备份文件 (backups/)" -ForegroundColor Gray
  Write-Host "    • 临时文件 (temp/)" -ForegroundColor Gray
  Write-Host "========================================" -ForegroundColor Cyan
}

function Invoke-Rebuild {
  Write-Section "重新编译（停止 + 清理 + 编译）"
  Invoke-Stop
  Invoke-Clean
  Invoke-Build
}

function Invoke-RunBackground {
  Write-Section "后台模式启动"
  Write-Host ""
  if (Test-Path $BINARY_PATH) {
    Write-Host "  ✓ 检测到编译产物: $BINARY_NAME" -ForegroundColor Green
  } else {
    Write-Host "  ✗ 未检测到编译产物，请先执行 build" -ForegroundColor Red
    Write-Host ""
    exit 1
  }
  Write-Host "  启动说明:" -ForegroundColor Gray
  Write-Host "  • 配置文件: .env（从工作目录加载）" -ForegroundColor Gray
  Write-Host "  • 数据目录: DATA_DIR（默认 ./data）" -ForegroundColor Gray
  Write-Host "  • 日志: 重定向到 NUL" -ForegroundColor Gray
  Write-Host ""

  $proc = Start-Process -FilePath $BINARY_PATH -WindowStyle Hidden -PassThru
  Write-Host "  ✓ 已后台启动" -ForegroundColor Green
  Write-Host "  进程 PID: $($proc.Id)" -ForegroundColor Cyan
  $port = Get-Port
  Write-Host "  访问地址: http://localhost:$port" -ForegroundColor Cyan
  Write-Host "  停止方式: .\voidtext.ps1 stop 或 Stop-Process -Id $($proc.Id)" -ForegroundColor Yellow
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
}

function Show-Help {
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host "  湮文 VoidText — 项目管理" -ForegroundColor Cyan
  Write-Host "========================================" -ForegroundColor Cyan
  Write-Host ""
  Write-Host "  用法: .\voidtext.ps1 [-Command] <命令>" -ForegroundColor White
  Write-Host ""
  Write-Host "  build            编译二进制文件" -ForegroundColor Green
  Write-Host "                  执行 go mod tidy/verify + go build" -ForegroundColor Gray
  Write-Host ""
  Write-Host "  dev              开发者模式运行" -ForegroundColor Green
  Write-Host "                  控制台打印日志, Gin debug 模式, go run 热重载" -ForegroundColor Gray
  Write-Host ""
  Write-Host "  run              生产环境运行" -ForegroundColor Green
  Write-Host "                  运行已编译的 $BINARY_NAME 二进制文件" -ForegroundColor Gray
  Write-Host ""
  Write-Host "  stop             停止运行中的进程" -ForegroundColor Green
  Write-Host "                  Stop-Process 终止 voidtext 进程" -ForegroundColor Gray
  Write-Host ""
  Write-Host "  clean            删除编译产物" -ForegroundColor Green
  Write-Host "                  保留 数据库、上传文件、备份等数据" -ForegroundColor Gray
  Write-Host ""
  Write-Host "  rebuild          停止 + 清理 + 重新编译" -ForegroundColor Green
  Write-Host ""
  Write-Host "  run-background   后台运行（不阻塞终端）" -ForegroundColor Green
  Write-Host ""
  Write-Host "  help             显示此帮助" -ForegroundColor Green
  Write-Host ""
  Write-Host "========================================" -ForegroundColor Cyan
}

# -- Main dispatch --
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
