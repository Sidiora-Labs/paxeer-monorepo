#!/usr/bin/make -f

# The embedded WASM module belongs to the repository-root Go module.
.DEFAULT_GOAL := help
.PHONY: help

ifneq ($(filter-out help,$(MAKECMDGOALS)),)
$(error wasm/Makefile is retired; run make paxd-build from the repository root; use help for module developer commands)
endif

help:
	@printf '%s\n' \
		'wasm/Makefile is retired: WASM is embedded in the root paxd module.' \
		'Run these commands from the repository root:' \
		'  make paxd-build' \
		'  make paxd-native-libs' \
		'  CGO_ENABLED=1 go test -mod=readonly ./wasm/x/wasm/...' \
		'Build native libraries before running the module tests.'
