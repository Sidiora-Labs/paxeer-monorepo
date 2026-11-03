.PHONY: paxeer-x-arbiter-native-authority-build paxeer-x-arbiter-native-authority-runtime
paxeer-x-arbiter-native-authority-build: $(BUILD_DIR)/tests/lxp_test_module_maintenance \
	$(BUILD_DIR)/bin/layerxd $(BUILD_DIR)/bin/layerx-genesis-build

paxeer-x-arbiter-native-authority-runtime:
	env CARGO_BUILD_JOBS=4 CARGO_TARGET_DIR='$(PROGRAMS_TARGET_DIR)' $(PROGRAMS_CARGO) build --locked --manifest-path programs/Cargo.toml -p layerx-programs-sandbox --features host-ffi
