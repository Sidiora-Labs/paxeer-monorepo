C_PROOF_SOURCES := tests/proof/lxp_proof_b_error_code_registry_d6.c \
	tests/proof/lxp_proof_lx_asset_h.c \
	tests/proof/lxp_proof_lx_service_h.c \
	tests/proof/lxp_proof_lxp_bridge_h.c \
	tests/proof/lxp_proof_lxp_module_h.c \
	tests/proof/lxp_proof_lxp_receipt_h.c \
	tests/proof/lxp_proof_programs_h.c \
	tests/proof/lxp_proof_result_registry.c
C_PROOF_CASES := $(patsubst tests/proof/lxp_proof_%.c,%,$(C_PROOF_SOURCES))
C_PROOF_TARGETS := $(addprefix test-proof-,$(C_PROOF_CASES))
C_PROOF_BINARIES := $(patsubst tests/%.c,$(BUILD_DIR)/tests/%,$(C_PROOF_SOURCES))

.PHONY: test-proof $(C_PROOF_TARGETS)
test-proof: $(C_PROOF_TARGETS)

$(C_PROOF_TARGETS): test-proof-%: $(BUILD_DIR)/tests/proof/lxp_proof_%
	$(RUN_PREFIX) $<

$(C_PROOF_BINARIES): $(BUILD_DIR)/tests/proof/%: tests/proof/%.c \
		tests/codec/lxp_kernel_case.h Makefile tools/paxeer-x/build/proof.mk \
		$(LIBRARY) $(PROGRAMS_RUNTIME_LIB) | programs-build
	@mkdir -p $(@D)
	$(CC) $(CPPFLAGS) $(CFLAGS) -MMD -MP -MF $@.d -MT $@ $< \
		$(LIBRARY) $(PROGRAMS_RUNTIME_LIB) $(LIBRARY) $(EXTRA_LDFLAGS) \
		$(PROGRAMS_NATIVE_LDLIBS) -lcrypto -lsqlite3 -pthread -ldl -lm -o $@

-include $(C_PROOF_BINARIES:=.d)
