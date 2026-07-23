# Changelog

## v0.3.3

- fix: WinDivert-based network attacks (delay, blackhole, package loss, package corruption) now also affect protocols without ports (e.g. ICMP) when no port is specified. The filter previously matched only `tcp`/`udp` packets, so portless traffic such as `ping` slipped through the attack. Port-scoped excludes now also only spare their tcp/udp port and no longer spare all ICMP traffic to/from the excluded address.
- fix: the build information log line (and other early startup logs) is no longer dropped from the on-disk log / Windows Event Log — the log writer is now attached synchronously before startup logging instead of in a background goroutine.

## v0.3.2

- build(deps): bump actions/setup-go from 6 to 7
- build(deps): bump github.com/steadybit/action-kit/go/action_kit_commons
- ci: skip build on .trivyignore.yml-only changes [skip ci]

## v0.3.1

- build(deps): bump github.com/steadybit/action-kit/go/action_kit_commons
- build(deps): bump golang.org/x/sys from 0.46.0 to 0.47.0
- build(deps): bump softprops/action-gh-release from 2.6.1 to 3.0.1
- refactor: register extension index via exthttp.RegisterRevisionedHandler (#188)

## v0.3.0

- feat: new `Exclude Hostnames` (`excludeHostname`) and `Exclude IPs/CIDRs` (`excludeIp`) parameters on the WinDivert-based network attacks (delay, blackhole, package loss, package corruption) — affect all traffic except the given hosts or IPs/CIDRs. Excludes always take precedence over the include restrictions. The existing filter parameters are relabeled to `Include Hostnames`, `Include IPs/CIDRs` and `Include Ports` to make the distinction explicit. The bandwidth attack is not covered: Windows QoS policies only support include-style match conditions.
- Update Go to 1.26.5
- Update dependencies

## v0.2.14

- build(deps): bump github.com/steadybit/action-kit/go/action_kit_commons
- chore(deps): bump golang.org/x/net to v0.55.0 (CVE-2026-39821) (#169)

## v0.2.13

- build(deps): bump github.com/steadybit/action-kit/go/action_kit_commons
- build(deps): bump golang.org/x/sys from 0.44.0 to 0.45.0
- chore: update to go 1.26.4
- feat: add weekly auto patch-release workflow

## v0.2.12

- Support discovery group attribute via `STEADYBIT_EXTENSION_DISCOVERY_GROUP` env var — when set, the extension adds `steadybit.group=<value>` to every discovered target
- Update dependencies

## v0.2.11

- Bump Go to 1.26.3
- Update dependencies

## v0.2.10

- Fixed version handling - all definitions returned by the extension now return a valid semver version string instead
  of "unknown". If you had installed the extension before, please make sure to delete existing definitions in the
  platform after upgrading by visiting "Settings" -> "Extensions" and deleting the existing extension definitions. This
  is required to make sure that the platform correctly detects new versions of the definitions provided by the extension.

## v0.2.9

- Bump Go to 1.26.2
- Update dependencies
- Fix windows installer to correctly detect the system architecture

## v0.2.8

- Update dependencies

## v0.2.7

- The stop process action reports an error if stopping the process is unsuccessful
- The stop process action now correctly correlates parallel executions by their ID
- If the extension runs with SYSTEM privileges, external tools are now directly executed, and not via a one-off SYSTEM task
- Support if-none-match for the extension list endpoint
- Update dependencies

## v0.2.6

 - fix: correctly handle svc.Interrogate

## v0.2.5

 - Update dependencies

## v0.0.1 (next)

 - Initial release
