# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@powershell -Command "Get-Content $(MAKEFILE_LIST) | Select-String -Pattern '^##' | ForEach-Object {$$_ -replace '^##', ''} | Format-Table -AutoSize"

## licenses-report: generate a report of all licenses
.PHONY: licenses-report
licenses-report:
ifeq ($(SKIP_LICENSES_REPORT), true)
	@echo "Skipping licenses report"
	@if exist .\licenses rmdir /s /q .\licenses
	@mkdir .\licenses
else
	@echo "Generating licenses report"
	@if exist .\licenses rmdir /s /q .\licenses
	go run github.com/google/go-licenses@v1.6.0 save . --save_path ./licenses
	go run github.com/google/go-licenses@v1.6.0 report . > ./licenses/THIRD-PARTY.csv
	copy LICENSE .\licenses\LICENSE.txt
endif

# ==================================================================================== #
# QUALITY CONTROL
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

## build: build the extension
.PHONY: build
build:
	goreleaser build --clean --snapshot --single-target -o extension.exe

## run: run the extension
.PHONY: run
run: tidy build
	.\extension.exe
