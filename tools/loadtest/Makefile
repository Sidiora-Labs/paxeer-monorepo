# Loadtest_v2 Makefile
# Generates Go bindings for smart contracts and builds the paxload CLI

# Directories
CONTRACTS_DIR := generator/contracts
SCENARIOS_DIR := generator/scenarios
BINDINGS_DIR := generator/bindings
BUILD_DIR := build

# Binary configuration
BINARY_NAME := paxload
INSTALL_PATH := $(GOPATH)/bin
ifeq ($(GOPATH),)
	INSTALL_PATH := $(HOME)/go/bin
endif

# Reviewed binding toolchain. No tool is installed implicitly by build/generate.
TOOLS_DIR := $(CURDIR)/.tools
SOLC := $(TOOLS_DIR)/solc
ABIGEN := $(TOOLS_DIR)/abigen
ABIGEN_VERSION := v1.17.0
SOLC_VERSION := 0.8.19+commit.7dd6d404.Linux.g++
SOLC_SHA256 := 7a5c1d3dc9a8eba62bb2ec37192c9178ae5fe8a54a56e5573fd3c9c17cd9eb48
SOLC_URL := https://raw.githubusercontent.com/ethereum/solc-bin/gh-pages/linux-amd64/solc-linux-amd64-v0.8.19+commit.7dd6d404

# Find all .sol files in contracts directory
SOL_FILES := $(wildcard $(CONTRACTS_DIR)/*.sol)
CONTRACT_NAMES := $(basename $(notdir $(SOL_FILES)))

# Generated files
ABI_FILES := $(addprefix $(BUILD_DIR)/, $(addsuffix .abi, $(CONTRACT_NAMES)))
BIN_FILES := $(addprefix $(BUILD_DIR)/, $(addsuffix .bin, $(CONTRACT_NAMES)))
BINDING_FILES := $(addprefix $(BINDINGS_DIR)/, $(addsuffix .go, $(CONTRACT_NAMES)))
SCENARIO_TEMPLATE_FILES := $(addprefix $(SCENARIOS_DIR)/, $(addsuffix .go, $(CONTRACT_NAMES)))

.PHONY: generate clean help build-cli install setup-node install-tools check-tools tidy build test lint

# Default target
help:
	@echo "Available targets:"
	@echo "  build        - Build the paxload CLI (alias for build-cli)"
	@echo "  test         - Run tests with coverage"
	@echo "  lint         - Run linting and static analysis"
	@echo "  setup-node   - Install the checksum-pinned native Solidity compiler"
	@echo "  generate     - Generate Go bindings and scenario templates for all contracts"
	@echo "  clean        - Remove generated files"
	@echo "  help         - Show this help message"
	@echo "  build-cli    - Build the paxload CLI"
	@echo "  install      - Install the paxload CLI"

# Historical target name retained; native solc requires no Node or nvm.
setup-node:
	@mkdir -p "$(TOOLS_DIR)" && chmod 700 "$(TOOLS_DIR)"
	@set -eu; tmp=$$(mktemp "$(TOOLS_DIR)/solc.XXXXXX"); trap 'rm -f "$$tmp"' EXIT HUP INT TERM; \
	curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$(SOLC_URL)" -o "$$tmp"; \
	printf '%s  %s\n' "$(SOLC_SHA256)" "$$tmp" | sha256sum --check --strict; \
	chmod 700 "$$tmp"; mv "$$tmp" "$(SOLC)"

# Main generate target
generate: build-cli check-tools $(BINDING_FILES) $(SCENARIO_TEMPLATE_FILES)
	@echo "🏭 Updating scenario factory..."
	@./scripts/update_factory.sh $(CONTRACT_NAMES)
	@echo "✅ Generated bindings and scenario templates for contracts: $(CONTRACT_NAMES)"

# Create build directory
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# Create bindings directory
$(BINDINGS_DIR):
	@mkdir -p $(BINDINGS_DIR)

# Create scenarios directory
$(SCENARIOS_DIR):
	@mkdir -p $(SCENARIOS_DIR)

# Compile a single contract to ABI and bytecode
$(BUILD_DIR)/%.abi $(BUILD_DIR)/%.bin &: $(CONTRACTS_DIR)/%.sol | $(BUILD_DIR) check-tools
	@echo "🔨 Compiling contract: $*"
	@$(SOLC) --abi --bin --optimize --overwrite -o $(BUILD_DIR) $<
	@echo "✅ Compiled: $*"

# Generate Go binding from ABI and bytecode
$(BINDINGS_DIR)/%.go: $(BUILD_DIR)/%.abi $(BUILD_DIR)/%.bin | $(BINDINGS_DIR) check-tools
	@echo "🏭 Generating Go binding: $*"
	@$(ABIGEN) --abi=$(BUILD_DIR)/$*.abi --bin=$(BUILD_DIR)/$*.bin --pkg=bindings --type=$* --out=$@
	@{ $(SOLC) --version; $(ABIGEN) --version; go version; sha256sum "$(SOLC)" "$(ABIGEN)" "$<" "$(BUILD_DIR)/$*.bin" "$@"; } > "$@.provenance"
	@echo "✅ Generated binding: $*"

# Generate scenario template files (only if they don't exist)
$(SCENARIOS_DIR)/%.go: | $(SCENARIOS_DIR)
	@./scripts/generate_scenario_template.sh $* $@

# Clean generated files
clean:
	@echo "🧹 Cleaning generated files ..."
	@rm -rf $(BUILD_DIR) $(BINDINGS_DIR)
	@echo "✅ Cleaned up generated files"

# Refuse missing or replaced tools; installing replacements is always explicit.
check-tools:
	@test -x "$(SOLC)" && test -x "$(ABIGEN)"
	@printf '%s  %s\n' "$(SOLC_SHA256)" "$(SOLC)" | sha256sum --check --strict
	@$(SOLC) --version | grep -F "Version: $(SOLC_VERSION)"
	@go version -m "$(ABIGEN)" | grep -E '^[[:space:]]*mod[[:space:]]+github.com/ethereum/go-ethereum[[:space:]]+$(ABIGEN_VERSION)[[:space:]]'
	@cd "$(TOOLS_DIR)" && sha256sum --check --strict abigen.sha256

install-tools: setup-node
	@GOBIN="$(TOOLS_DIR)" go install github.com/ethereum/go-ethereum/cmd/abigen@$(ABIGEN_VERSION)
	@cd "$(TOOLS_DIR)" && sha256sum abigen > abigen.sha256
	@$(MAKE) check-tools

# Dependency maintenance is a separate, deliberate action.
tidy:
	go mod tidy

# Build the paxload CLI binary
build-cli: | $(BUILD_DIR)
	@echo "🔨 Building CLI"
	@go build -mod=readonly -trimpath -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "✅ Built CLI: $(BUILD_DIR)/$(BINARY_NAME)"

# Install the paxload CLI
install: build-cli
	@echo "📦 Installing CLI ..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✅ Installed CLI: $(BINARY_NAME)"

# Build the paxload CLI binary (alias for build-cli)
build: build-cli

# Run tests with coverage
test:
	@echo "🔍 Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@echo "✅ Tests passed"

# Run linting and static analysis
lint:
	@echo "🔍 Running linting and static analysis..."
	@golangci-lint run
	@echo "✅ Linting and static analysis passed"
