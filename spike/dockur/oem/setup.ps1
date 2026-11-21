# Installs the extension, WinDivert and the probe from the share the runner fills (\\host.lan\Data is /shared in the
# container) and starts them as SYSTEM, which loading the WinDivert driver needs.
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$share = "\\host.lan\Data"
$dir = "C:\steadybit"
"setup started $(Get-Date -Format o)"

# dockur's share only allows guest access, which Windows Server refuses by default.
Set-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Services\LanmanWorkstation\Parameters" AllowInsecureGuestAuth 1 -Type DWord
for ($i = 0; -not (Test-Path "$share\payload\extension-host-windows.exe"); $i++) {
  if ($i -ge 60) { throw "share $share not reachable" }
  Start-Sleep 5
}

New-Item -ItemType Directory $dir -Force | Out-Null
Copy-Item "$share\payload\*" $dir -Recurse -Force
Get-ChildItem $dir | ForEach-Object { "{0} {1}" -f $_.Name, $_.Length }

# wdna.exe needs the Visual C++ runtime.
$redist = Start-Process "$dir\vc_redist.x64.exe" -ArgumentList "/install", "/quiet", "/norestart" -Wait -PassThru
"vc_redist exit code $($redist.ExitCode)"

New-NetFirewallRule -DisplayName "steadybit spike" -Direction Inbound -Protocol TCP -LocalPort 8080, 8085 -Action Allow | Out-Null

Set-Content "$dir\run-extension.cmd" "@echo off`r`nset STEADYBIT_EXTENSION_BIND_ADDRESS=0.0.0.0`r`nset STEADYBIT_LOG_LEVEL=debug`r`nset PATH=%PATH%;$dir`r`n`"$dir\extension-host-windows.exe`" > `"$dir\extension.log`" 2>&1`r`n"
schtasks /create /tn steadybit-extension /tr "$dir\run-extension.cmd" /sc onstart /ru SYSTEM /rl HIGHEST /f
schtasks /create /tn steadybit-probe /tr "$dir\probe.exe" /sc onstart /ru SYSTEM /rl HIGHEST /f
schtasks /run /tn steadybit-extension
schtasks /run /tn steadybit-probe

for ($i = 0; ; $i++) {
  try {
    Invoke-WebRequest "http://127.0.0.1:8085/" -UseBasicParsing | Out-Null
    break
  } catch {
    if ($i -ge 60) { throw "the extension did not start: $_" }
    Start-Sleep 2
  }
}
"$(hostname) $((Get-CimInstance Win32_OperatingSystem).Caption) $(Get-Date -Format o)" | Set-Content "$share\ready"
"setup done $(Get-Date -Format o)"
