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
