$scriptPath = $PSScriptRoot
$distPath = "$scriptPath\..\dist"
$extractPath = "$scriptPath\..\windowspkg\WindowsHostExtensionInstaller\Artifacts"
$solutionPath = "$scriptPath\..\windowspkg\WindowsHostExtensionInstaller"

Write-Output "Looking for latest ZIP file in: $distPath"
$latestZip = (Get-ChildItem -Path "$distPath" -Filter "*.zip" -File | Sort-Object LastWriteTime -Descending | Select-Object -First 1).FullName

if ([string]::IsNullOrEmpty($latestZip)) {
    Write-Error "No ZIP file found in $distPath" -ErrorAction Stop
}

Write-Output "Found ZIP file: $latestZip"

if (-not (Test-Path $extractPath)) {
    Write-Output "Creating extraction directory: $extractPath"
    New-Item -ItemType Directory -Path $extractPath -Force | Out-Null
}

Write-Output "Clearing Artifacts directory except .gitkeep"
Get-ChildItem -Path $extractPath -Exclude ".gitkeep" -Recurse | Remove-Item -Force -Recurse

Write-Output "Extracting ZIP file to: $extractPath"
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory($latestZip, $extractPath)

Write-Output "Extraction completed."

Write-Output "Running MSBuild in: $solutionPath"
Push-Location $solutionPath
msbuild -Restore WindowsHostExtensionInstaller.sln /p:Configuration=Release /m /p:OutDir=..\..\dist
Pop-Location

Write-Output "MSBuild completed."
