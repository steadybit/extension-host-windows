$ProgressPreference = 'SilentlyContinue'

$scriptPath = $PSScriptRoot
$distPath = "$scriptPath\..\dist"
$buildPath = "$scriptPath\..\dist\coreutils\target\debug"
$artifactPath = "$scriptPath\..\windowspkg\WindowsHostExtensionInstaller\Artifacts"


Push-Location $distPath

git clone https://github.com/uutils/coreutils --depth 1
Push-Location .\coreutils 
cargo build --features "dd" --no-default-features
Copy-Item "$buildPath\coreutils.exe" $artifactPath

Pop-Location
Pop-Location