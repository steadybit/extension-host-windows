# Parameters with default value
param(
  [string]$DownloadDir = "dist",
  [string]$ReleaseVersion = "latest",
  [switch]$IncludePrereleases = $true
)

# Create download directory if it doesn't exist
if (-not (Test-Path $DownloadDir)) {
  Write-Host "Creating directory: $DownloadDir"
  New-Item -ItemType Directory -Path $DownloadDir | Out-Null
}

Write-Host "Downloading WinDivert release \"$ReleaseVersion\" from steadybit/WinDivert to $DownloadDir"
if ($IncludePrereleases) {
  Write-Host "Including prereleases!"
}

try {
  # Get specific version or all releases (including prereleases)
  if ($ReleaseVersion -ne "latest") {
    # Fetch specific release by tag
    try {
      $release = Invoke-RestMethod -Uri "https://api.github.com/repos/steadybit/WinDivert/releases/tags/$ReleaseVersion"
    } catch {
      Write-Host "Error: Release version '$ReleaseVersion' not found"
      exit 1
    }
  } else {
    # Get all releases and filter based on parameters
    $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/steadybit/WinDivert/releases"

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

  # Download the zip file to a temporary location
  $tempZipPath = [System.IO.Path]::GetTempFileName() + ".zip"
  Write-Host "Downloading $($windowsBuildAsset.name) to temporary location..."
  Invoke-WebRequest -Uri $windowsBuildAsset.browser_download_url -OutFile $tempZipPath

  # Extract the zip file to the destination directory
  Write-Host "Extracting to $DownloadDir..."
  Expand-Archive -Path $tempZipPath -DestinationPath $DownloadDir -Force

  # Clean up the temporary zip file
  Remove-Item -Path $tempZipPath -Force

  Write-Host "Successfully downloaded and extracted windows-build to $DownloadDir"
} catch {
  Write-Host "Error: Failed to download or extract release: $_"
  exit 1
}
