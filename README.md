<img src="./logo.svg" height="130" align="right" alt="Host logo">

# Steadybit extension-host-windows

This [Steadybit](https://www.steadybit.com/) extension provides a host discovery and various actions for Windows host targets.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_host_windows).

## Configuration

| Environment Variable                                     | Helm value                         | Meaning                                                                                                                                                                                                                       | Required | Default |
|----------------------------------------------------------|------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|---------|
| `STEADYBIT_LABEL_<key>=<value>`                          |                                    | Environment variables starting with `STEADYBIT_LABEL_` will be added to discovered targets' attributes. <br>**Example:** `STEADYBIT_LABEL_TEAM=Fullfillment` adds to each discovered target the attribute `team=Fullfillment` | no       |         |
| `STEADYBIT_DISCOVERY_ENV_LIST`                           |                                    | List of environment variables to be evaluated and added to discovered targets' attributes. <br> **Example:** `STEADYBIT_DISCOVERY_ENV_LIST=STAGE` adds to each target the attribute `stage=<value of $STAGE>`                 | no       |         |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_HOST` | discovery.attributes.excludes.host | List of Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                                                                                        | false    |         |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

## Installation

### Windows Installer

**Note**: Only x64 systems are supported.

Once available, download the latest Windows installer from the project`s [GitHub release page](https://github.com/steadybit/WinDivert/releases).

As the extension requires extended privileges to execute host attacks, like injecting network traffic errors, the installer and the extension need to be executed as an **Administrator user**.
During installation, a Windows Service named `SteadybitWindowsExtensionHost` is created and configured. It runs on startup on port `8085`.

#### Pre-Release Versions

Pre-Release versions of the extension contain a test-signed Windows network driver. The driver is used to execute network attacks and essential to the extension.

By default, Windows does not load test-signed kernel-mode drivers. To allow this several things must be done:
- Turn off secure boot (if you use bitlocker volume encryption don't forget to retrieve recovery key beforehand)
- Enable test signing via CLI ```Bcdedit.exe -set TESTSIGNING ON```
- Restart the machine

## Extension registration

Make sure that the extension is registered with the Steadybit agent. Please refer to
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-registration) for more
information about extension registration and how to verify.

In many cases adding the `STEADYBIT_AGENT_EXTENSIONS_REGISTRATIONS_<n>_URL` environment variable to the Steadybit agent is sufficient:

```shell
STEADYBIT_AGENT_EXTENSIONS_REGISTRATIONS_0_URL=http://<extension-windows-host-ip>:8085/
```

## Security

We limit the permissions required by the extension to the absolute minimum.

The extension must be executed as `Administrator` to perform network attacks. Furthermore, the "limit outgoing bandwidth attack" creates and removes network quality of service policies in the `SYSTEM` context.

## Troubleshooting

In case of problems, the extension logs are always a good starting point for investigation. They are available as Windows application events or in the logfile `%PROGRAMDATA%/Steadybit GmbH/extension-host-windows.log`.

### Extension can not be reached

Please check if the Windows service `SteadybitWindowsExtensionHost` is started correctly and (re-)start it.
