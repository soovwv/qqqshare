param([string]$Version = "0.2.0")
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$go = Join-Path $root ".tools\go\bin\go.exe"
if (-not (Test-Path $go)) { throw "Portable Go toolchain not found at $go" }
$dist = Join-Path $root "dist"
if (Test-Path $dist) { Remove-Item -LiteralPath $dist -Recurse -Force }
New-Item -ItemType Directory -Force -Path $dist | Out-Null
$env:CGO_ENABLED = "0"
$ldflags = "-s -w -X main.version=$Version"

function Build-Go([string]$os, [string]$arch, [string]$out) {
  $env:GOOS=$os; $env:GOARCH=$arch
  & $go build -trimpath -buildvcs=false -ldflags $ldflags -o $out $root
  if ($LASTEXITCODE -ne 0) { throw "Build failed: $os/$arch" }
}

$win64 = Join-Path $dist "QQQShare-Windows-x64"
$winArm = Join-Path $dist "QQQShare-Windows-arm64"
New-Item -ItemType Directory -Force -Path $win64,$winArm | Out-Null
Build-Go "windows" "amd64" (Join-Path $win64 "QQQShare.exe")
Build-Go "windows" "arm64" (Join-Path $winArm "QQQShare.exe")
Compress-Archive -Path "$win64\*" -DestinationPath "$win64.zip"
Compress-Archive -Path "$winArm\*" -DestinationPath "$winArm.zip"

foreach ($arch in @("amd64","arm64")) {
  $label = if ($arch -eq "amd64") { "Intel" } else { "AppleSilicon" }
  $bundle = Join-Path $dist "QQQShare-macOS-$label\QQQShare.app"
  $macos = Join-Path $bundle "Contents\MacOS"; $resources = Join-Path $bundle "Contents\Resources"
  New-Item -ItemType Directory -Force -Path $macos,$resources | Out-Null
  Build-Go "darwin" $arch (Join-Path $macos "QQQShare")
  Copy-Item (Join-Path $root "assets\app-icon.icns") (Join-Path $resources "AppIcon.icns")
  $plist=(Get-Content -Raw (Join-Path $root "packaging\Info.plist")).Replace("__VERSION__",$Version)
  Set-Content -LiteralPath (Join-Path $bundle "Contents\Info.plist") -Value $plist -Encoding UTF8
  Compress-Archive -Path $bundle -DestinationPath (Join-Path $dist "QQQShare-macOS-$label.zip")
}

Get-ChildItem $dist -Filter *.zip | Get-FileHash -Algorithm SHA256 |
  ForEach-Object { "$($_.Hash.ToLower())  $([IO.Path]::GetFileName($_.Path))" } |
  Set-Content -LiteralPath (Join-Path $dist "checksums.txt") -Encoding ascii
Write-Host "Release $Version created in $dist"
