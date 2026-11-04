.PHONY: test-governance-audit

$(BUILD_DIR)/tests/test_governance_audit: tests/modules/test_governance_audit.c \
		src/modules/governance/lxp_emergency_store.h $(LIBRARY) \
		$(PROGRAMS_RUNTIME_LIB) | programs-build
	@mkdir -p $(@D)
	$(CC) $(CPPFLAGS) $(CFLAGS) $< $(LIBRARY) $(PROGRAMS_RUNTIME_LIB) \
		$(LIBRARY) $(EXTRA_LDFLAGS) $(PROGRAMS_NATIVE_LDLIBS) \
		-lcrypto -pthread -ldl -lm -o $@

test-governance-audit: $(BUILD_DIR)/tests/test_governance_audit
	$(RUN_PREFIX) $(BUILD_DIR)/tests/test_governance_audit

test-governance-emergency: test-governance-audit
