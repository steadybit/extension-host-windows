# Builds and downloads the tools the attacks run (memfill, steadybit-stress-cpu, devzero, coreutils, diskspd) into
# OutDir. Used for the installer and for the e2e tests, which run the extension with the tools next to it.
param(
  [Parameter(Mandatory = $true)][string]$OutDir,
  [string]$GithubToken = $env:PAT_TOKEN
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$repoPath = Resolve-Path "$PSScriptRoot\.."
New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
$OutDir = Resolve-Path $OutDir

$headers = @{}
if ($GithubToken) {
  $headers["Authorization"] = "token $GithubToken"
}
$downloadDir = Join-Path ([System.IO.Path]::GetTempPath()) "attack-tools-$(Get-Random)"
New-Item -ItemType Directory -Path $downloadDir -Force | Out-Null
Add-Type -AssemblyName System.IO.Compression.FileSystem

try {
  Write-Output "Downloading the latest release of memfill"
  $memfillRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/steadybit/memfill/releases/latest" -Headers $headers
  $memfillAsset = $memfillRelease.assets | Where-Object { $_.name -like "*.exe" } | Select-Object -First 1
  if ($null -eq $memfillAsset) {
    Write-Error "No .exe asset found in the latest memfill release."
  }
  Invoke-WebRequest -Uri $memfillAsset.browser_download_url -OutFile "$OutDir\memfill.exe" -Headers $headers

  Write-Output "Publishing steadybit-stress-cpu"
  dotnet publish "$repoPath\steadybit-stress-cpu\steadybit-stress-cpu.csproj" -c Release -r "win-x64" -o $OutDir /p:SelfContained=true /p:PublishSingleFile=true /p:PublishTrimmed=true
  if ($LASTEXITCODE -ne 0) {
    Write-Error "dotnet publish failed with exit code $LASTEXITCODE"
  }

  Write-Output "Building devzero"
  Push-Location "$repoPath\devzero"
  try {
    go build -o "$OutDir\devzero.exe" main.go
    if ($LASTEXITCODE -ne 0) {
      Write-Error "go build of devzero failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }

  Write-Output "Downloading coreutils"
  Invoke-WebRequest -Uri "https://github.com/uutils/coreutils/releases/download/0.1.0/coreutils-0.1.0-x86_64-pc-windows-msvc.zip" -OutFile "$downloadDir\CoreUtils.zip" -Headers $headers
  [System.IO.Compression.ZipFile]::ExtractToDirectory("$downloadDir\CoreUtils.zip", "$downloadDir\CoreUtils")
  Copy-Item "$downloadDir\CoreUtils\coreutils-0.1.0-x86_64-pc-windows-msvc\coreutils.exe" $OutDir

  Write-Output "Downloading diskspd"
  Invoke-WebRequest -Uri "https://github.com/microsoft/diskspd/releases/download/v2.2/DiskSpd.ZIP" -OutFile "$downloadDir\DiskSpd.zip" -Headers $headers
  [System.IO.Compression.ZipFile]::ExtractToDirectory("$downloadDir\DiskSpd.zip", "$downloadDir\DiskSpd")
  Copy-Item "$downloadDir\DiskSpd\x86\diskspd.exe" $OutDir
} finally {
  Remove-Item $downloadDir -Recurse -Force -ErrorAction SilentlyContinue
}
