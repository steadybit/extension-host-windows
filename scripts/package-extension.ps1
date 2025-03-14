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
& "$scriptPath\download-windivert.ps1" -DownloadDir "$winDivertPath"

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
