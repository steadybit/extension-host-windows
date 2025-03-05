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

### Windows Binaries

Download the latest binaries from the project`s [GitHub release page](https://github.com/steadybit/WinDivert/releases).

## Extension registration

Make sure that the extension is registered with the agent. In most cases this is done automatically. Please refer to
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-registration) for more
information about extension registration and how to verify.
