#include "lxp_kernel_case.h"

#include "layerx/lxp_genesis.h"
#include "layerx/lxp_kernel.h"
#include "layerx/programs.h"

#include "../../src/modules/programs/storage.h"

static int fee_relay_type_refused(void)
{
    const lxp_module_iface *interfaces[] = {
        programs_module_registration(),
        programs_module_registration_v2(),
        programs_module_registration_v3(),
        programs_module_registration_v4(),
        programs_module_registration_v4_storage_retirement(),
        programs_module_registration_v5()
    };
    const size_t expected_counts[] = {5U, 7U, 8U, 9U, 10U, 10U};
    static lxp_kernel kernel;
    static lxp_state_store state;
    static lxp_state_journal journal;
    uint8_t arena_bytes[4096];
    const uint8_t payload[32] = {1U};
    uint32_t parameters = 1U;
    size_t i;

    LXP_CASE_CHECK(LX_PROGRAMS_FEE_GOVERNANCE == UINT32_C(0x00090008));
    for (i = 0U; i < sizeof(interfaces) / sizeof(interfaces[0]); ++i) {
        const lxp_module_iface *iface = interfaces[i];
        const lxp_module_registration *resolved = NULL;
        lxp_module_ctx ctx;
        lxp_arena arena;
        void *decoded = NULL;
        size_t j;

        LXP_CASE_CHECK(iface != NULL);
        LXP_CASE_CHECK(iface->activity_type_count == expected_counts[i]);
        LXP_CASE_CHECK(lxp_state_store_init(&state, 0U) == LXP_OK);
        LXP_CASE_CHECK(lxp_kernel_create(&kernel, &state, &journal,
                                        &parameters, 0U) == LXP_OK);
        LXP_CASE_CHECK(lxp_kernel_register_module(&kernel, iface) == LXP_OK);
        for (j = 0U; j < iface->activity_type_count; ++j) {
            LXP_CASE_CHECK(iface->activity_types[j] !=
                           LX_PROGRAMS_FEE_GOVERNANCE);
            LXP_CASE_CHECK(lxp_kernel_module_for_activity(
                &kernel, iface->activity_types[j], 0U, &resolved) == LXP_OK);
            LXP_CASE_CHECK(resolved->iface == iface);
        }
        LXP_CASE_CHECK(lxp_kernel_module_for_activity(
            &kernel, LX_PROGRAMS_FEE_GOVERNANCE, 0U, &resolved) ==
            LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes,
                                      sizeof(arena_bytes)) == LXP_OK);
        LXP_CASE_CHECK(lxp_module_ctx_init(
            &ctx, &kernel, LXP_MODULE_PROGRAMS, 0U, 0U, 0U,
            UINT64_MAX, &arena, true) == LXP_OK);
        LXP_CASE_CHECK(iface->decode(
            &ctx, lxp_activity_type_ordinal(LX_PROGRAMS_FEE_GOVERNANCE),
            payload, sizeof(payload), &decoded) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(decoded == NULL && ctx.staged_count == 0U);
        LXP_CASE_CHECK(kernel.module_kv_count == 0U);
        LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    }
    return 0;
}

static int fee_observe_batch_unchanged(void)
{
    static uint8_t arena_bytes[8U * 1024U * 1024U];
    static lxp_kernel kernel;
    static lxp_state_store state;
    static lxp_state_journal journal;
    static lxp_genesis_manifest manifest;
    static lx_account_registry accounts;
    const lx_programs_fee_schedule initial = {
        1U, 2U, 3U, 5U, 7U, 11U, 13U, 100U
    };
    const uint64_t expected_prices[] = {2U, 3U, 5U, 7U, 11U, 13U, 100U};
    lxp_transfer_asset_state asset = {{0x41U}, true, false};
    lx_programs_transfer_runtime runtime;
    lx_programs_fee_genesis_parameters genesis;
    lx_programs_fee_schedule selected;
    lxp_programs_occupancy_receipt receipt;
    lxp_programs_occupancy_receipt decoded;
    lxp_programs_storage_cell cell;
    lxp_module_ctx ctx;
    lxp_arena arena;
    lxp_byte_span encoded;
    uint8_t selected_asset[32];
    uint8_t namespace_bytes[65] = {0x21U};
    const uint8_t cell_key[] = {0x31U};
    const uint8_t cell_value[] = {0x41U, 0x42U};
    uint32_t parameters = 1U;
    size_t i;

    (void)memset(&runtime, 0, sizeof(runtime));
    (void)memset(&genesis, 0, sizeof(genesis));
    (void)memset(&manifest, 0, sizeof(manifest));
    manifest.signer_public_key[0] = 1U;
    genesis.schedule = initial;
    (void)memcpy(genesis.occupancy_asset_id, asset.asset_id, 32U);
    genesis.target_occupancy_byte_batches = 2U;
    genesis.response_denominator = 1U;
    genesis.maximum_change_numerator = 1U;
    genesis.maximum_change_denominator = 10U;
    genesis.minimum_fee_units_per_occupancy_byte_batch = 10U;
    genesis.maximum_fee_units_per_occupancy_byte_batch = 1000U;
    runtime.accounts = &accounts;
    runtime.assets = &asset;
    runtime.asset_count = 1U;
    runtime.fee_schedule = initial;
    (void)memcpy(runtime.occupancy_asset_id, asset.asset_id, 32U);
    runtime.resolve_occupancy_parameters =
        lxp_programs_fee_governance_resolve_runtime;
    runtime.occupancy_parameter_context = &kernel;

    LXP_CASE_CHECK(lx_account_registry_init(&accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &state, &journal,
                                    &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(
        &kernel, programs_module_registration_v5()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(
        &kernel, LXP_MODULE_PROGRAMS, &runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(&manifest, &genesis) ==
                   LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_materialize(&manifest, &kernel) ==
                   LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes,
                                  sizeof(arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(
        &ctx, &kernel, LXP_MODULE_PROGRAMS, 0U, 0U, 0U,
        UINT64_MAX, &arena, true) == LXP_OK);
    namespace_bytes[33] = 0x51U;
    cell = (lxp_programs_storage_cell){
        cell_key, (uint16_t)sizeof(cell_key), cell_value,
        (uint32_t)sizeof(cell_value)
    };
    LXP_CASE_CHECK(lxp_programs_storage_stage_final(
        &ctx, namespace_bytes, (uint16_t)sizeof(namespace_bytes), &cell, 1U) ==
        LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&kernel, kernel.current_state_root) == LXP_OK);

    LXP_CASE_CHECK(lxp_programs_finalize_occupancy_batch_selected(
        &kernel, LXP_PROTOCOL_VERSION_OCCUPANCY, 1U, 1U, 1000U, 0U,
        parameters, &arena, &receipt, &encoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_occupancy_receipt_decode(
        encoded.bytes, encoded.length, &decoded) == LXP_OK);
    LXP_CASE_CHECK(decoded.batch_number == 1U && decoded.global_sequence == 0U);
    LXP_CASE_CHECK(decoded.parameter_version == parameters);
    LXP_CASE_CHECK(decoded.schedule_version == 1U);
    LXP_CASE_CHECK(decoded.byte_batches.hi == 0U && decoded.byte_batches.lo == 3U);
    LXP_CASE_CHECK(lxp_u128_is_zero(decoded.fee_units));
    LXP_CASE_CHECK(memcmp(decoded.occupancy_asset_id, asset.asset_id, 32U) == 0);
    for (i = 0U; i < sizeof(expected_prices) / sizeof(expected_prices[0]); ++i)
        LXP_CASE_CHECK(decoded.schedule_prices[i] == expected_prices[i]);
    LXP_CASE_CHECK(lxp_programs_fee_governance_resolve_runtime(
        &kernel, 0U, &selected, selected_asset) == LXP_OK);
    LXP_CASE_CHECK(selected.version == 2U && selected.occupancy_byte_batch == 110U);
    LXP_CASE_CHECK(selected.cpu == initial.cpu &&
                   selected.memory_byte == initial.memory_byte &&
                   selected.storage_read_byte == initial.storage_read_byte &&
                   selected.storage_write_byte == initial.storage_write_byte &&
                   selected.output_value == initial.output_value &&
                   selected.output_byte == initial.output_byte);
    LXP_CASE_CHECK(memcmp(selected_asset, asset.asset_id, 32U) == 0);
    LXP_CASE_CHECK(lxp_programs_fee_governance_resolve_runtime(
        &kernel, 1U, &selected, selected_asset) == LXP_OK);
    LXP_CASE_CHECK(selected.version == 1U && selected.occupancy_byte_batch == 100U);
    LXP_CASE_CHECK(lxp_module_ctx_init(
        &ctx, &kernel, LXP_MODULE_PROGRAMS, 1000U, 0U, 1U,
        UINT64_MAX, &arena, true) == LXP_OK);
    ctx.batch_number = 1U;
    LXP_CASE_CHECK(lxp_programs_fee_governance_observe_batch(&ctx, &decoded) ==
                   LXP_ERR_CONTEXT_MISMATCH);
    LXP_CASE_CHECK(ctx.staged_count == 0U);
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "fee_relay_type_refused", fee_relay_type_refused },
        { "fee_observe_batch_unchanged", fee_observe_batch_unchanged }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
