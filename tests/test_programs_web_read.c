#include "layerx/programs.h"
#include "layerx/lxp_kernel.h"

#include "lxp_test_harness.c"

#include <stdint.h>
#include <stdio.h>
#include <string.h>

#define WEB_READ_CHECK(condition) do { if (!(condition)) { \
    (void)fprintf(stderr, "programs web read check failed at line %d\n", \
                  __LINE__); \
    return 1; } } while (0)

/* Historical web_read stays disabled even under the current registration.
 * Native xweb answers belong to the keeper, never to this legacy guest ABI. */
static int web_read_case(const lxp_module_iface *registration,
                          uint16_t guest_abi)
{
    static const uint8_t entrypoint[] = "layerx_call";
    static const uint64_t requests[] = {
        UINT64_C(0x0102030405060708), UINT64_C(0x0102030405060709),
        UINT64_MAX
    };
    enum {
        CALL_FIXED_BYTES = 32 + 2 + 2 + 4 + 2 + 4 + 4 +
                           LX_PROGRAMS_CALL_BUDGET_FIELDS * 8
    };
    static uint8_t arena_bytes[LXP_MAX_ACTIVITY_BYTES + 4096U];
    static lxp_kernel kernel;
    static lxp_state_store state;
    static lxp_state_journal journal;
    uint8_t payload[CALL_FIXED_BYTES + sizeof(entrypoint) - 1U] = {0};
    uint64_t parameters = 1U;
    uint64_t token;
    lxp_arena arena;
    lxp_module_ctx ctx;
    void *decoded = NULL;
    size_t index;
    uint16_t section;
    payload[0] = 0x71U;
    payload[33] = (uint8_t)guest_abi;
    payload[35] = sizeof(entrypoint) - 1U;
    (void)memcpy(payload + CALL_FIXED_BYTES, entrypoint,
                 sizeof(entrypoint) - 1U);
    WEB_READ_CHECK(lxp_state_store_init(&state, 1U) == LXP_OK);
    WEB_READ_CHECK(lxp_kernel_create(&kernel, &state, &journal, &parameters,
                                     0U) == LXP_OK);
    WEB_READ_CHECK(lxp_kernel_register_module(&kernel,
        registration) == LXP_OK);
    WEB_READ_CHECK(lxp_arena_init(&arena, arena_bytes,
                                  sizeof(arena_bytes)) == LXP_OK);
    WEB_READ_CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_PROGRAMS,
        10U, 0U, state.next_sequence, 100000U, &arena, false) == LXP_OK);
    ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    WEB_READ_CHECK(lxp_programs_call_decode(&ctx, payload, sizeof(payload),
                                            &decoded) == LXP_OK);
    WEB_READ_CHECK(decoded != NULL);
    token = (uint64_t)(uintptr_t)decoded;
    for (index = 0U; index < sizeof(requests) / sizeof(requests[0]); ++index) {
        WEB_READ_CHECK(layerx_programs_call_web_view_begin(token,
            0x71U, 0U, 0U, 0U, requests[index]) == LXP_ERR_MODULE_DISABLED);
        WEB_READ_CHECK(layerx_programs_call_web_view_begin(token,
            0x73U, 0U, 0U, 0U, requests[index]) == LXP_ERR_MODULE_DISABLED);
    }
    for (section = 0U; section < 4U; ++section) {
        WEB_READ_CHECK(layerx_programs_call_web_view_byte(token, section,
            0U) == LXP_ERR_MODULE_DISABLED);
        WEB_READ_CHECK(layerx_programs_call_web_view_byte(token, section,
            UINT32_MAX) == LXP_ERR_MODULE_DISABLED);
    }
    WEB_READ_CHECK(layerx_programs_call_web_view_begin(0U,
        0U, 0U, 0U, 0U, requests[0]) == LXP_ERR_NON_CANONICAL);
    WEB_READ_CHECK(kernel.module_kv_count == 0U && state.next_sequence == 1U &&
                   state.count == 0U);
    WEB_READ_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

static int historical_web_read_refusal(void)
{
    return web_read_case(programs_module_registration_v4(),
                          LX_PROGRAMS_GUEST_ABI_V4_VERSION);
}

static int current_registration_web_read_refusal(void)
{
    return web_read_case(programs_module_registration_v5(),
                          LX_PROGRAMS_GUEST_ABI_V5_VERSION);
}

int main(int argc, char **argv)
{
    if (lxp_test_register("historical_web_read_refusal",
            historical_web_read_refusal) != 0 ||
        lxp_test_register("current_registration_web_read_refusal",
            current_registration_web_read_refusal) != 0)
        return 1;
    return lxp_test_main(argc, argv);
}
