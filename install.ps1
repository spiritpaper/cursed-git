# Cursed GIT installer for Windows:
#
#   irm https://raw.githubusercontent.com/spiritpaper/cursed-git/main/install.ps1 | iex
#
# Downloads the latest release, checks it against the release's checksums, puts cg.exe in
# %LOCALAPPDATA%\cursed-git\bin and adds that folder to your user PATH.
# Pick a version with $env:CG_VERSION = "v0.1.0".
$ErrorActionPreference = "Stop"

$repo = "spiritpaper/cursed-git"
$dir = if ($env:CG_INSTALL_DIR) { $env:CG_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "cursed-git\bin" }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { throw "cg install: unsupported CPU $($env:PROCESSOR_ARCHITECTURE)" }
}

$version = $env:CG_VERSION
if (-not $version) {
    $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
}

$name = "cg-$version-windows-$arch"
$base = "https://github.com/$repo/releases/download/$version"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Write-Host "Downloading Cursed GIT $version for windows/$arch..."
    Invoke-WebRequest "$base/$name.zip" -OutFile "$tmp\$name.zip" -UseBasicParsing
    Invoke-WebRequest "$base/checksums.txt" -OutFile "$tmp\checksums.txt" -UseBasicParsing

    $want = (Get-Content "$tmp\checksums.txt" | Where-Object { $_ -match " $([regex]::Escape("$name.zip"))$" }) -split " " | Select-Object -First 1
    $got = (Get-FileHash "$tmp\$name.zip" -Algorithm SHA256).Hash.ToLower()
    if (-not $want -or $got -ne $want) { throw "cg install: checksum mismatch: the download isn't what was released. Nothing was installed." }

    Expand-Archive "$tmp\$name.zip" -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    Copy-Item "$tmp\$name\cg.exe" (Join-Path $dir "cg.exe") -Force
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "Installed: $dir\cg.exe"
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $dir) {
    [Environment]::SetEnvironmentVariable("Path", "$dir;$userPath", "User")
    $env:Path = "$dir;$env:Path"
    Write-Host "Added $dir to your PATH (new terminals will see it)."
}
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Host ""
    Write-Host "cg hands most commands to git, and git isn't installed: https://git-scm.com/downloads"
}
Write-Host ""
Write-Host "Start a game:  cg init        (Miranda)    or    cg init --preset gentle    (Juniper)"
Write-Host "She plays in Windows Terminal, PowerShell and cmd. (In Git Bash she stays quiet.)"
