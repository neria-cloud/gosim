.DEFAULT_GOAL := help

.PHONY: help build check-fmt fix-fmt lint lint-full lint-staged lint-tools install-hooks test-lint-tools go-generate certification-matrix certification-colima gosim-tool gosim-prepare-selftest go-test go-race-test test test-unit test-unit-race test-race test-large test-report-tools

# Independent test cases and package binaries run concurrently on the local host.
UNIT_PARALLEL ?= 2
UNIT_PACKAGES ?= 2
TEST_REPORTS_DIR ?= test-reports
# Optional override; otherwise Go resolves the compiler normally.
GOSIM_TOOLCHAIN ?=
export GOSIM_TOOLCHAIN

# Defaults for explicitly invoked generation and manual certification.
MATRIX_PLATFORM ?= darwin/arm64
COLIMA_PLATFORM ?= linux/arm64
MATRIX_TOOLCHAINS ?= go1.25.14,go1.26.9
SIMPACKAGES ?= ./internal/tests/rt ./internal/tests/fs ./internal/tests/net ./internal/tests/integration ./nemesis ./examples/simple ./examples/bolt ./examples/network/machines ./examples/network/latency
GOSIM ?= $(CURDIR)/.gosim/gosimtool

help: ## List available targets
	@awk 'BEGIN { FS = ":.*## "; print "Usage: make <target>\n\nTargets:" } /^[a-zA-Z0-9_-]+:.*## / { printf "  %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

check-fmt: ## Check Go formatting
	./.ci/gofmt check

fix-fmt: ## Format Go sources
	./.ci/gofmt fix

lint: ## Lint changed Go files under cmd/ and examples/ (staged, unstaged, untracked)
	@python3 -B scripts/lint.py

lint-full: ## Lint all Go files under cmd/ and examples/, including unchanged files
	@python3 -B scripts/lint.py --full

lint-staged: ## Lint staged cmd/ and examples/ Go files in an isolated index snapshot
	@python3 -B scripts/lint_staged.py

lint-tools: ## Install the lint tool versions used by gquorum
	@mkdir -p .gosim/lint/golangci-v2.14.0 .gosim/lint/revive-v1.17.1
	GOBIN="$(CURDIR)/.gosim/lint/golangci-v2.14.0" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
	GOBIN="$(CURDIR)/.gosim/lint/revive-v1.17.1" go install github.com/mgechev/revive@v1.17.1

install-hooks: ## Enable the repository's pre-commit lint hook
	git config --local core.hooksPath .githooks

test-lint-tools: ## Check changed-file selection and staged lint isolation
	python3 -B -m unittest discover -s scripts -p 'lint_test.py'

go-generate: ## Generate Go adapters and filesystem model stringers
	GOTOOLCHAIN=go1.25.14 go run ./cmd/gosim-gen -toolchain=go1.25.14 -profile=go125 -arches=amd64,arm64 -output=.
	GOTOOLCHAIN=go1.26.9 go run ./cmd/gosim-gen -toolchain=go1.26.9 -profile=go126 -arches=amd64,arm64 -output=.
	go generate ./internal/fs/model

certification-matrix: ## Run the manual native certification matrix
	./.ci/crossarch-tests/run-matrix.sh --native --platform "$(MATRIX_PLATFORM)" --toolchains "$(MATRIX_TOOLCHAINS)" --output .gosim/certification

certification-colima: ## Run the manual Colima certification matrix
	./.ci/crossarch-tests/run-matrix.sh --context colima --platform "$(COLIMA_PLATFORM)" --toolchains "$(MATRIX_TOOLCHAINS)" --output .gosim/certification

gosim-tool: ## Build the independent gosimtool launcher
	@mkdir -p .gosim
	$(if $(GOSIM_TOOLCHAIN),GOTOOLCHAIN="$(GOSIM_TOOLCHAIN)") go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool ./cmd/gosim

gosim-prepare-selftest: gosim-tool ## Prepare selftest artifacts with the launcher
	"$(GOSIM)" prepare-selftest

go-test: test ## Run ordinary local tests

go-race-test: test-race ## Run the race suite when explicitly requested

build: ## Build gosimtool, gosim-worker and gosim-gen in .gosim/
	@mkdir -p .gosim
	@set -eu; \
	gosim_build_dir=$$(mktemp -d .gosim/build.XXXXXX); \
	trap 'rm -rf "$$gosim_build_dir"' 0; \
	$(if $(GOSIM_TOOLCHAIN),GOTOOLCHAIN="$(GOSIM_TOOLCHAIN)") go build -ldflags=-checklinkname=0 -tags=linkname -o "$$gosim_build_dir/" ./cmd/...; \
	mv -f "$$gosim_build_dir/gosim" .gosim/gosimtool; \
	mv -f "$$gosim_build_dir/gosim-worker" .gosim/gosim-worker; \
	mv -f "$$gosim_build_dir/gosim-gen" .gosim/gosim-gen

test-unit: ## Run uncached local native and simulation tests with persistent reports
	@python3 -B scripts/test_report.py unit --reports-dir "$(TEST_REPORTS_DIR)" --parallel=$(UNIT_PARALLEL) --packages=$(UNIT_PACKAGES)

test: test-unit ## Run local tests; no cross-architecture jobs, Docker, Colima or VMs

test-large: ## Run expensive candidate, translation and CLI suites with persistent reports
	@python3 -B scripts/test_report.py large --reports-dir "$(TEST_REPORTS_DIR)" --parallel=$(UNIT_PARALLEL) --packages=$(UNIT_PACKAGES) $(if $(LARGE_TUPLE),--tuple="$(LARGE_TUPLE)" --go="$(LARGE_GO)") $(if $(filter 1,$(LARGE_RACE)),--race)

test-unit-race: ## Run the local race suite for final verification
	@python3 -B scripts/test_report.py unit --race --reports-dir "$(TEST_REPORTS_DIR)" --parallel=$(UNIT_PARALLEL) --packages=$(UNIT_PACKAGES)

test-race: test-unit-race ## Run local race tests with persistent reports

test-report-tools: ## Check report generation and orchestration
	python3 -B -m unittest discover -s scripts -p 'test_report_test.py'
	python3 -B -m unittest discover -s .ci/crossarch-tests -p '*_test.py'
