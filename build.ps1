# ==========================================================
# Build GameAP WebMonBot (Windows and Linux binaries)
# ==========================================================
$ErrorActionPreference = "Stop"

$SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Definition
$BIN_DIR = Join-Path $SCRIPT_DIR "bin"

if (-not (Test-Path $BIN_DIR)) {
    New-Item -ItemType Directory -Path $BIN_DIR | Out-Null
}

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host " Building GameAP WebMonBot (Go)" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# 0. Generate Windows icon and metadata resource if go-winres is available
if (Get-Command go-winres -ErrorAction SilentlyContinue) {
    if (Test-Path (Join-Path $SCRIPT_DIR "assets\icon.ico")) {
        go-winres simply --arch amd64 --icon (Join-Path $SCRIPT_DIR "assets\icon.ico") --file-version 1.0.0 --product-version 1.0.0 --product-name "GameAP WebMon" --file-description "GameAP WebMonBot Discord Server Monitor" --copyright "br1ckz" --out (Join-Path $SCRIPT_DIR "cmd\bot\rsrc") 2>$null
    }
}

# 1. Build Windows binary
$WIN_OUTPUT = Join-Path $BIN_DIR "webmonbot.exe"
Write-Host "-> Compiling Windows binary ($WIN_OUTPUT)..." -ForegroundColor Yellow
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o $WIN_OUTPUT ./cmd/bot

# 2. Build Linux binary
$LINUX_OUTPUT = Join-Path $BIN_DIR "webmonbot-linux-amd64"
Write-Host "-> Compiling Linux binary ($LINUX_OUTPUT)..." -ForegroundColor Yellow
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o $LINUX_OUTPUT ./cmd/bot

# Reset env
Remove-Item env:GOOS -ErrorAction SilentlyContinue
Remove-Item env:GOARCH -ErrorAction SilentlyContinue
Remove-Item env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Build complete!" -ForegroundColor Green
Get-ChildItem -Path $BIN_DIR | Format-Table Name, Length, LastWriteTime
