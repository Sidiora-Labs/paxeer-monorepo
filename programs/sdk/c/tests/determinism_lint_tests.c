#define _XOPEN_SOURCE 700
#define main determinism_lint_test_cli
#include "../tools/determinism_lint.c"
#undef main

#define LINT_CHECK(condition) do { if (!(condition)) { \
    (void)fprintf(stderr, "determinism regression line %d\n", __LINE__); \
    return 1; } } while (0)

static int lint_source_case(const char *source, int accepted)
{
    lint_state state;
    int result;
    memset(&state, 0, sizeof(state));
    result = check_source("regression.c", source, &state);
    LINT_CHECK(result == accepted);
    LINT_CHECK(accepted ? state.rule == NULL : state.rule == RULE_IMPORT_DECLARATION);
    return 0;
}

static int test_repair_245_parameterized_imports(void)
{
    static const char versioned[] =
        "#define LXP_PROGRAM_IMPORT_VERSIONED(module_name, function_name) \\\n"
        " __attribute__((import_module(module_name), import_name(function_name)))\n";
    char source[2048];
    unsigned selected;
    (void)snprintf(source, sizeof(source), "%s%s", versioned,
        "const char *unrelated = \"layerx_v2\";\n");
    LINT_CHECK(lint_source_case(source, 1) == 0);
    (void)snprintf(source, sizeof(source), "%s%s", versioned,
        "LXP_PROGRAM_IMPORT_VERSIONED(\"layerx_v2\", \"response_write\")\n"
        "extern int response(int, int, int);\n"
        "LXP_PROGRAM_IMPORT_VERSIONED(\"layerx_v3\", \"oracle_read\")\n"
        "extern int oracle(int, int, int, int);\n"
        "LXP_PROGRAM_IMPORT_VERSIONED(\"layerx_v4\", \"web_read\")\n"
        "extern int web(int, int, int, int);\n");
    for (selected = 1U; selected <= 4U; ++selected) {
        selected_abi_version = selected;
        LINT_CHECK(lint_source_case(source, 1) == 0);
    }
    selected_abi_version = 1U;
    LINT_CHECK(lint_source_case(
        "#define LXP_PROGRAM_IMPORT(function_name) \\\n"
        " __attribute__((import_module(\"layerx_v1\"), import_name(function_name)))\n"
        "LXP_PROGRAM_IMPORT(\"storage_read\") extern int read(int, int, int, int);", 1) == 0);
    LINT_CHECK(lint_source_case(
        "#define LXP_PROGRAM_IMPORT_VERSIONED(module_name, function_name)\n"
        "const char *unrelated = \"wasi_snapshot_preview1\";", 1) == 0);
    LINT_CHECK(lint_source_case(
        "/* import_module(\"wasi\") */ const char *s = \"import_module\\\"\";\n"
        "// import_module(\"wasi\")\n"
        "__attribute__((import_module /* gap */ (\"layerx_v1\"),\n"
        "import_name(\"storage_read\"))) extern int read(int, int, int, int);", 1) == 0);
    LINT_CHECK(lint_source_case(
        "__attribute__((import_module(module_name), import_name(function_name)))\n"
        "const char *later = \"layerx_v1\";", 0) == 0);
    LINT_CHECK(lint_source_case(
        "__attribute__((import_module(\"layerx_v1\"), import_name(function_name)))\n"
        "const char *later = \"storage_read\";", 0) == 0);
    LINT_CHECK(lint_source_case(
        "__attribute__((import_module(\"layerx_v1\", import_name(\"storage_read\")))", 0) == 0);
    LINT_CHECK(lint_source_case("import_module(\"layerx_v1", 0) == 0);
    LINT_CHECK(lint_source_case("import_module(\"layerx_v2\")", 0) == 0);
    LINT_CHECK(lint_source_case("import_module(\"wasi\")", 0) == 0);
    LINT_CHECK(lint_source_case("LXP_PROGRAM_IMPORT(\"clock_gettime\")", 0) == 0);
    LINT_CHECK(lint_source_case(
        "LXP_PROGRAM_IMPORT_VERSIONED(\"layerx_v2\", \"storage_read\")", 0) == 0);
    LINT_CHECK(lint_source_case(
        "LXP_PROGRAM_IMPORT_VERSIONED(\"layerx_v5\", \"web_read\")", 0) == 0);
    LINT_CHECK(lint_source_case(
        "#define LXP_PROGRAM_IMPORT_VERSIONED(module_name, function_name) \\\n"
        " __attribute__((import_module(\"wasi\"), import_name(function_name)))", 0) == 0);
    LINT_CHECK(lint_source_case(
        "#define OTHER(module_name) import_module(module_name)\n"
        "const char *later = \"layerx_v1\";", 0) == 0);
    puts("PASS test_repair_245_parameterized_imports");
    return 0;
}

static int test_repair_245_paid_counter_sources(void)
{
    static const char *const sources[] = {
        "programs/sdk/c/src/abi.c", "programs/sdk/c/src/amount.c",
        "programs/sdk/c/src/bytes.c", "programs/sdk/c/src/calls.c",
        "programs/sdk/c/src/capability.c", "programs/sdk/c/src/entry.c",
        "programs/sdk/c/src/events.c", "programs/sdk/c/src/receipts.c",
        "programs/sdk/c/src/runtime.c", "programs/sdk/c/src/storage.c",
        "programs/sdk/c/src/transfers.c",
        "programs/sdk/c/examples/paid-counter/src/program.c",
        "programs/sdk/c/src/host.h", "programs/sdk/c/src/internal.h",
        "programs/sdk/c/include/layerx/program.h",
        "programs/sdk/c/src/host_versioned.h"
    };
    size_t index;
    for (index = 0U; index < sizeof(sources) / sizeof(sources[0]); ++index) {
        size_t length;
        uint8_t *source = read_file(sources[index], &length);
        lint_state state;
        int result;
        LINT_CHECK(source != NULL && length != 0U);
        memset(&state, 0, sizeof(state));
        result = check_source(sources[index], (const char *)source, &state);
        free(source);
        LINT_CHECK(result == 1 && state.rule == NULL);
    }
    puts("PASS test_repair_245_paid_counter_sources");
    return 0;
}

static int lint_compiled_case(const uint8_t *bytes, size_t length, const char *rule)
{
    lint_state state;
    int accepted;
    memset(&state, 0, sizeof(state));
    state.bytes = bytes;
    state.length = length;
    accepted = check_module(&state);
    LINT_CHECK(accepted == (rule == NULL));
    LINT_CHECK(state.rule == rule);
    return 0;
}

static int lint_paid_counter_offsets(const uint8_t *bytes, size_t length,
    size_t *module_offset, size_t *name_offset, size_t *parameter_offset)
{
    lint_state state;
    uint32_t import_type = UINT32_MAX;
    size_t type_start = 0U;
    memset(&state, 0, sizeof(state));
    state.bytes = bytes;
    state.length = length;
    state.cursor = 8U;
    while (state.cursor < length) {
        uint8_t id;
        uint32_t section_length;
        size_t end;
        LINT_CHECK(read_byte(&state, &id) && read_u32_leb(&state, &section_length));
        LINT_CHECK(section_length <= length - state.cursor);
        end = state.cursor + section_length;
        if (id == 1U) type_start = state.cursor;
        if (id == 2U) {
            uint32_t count, index;
            LINT_CHECK(read_u32_leb(&state, &count));
            for (index = 0U; index < count; ++index) {
                char module[256], name[256];
                uint8_t kind;
                uint32_t type;
                size_t module_end, name_end;
                LINT_CHECK(read_name(&state, module, sizeof(module)));
                module_end = state.cursor;
                LINT_CHECK(read_name(&state, name, sizeof(name)));
                name_end = state.cursor;
                LINT_CHECK(read_byte(&state, &kind) && kind == 0U && read_u32_leb(&state, &type));
                if (strcmp(module, "layerx_v1") == 0 && strcmp(name, "storage_read") == 0) {
                    *module_offset = module_end - strlen(module);
                    *name_offset = name_end - strlen(name);
                    import_type = type;
                }
            }
        }
        state.cursor = end;
    }
    LINT_CHECK(type_start != 0U && import_type != UINT32_MAX);
    state.cursor = type_start;
    {
        uint32_t count, index;
        LINT_CHECK(read_u32_leb(&state, &count) && import_type < count);
        for (index = 0U; index < count; ++index) {
            uint8_t form;
            uint32_t parameters, results;
            LINT_CHECK(read_byte(&state, &form) && form == 0x60U);
            LINT_CHECK(read_u32_leb(&state, &parameters));
            if (index == import_type) {
                LINT_CHECK(parameters == 4U && bytes[state.cursor] == 0x7FU);
                *parameter_offset = state.cursor;
                return 0;
            }
            LINT_CHECK(parameters <= length - state.cursor);
            state.cursor += parameters;
            LINT_CHECK(read_u32_leb(&state, &results) && results <= length - state.cursor);
            state.cursor += results;
        }
    }
    return 1;
}

static int test_repair_245_compiled_imports(void)
{
    size_t length, module_offset = 0U, name_offset = 0U, parameter_offset = 0U;
    uint8_t *bytes = read_file("programs/sdk/c/examples/paid-counter/build/paid-counter.wasm", &length);
    uint8_t saved;
    LINT_CHECK(bytes != NULL);
    LINT_CHECK(lint_compiled_case(bytes, length, NULL) == 0);
    LINT_CHECK(lint_paid_counter_offsets(bytes, length, &module_offset,
        &name_offset, &parameter_offset) == 0);
    saved = bytes[module_offset + 8U];
    bytes[module_offset + 8U] = '2';
    LINT_CHECK(lint_compiled_case(bytes, length, RULE_FORBIDDEN_IMPORT) == 0);
    selected_abi_version = 4U;
    LINT_CHECK(lint_compiled_case(bytes, length, RULE_FORBIDDEN_IMPORT) == 0);
    selected_abi_version = 1U;
    bytes[module_offset + 8U] = saved;
    saved = bytes[name_offset];
    bytes[name_offset] = 'x';
    LINT_CHECK(lint_compiled_case(bytes, length, RULE_FORBIDDEN_IMPORT) == 0);
    bytes[name_offset] = saved;
    bytes[parameter_offset] = 0x7EU;
    LINT_CHECK(lint_compiled_case(bytes, length, RULE_IMPORT_SIGNATURE) == 0);
    bytes[parameter_offset] = 0x7FU;
    LINT_CHECK(lint_compiled_case(bytes, length, NULL) == 0);
    free(bytes);
    puts("PASS test_repair_245_compiled_imports");
    return 0;
}

static int test_repair_245_determinism_lint(void)
{
    selected_abi_version = 1U;
    LINT_CHECK(select_default_manifest() && load_manifest());
    LINT_CHECK(test_repair_245_parameterized_imports() == 0);
    LINT_CHECK(test_repair_245_paid_counter_sources() == 0);
    LINT_CHECK(test_repair_245_compiled_imports() == 0);
    return 0;
}
