#!/usr/bin/make -f

BUILDDIR ?= $(CURDIR)/build

BUILD_TAGS?=tendermint

# If building a release, please checkout the version tag to get the correct version setting
ifneq ($(shell git symbolic-ref -q --short HEAD),)
VERSION := unreleased-$(shell git symbolic-ref -q --short HEAD)-$(shell git rev-parse HEAD)
else
VERSION := $(shell git describe --tags --always --match 'paxeer-network/v*' | sed 's|^paxeer-network/||')
endif

LD_FLAGS = -X github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version.TMVersion=$(VERSION)
BUILD_FLAGS = -mod=readonly -ldflags "$(LD_FLAGS)"
CGO_ENABLED ?= 0

BUF = go run github.com/bufbuild/buf/cmd/buf@v1.4.0

# handle nostrip
ifeq (,$(findstring nostrip,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
  LD_FLAGS += -s -w
endif

# handle race
ifeq (race,$(findstring race,$(TENDERMINT_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_FLAGS += -race
endif

# handle cleveldb
ifeq (cleveldb,$(findstring cleveldb,$(TENDERMINT_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += cleveldb
endif

# handle badgerdb
ifeq (badgerdb,$(findstring badgerdb,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_TAGS += badgerdb
endif

# handle rocksdb
ifeq (rocksdb,$(findstring rocksdb,$(TENDERMINT_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += rocksdb
endif

# handle boltdb
ifeq (boltdb,$(findstring boltdb,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_TAGS += boltdb
endif

# allow users to pass additional flags via the conventional LDFLAGS variable
LD_FLAGS += $(LDFLAGS)

all: build test install
.PHONY: all

include test/Makefile

###############################################################################
###                                Build Tendermint                         ###
###############################################################################

build: $(BUILDDIR)/
	CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o $(BUILDDIR)/ ./cmd/tendermint/
.PHONY: build

install:
	CGO_ENABLED=$(CGO_ENABLED) go install $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' ./cmd/tendermint
.PHONY: install

$(BUILDDIR)/:
	mkdir -p $@

###############################################################################
###				Privval Server                              ###
###############################################################################

build_privval_server:
	@go build -mod=readonly -o $(BUILDDIR)/ ./cmd/priv_val_server/...
.PHONY: build_privval_server

generate_test_cert:
	# generate self signing ceritificate authority
	@certstrap init --common-name "root CA" --expires "20 years"
	# generate server cerificate
	@certstrap request-cert -cn server -ip 127.0.0.1
	# self-sign server cerificate with rootCA
	@certstrap sign server --CA "root CA"
	# generate client cerificate
	@certstrap request-cert -cn client -ip 127.0.0.1
	# self-sign client cerificate with rootCA
	@certstrap sign client --CA "root CA"
.PHONY: generate_test_cert

###############################################################################
###                                  Libs                                   ###
###############################################################################

# generates certificates for TLS testing in remotedb and RPC server
gen_certs: clean_certs
	certstrap init --common-name "tendermint.com" --passphrase ""
	certstrap request-cert --common-name "server" -ip "127.0.0.1" --passphrase ""
	certstrap sign "server" --CA "tendermint.com" --passphrase ""
	mv out/server.crt rpc/jsonrpc/server/test.crt
	mv out/server.key rpc/jsonrpc/server/test.key
	rm -rf out
.PHONY: gen_certs

# deletes generated certificates
clean_certs:
	rm -f rpc/jsonrpc/server/test.crt
	rm -f rpc/jsonrpc/server/test.key
.PHONY: clean_certs

###############################################################################
###                  Formatting, linting, and vetting                       ###
###############################################################################

format:
	find . -name '*.go' -type f -not -path "*.git*" -not -name '*.pb.go' -not -name '*pb_test.go' | xargs gofmt -w -s
	find . -name '*.go' -type f -not -path "*.git*"  -not -name '*.pb.go' -not -name '*pb_test.go' | xargs goimports -w -local github.com/Sidiora-Labs/Paxeer-X-Network
.PHONY: format

DESTINATION = ./index.html.md

###############################################################################
###                                Mocks                                    ###
###############################################################################

mockery:
	go generate -run="./scripts/mockery_generate.sh" ./...
.PHONY: mockery

###############################################################################
###                               Metrics                                   ###
###############################################################################

metrics: testdata-metrics
	go generate -run="scripts/metricsgen" ./...
.PHONY: metrics

	# By convention, the go tool ignores subdirectories of directories named
	# 'testdata'. This command invokes the generate command on the folder directly
	# to avoid this.
testdata-metrics:
	ls ./scripts/metricsgen/testdata | xargs -I{} go generate -run="scripts/metricsgen" ./scripts/metricsgen/testdata/{}
.PHONY: testdata-metrics

# Build linux binary on other platforms
build-linux:
	GOOS=linux GOARCH=amd64 $(MAKE) build
.PHONY: build-linux

clean:
	rm -rf $(CURDIR)/artifacts/ $(BUILDDIR)/

# Implements test splitting and running. This is pulled directly from
# the github action workflows for better local reproducibility.

GO_TEST_FILES != find $(CURDIR) -name "*_test.go"

# default to four splits by default
NUM_SPLIT ?= 4

$(BUILDDIR):
	mkdir -p $@

# The format statement filters out all packages that don't have tests.
# Note we need to check for both in-package tests (.TestGoFiles) and
# out-of-package tests (.XTestGoFiles).
$(BUILDDIR)/packages.txt:$(GO_TEST_FILES) $(BUILDDIR)
	go list -f "{{ if (or .TestGoFiles .XTestGoFiles) }}{{ .ImportPath }}{{ end }}" ./... | sort > $@

split-test-packages:$(BUILDDIR)/packages.txt
	split -d -n l/$(NUM_SPLIT) $< $<.
test-group-%:split-test-packages
	cat $(BUILDDIR)/packages.txt.$* | xargs go test -mod=readonly -timeout=15m -race -covermode=atomic -coverprofile=$*.profile.out
