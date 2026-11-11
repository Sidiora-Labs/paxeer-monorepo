# KindleLaunch — root Makefile (drives every leaf Go module via go.work).
# Verification gates mirror knowledge/kindlelaunch.frozen.kvx SECTION 17 + 20.
#
# Usage:
#   make build vet fmt lint test race cover cover-check   # full local gate
#   make ci                                               # what CI runs
#   make tidy sqlc                                         # maintenance

SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c
GO ?= go
GOFLAGS ?=

# Exercise each module independently, as the workflow matrix does.
export GOWORK := off
export GOTOOLCHAIN := go1.27.3

TEST_BASELINE := $(abspath ../../tests/ci/data/launchpad_go_test_baseline.json)
TEST_RUNNER := $(abspath ../../tests/ci/data/launchpad_go_tests.py)
COVER_INVENTORY := $(abspath ../../tools/launchpad/coverage_inventory.py)

# All independent modules (each owns a go.mod). Order: foundations first.
MODULES := \
	../../sdk/launchpad/shared \
	../../sdk/launchpad/protocol \
	api \
	indexer \
	pnl-tracker \
	ranking-algo \
	stats-workers \
	trading-charts \
	gateway \
	livestream \
	metadata \
	social \
	user \
	discovery \
	notifications \
	security \
	security/control


# Money / correctness-critical modules: 90% coverage gate (others 85%).
MONEY_MODULES := ../../sdk/launchpad/shared ../../sdk/launchpad/protocol indexer pnl-tracker trading-charts

.PHONY: all ci build vet fmt fmt-check lint test test-module race cover cover-check tidy sqlc clean help

all: build vet fmt-check lint race cover-check ## Run the full local verification gate

ci: test build vet fmt-check lint race cover-check ## Exactly what CI enforces on every PR

# has_pkgs <module> -> assign pkgs, or stop on package discovery failure.
# Only successful discovery may skip an empty scaffold.
define has_pkgs
pkgs=$$(cd $(1) && $(GO) list ./...) || { status=$$?; echo ">> package discovery failed for $(1)" >&2; exit $$status; }
endef

build: ## go build ./... in every module that has packages
	# The workflow's GOFLAGS includes the test timeout; go build does not accept it.
	@for m in $(MODULES); do $(call has_pkgs,$$m); if [ -z "$$pkgs" ]; then echo ">> skip $$m (no packages)"; continue; fi; echo ">> build $$m"; ( cd $$m && $(GO) build $(filter-out -timeout=%,$(GOFLAGS)) ./... ) || exit 1; done

vet: ## go vet ./... in every module that has packages
	@for m in $(MODULES); do $(call has_pkgs,$$m); if [ -z "$$pkgs" ]; then echo ">> skip $$m (no packages)"; continue; fi; echo ">> vet $$m"; ( cd $$m && $(GO) vet ./... ) || exit 1; done

fmt: ## gofmt -w + goimports -w across the repo
	@gofmt -w $(MODULES)
	@if command -v goimports >/dev/null; then goimports -w $(MODULES); fi

fmt-check: ## fail if any file is not gofmt-clean
	@out=$$(gofmt -l $(MODULES)); if [ -n "$$out" ]; then echo "gofmt needs to run on:"; echo "$$out"; exit 1; fi

lint: ## golangci-lint run in every module (zero warnings, SECTION 17)
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not installed"; exit 1; }
	@for m in $(MODULES); do $(call has_pkgs,$$m); if [ -z "$$pkgs" ]; then echo ">> skip $$m (no packages)"; continue; fi; echo ">> lint $$m"; ( cd $$m && golangci-lint run ) || exit 1; done

test race cover: $(TEST_BASELINE) $(TEST_RUNNER)

test: ## go test ./... in every module that has packages
	@for m in $(MODULES); do $(call has_pkgs,$$m); if [ -z "$$pkgs" ]; then echo ">> skip $$m (no packages)"; continue; fi; echo ">> test $$m"; python3 $(TEST_RUNNER) --module "$$m" --go "$(GO)" --flags="$(GOFLAGS)" || exit 1; done

test-module: ## go test -count=1 one module with an anchored -run (MODULE=<dir> RUN=<regex>)
	@valid=; for m in $(MODULES); do if [ "$$m" = '$(MODULE)' ]; then valid=1; break; fi; done; \
		[ "$$valid" = 1 ] || { echo 'MODULE must be one of $(MODULES)'; exit 1; }
	@case '$(RUN)' in ^*'$$') ;; *) echo 'RUN must be an anchored regex'; exit 1 ;; esac
	cd $(MODULE) && $(GO) test -count=1 $(GOFLAGS) ./... -run '$(RUN)'

race: ## go test -race ./... in every module (zero data races, SECTION 17)
	@for m in $(MODULES); do $(call has_pkgs,$$m); if [ -z "$$pkgs" ]; then echo ">> skip $$m (no packages)"; continue; fi; echo ">> race $$m"; python3 $(TEST_RUNNER) --module "$$m" --go "$(GO)" --flags="$(strip $(GOFLAGS) -race)" || exit 1; done

cover: ## write fresh, run-bound per-module coverage profiles into .cover/
	@mkdir -p .cover
	@rm -f .cover/*.out .cover/run.json .cover/run-id
	@run_id=$$(python3 $(COVER_INVENTORY) begin .cover .cover/run.json --modules "$(MODULES)"); \
	for m in $(MODULES); do \
		$(call has_pkgs,$$m); \
		if [ -z "$$pkgs" ]; then \
			python3 $(COVER_INVENTORY) record .cover .cover/run.json --run-id "$$run_id" --module "$$m" --empty no-packages; \
			continue; \
		fi; \
		name=$${m//\//_}; \
		echo ">> cover $$m"; \
		python3 $(TEST_RUNNER) --module "$$m" --go "$(GO)" --flags="$(GOFLAGS)" --profile "$(CURDIR)/.cover/$$name.out"; \
		empty=(); \
		if ! awk 'NR > 1 && $$2 > 0 {found=1} END {exit !found}' ".cover/$$name.out"; then empty=(--empty no-statements); fi; \
		python3 $(COVER_INVENTORY) record .cover .cover/run.json --run-id "$$run_id" --module "$$m" "$${empty[@]}"; \
	done; \
	python3 $(COVER_INVENTORY) finish .cover .cover/run.json --run-id "$$run_id"; \
	printf '%s\n' "$$run_id" > .cover/run-id

cover-check: cover ## enforce the coverage gate (85% repo / 90% money modules)
	@MODULES="$(MODULES)" RUN_MANIFEST=.cover/run.json COVER_RUN_ID="$$(cat .cover/run-id)" MONEY="$(MONEY_MODULES)" ../../tools/launchpad/coverage-gate.sh

tidy: ## go mod tidy in every module
	@for m in $(MODULES); do echo ">> tidy $$m"; ( cd $$m && $(GO) mod tidy ); done

sqlc: ## regenerate type-safe DB code (sqlc) where a sqlc.yaml exists
	@command -v sqlc >/dev/null || { echo "sqlc not installed"; exit 1; }
	@for m in $(MODULES); do if [ -f $$m/sqlc.yaml ]; then echo ">> sqlc $$m"; ( cd $$m && sqlc generate ); fi; done

clean: ## remove build/coverage artifacts
	@rm -rf .cover
	@for m in $(MODULES); do ( cd $$m && $(GO) clean ./... ); done

help: ## list targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
