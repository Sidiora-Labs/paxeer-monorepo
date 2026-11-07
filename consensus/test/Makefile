#!/usr/bin/make -f

########################################
### Testing

PACKAGES=$(shell go list ./...)

BINDIR ?= $(GOPATH)/bin

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
