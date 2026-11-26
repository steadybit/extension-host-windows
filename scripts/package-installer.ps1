param(
  [string]$GithubToken = $env:PAT_TOKEN
)


if ($Env:SKIP_LICENSES_REPORT -eq "true"){
    Write-Output "License report must be set to 'false' in order to package the installer."
    return 1
}

$ProgressPreference = 'SilentlyContinue'
$scriptPath = $PSScriptRoot
$distPath = "$scriptPath\..\dist"
$artifactPath = "$scriptPath\..\windowspkg\WindowsHostExtensionInstaller\Artifacts"
$solutionPath = "$scriptPath\..\windowspkg\WindowsHostExtensionInstaller"

Write-Output "Looking for latest ZIP file in: $distPath"
$latestZip = (Get-ChildItem -Path "$distPath" -Filter "*.zip" -File | Sort-Object LastWriteTime -Descending | Select-Object -First 1).FullName

if ([string]::IsNullOrEmpty($latestZip)) {
    Write-Error "No ZIP file found in $distPath" -ErrorAction Stop
}

Write-Output "Found ZIP file: $latestZip"

if (-not (Test-Path $artifactPath)) {
    Write-Output "Creating extraction directory: $artifactPath"
    New-Item -ItemType Directory -Path $artifactPath -Force | Out-Null
}

Write-Output "Clearing Artifacts directory except .gitkeep"
Get-ChildItem -Path $artifactPath -Exclude ".gitkeep" -Recurse | Remove-Item -Force -Recurse

Write-Output "Extracting ZIP file to: $artifactPath"
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory($latestZip, $artifactPath)

Write-Output "Extraction completed."

Copy-Item licenses\THIRD-PARTY-LICENSES.csv windowspkg\WindowsHostExtensionInstaller\Artifacts

& "$scriptPath\build-attack-tools.ps1" -OutDir $artifactPath -GithubToken $GithubToken

Write-Output "Running MSBuild in: $solutionPath"
Push-Location $solutionPath
msbuild -Restore WindowsHostExtensionInstaller.sln /p:Configuration=Release /m /p:OutDir=..\..\dist
if ($LASTEXITCODE -ne 0) {
    Write-Error "MSBuild failed with exit code $LASTEXITCODE" -ErrorAction Stop
}
Pop-Location


Write-Output "MSBuild completed."