$scriptPath = $PSScriptRoot

$distPath = "$scriptPath\..\dist"
Write-Output "Searching latest ZIP file $distPath"
$latestZip = (Get-ChildItem -Path "$distPath" -Filter "*.zip" -File | Sort-Object LastWriteTime -Descending | Select-Object -First 1).FullName
if ([string]::IsNullOrEmpty($latestZip)) {
  Write-Error "No ZIP file found in $distPath" -ErrorAction Stop
}
Write-Output "Found ZIP file $latestZip"

Write-Output "Downloading WinDivert release"
$winDivertPath = "$distPath\WinDivert"
$releaseVersion = if ($env:STEADYBIT_WINDIVERT_VERSION) { $env:STEADYBIT_WINDIVERT_VERSION } else { "latest" }
& "$scriptPath\download-windivert.ps1" -DownloadDir "$winDivertPath" -ReleaseVersion "$releaseVersion"
# $LASTEXITCODE stays unset when the download succeeds, so the files are what tells success.
foreach ($file in "wdna.exe", "windivertctl.exe", "WinDivert.dll", "WinDivert64.sys") {
  if (-not (Test-Path "$winDivertPath\$file")) {
    Write-Error "Downloading WinDivert failed: $file is missing, the extension would ship without it" -ErrorAction Stop
  }
}

# Create a temp location for the extraction
$tempDir = "$distPath\temp_extract_$(Get-Random)"
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

# Extract original zip
Write-Output "Extracting original zip content"
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory($latestZip, $tempDir)

# Copy WinDivert files
Write-Output "Adding WinDivert files"
Copy-Item -Path "$winDivertPath\*" -Destination $tempDir -Recurse -Force

# wdna.exe, and coreutils.exe in the installer, need vcruntime140.dll, which Windows does not include. It is shipped
# next to them (app-local deployment), taken from the Visual C++ redistributable files of the Visual Studio on the
# build machine.
Write-Output "Adding the Visual C++ runtime"
$vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
$vsPath = & $vswhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
$vcRuntime = Get-ChildItem "$vsPath\VC\Redist\MSVC\*\x64\Microsoft.VC*.CRT\vcruntime140.dll" |
    Sort-Object { $v = $_.VersionInfo; [version]::new($v.FileMajorPart, $v.FileMinorPart, $v.FileBuildPart, $v.FilePrivatePart) } -Descending |
    Select-Object -First 1
if (-not $vcRuntime) {
    Write-Error "vcruntime140.dll not found in the Visual Studio installation $vsPath" -ErrorAction Stop
}
Write-Output "Using $($vcRuntime.FullName), version $($vcRuntime.VersionInfo.FileVersion)"
Copy-Item $vcRuntime.FullName $tempDir

# Create new zip file
$tempZip = "$distPath\temp_$(Get-Random).zip"
Write-Output "Creating new zip file"
[System.IO.Compression.ZipFile]::CreateFromDirectory($tempDir, $tempZip)

# Replace original with new
Remove-Item -Path $latestZip -Force
Move-Item -Path $tempZip -Destination $latestZip

# Clean up
Remove-Item -Path $tempDir -Recurse -Force

Write-Output "Packaged release $latestZip"
