# Parameters with default value
param(
  [string]$DownloadDir = "dist",
  [string]$ReleaseVersion = "latest",
  [switch]$IncludePrereleases = $true,
  [switch]$ForceDownload = $false,
  [string]$GithubToken = $env:PAT_TOKEN
)

$ProgressPreference = 'SilentlyContinue'
# Create download directory if it doesn't exist
if (-not (Test-Path $DownloadDir)) {
  Write-Host "Creating directory: $DownloadDir"
  New-Item -ItemType Directory -Path $DownloadDir | Out-Null
}

Write-Host "Downloading WinDivert release $ReleaseVersion from steadybit/WinDivert to $DownloadDir"
if ($IncludePrereleases) {
  Write-Host "Including prereleases!"
}

try {
  $headers = @{}
  if ($GitHubToken) {
    $headers["Authorization"] = "token $GitHubToken"
    Write-Host "Using GitHub token for authentication."
  }
  # Get specific version or all releases (including prereleases)
  if ($ReleaseVersion -ne "latest") {
    # Fetch specific release by tag
    try {
      $release = Invoke-RestMethod -Uri "https://api.github.com/repos/steadybit/WinDivert/releases/tags/$ReleaseVersion" -Headers $headers
    } catch {
      Write-Host "Error: Release version '$ReleaseVersion' not found"
      exit 1
    }
  } else {
    # Get all releases and filter based on parameters
    $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/steadybit/WinDivert/releases" -Headers $headers

    # Filter based on prerelease parameter
    if (-not $IncludePrereleases) {
      $releases = $releases | Where-Object { -not $_.prerelease }
    }

    # Get the most recent release
    if ($releases.Count -eq 0) {
      Write-Host "Error: No releases found"
      exit 1
    }

    $release = $releases[0]
  }

  Write-Host "Found release: $($release.name) (tag: $($release.tag_name))"
  if ($release.prerelease) {
    Write-Host "Note: This is a prerelease!"
  }

  # Find the windows-build zip file
  $windowsBuildAsset = $release.assets | Where-Object { $_.name -like "windows-build*.zip" } | Select-Object -First 1
  if ($null -eq $windowsBuildAsset) {
    Write-Host "Error: No windows-build zip file found in the release metadata"
    exit 1
  }

   # Create a dedicated temp directory named after the tag name
  $tempDir = Join-Path $env:TEMP "steadybit-extension-host-windows"
  $releaseTempDir = Join-Path $tempDir $($release.tag_name)
  $assetTempPath = Join-Path $releaseTempDir $($windowsBuildAsset.id)
  $assetZipPath = Join-Path $assetTempPath "$($windowsBuildAsset.name)"


  # Check if the file already exists locally
  if ((Test-Path $assetZipPath) -and (-not $ForceDownload)) {
    Write-Host "Using existing download from $assetZipPath"
  } else {
    # Create temp directory if it doesn't exist
    if (-not (Test-Path $assetTempPath)) {
      New-Item -ItemType Directory -Path $assetTempPath | Out-Null
    }

    # Download the zip file to the dedicated temp location
    Write-Host "Downloading $($windowsBuildAsset.name) to $assetZipPath"
    Write-Host "URL: $($windowsBuildAsset.browser_download_url)"
    Write-Host "Headers: $($headers["Authorization"])"

    $assetApiUrl = "https://api.github.com/repos/steadybit/WinDivert/releases/assets/$($windowsBuildAsset.id)"
    $assetHeaders = $headers.Clone()
    $assetHeaders["Accept"] = "application/octet-stream"
    Invoke-WebRequest -Uri $assetApiUrl -OutFile $assetZipPath -Headers $assetHeaders
  }

  # Extract the zip file to the destination directory
  Write-Host "Extracting to $DownloadDir..."
  Expand-Archive -Path $assetZipPath -DestinationPath $DownloadDir -Force

  Write-Host "Successfully downloaded and extracted windows-build to $DownloadDir"
} catch {
  Write-Host "Error: $($_.Exception.Message)"
  Write-Host "Error: Failed to download or extract release: $_"
  exit 1
}
