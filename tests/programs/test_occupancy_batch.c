#include "layerx/programs.h"

#include "layerx/lxp_crypto.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_kernel.h"

#include "../../src/modules/programs/storage.h"

#include <string.h>

static const uint8_t fee_active_key[] = "progfee/active/v1";
static const uint8_t fee_history_prefix[] = "progfee/history/v1/";

static lxp_result occupancy_parameters(
    void *context, uint32_t version, lx_programs_fee_schedule *schedule,
    uint8_t asset_id[32])
{
    return lxp_programs_fee_governance_resolve_runtime(
        context, version, schedule, asset_id);
}

static lxp_result seed_fee_governance(
    lxp_kernel *kernel, const lx_programs_transfer_runtime *runtime)
{
    lxp_genesis_manifest manifest;
    lx_programs_fee_genesis_parameters parameters;
    size_t index;
    lxp_result status;
    if (kernel == NULL || runtime == NULL) return LXP_ERR_NON_CANONICAL;
    (void)memset(&manifest, 0, sizeof(manifest));
    manifest.signer_public_key[0] = 1U;
    (void)memset(&parameters, 0, sizeof(parameters));
    parameters.schedule = runtime->fee_schedule;
    (void)memcpy(parameters.occupancy_asset_id,
                 runtime->occupancy_asset_id, 32U);
    parameters.target_occupancy_byte_batches = 3U;
    parameters.response_denominator = 1U;
    parameters.maximum_change_numerator = 1U;
    parameters.maximum_change_denominator = 1U;
    parameters.minimum_fee_units_per_occupancy_byte_batch = 1U;
    parameters.maximum_fee_units_per_occupancy_byte_batch = 10U;
    status = lxp_programs_fee_genesis_append(&manifest, &parameters);
    if (status != LXP_OK) return status;
    if (kernel->module_kv_count > LXP_KERNEL_MAX_MODULE_KV -
                                      manifest.module_value_count)
        return LXP_ERR_LENGTH_LIMIT;
    for (index = 0U; index < manifest.module_value_count; ++index) {
        const lxp_genesis_module_value *value = &manifest.module_values[index];
        lxp_module_kv_entry *entry =
            &kernel->module_kv[kernel->module_kv_count++];
        size_t key_length;
        if (memcmp(value->key, fee_active_key,
                   sizeof(fee_active_key) - 1U) == 0)
            key_length = sizeof(fee_active_key) - 1U;
        else if (memcmp(value->key, fee_history_prefix,
                        sizeof(fee_history_prefix) - 1U) == 0)
            key_length = sizeof(fee_history_prefix) - 1U + 4U;
        else
            return LXP_FATAL_INVARIANT;
        (void)memset(entry, 0, sizeof(*entry));
        entry->module_id = value->module_id;
        entry->key_length = (uint16_t)key_length;
        entry->value_length = (uint32_t)value->value_length;
        (void)memcpy(entry->key, value->key, key_length);
        (void)memcpy(entry->value, value->value, value->value_length);
    }
    return LXP_OK;
}

static lxp_result seed_principal_storage(lxp_kernel *kernel, lxp_arena *arena)
{
    lxp_module_ctx ctx;
    lxp_programs_storage_cell cell;
    uint8_t namespace_bytes[65] = {0x21U};
    uint8_t cell_key[1] = {0x31U};
    uint8_t cell_value[2] = {0x41U, 0x42U};
    uint8_t root[32];
    lxp_result status;
    namespace_bytes[32] = 0U;
    namespace_bytes[33] = 0x51U;
    cell = (lxp_programs_storage_cell){
        cell_key, (uint16_t)sizeof(cell_key), cell_value,
        (uint32_t)sizeof(cell_value)
    };
    status = lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_PROGRAMS,
                                 0U, 0U, 0U, UINT64_MAX, arena, true);
    if (status == LXP_OK)
        status = lxp_programs_storage_stage_final(
            &ctx, namespace_bytes, (uint16_t)sizeof(namespace_bytes),
            &cell, 1U);
    if (status == LXP_OK) status = lxp_module_ctx_commit(&ctx);
    if (status == LXP_OK) status = lxp_state_root(kernel, root);
    if (status == LXP_OK)
        (void)memcpy(kernel->current_state_root, root, sizeof(root));
    return status;
}

int main(void)
{
    static uint8_t arena_bytes[8U * 1024U * 1024U];
    lxp_arena arena;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_programs_occupancy_receipt first;
    lxp_programs_occupancy_receipt second;
    lxp_byte_span first_encoded;
    lxp_byte_span second_encoded;
    lx_account_registry accounts;
    lxp_transfer_asset_state asset = {{0x41U}, true, false};
    lx_programs_transfer_runtime runtime;
    lxp_programs_occupancy_receipt decoded;
    lx_programs_fee_schedule selected;
    uint8_t selected_asset[32];
    uint32_t parameters = 1U;
    (void)memset(&runtime, 0, sizeof(runtime));
    runtime.accounts = &accounts;
    runtime.assets = &asset;
    runtime.asset_count = 1U;
    runtime.fee_schedule = (lx_programs_fee_schedule){
        1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U
    };
    (void)memcpy(runtime.occupancy_asset_id, asset.asset_id, 32U);
    runtime.resolve_occupancy_parameters = occupancy_parameters;
    runtime.occupancy_parameter_context = &kernel;
    if (lx_account_registry_init(&accounts) != LXP_OK ||
        lxp_state_store_init(&state, 0U) != LXP_OK ||
        lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(&kernel, programs_module_registration()) !=
            LXP_OK ||
        lxp_kernel_bind_module_runtime(&kernel, LXP_MODULE_PROGRAMS,
                                       &runtime) != LXP_OK ||
        seed_fee_governance(&kernel, &runtime) != LXP_OK ||
        lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK ||
        seed_principal_storage(&kernel, &arena) != LXP_OK)
        return 1;
    if (lxp_programs_finalize_occupancy_batch(
            &kernel, 1U, 1000U, 0U, parameters, &arena, &first,
            &first_encoded) != LXP_OK ||
        first_encoded.length == 0U ||
        memcmp(first.resulting_state_root, kernel.current_state_root, 32U) != 0 ||
        state.next_sequence != 1U || !state.account_root_required ||
        lxp_programs_occupancy_receipt_decode(
            first_encoded.bytes, first_encoded.length, &decoded) != LXP_OK ||
        decoded.batch_number != 1U || decoded.global_sequence != 0U ||
        decoded.parameter_version != 1U || decoded.schedule_version != 1U ||
        decoded.byte_batches.hi != 0U || decoded.byte_batches.lo != 3U ||
        !lxp_u128_is_zero(decoded.fee_units) ||
        lxp_ct_is_zero(decoded.settlement_evidence_digest, 32U) ||
        lxp_ct_is_zero(decoded.resulting_state_root, 32U) ||
        lxp_programs_fee_governance_resolve_runtime(
            &kernel, 0U, &selected, selected_asset) != LXP_OK ||
        selected.version != 1U || selected.occupancy_byte_batch != 1U ||
        memcmp(selected_asset, asset.asset_id, 32U) != 0)
        return 1;
    if (lxp_programs_finalize_occupancy_batch(
            &kernel, 1U, 1000U, 0U, parameters, &arena, &second,
            &second_encoded) != LXP_ERR_IDEMPOTENT_REPLAY)
        return 1;
    parameters = 2U;
    if (lxp_programs_finalize_occupancy_batch(
            &kernel, 2U, 2000U, 1U, parameters, &arena, &second,
            &second_encoded) != LXP_OK ||
        lxp_programs_occupancy_receipt_decode(
            second_encoded.bytes, second_encoded.length, &decoded) != LXP_OK ||
        decoded.global_sequence != 1U ||
        decoded.schedule_version != 1U || state.next_sequence != 2U ||
        decoded.schedule_prices[6] != 1U || decoded.byte_batches.hi != 0U ||
        decoded.byte_batches.lo != 3U || !lxp_u128_is_zero(decoded.fee_units) ||
        memcmp(second.resulting_state_root,
               first.resulting_state_root, 32U) == 0 ||
        lxp_programs_fee_governance_resolve_runtime(
            &kernel, 0U, &selected, selected_asset) != LXP_OK ||
        selected.version != 1U || selected.occupancy_byte_batch != 1U)
        return 1;
    if (lxp_programs_finalize_occupancy_batch(
            &kernel, 2U, 2000U, 1U, parameters, &arena, &second,
            &second_encoded) != LXP_ERR_IDEMPOTENT_REPLAY)
        return 1;
    parameters = 4U;
    if (lxp_programs_finalize_occupancy_batch(
            &kernel, 3U, 3000U, 2U, parameters, &arena, &second,
            &second_encoded) != LXP_OK ||
        state.next_sequence != 3U ||
        lxp_programs_occupancy_receipt_decode(
            second_encoded.bytes, second_encoded.length, &decoded) != LXP_OK ||
        decoded.parameter_version != 4U || decoded.schedule_version != 1U ||
        lxp_programs_fee_governance_resolve_runtime(
            &kernel, 4U, &selected, selected_asset) !=
            LXP_ERR_VERSION_UNSUPPORTED)
        return 1;
    return 0;
}
