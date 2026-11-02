# Preserve explicit refusals for callers of the retired qualification entrypoints.
PAXEER_X_REPLAY_AUTHORITY_PRODUCER := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))../../../tools/qualification/paxeer-x/replay-authority.py

.PHONY: paxeer-x-replay-authority-build $(BUILD_DIR)/generated/replay-authority-fixture-base.inc
paxeer-x-replay-authority-build:
	python3 "$(PAXEER_X_REPLAY_AUTHORITY_PRODUCER)" --build

$(BUILD_DIR)/generated/replay-authority-fixture-base.inc:
	python3 "$(PAXEER_X_REPLAY_AUTHORITY_PRODUCER)" --prepare-fixture "$@"
