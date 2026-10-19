.DEFAULT_GOAL := help

.PHONY: help test test-unit test-unit-race test-race test-large test-report-tools

# Independent test cases and package binaries run concurrently on the local host.
UNIT_PARALLEL ?= 4
UNIT_PACKAGES ?= 2
TEST_REPORTS_DIR ?= test-reports
# Optional override; otherwise Go resolves the compiler normally.
GOSIM_TOOLCHAIN ?=
export GOSIM_TOOLCHAIN

help: ## List available targets
	@awk 'BEGIN { FS = ":.*## "; print "Usage: make <target>\n\nTargets:" } /^[a-zA-Z0-9_-]+:.*## / { printf "  %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

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
