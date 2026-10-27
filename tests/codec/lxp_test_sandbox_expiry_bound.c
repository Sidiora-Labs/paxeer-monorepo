#include "lxp_kernel_case.h"

#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "../../src/modules/programs/sandbox.h"

typedef struct expiry_bound_fixture {
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx ctx;
    lxp_arena arena;
    uint8_t arena_bytes[4096];
    lxp_effect_buffer effects;
} expiry_bound_fixture;

static int expiry_bound_init(expiry_bound_fixture *fixture)
{
    static const uint64_t parameters = 1U;

    (void)memset(fixture, 0, sizeof(*fixture));
    LXP_CASE_CHECK(lxp_state_store_init(&fixture->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&fixture->kernel, &fixture->state,
                    &fixture->journal, &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&fixture->kernel,
                    programs_module_registration()) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&fixture->arena, fixture->arena_bytes,
                    sizeof(fixture->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&fixture->ctx, &fixture->kernel,
                    LXP_MODULE_PROGRAMS, 1U, 0U, 17U, UINT64_MAX,
                    &fixture->arena, true) == LXP_OK);
    fixture->ctx.batch_number = 7U;
    fixture->ctx.activity_id[0] = 0xa5U;
    LXP_CASE_CHECK(lxp_effect_buffer_init(&fixture->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&fixture->ctx,
                    &fixture->effects) == LXP_OK);
    return 0;
}

static void expiry_bound_u64(uint8_t *bytes, uint64_t value)
{
    size_t i;

    for (i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> (56U - 8U * i));
}

static int expiry_bound_cursor(expiry_bound_fixture *fixture, uint64_t boundary)
{
    static const uint8_t key[] = "ex/head/v1";
    uint8_t expected[96] = {0};
    uint8_t binding[20] = {0};
    const uint8_t *cursor;
    size_t length;

    (void)memcpy(expected, "LXEX2", 5U);
    expiry_bound_u64(expected + 5U, boundary);
    expiry_bound_u64(expected + 13U, fixture->ctx.global_sequence);
    expiry_bound_u64(binding, boundary);
    expiry_bound_u64(binding + 8U, fixture->ctx.global_sequence);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_CONTEXT_HASH, binding,
                    sizeof(binding), expected + 57U) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&fixture->ctx, key, sizeof(key) - 1U,
                    &cursor, &length) == LXP_OK);
    LXP_CASE_CHECK(length == sizeof(expected));
    LXP_CASE_CHECK(memcmp(cursor, expected, sizeof(expected)) == 0);
    return 0;
}

static int sweep_limit_128_accepted(void)
{
    static expiry_bound_fixture fixture;
    const uint8_t value[] = {0x51U, 0x28U};
    const uint8_t *found;
    size_t length;
    size_t i;

    LXP_CASE_CHECK(expiry_bound_init(&fixture) == 0);
    LXP_CASE_CHECK(LXP_MODULE_MAX_STAGED_WRITES == 512);
    for (i = 0U; i < 511U; ++i) {
        uint8_t key[] = {0xf1U, (uint8_t)(i >> 8U), (uint8_t)i};

        LXP_CASE_CHECK(lxp_ctx_kv_put(&fixture.ctx, key, sizeof(key),
                        value, sizeof(value)) == LXP_OK);
    }
    LXP_CASE_CHECK(fixture.ctx.staged_count == 511U);
    LXP_CASE_CHECK(layerx_programs_sandbox_sweep_host(
                    (uint64_t)(uintptr_t)&fixture.ctx, 7U, 128U) == LXP_OK);
    LXP_CASE_CHECK(fixture.ctx.staged_count == 512U);
    LXP_CASE_CHECK(fixture.ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(fixture.kernel.module_kv_count == 0U);
    LXP_CASE_CHECK(expiry_bound_cursor(&fixture, 7U) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&fixture.ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture.kernel.module_kv_count == 512U);
    LXP_CASE_CHECK(fixture.ctx.staged_count == 0U);
    for (i = 0U; i < 511U; ++i) {
        uint8_t key[] = {0xf1U, (uint8_t)(i >> 8U), (uint8_t)i};

        LXP_CASE_CHECK(lxp_ctx_kv_get(&fixture.ctx, key, sizeof(key),
                        &found, &length) == LXP_OK);
        LXP_CASE_CHECK(length == sizeof(value));
        LXP_CASE_CHECK(memcmp(found, value, sizeof(value)) == 0);
    }
    LXP_CASE_CHECK(lxp_programs_sandbox_finalize_expiry_batch(
                    &fixture.ctx, 8U) == LXP_OK);
    LXP_CASE_CHECK(fixture.ctx.staged_count == 1U);
    LXP_CASE_CHECK(expiry_bound_cursor(&fixture, 8U) == 0);
    lxp_module_ctx_rollback(&fixture.ctx);
    LXP_CASE_CHECK(lxp_state_store_destroy(&fixture.state) == LXP_OK);
    return 0;
}

static int sweep_limit_above_bound_refused(void)
{
    static expiry_bound_fixture fixture;
    static expiry_bound_fixture before;
    static const uint32_t limits[] = {0U, 129U};
    const uint8_t key[] = {0xf1U};
    const uint8_t value[] = {0x73U};
    size_t i;

    LXP_CASE_CHECK(expiry_bound_init(&fixture) == 0);
    LXP_CASE_CHECK(lxp_programs_sandbox_finalize_expiry_batch(
                    &fixture.ctx, 7U) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&fixture.ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_kv_put(&fixture.ctx, key, sizeof(key),
                    value, sizeof(value)) == LXP_OK);
    (void)memcpy(&before, &fixture, sizeof(before));
    for (i = 0U; i < sizeof(limits) / sizeof(limits[0]); ++i) {
        LXP_CASE_CHECK(layerx_programs_sandbox_sweep_host(
                        (uint64_t)(uintptr_t)&fixture.ctx, 8U,
                        limits[i]) == LXP_ERR_NON_CANONICAL);
        LXP_CASE_CHECK(memcmp(&fixture, &before, sizeof(fixture)) == 0);
    }
    lxp_module_ctx_rollback(&fixture.ctx);
    LXP_CASE_CHECK(lxp_state_store_destroy(&fixture.state) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "sweep_limit_128_accepted", sweep_limit_128_accepted },
        { "sweep_limit_above_bound_refused", sweep_limit_above_bound_refused }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
