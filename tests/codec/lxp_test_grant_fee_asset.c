#include "lxp_kernel_case.h"

#include "layerx/lxp_authority.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_governance.h"
#include "layerx/lxp_module_ctx.h"
#include "layerx/programs.h"

typedef struct fee_asset_fixture {
    lxp_kernel kernel;
    lxp_state_store store;
    lxp_state_journal journal;
    lxp_fee_params parameters;
    lx_account_registry accounts;
    lx_programs_transfer_runtime runtime;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_arena arena;
    uint8_t arena_bytes[2U * LXP_MAX_ACTIVITY_BYTES];
    lxp_authority_resolved authority;
    uint8_t asset_id[32];
} fee_asset_fixture;

static const uint8_t owner_did[] = "did:layerx:fee-asset-owner";

static void put_be(uint8_t *out, uint64_t value, size_t width)
{
    for (size_t i = 0U; i < width; ++i)
        out[width - 1U - i] = (uint8_t)(value >> (8U * i));
}

static int fee_asset_open(fee_asset_fixture *f)
{
    static lxp_genesis_manifest manifest;
    static const uint8_t policy_key[32] = LXP_NATIVE_FEE_AUTHORITY_PARAMETER;
    lx_programs_fee_genesis_parameters fees = {0};
    lx_programs_fee_schedule selected;
    uint8_t selected_asset[32];
    lxp_module_kv_entry *entry;

    (void)memset(f, 0, sizeof(*f));
    (void)memset(&manifest, 0, sizeof(manifest));
    f->parameters.version = 1U;
    f->parameters.multiplier_basis_points = 10000U;
    LXP_CASE_CHECK(lxp_state_store_init(&f->store, 2U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->store, &f->journal,
        &f->parameters, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
        lxp_governance_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
        programs_module_registration()) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    f->runtime.resolve_occupancy_parameters =
        lxp_programs_fee_governance_resolve_runtime;
    f->runtime.occupancy_parameter_context = &f->kernel;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_PROGRAMS, &f->runtime) == LXP_OK);
    f->asset_id[0] = 0x41U;
    f->asset_id[31] = 0x42U;
    manifest.signer_public_key[0] = 1U;
    fees.schedule = (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U};
    (void)memcpy(fees.occupancy_asset_id, f->asset_id, 32U);
    fees.target_occupancy_byte_batches = 3U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 1U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 10U;
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(&manifest, &fees) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_materialize(&manifest,
        &f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_governance_resolve_runtime(&f->kernel,
        0U, &selected, selected_asset) == LXP_OK);
    LXP_CASE_CHECK(selected.version == 1U &&
        memcmp(selected_asset, f->asset_id, 32U) == 0);
    LXP_CASE_CHECK(lxp_ct_is_zero(f->runtime.occupancy_asset_id, 32U));

    entry = &f->kernel.module_kv[f->kernel.module_kv_count++];
    entry->module_id = LXP_MODULE_GOVERNANCE;
    entry->key_length = 32U;
    entry->value_length = 32U;
    (void)memcpy(entry->key, policy_key, 32U);
    entry->value[31] = 2U;

    f->authority.kind = LXP_AUTHORITY_OWNER;
    for (size_t i = 0U; i < 32U; ++i)
        f->authority.verified_key[i] = (uint8_t)(0x21U + i);
    LXP_CASE_CHECK(lxp_did_id_derive(owner_did, sizeof(owner_did) - 1U,
        f->authority.actor) == LXP_OK);
    entry = &f->kernel.module_kv[f->kernel.module_kv_count++];
    entry->module_id = LXP_MODULE_GOVERNANCE;
    entry->key_length = 32U;
    entry->value_length = 223U;
    (void)memcpy(entry->key, f->authority.actor, 32U);
    (void)memcpy(entry->value, "LXGI1", 5U);
    (void)memcpy(entry->value + 5U, f->authority.actor, 32U);
    (void)memcpy(entry->value + 37U, f->authority.verified_key, 32U);
    put_be(entry->value + 69U, 1U, 8U);
    put_be(entry->value + 215U, 1U, 8U);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
        sizeof(f->arena_bytes)) == LXP_OK);
    return 0;
}

static lxp_result fee_asset_context(fee_asset_fixture *f, uint64_t sequence)
{
    lxp_result status = lxp_arena_reset(&f->arena, 0U);
    if (status == LXP_OK)
        status = lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_GOVERNANCE,
            1100U, 1U, sequence, 100000U, &f->arena, true);
    if (status == LXP_OK) status = lxp_effect_buffer_init(&f->effects);
    if (status == LXP_OK)
        status = lxp_module_ctx_bind_effects(&f->ctx, &f->effects);
    return status;
}

static lxp_result fee_asset_execute(fee_asset_fixture *f, uint16_t ordinal,
    const uint8_t *payload, size_t length, uint64_t sequence)
{
    const lxp_module_iface *module = lxp_governance_module_iface();
    lxp_activity activity = {0};
    void *decoded = NULL;
    lxp_result status = fee_asset_context(f, sequence);
    activity.protocol_version = 3U;
    activity.network_id = 77U;
    activity.activity_type = ((uint32_t)LXP_MODULE_GOVERNANCE << 16U) | ordinal;
    activity.actor_did = (lxp_byte_span){owner_did, sizeof(owner_did) - 1U};
    activity.authority = (lxp_byte_span){f->authority.verified_key, 32U};
    activity.payload = (lxp_byte_span){payload, length};
    if (status == LXP_OK)
        status = module->decode(&f->ctx, ordinal, payload, length, &decoded);
    if (status == LXP_OK)
        status = module->validate(&f->ctx, &activity, &f->authority, decoded);
    if (status == LXP_OK)
        status = module->execute(&f->ctx, &activity, &f->authority, decoded,
            &f->effects);
    if (status == LXP_OK) status = lxp_module_ctx_prepare_commit(&f->ctx);
    if (status == LXP_OK) status = lxp_module_ctx_commit(&f->ctx);
    return status;
}

static int fee_asset_grant(fee_asset_fixture *f, lxp_authority_grant *grant)
{
    uint8_t session_key[32];
    for (size_t i = 0U; i < 32U; ++i) session_key[i] = (uint8_t)(0x41U + i);
    LXP_CASE_CHECK(lxp_session_key_bind(grant, f->authority.actor, session_key,
        UINT64_C(1) << LXP_MODULE_PROGRAMS, 3U, 3U, 10U, 5000U, 1U) == LXP_OK);
    grant->fee_budget.present = true;
    (void)memcpy(grant->fee_budget.asset_id, f->asset_id, 32U);
    grant->fee_budget.maximum_per_activity = (lxp_u128){0U, 4U};
    grant->fee_budget.maximum_total = (lxp_u128){0U, 20U};
    grant->fee_budget.maximum_per_period = (lxp_u128){0U, 8U};
    grant->fee_budget.period_length = 1000U;
    grant->fee_budget.period_start = grant->not_before;
    return 0;
}

static int fee_asset_issue(fee_asset_fixture *f, lxp_authority_grant *grant,
    const uint8_t predecessor[32], const uint8_t commitment[32],
    uint64_t sequence, lxp_result expected)
{
    uint8_t storage[2048], payload[1024] = {0x71U, 5U, 1U, 3U};
    uint8_t action_key[32] = {1U};
    lxp_arena arena;
    lxp_byte_span body;
    size_t length;
    LXP_CASE_CHECK(lxp_grant_id_compute(grant, grant->grant_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, storage, sizeof(storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_grant_encode(grant, &arena, &body) == LXP_OK);
    length = 52U + body.length + (predecessor == NULL ? 0U : 72U);
    LXP_CASE_CHECK(length <= sizeof(payload));
    put_be(payload + 4U, body.length, 4U);
    (void)memcpy(payload + 8U, body.bytes, body.length);
    put_be(payload + 8U + body.length, 100U, 8U);
    put_be(payload + 16U + body.length, 32U, 4U);
    (void)memcpy(payload + 20U + body.length, action_key, 32U);
    if (predecessor != NULL) {
        payload[2] = 2U;
        payload[3] = 5U;
        put_be(payload + 52U + body.length, 32U, 4U);
        (void)memcpy(payload + 56U + body.length, predecessor, 32U);
        put_be(payload + 88U + body.length, 32U, 4U);
        (void)memcpy(payload + 92U + body.length, commitment, 32U);
    }
    LXP_CASE_CHECK(fee_asset_execute(f, 5U, payload, length, sequence) == expected);
    LXP_CASE_CHECK(lxp_ct_is_zero(f->runtime.occupancy_asset_id, 32U));
    return 0;
}

static int grant_fee_asset_resolves_governed_record(void)
{
    static fee_asset_fixture f;
    lxp_authority_grant grant, loaded, successor;
    uint8_t key[33], counters[72], commitment[32], wrong[32], summary[209];
    uint8_t revoke[45] = {0x71U, 6U, 0U, 3U};
    const uint8_t *stored;
    size_t length;

    LXP_CASE_CHECK(fee_asset_open(&f) == 0);
    LXP_CASE_CHECK(fee_asset_grant(&f, &grant) == 0);
    LXP_CASE_CHECK(fee_asset_issue(&f, &grant, NULL, NULL, 2U, LXP_OK) == 0);
    LXP_CASE_CHECK(lxp_authority_grant_load(&f.kernel, grant.grant_id, &loaded) == LXP_OK);
    LXP_CASE_CHECK(loaded.fee_budget.present &&
        memcmp(loaded.fee_budget.asset_id, f.asset_id, 32U) == 0);
    LXP_CASE_CHECK(lxp_governance_session_key_read(&f.kernel,
        grant.grant_id, summary) == LXP_OK);
    LXP_CASE_CHECK(memcmp(summary + 133U, grant.key, 32U) == 0);

    LXP_CASE_CHECK(lxp_authority_fee_charge(&loaded.fee_budget,
        (lxp_u128){0U, 2U}, 1000U) == LXP_OK);
    LXP_CASE_CHECK(lxp_authority_fee_charge(&loaded.fee_budget,
        (lxp_u128){0U, 3U}, 1050U) == LXP_OK);
    LXP_CASE_CHECK(fee_asset_context(&f, 3U) == LXP_OK);
    lxp_authority_fee_record_key(grant.grant_id, key);
    LXP_CASE_CHECK(lxp_authority_fee_record_encode(&loaded, counters) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_kv_put(&f.ctx, key, sizeof(key), counters,
        sizeof(counters)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_prepare_commit(&f.ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f.ctx) == LXP_OK);
    (void)memcpy(revoke + 4U, grant.grant_id, 32U);
    revoke[36] = 1U;
    put_be(revoke + 37U, 4U, 8U);
    LXP_CASE_CHECK(fee_asset_execute(&f, 6U, revoke, sizeof(revoke), 4U) == LXP_OK);
    LXP_CASE_CHECK(lxp_authority_grant_load(&f.kernel, grant.grant_id, &loaded) == LXP_OK);
    LXP_CASE_CHECK(loaded.revoked && loaded.revoked_at_sequence == 4U);
    LXP_CASE_CHECK(lxp_authority_session_charge_commitment(&loaded, commitment) == LXP_OK);
    successor = grant;
    successor.key[0] = 0x61U;
    successor.grantor_revocation_sequence = 4U;
    (void)memcpy(wrong, commitment, 32U);
    wrong[0] ^= 1U;
    LXP_CASE_CHECK(fee_asset_issue(&f, &successor, grant.grant_id, wrong,
        5U, LXP_ERR_AUTH_SCOPE) == 0);
    LXP_CASE_CHECK(f.ctx.staged_count == 0U);
    LXP_CASE_CHECK(lxp_authority_grant_load(&f.kernel, successor.grant_id,
        &loaded) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(fee_asset_issue(&f, &successor, grant.grant_id, commitment,
        5U, LXP_OK) == 0);
    LXP_CASE_CHECK(lxp_authority_grant_load(&f.kernel, successor.grant_id, &loaded) == LXP_OK);
    LXP_CASE_CHECK(!loaded.revoked && loaded.fee_budget.spent_total.hi == 0U &&
        loaded.fee_budget.spent_total.lo == 5U &&
        loaded.fee_budget.spent_this_period.hi == 0U &&
        loaded.fee_budget.spent_this_period.lo == 3U &&
        loaded.fee_budget.period_start == 1010U);
    lxp_authority_session_successor_key(grant.grant_id, key);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&f.ctx, key, sizeof(key), &stored, &length) == LXP_OK);
    LXP_CASE_CHECK(length == 32U && memcmp(stored, successor.grant_id, 32U) == 0);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f.store) == LXP_OK);
    return 0;
}

static int grant_fee_asset_mismatch_refused(void)
{
    static fee_asset_fixture f;
    lxp_authority_grant grant, loaded;
    size_t count;

    LXP_CASE_CHECK(fee_asset_open(&f) == 0);
    LXP_CASE_CHECK(fee_asset_grant(&f, &grant) == 0);
    count = f.kernel.module_kv_count;
    grant.fee_budget.asset_id[31] ^= 1U;
    LXP_CASE_CHECK(fee_asset_issue(&f, &grant, NULL, NULL,
        2U, LXP_ERR_AUTH_SCOPE) == 0);
    LXP_CASE_CHECK(f.ctx.staged_count == 0U && f.effects.count == 0U);
    LXP_CASE_CHECK(f.kernel.module_kv_count == count);
    LXP_CASE_CHECK(lxp_authority_grant_load(&f.kernel, grant.grant_id,
        &loaded) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f.store) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "grant_fee_asset_resolves_governed_record", grant_fee_asset_resolves_governed_record },
        { "grant_fee_asset_mismatch_refused", grant_fee_asset_mismatch_refused }
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
