<#
.SYNOPSIS
Downloads a specific GitHub artifact from the steadybit/extension-host-windows repository and runs the installer.

.DESCRIPTION
This script takes a GitHub artifact ID, downloads the artifact from the steadybit/extension-host-windows repository,
extracts it, and runs the contained installer.

.PARAMETER ArtifactId
The GitHub artifact ID to download.

.PARAMETER Token
Optional GitHub personal access token with appropriate permissions.

.PARAMETER LogFile
Optional path to a log file for the MSI installation. Default is Windows temp directory.

.PARAMETER OutputPath
Optional path where the artifact will be downloaded and extracted. Default is the current directory.

.EXAMPLE
.\download-github-artifact.ps1 -ArtifactId 12345678
#>

param (
  [Parameter(Mandatory = $true)]
  [string]$ArtifactId,

  [Parameter(Mandatory = $false)]
  [string]$Token = "",

  [Parameter(Mandatory = $false)]
  [string]$LogFile = "$env:TEMP\steadybit-install-$((Get-Date).ToString('yyyyMMdd-HHmmss')).log",

  [Parameter(Mandatory = $false)]
  [string]$OutputPath = (Get-Location).Path
)

$ProgressPreference = 'SilentlyContinue'

# Constants
$repoOwner = "steadybit"
$repoName = "extension-host-windows"
$artifactFileName = "artifact.zip"
$extractFolderName = "extracted-artifact"

function Write-Log {
  param (
    [string]$Message,
    [string]$Level = "INFO"
  )

  Write-Host "[$Level] [$(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')] $Message"
}

# Check if running as administrator
function Test-Admin {
  $currentUser = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  $currentUser.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
}

# If script is not running as admin, restart with elevation
if (-not (Test-Admin)) {
  Write-Log "Restarting script with elevated privileges..."
  $arguments = "-ExecutionPolicy Bypass -File `"$($MyInvocation.MyCommand.Path)`""
  foreach ($param in $PSBoundParameters.GetEnumerator()) {
    if ($param.Key -eq "RunAsAdmin") { continue }
    $arguments += " -$($param.Key) `"$($param.Value)`""
  }
  Start-Process powershell.exe -Verb RunAs -ArgumentList $arguments
  exit
}

# Ensure OutputPath exists
if (-not (Test-Path $OutputPath)) {
  Write-Log "Creating output directory: $OutputPath"
  New-Item -ItemType Directory -Path $OutputPath -Force | Out-Null
}

# Set full paths
$artifactPath = Join-Path -Path $OutputPath -ChildPath $artifactFileName
$extractPath = Join-Path -Path $OutputPath -ChildPath $extractFolderName

# Remove old files if they exist
if (Test-Path $artifactPath) {
  Write-Log "Removing existing artifact file: $artifactPath"
  Remove-Item -Path $artifactPath -Force
}

if (Test-Path $extractPath) {
  Write-Log "Removing existing extract directory: $extractPath"
  Remove-Item -Path $extractPath -Force -Recurse
}

Write-Log "Creating extract directory: $extractPath"
New-Item -ItemType Directory -Path $extractPath -Force | Out-Null

# Download artifact
$apiUrl = "https://api.github.com/repos/$repoOwner/$repoName/actions/artifacts/$ArtifactId/zip"
Write-Log "Downloading artifact with ID: $ArtifactId from $apiUrl"
$headers = @{
  "Accept" = "application/vnd.github.v3+json"
}

if ($Token) {
  Write-Log "Using provided GitHub token for authentication"
  $headers["Authorization"] = "Bearer $Token"
}

try {
  Invoke-WebRequest -Uri $apiUrl -OutFile $artifactPath -Headers $headers
  Write-Log "Artifact downloaded successfully to: $artifactPath"
} catch {
  Write-Log "Failed to download artifact: $_" -Level "ERROR"
  exit 1
}

# Extract the artifact
Write-Log "Extracting artifact to: $extractPath"
try {
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  [System.IO.Compression.ZipFile]::ExtractToDirectory($artifactPath, $extractPath)
  Write-Log "Artifact extracted successfully"
} catch {
  Write-Log "Failed to extract artifact: $_" -Level "ERROR"
  exit 1
}

# Look for the MSI installer in the extracted directory
Write-Log "Looking for MSI installer in extracted files"
$msiFile = Get-ChildItem -Path $extractPath -Filter "*.msi" -Recurse | Select-Object -First 1
if (-not $msiFile) {
  Write-Log "No MSI installer found in the extracted artifact" -Level "ERROR"
  exit 1
}

# Run the MSI installer
Write-Log "Found MSI installer: $($msiFile.FullName)"
Write-Log "Starting installation"
$process = Start-Process -FilePath "msiexec.exe" -ArgumentList "/i", "`"$($msiFile.FullName)XXX`"", "/qn",  "/l*v", "`"$LogFile`"" -Wait -PassThru
if ($process.ExitCode -eq 0) {
  Write-Log "Installation completed successfully"
} else {
  Write-Log "Installation failed with exit code: $($process.ExitCode)" -Level "ERROR"

  # Print log file content in case of an error
  if (Test-Path $LogFile) {
    Write-Log "Log file content:" -Level "ERROR"
    Get-Content -Path $LogFile | ForEach-Object { Write-Log $_ -Level "ERROR" }
  } else {
    Write-Log "Log file not found at: $LogFile" -Level "ERROR"
  }

  exit 1
}

Write-Log "Cleaning up downloaded files"
Remove-Item -Path $artifactPath -Force
Remove-Item -Path $extractPath -Force -Recurse

Write-Log "Process completed successfully"
