# Install viewmd as "viewmd.exe" on this machine (Windows).
#
#   irm https://github.com/NawaMan/MarkDownViewer/releases/latest/download/install.ps1 | iex
#
# Environment:
#   VIEWMD_VERSION      Tag to install (e.g. v0.3.0). Defaults to the release
#                       this copy of the script shipped with.
#   VIEWMD_INSTALL_DIR  Where to put the binary. Defaults to
#                       %LOCALAPPDATA%\Programs\viewmd.

#Requires -Version 5.1
Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$Repo = 'NawaMan/MarkDownViewer'

# The release workflow rewrites this line to the tag being published, so a
# script pulled from a pinned release installs that same release. A copy from a
# git checkout still says "dev" and means "whatever is newest".
$VersionDefault = 'dev'

$version = if ($env:VIEWMD_VERSION) { $env:VIEWMD_VERSION } else { $VersionDefault }
$base = if ($version -eq 'dev') {
  "https://github.com/$Repo/releases/latest/download"
} else {
  "https://github.com/$Repo/releases/download/$version"
}

$arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
  'X64'   { 'amd64' }
  'Arm64' { 'arm64' }
  default { throw "unsupported architecture $_" }
}
$asset = "viewmd-windows-$arch.exe"

$onWindows = ($PSVersionTable.PSVersion.Major -lt 6) -or $IsWindows
$dir = $env:VIEWMD_INSTALL_DIR
if (-not $dir) {
  $dir = if ($env:LOCALAPPDATA) { Join-Path $env:LOCALAPPDATA 'Programs\viewmd' }
         else { Join-Path $HOME '.local/bin' }
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
try {
  Write-Host "Downloading $asset ($version)"
  $exe = Join-Path $tmp 'viewmd.exe'
  $sums = Join-Path $tmp 'SHA256SUMS'
  Invoke-WebRequest -Uri "$base/$asset" -OutFile $exe -UseBasicParsing
  Invoke-WebRequest -Uri "$base/SHA256SUMS" -OutFile $sums -UseBasicParsing

  $want = $null
  foreach ($line in Get-Content $sums) {
    $parts = $line -split '\s+', 2
    if ($parts.Count -eq 2 -and $parts[1].TrimStart('*').Trim() -eq $asset) { $want = $parts[0]; break }
  }
  if (-not $want) { throw "SHA256SUMS has no entry for $asset" }
  $got = (Get-FileHash -Algorithm SHA256 -Path $exe).Hash
  if ($got -ne $want.ToUpperInvariant()) {
    throw "checksum mismatch for ${asset} (expected $want, got $got)"
  }

  New-Item -ItemType Directory -Path $dir -Force | Out-Null
  $dest = Join-Path $dir 'viewmd.exe'
  Move-Item -Path $exe -Destination $dest -Force

  if ($onWindows) {
    # A binary that lands but cannot start must not be called a success.
    $installed = & $dest version 2>$null
    if ($LASTEXITCODE -ne 0) { throw "installed $dest, but it does not run on this system" }
    Write-Host "Installed viewmd $installed to $dest"
    # Persist the directory on the user's PATH so a new shell finds viewmd.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $dir) {
      [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir".TrimStart(';'), 'User')
      Write-Host "Added $dir to your user PATH — restart the shell to pick it up"
    }
  } else {
    Write-Host "Installed $asset to $dest (not runnable on this OS)"
  }
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
