@echo off
rem dockur runs this at the end of the unattended Windows installation, see https://github.com/dockur/windows#how-do-i-run-a-script-after-installation
powershell -NoProfile -ExecutionPolicy Bypass -File C:\OEM\setup.ps1 > C:\OEM\setup.log 2>&1
copy /y C:\OEM\setup.log \\host.lan\Data\setup.log
