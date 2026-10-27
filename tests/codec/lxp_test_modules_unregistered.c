#include "lxp_kernel_case.h"

#include "layerx/programs.h"
#include "layerx/lx_web.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_module_ctx.h"

extern lxp_result lx_web_committed_read(
    lxp_module_ctx *ctx, const uint8_t program_id[32], uint64_t request_id,
    lx_web_answer *answer) __attribute__((weak));

extern lxp_result lxp_bridge_credit_bind_receipt(
    lxp_receipt *receipt, const lxp_module_ctx *ctx) __attribute__((weak));

static lxp_kernel kernel;
static lxp_state_store state;
static lxp_state_journal journal;
static lxp_genesis_manifest manifest;
static lxp_module_ctx ctx;
static uint8_t arena_bytes[LXP_MAX_ACTIVITY_BYTES];
static lxp_arena arena;

static int initialize(void)
{
    static const uint64_t parameters = 1U;
    lxp_genesis_module_plan plan;
    LXP_CASE_CHECK(lxp_state_store_init(&state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &state, &journal,
                                    &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_default(
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT, false, &plan) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_register(&plan, &kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) == LXP_OK);
    return 0;
}

static int web_and_bridge_rows_absent(void)
{
    static const uint16_t retired[] = {LXP_MODULE_BRIDGE, LXP_MODULE_WEB};
    static const char *const names[] = {"bridge", "web"};
    const lxp_genesis_module_entry *table;
    lxp_genesis_module_plan plan;
    uint8_t key[32];
    size_t count = 0U;
    size_t index;
    size_t slot;
    LXP_CASE_CHECK(LXP_MODULE_BUDGET == 3);
    LXP_CASE_CHECK(LXP_MODULE_BRIDGE == 8);
    LXP_CASE_CHECK(LXP_MODULE_WEB == 11);
    LXP_CASE_CHECK(LXP_MODULE_RESERVED_COUNT == 11);
    table = lxp_genesis_module_table(&count);
    LXP_CASE_CHECK(table != NULL && count == 9U);
    for (index = 0U; index < count; ++index) {
        LXP_CASE_CHECK(table[index].module_id != LXP_MODULE_BRIDGE);
        LXP_CASE_CHECK(table[index].module_id != LXP_MODULE_WEB);
        LXP_CASE_CHECK(table[index].iface()->module_id == table[index].module_id);
    }
    LXP_CASE_CHECK(lxp_genesis_module_plan_default(
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT, false, &plan) == LXP_OK);
    LXP_CASE_CHECK(plan.count == 3U);
    LXP_CASE_CHECK(lxp_genesis_module_plan_default(
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT, true, &plan) == LXP_ERR_UNKNOWN_MODULE);
    memset(&manifest, 0, sizeof(manifest));
    manifest.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    LXP_CASE_CHECK(lxp_genesis_module_plan_resolve(&manifest, &plan) == LXP_OK);
    LXP_CASE_CHECK(plan.count == 3U);
    for (slot = 0U; slot < sizeof(retired) / sizeof(retired[0]); ++slot) {
        LXP_CASE_CHECK(lxp_genesis_module_enable_key(retired[slot], key) ==
                       LXP_ERR_UNKNOWN_MODULE);
        manifest.parameter_count = 1U;
        memset(&manifest.parameters[0], 0, sizeof(manifest.parameters[0]));
        manifest.parameters[0].module_id = LXP_MODULE_GOVERNANCE;
        memcpy(manifest.parameters[0].key, "module-enable:",
               LXP_GENESIS_MODULE_ENABLE_PREFIX_BYTES);
        memcpy(manifest.parameters[0].key + LXP_GENESIS_MODULE_ENABLE_PREFIX_BYTES,
               names[slot], strlen(names[slot]));
        manifest.parameters[0].value[31] = 1U;
        LXP_CASE_CHECK(lxp_genesis_module_plan_resolve(&manifest, &plan) ==
                       LXP_ERR_UNKNOWN_MODULE);
        manifest.parameter_count = 0U;
        manifest.module_value_count = 1U;
        memset(&manifest.module_values[0], 0, sizeof(manifest.module_values[0]));
        manifest.module_values[0].module_id = retired[slot];
        memcpy(manifest.module_values[0].key, "custody-credit-profile/v1", 24U);
        manifest.module_values[0].value_length = 1U;
        manifest.module_values[0].value[0] = 1U;
        LXP_CASE_CHECK(lxp_genesis_module_plan_resolve(&manifest, &plan) ==
                       LXP_ERR_UNKNOWN_MODULE);
        manifest.module_value_count = 0U;
    }
    return 0;
}

static int program_web_read_disabled(void)
{
    static const uint8_t entrypoint[] = "layerx_call";
    uint8_t payload[50U + LX_PROGRAMS_CALL_BUDGET_FIELDS * 8U +
                    sizeof(entrypoint) - 1U] = {0};
    void *decoded = NULL;
    uint64_t token;
    size_t index;
    LXP_CASE_CHECK(initialize() == 0);
    LXP_CASE_CHECK(lx_web_committed_read == NULL);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_PROGRAMS,
        1U, 0U, 1U, 1000000U, &arena, false) == LXP_OK);
    ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    payload[0] = 1U;
    payload[33] = 1U;
    payload[35] = (uint8_t)(sizeof(entrypoint) - 1U);
    for (index = 0U; index < LX_PROGRAMS_CALL_BUDGET_FIELDS; ++index)
        payload[50U + index * 8U + 7U] = 1U;
    memcpy(payload + 50U + LX_PROGRAMS_CALL_BUDGET_FIELDS * 8U,
           entrypoint, sizeof(entrypoint) - 1U);
    LXP_CASE_CHECK(lxp_programs_call_decode(&ctx, payload, sizeof(payload),
                                           &decoded) == LXP_OK);
    LXP_CASE_CHECK(decoded != NULL);
    token = (uint64_t)(uintptr_t)decoded;
    LXP_CASE_CHECK(layerx_programs_call_web_view_begin(
        token, 1U, 2U, 3U, 4U, 1U) == LXP_ERR_MODULE_DISABLED);
    LXP_CASE_CHECK(layerx_programs_call_web_view_begin(
        token, UINT64_MAX, UINT64_MAX, UINT64_MAX, UINT64_MAX,
        UINT64_MAX) == LXP_ERR_MODULE_DISABLED);
    for (index = 0U; index < 3U; ++index)
        LXP_CASE_CHECK(layerx_programs_call_web_view_byte(
            token, (uint16_t)index, 0U) == LXP_ERR_MODULE_DISABLED);
    LXP_CASE_CHECK(ctx.staged_count == 0U);
    LXP_CASE_CHECK(layerx_programs_call_web_view_begin(
        0U, 0U, 0U, 0U, 0U, 0U) == LXP_ERR_NON_CANONICAL);
    lxp_module_ctx_rollback(&ctx);
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

static int bridge_receipt_bind_absent(void)
{
    const lxp_module_registration *registration = NULL;
    LXP_CASE_CHECK(initialize() == 0);
    LXP_CASE_CHECK(lxp_bridge_credit_bind_receipt == NULL);
    LXP_CASE_CHECK(lxp_kernel_module_by_id(&kernel, LXP_MODULE_BRIDGE,
        kernel.epoch, &registration) == LXP_ERR_MODULE_DISABLED);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&kernel,
        ((uint32_t)LXP_MODULE_BRIDGE << 16U) | 1U, kernel.epoch,
        &registration) == LXP_ERR_MODULE_DISABLED);
    LXP_CASE_CHECK(lxp_kernel_module_by_id(&kernel, LXP_MODULE_WEB,
        kernel.epoch, &registration) == LXP_ERR_MODULE_DISABLED);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&kernel,
        ((uint32_t)LXP_MODULE_WEB << 16U) | 1U, kernel.epoch,
        &registration) == LXP_ERR_MODULE_DISABLED);
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"web_and_bridge_rows_absent", web_and_bridge_rows_absent},
        {"program_web_read_disabled", program_web_read_disabled},
        {"bridge_receipt_bind_absent", bridge_receipt_bind_absent}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
