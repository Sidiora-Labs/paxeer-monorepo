#!/usr/bin/make -f

########################################
### Testing

PACKAGES=$(shell go list ./...)

BINDIR ?= $(GOPATH)/bin

### coverage, app, persistence, and libs tests
test_cover:
	# run the go unit tests with coverage
	bash test/test_cover.sh
.PHONY: test_cover

test_release:
	@go test -tags release $(PACKAGES)
.PHONY: test_release

### go tests
test:
	@echo "--> Running go test"
	@go test -p 1 $(PACKAGES) -tags deadlock
.PHONY: test

test_race:
	@echo "--> Running go test --race"
	@go test -p 1 -v -race $(PACKAGES)
.PHONY: test_race
