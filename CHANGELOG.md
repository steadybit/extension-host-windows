# Changelog

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
