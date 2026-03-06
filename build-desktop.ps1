param(
  [switch]$SkipInstall,
  [switch]$Run
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Require-Command {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Name,
    [string]$InstallHint
  )

  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    Write-Error "$Name was not found in PATH. $InstallHint"
  }
}

function Assert-LastExitCode {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Action
  )

  if ($LASTEXITCODE -ne 0) {
    throw "$Action failed with exit code $LASTEXITCODE"
  }
}

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$windowDir = Join-Path $repoRoot 'window'
$frontendDistDir = Join-Path $repoRoot 'dist'
$windowDistDir = Join-Path $windowDir 'dist'
$releaseDir = Join-Path $repoRoot 'release\web-serial-debug-windows'
$exePath = Join-Path $releaseDir 'web-serial-debug.exe'
$zipPath = Join-Path $repoRoot 'release\web-serial-debug-windows.zip'

Write-Host "Repository root: $repoRoot"

Require-Command -Name 'npm' -InstallHint 'Install Node.js 18+ first.'
Require-Command -Name 'go' -InstallHint 'Install Go 1.20+ and reopen the terminal.'

Push-Location $repoRoot
try {
  if (-not $SkipInstall) {
    Write-Host "Installing frontend dependencies..."
    npm install
    Assert-LastExitCode 'npm install'
  }

  Write-Host "Building desktop frontend..."
  npm run build:desktop
  Assert-LastExitCode 'npm run build:desktop'

  if (-not (Test-Path $frontendDistDir)) {
    throw "Frontend build output was not created: $frontendDistDir"
  }

  Write-Host "Syncing frontend bundle into window/dist..."
  if (Test-Path $windowDistDir) {
    Remove-Item -Recurse -Force $windowDistDir
  }
  Copy-Item -Recurse -Force $frontendDistDir $windowDistDir

  Write-Host "Preparing release directory..."
  if (Test-Path $releaseDir) {
    Remove-Item -Recurse -Force $releaseDir
  }
  New-Item -ItemType Directory -Force $releaseDir | Out-Null

  Push-Location $windowDir
  try {
    Write-Host "Generating app icon..."
    node (Join-Path $repoRoot 'scripts\build-icon.mjs')
    Assert-LastExitCode 'build-icon'

    Write-Host "Compiling icon resource..."
    $windresCandidates = @(
      (Join-Path $env:USERPROFILE 'tools\llvm-mingw\llvm-mingw-20260224-msvcrt-x86_64\bin\x86_64-w64-mingw32-windres.exe'),
      'C:\msys64\mingw64\bin\windres.exe',
      'C:\Program Files\Git\mingw64\bin\windres.exe'
    )
    $windres = $windresCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $windres) {
      $wr = Get-Command windres -ErrorAction SilentlyContinue
      if ($wr) { $windres = $wr.Source }
    }
    if ($windres) {
      & $windres -o rsrc_windows.syso resource.rc
      Assert-LastExitCode 'windres'
      Write-Host "  Icon resource compiled: rsrc_windows.syso"
    } else {
      Write-Warning 'windres not found, building without icon'
    }

    Write-Host "Building desktop executable..."
    go build -o $exePath
    Assert-LastExitCode 'go build'
  }
  finally {
    Pop-Location
  }

  Write-Host "Copying frontend bundle next to the executable..."
  Copy-Item -Recurse -Force $windowDistDir (Join-Path $releaseDir 'dist')

  Write-Host "Creating zip release package..."
  if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath
  }
  Compress-Archive -Path (Join-Path $releaseDir '*') -DestinationPath $zipPath

  Write-Host ""
  Write-Host "Desktop package is ready:"
  Write-Host "  EXE : $exePath"
  Write-Host "  DIST: $(Join-Path $releaseDir 'dist')"
  Write-Host "  ZIP : $zipPath"

  if ($Run) {
    Write-Host "Launching desktop app..."
    Start-Process -FilePath $exePath -WorkingDirectory $releaseDir
  }
}
finally {
  Pop-Location
}
