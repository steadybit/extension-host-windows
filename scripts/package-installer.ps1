if ($Env:SKIP_LICENSES_REPORT -eq "true"){
    Write-Output "License report must be set to 'false' in order to package the installer."
    return 1
}

$ProgressPreference = 'SilentlyContinue'
$scriptPath = $PSScriptRoot
$distPath = "$scriptPath\..\dist"
$devzeroPath = "$scriptPath\..\devzero"
$artifactPath = "$scriptPath\..\windowspkg\WindowsHostExtensionInstaller\Artifacts"
$solutionPath = "$scriptPath\..\windowspkg\WindowsHostExtensionInstaller"
$cpuStressPath = "$scriptPath\..\steadybit-stress-cpu"

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

Write-Output "Downloading latest release of memfill..."

$memfillApiUrl = "https://api.github.com/repos/steadybit/memfill/releases/latest"
$memfillRelease = Invoke-RestMethod -Uri $memfillApiUrl -Headers @{"User-Agent"="PowerShell"}
$memfillAsset = $memfillRelease.assets | Where-Object { $_.name -like "*.exe" } | Select-Object -First 1

if ($null -eq $memfillAsset) {
    Write-Error "No .exe asset found in the latest memfill release." -ErrorAction Stop
}

$memfillExePath = "$artifactPath\memfill.exe"
Invoke-WebRequest -Uri $memfillAsset.browser_download_url -OutFile $memfillExePath -Headers @{"User-Agent"="PowerShell"}

Write-Output "memfill.exe added to artifacts."

Copy-Item licenses\THIRD-PARTY-LICENSES.csv windowspkg\WindowsHostExtensionInstaller\Artifacts

Write-Output "Running dotnet publish in: $cpuStressPath"
Push-Location $cpuStressPath
dotnet publish -c Release -r "win-x64"  -o $artifactPath /p:SelfContained=true /p:PublishSingleFile=true /p:PublishTrimmed=true
Pop-Location

Write-Output "Building devzero in: $devzeroPath"
Push-Location $devzeroPath
go build -o $artifactPath\devzero.exe main.go
Pop-Location

Push-Location $artifactPath
Write-Output "Downloading and extracting coreutils"
Invoke-WebRequest -Uri https://github.com/uutils/coreutils/releases/download/0.1.0/coreutils-0.1.0-x86_64-pc-windows-msvc.zip  -OutFile CoreUtils.zip
[System.IO.Compression.ZipFile]::ExtractToDirectory("$artifactPath\CoreUtils.zip", "$artifactPath\CoreUtils")
Copy-Item "$artifactPath\CoreUtils\coreutils-0.1.0-x86_64-pc-windows-msvc\coreutils.exe" $artifactPath
Pop-Location

Push-Location $artifactPath
Write-Output "Downloading and extracting diskspd"
Invoke-WebRequest -Uri https://github.com/microsoft/diskspd/releases/download/v2.2/DiskSpd.ZIP -OutFile DiskSpd.zip
[System.IO.Compression.ZipFile]::ExtractToDirectory("$artifactPath\DiskSpd.zip", "$artifactPath\DiskSpd")
Copy-Item "$artifactPath\DiskSpd\x86\diskspd.exe" $artifactPath

Pop-Location

Write-Output "Running MSBuild in: $solutionPath"
Push-Location $solutionPath
msbuild -Restore WindowsHostExtensionInstaller.sln /p:Configuration=Release /m /p:OutDir=..\..\dist
Pop-Location


Write-Output "MSBuild completed."