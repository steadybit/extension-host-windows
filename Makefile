# ==================================================================================== #
#
# HELPERS
#
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@powershell -Command "Get-Content $(MAKEFILE_LIST) | Select-String -Pattern '^##' | ForEach-Object {$$_ -replace '^##', ''} | Format-Table -AutoSize"

## licenses-report: generate a report of all licenses
.PHONY: licenses-report
licenses-report:
ifeq ($(SKIP_LICENSES_REPORT), false)
	go run github.com/google/go-licenses@v1.6.0 save . --save_path licenses
	go run github.com/google/go-licenses@v1.6.0 report . > licenses/THIRD-PARTY.csv
	powershell -Command "copy LICENSE licenses\LICENSE.txt"
endif

# ==================================================================================== #
#
# QUALITY CONTROL
#
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest "-checks=all,-SA1019,-ST1000,-ST1003,-U1000" ./...
	go test -vet=off -coverprofile=coverage.out -timeout 45m ./...
	go mod verify

# ====================================================================================
#
# BUILD
#
# ====================================================================================

## clean: clean up the output directories
.PHONY: clean
clean:
	powershell -Command "if (Test-Path 'dist') { Remove-Item -Path 'dist' -Force -Recurse }"
	powershell -Command "if (Test-Path 'licenses') { Remove-Item 'licenses' -Recurse -Force }"

## build: build the extension
.PHONY: build
build:
	go run github.com/goreleaser/goreleaser/v2@latest build --clean --snapshot --single-target -o extension.exe

# ====================================================================================
#
# Package
#
# ====================================================================================

## release: package the extension release only
.PHONY: release
release: clean licenses-report
	go run github.com/goreleaser/goreleaser/v2@latest release --clean --snapshot

## artifact: package a ZIP with the extension and all required files
.PHONY: artifact
artifact: release
	powershell -ExecutionPolicy "Bypass" -File "scripts/package-extension.ps1"

## installer: installs the extension via the build installer
.PHONY: installer
installer: artifact
	powershell -ExecutionPolicy "Bypass" -File "scripts/package-installer.ps1"
