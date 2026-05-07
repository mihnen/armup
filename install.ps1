#Requires -Version 5
# armup installer (Windows).
#
#   iwr https://raw.githubusercontent.com/mihnen/armup/master/install.ps1 | iex
#
# Picks the latest release, verifies SHA-256, drops armup.exe into
# $env:ARMUP_INSTALL_DIR (default: $env:USERPROFILE\bin). Then run `armup init`.

$ErrorActionPreference = 'Stop'

$Owner = 'mihnen'
$Repo  = 'armup'
$InstallDir = if ($env:ARMUP_INSTALL_DIR) { $env:ARMUP_INSTALL_DIR } else { Join-Path $env:USERPROFILE 'bin' }

# armup ships windows-amd64 only today.
$os   = 'windows'
$arch = 'amd64'

Write-Host 'Querying latest release...'
# Use /releases (not /releases/latest) so we still find prerelease tags;
# /latest skips prereleases and 404s if there are no stable releases.
$releases = Invoke-RestMethod "https://api.github.com/repos/$Owner/$Repo/releases" -UseBasicParsing
$release = $releases | Where-Object { -not $_.draft } | Select-Object -First 1
$tag = $release.tag_name
if (-not $tag) { throw "could not determine latest release tag" }
Write-Host "Latest release: $tag"

$file    = "armup-$tag-$os-$arch.zip"
$url     = "https://github.com/$Owner/$Repo/releases/download/$tag/$file"
$sumsUrl = "https://github.com/$Owner/$Repo/releases/download/$tag/SHA256SUMS"

$tmp = New-Item -ItemType Directory -Path (Join-Path $env:TEMP "armup-install-$([guid]::NewGuid().ToString('N'))")
try {
    $archivePath = Join-Path $tmp.FullName $file
    Write-Host "Downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $archivePath -UseBasicParsing

    Write-Host 'Verifying checksum'
    $sumsPath = Join-Path $tmp.FullName 'SHA256SUMS'
    Invoke-WebRequest -Uri $sumsUrl -OutFile $sumsPath -UseBasicParsing

    $expected = $null
    foreach ($line in Get-Content $sumsPath) {
        if ($line -match "^([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($file))\s*$") {
            $expected = $matches[1].ToLower()
            break
        }
    }
    if (-not $expected) { throw "could not find $file in SHA256SUMS" }
    $actual = (Get-FileHash $archivePath -Algorithm SHA256).Hash.ToLower()
    if ($expected -ne $actual) {
        throw "checksum mismatch: expected $expected, got $actual"
    }

    Expand-Archive -Path $archivePath -DestinationPath $tmp.FullName -Force
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }
    $src = Join-Path $tmp.FullName "armup-$tag-$os-$arch\armup.exe"
    Copy-Item -Force $src (Join-Path $InstallDir 'armup.exe')

    Write-Host
    Write-Host "Installed armup $tag to $InstallDir\armup.exe"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not ($userPath -split ';' | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) {
        Write-Host
        Write-Host "WARNING: $InstallDir is not on your User PATH."
        Write-Host 'Add it via:  System Properties -> Environment Variables -> User Variables -> Path'
    }
    Write-Host
    Write-Host "Next: run 'armup init' to create the toolchain data directory and"
    Write-Host "      add its bin/ to PATH (HKCU\Environment\Path), then open a"
    Write-Host '      fresh terminal.'
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
