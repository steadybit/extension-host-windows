# Contributing

## Prerequisites

To build this extension locally, you need:

- [Go](https://go.dev/dl/) 1.26 or later (with Windows cross-compilation support if building from non-Windows)
- [GNU Make](https://www.gnu.org/software/make/)
- A Windows build environment for producing the MSI installer (the `installer` target requires a Windows host)

## Getting Started

1. Clone the repository
2. `$ make tidy`
3. `$ make build`

## Tasks

The `Makefile` in the project root contains commands to easily run common admin tasks. Run `make help` to list all available targets.

| Command            | Meaning                                                                     |
|--------------------|-----------------------------------------------------------------------------|
| `$ make tidy`      | Format all code using `go fmt` and tidy the `go.mod` file.                  |
| `$ make audit`     | Run `go vet`, `staticcheck`, execute all tests and verify required modules. |
| `$ make build`     | Build the Windows extension binary.                                         |
| `$ make release`   | Package the extension for release.                                          |
| `$ make artifact`  | Package a ZIP with the extension and all required files.                    |
| `$ make installer` | Build the Windows MSI installer (requires Windows host).                    |

## Releasing the Code

To make a new release, do the following:

 1. Update the `CHANGELOG.md`
 2. Commit and push the changelog changes.
 3. Set the tag `git tag -a vX.X.X -m vX.X.X`
 4. Push the tag.

## Contributor License Agreement (CLA)

In order to accept your pull request, we need you to submit a CLA. You only need to do this once. If you are submitting a pull request for the first time, just submit a Pull Request and our CLA Bot will give you instructions on how to sign the CLA before merging your Pull Request.

All contributors must sign an [Individual Contributor License Agreement](https://github.com/steadybit/.github/blob/main/.github/cla/individual-cla.md).

If contributing on behalf of your company, your company must sign a [Corporate Contributor License Agreement](https://github.com/steadybit/.github/blob/main/.github/cla/corporate-cla.md). If so, please contact us via office@steadybit.com.

If for any reason, your first contribution is in a PR created by other contributor, please just add a comment to the PR
with the following text to agree our CLA: "I have read the CLA Document and I hereby sign the CLA".
