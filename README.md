<img src="./logo.svg" height="130" align="right" alt="Host logo">

# Steadybit extension-host-windows

This [Steadybit](https://www.steadybit.com/) extension provides a host discovery and various actions for Windows host targets.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_host_windows).

## Configuration

| Environment Variable                                     | Meaning                                                                                                                                                                                                                       | Required | Default |
|----------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|---------|
| `STEADYBIT_LABEL_<key>=<value>`                          | Environment variables starting with `STEADYBIT_LABEL_` will be added to discovered targets' attributes. <br>**Example:** `STEADYBIT_LABEL_TEAM=Fullfillment` adds to each discovered target the attribute `team=Fullfillment` | no       |         |
| `STEADYBIT_DISCOVERY_ENV_LIST`                           | List of environment variables to be evaluated and added to discovered targets' attributes. <br> **Example:** `STEADYBIT_DISCOVERY_ENV_LIST=STAGE` adds to each target the attribute `stage=<value of $STAGE>`                 | no       |         |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_HOST` | List of Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                                                                                        | false    |         |

Beyond the settings above, this extension supports the configuration common to all Steadybit
extensions:

- [extension-kit](https://github.com/steadybit/extension-kit#environment-variables) — HTTP and
  health ports, TLS and mutual TLS, unix domain socket, logging, and pprof.
- [Target Filtering](https://github.com/steadybit/discovery-kit/blob/main/docs/target-filtering.md) —
  stop the extension reporting targets you do not want.
- [Group Matching](https://github.com/steadybit/discovery-kit/blob/main/docs/target-enrichment.md#group-matching) —
  tag discovered targets with a group, so enrichment rules only match within it.

## Installation

### Windows Installer

**Note**: Only x64 systems are supported.

Download the latest Windows installer [here](https://windows-registry.steadybit.com/steadybit-extension-host-windows/windows-host-extension-latest.zip). SHA-256 checksum of the artifact can be found [here](https://windows-registry.steadybit.com/steadybit-extension-host-windows/windows-host-extension-latest.sha256).

Specific versions of the Windows Host Extension can be found in the [Steadybit Windows Registry](https://windows-registry.steadybit.com/#steadybit-extension-host-windows/).

As the extension requires extended privileges to execute host attacks, like injecting network traffic errors, the installer and the extension need to be executed as an **Administrator user**.
During installation, a Windows Service named `SteadybitWindowsHostExtension` is created and configured. It runs on startup on port `8085` and, by default, only accepts connections from the same host (`localhost`).

#### Pre-Release Versions

Pre-Release versions of the extension may contain a test-signed Windows network driver.
The driver is used to execute network attacks and essential to the extension.

By default, Windows does not load test-signed kernel-mode drivers. To allow this several things must be done:
- Turn off secure boot (if you use bitlocker volume encryption don't forget to retrieve recovery key beforehand)
- Enable test signing via CLI ```Bcdedit.exe -set TESTSIGNING ON```
- Restart the machine

## Extension registration

Make sure that the extension is registered with the Steadybit agent. Please refer to
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-registration) for more
information about extension registration and how to verify.

A Steadybit agent on the same host discovers the extension automatically through the Windows registry.

To register the extension with an agent on another host, first make the extension accept connections from other hosts (see [Accepting connections from other hosts](#accepting-connections-from-other-hosts)). Then add the `STEADYBIT_AGENT_EXTENSIONS_REGISTRATIONS_<n>_URL` environment variable to the Steadybit agent:

```shell
STEADYBIT_AGENT_EXTENSIONS_REGISTRATIONS_0_URL=http://<extension-windows-host-ip>:8085/
```

## Security

We limit the permissions required by the extension to the absolute minimum.

The extension must be executed as `Administrator` to perform network attacks. Furthermore, the "limit outgoing bandwidth attack" creates and removes network quality of service policies in the `SYSTEM` context.

### Accepting connections from other hosts

The extension API is not authenticated unless [mutual TLS](https://github.com/steadybit/extension-kit#environment-variables) is configured: anyone who can reach port `8085` can run attacks on the host, bypassing the platform's permissions and audit log. The extension therefore binds its API (`8085`) and health endpoints (`8081`) to `localhost` only.

Only if an agent on another host must reach the extension, set the system environment variable `STEADYBIT_EXTENSION_BIND_ADDRESS` to `0.0.0.0` (all interfaces) or to a specific IP address of the host, and restart the `SteadybitWindowsHostExtension` service:

```powershell
[Environment]::SetEnvironmentVariable("STEADYBIT_EXTENSION_BIND_ADDRESS", "0.0.0.0", "Machine")
Restart-Service SteadybitWindowsHostExtension
```

Restrict access to port `8085` to the agent's host, for example with a Windows Firewall rule, or configure mutual TLS.

## Troubleshooting

In case of problems, the extension logs are always a good starting point for investigation. They are available as Windows application events or in the logfile `%PROGRAMDATA%/Steadybit GmbH/extension-host-windows.log`.

### Extension can not be reached

Please check if the Windows service `SteadybitWindowsHostExtension` is started correctly and (re-)start it.

### Cheatsheet

Run the commands in an elevated (Administrator) PowerShell on the Windows host.

#### Is the extension running?

```powershell
# state of the extension service
Get-Service SteadybitWindowsHostExtension
# the extension answers on its API port (run on the host itself, the extension only listens on localhost by default)
Invoke-RestMethod http://localhost:8085/
# (re-)start it
Restart-Service SteadybitWindowsHostExtension
```

#### What is the extension doing?

```powershell
# follow the extension log
Get-Content "$env:ProgramData\Steadybit GmbH\extension-host-windows.log" -Wait -Tail 50
# the latest entries of the Windows application event log
Get-EventLog -LogName Application -Source SteadybitExtensionHostWindows -Newest 20 | Format-List TimeGenerated, EntryType, Message
```

For more details, raise the log level to `debug` (or `trace`) and restart the service. Remove the variable again afterwards:

```powershell
[Environment]::SetEnvironmentVariable("STEADYBIT_LOG_LEVEL", "debug", "Machine")
Restart-Service SteadybitWindowsHostExtension
```

#### What is the network driver doing?

The network attacks "Block Traffic", "Block DNS", "Delay Outgoing Traffic", "Drop Outgoing Traffic" and "Corrupt Outgoing Packages" are executed by `wdna.exe`, which loads the [WinDivert](https://github.com/steadybit/WinDivert) driver for the duration of the attack. Neither the process nor the driver are present while no attack is running.

```powershell
# is an attack running? wdna.exe only lives for the duration of an attack
Get-Process wdna -ErrorAction SilentlyContinue
# is the driver loaded? STATE is RUNNING during an attack, the service does not exist otherwise
sc.exe query windivert
# which traffic does the attack match? The extension logs "starting wdna.exe" with the mode, the duration and the
# path of the filter file, which contains the WinDivert filter (https://reqrypt.org/windivert-doc.html#filter_language)
Select-String "starting wdna.exe" "$env:ProgramData\Steadybit GmbH\extension-host-windows.log" | Select-Object -Last 1
Get-Content <path of the filter file from the log line above>
# end a network attack by hand, in case one is left over (the installer adds wdna_shutdown.exe to the PATH)
wdna_shutdown.exe
sc.exe stop windivert
```

If the driver fails to load, check whether the host accepts it:

```powershell
# pre-release versions ship a test-signed driver, which needs "testsigning Yes" (see "Pre-Release Versions")
bcdedit.exe /enum "{current}" | Select-String testsigning
# driver load failures reported by Windows
Get-WinEvent -FilterHashtable @{ LogName = "System"; ProviderName = "Service Control Manager" } -MaxEvents 200 | Where-Object Message -Match "windivert"
```

"Limit Outgoing Bandwidth" does not use the driver, but Windows policy-based QoS:

```powershell
# the QoS policies of a running attack; none are left once the attack ended
Get-NetQosPolicy -PolicyStore ActiveStore | Where-Object Name -Like "STEADYBIT_QOS_*"
# remove leftovers by hand
Get-NetQosPolicy -PolicyStore ActiveStore | Where-Object Name -Like "STEADYBIT_QOS_*" | Remove-NetQosPolicy -Confirm:$false
```

#### Which value does the "Network Interface" parameter expect?

The name (alias) of the interface as Windows shows it, e.g. `Ethernet`, `Ethernet 2`, `Wi-Fi` or `vEthernet (WSL)`. Enter names with spaces or parentheses as they are, without quotes; upper and lower case do not matter. The names are also reported in the `host.nic` attribute of the target, and an attack with an unknown name fails listing the available ones.

```powershell
# all interface names, including the loopback interface used by localhost traffic
Get-NetIPInterface | Select-Object InterfaceAlias, InterfaceIndex -Unique
# the physical and virtual adapters with their state
Get-NetAdapter | Format-Table Name, InterfaceDescription, ifIndex, Status
```

#### Sparing traffic from a network attack

"Block Traffic", "Delay Outgoing Traffic", "Drop Outgoing Traffic" and "Corrupt Outgoing Packages" take excludes, which always win over the include restrictions:

- **Exclude IPs/CIDRs** and **Exclude Hostnames** spare all traffic to/from these addresses, on every port.
- **Exclude Ports** spares the traffic to/from these ports or port ranges (e.g. `3389` or `8000-8100`), on every address. Use it e.g. to keep RDP (`3389`) or WinRM (`5985-5986`) sessions alive while all other traffic is affected.

Sparing a port on a single address only is not supported. "Limit Outgoing Bandwidth" and "Block DNS" have no excludes.
