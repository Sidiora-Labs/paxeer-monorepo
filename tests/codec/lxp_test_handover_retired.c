#include "lxp_kernel_case.h"

#include "layerx/programs.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_governance.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_ledger.h"
#include "layerx/lxp_module_ctx.h"

#include <openssl/evp.h>

static const uint8_t sequencer_seed[32] = {7U};
static const uint8_t governor_seed[32] = {9U};
static const uint8_t governor_did[] = "did:lxp:retired-handover-governor";
static const uint8_t asset_id[32] = {1U};
static lxp_kernel kernel;
static lxp_state_store state;
static lxp_state_journal journal;
static lx_account_registry accounts;
static lxp_genesis_manifest manifest;
static lxp_identity_store identities;
static uint8_t arena_bytes[4U * LXP_GENESIS_MAX_ENCODED_BYTES];
static lxp_arena arena;

static int public_key_for(const uint8_t seed[32], uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, 32U);
    size_t length = 32U;
    int ok = key != NULL && EVP_PKEY_get_raw_public_key(key, public_key, &length) == 1 &&
        length == 32U;
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(ok);
    return 0;
}

static int sign_activity(lxp_activity *activity, const uint8_t seed[32],
                         uint8_t signature[64])
{
    uint8_t digest[32];
    size_t length = 64U;
    EVP_PKEY *key;
    EVP_MD_CTX *context;
    int ok;
    LXP_CASE_CHECK(lxp_activity_signing_preimage(activity, digest) == LXP_OK);
    key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, 32U);
    context = EVP_MD_CTX_new();
    ok = key != NULL && context != NULL &&
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(context, signature, &length, digest, sizeof(digest)) == 1 &&
        length == 64U;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(ok);
    activity->signature = (lxp_byte_span){signature, 64U};
    return 0;
}

static int initialize(bool enabled)
{
    lxp_genesis_module_plan plan;
    lx_programs_metering_schedule metering = {0};
    lx_programs_fee_genesis_parameters fees = {0};
    uint8_t governor_key[32];
    size_t index = 0U;
    memset(&manifest, 0, sizeof(manifest));
    memset(&identities, 0, sizeof(identities));
    LXP_CASE_CHECK(public_key_for(sequencer_seed, manifest.signer_public_key) == 0);
    LXP_CASE_CHECK(public_key_for(governor_seed, governor_key) == 0);
    manifest.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    manifest.network_id = 42U;
    manifest.genesis_timestamp_ms = 1U;
    if (enabled) {
        manifest.parameters[index].module_id = LXP_MODULE_GOVERNANCE;
        memcpy(manifest.parameters[index].key, "handover-authority", 18U);
        memcpy(manifest.parameters[index++].value, governor_key, 32U);
    }
    manifest.parameters[index].module_id = LXP_MODULE_GOVERNANCE;
    memcpy(manifest.parameters[index].key, "parameter-version", 17U);
    manifest.parameters[index++].value[31] = 1U;
    manifest.parameter_count = index;
    manifest.guarantor_count = 1U;
    manifest.guarantors[0].guarantor_id[0] = 1U;
    memset(manifest.guarantors[0].public_key, 1, 33U);
    metering.version = 1U;
    for (index = 0U; index < 5U; ++index) metering.coefficients[index] = 1U;
    metering.coefficients[5] = 8U;
    metering.coefficients[6] = 8U;
    metering.coefficients[7] = 64U;
    metering.coefficients[8] = 8U;
    metering.activation_batch = 1U;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    LXP_CASE_CHECK(lxp_hash_payload(manifest.signer_public_key, 32U,
                                   metering.authority_digest) == LXP_OK);
    fees.schedule = (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 100U};
    memcpy(fees.occupancy_asset_id, asset_id, 32U);
    fees.target_occupancy_byte_batches = 100U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 10U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 1000U;
    LXP_CASE_CHECK(lxp_genesis_fresh_empty_accounts(&manifest, asset_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_append(&manifest, &metering) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(&manifest, &fees) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&state, &accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &state, &journal, &manifest, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_resolve(&manifest, &plan) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_register(&plan, &kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_materialize(&manifest, &arena, &kernel) == LXP_OK);
    LXP_CASE_CHECK(kernel.governance_authority.enabled == enabled);
    LXP_CASE_CHECK(kernel.governance_authority.network_id == manifest.network_id);
    LXP_CASE_CHECK(!kernel.handover.enabled && !kernel.handover.pending);
    if (enabled)
        LXP_CASE_CHECK(memcmp(kernel.governance_authority.governance_public_key,
                              governor_key, 32U) == 0);
    else
        LXP_CASE_CHECK(lxp_ct_is_zero(kernel.governance_authority.governance_public_key, 32U));
    return 0;
}

static int release(void)
{
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    lx_account_registry_release(&accounts);
    return 0;
}

static int handover_activity_refused(void)
{
    const lxp_module_registration *registration = NULL;
    for (unsigned int enabled = 0U; enabled < 2U; ++enabled) {
        LXP_CASE_CHECK(initialize(enabled != 0U) == 0);
        for (unsigned int flag = 0U; flag < 2U; ++flag) {
            const lxp_module_iface *iface = lxp_governance_module_iface_for_handover(flag != 0U);
            LXP_CASE_CHECK(iface == lxp_governance_module_iface());
            LXP_CASE_CHECK(iface->activity_type_count == 8U);
            for (size_t i = 0U; i < iface->activity_type_count; ++i)
                LXP_CASE_CHECK(iface->activity_types[i] != LXP_GOVERNANCE_HANDOVER);
        }
        LXP_CASE_CHECK(!lxp_governance_activity(LXP_GOVERNANCE_HANDOVER));
        LXP_CASE_CHECK(lxp_kernel_module_for_activity(&kernel, LXP_GOVERNANCE_HANDOVER,
            kernel.epoch, &registration) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(lxp_kernel_module_for_activity(&kernel, LXP_GOVERNANCE_PROGRAMS_FEE,
            kernel.epoch, &registration) == LXP_OK);
        LXP_CASE_CHECK(lxp_kernel_module_for_activity(&kernel, LXP_GOVERNANCE_MIGRATION_BUDGET,
            kernel.epoch, &registration) == LXP_OK);
        LXP_CASE_CHECK(release() == 0);
    }
    return 0;
}

static void write64(uint8_t *bytes, uint64_t value)
{
    for (size_t i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> (56U - i * 8U));
}

static int dispatch_proposal(lxp_activity *activity, const lxp_authority_resolved *authority,
                             lxp_result expected)
{
    static lxp_module_ctx ctx;
    static lxp_effect_buffer effects;
    const lxp_module_registration *registration;
    lxp_result result;
    size_t count = kernel.module_kv_count;
    LXP_CASE_CHECK(lxp_arena_reset(&arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&kernel, activity->activity_type,
        kernel.epoch, &registration) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_GOVERNANCE,
        10U, kernel.epoch, 1U, 1000000U, &arena, false) == LXP_OK);
    ctx.protocol_version = activity->protocol_version;
    ctx.batch_number = 1U;
    LXP_CASE_CHECK(lxp_effect_buffer_init(&effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&ctx, &effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_dispatch(registration, &ctx, activity, authority,
        &effects, &result) == LXP_OK);
    LXP_CASE_CHECK(result == expected);
    if (expected == LXP_OK) {
        LXP_CASE_CHECK(effects.count == 1U && ctx.staged_count == 1U);
        LXP_CASE_CHECK(effects.effects[0].body_length == activity->payload.length);
        LXP_CASE_CHECK(memcmp(effects.effects[0].body, activity->payload.bytes,
                              activity->payload.length) == 0);
        LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
        LXP_CASE_CHECK(kernel.module_kv_count == count + 1U);
    } else {
        LXP_CASE_CHECK(effects.count == 0U && ctx.staged_count == 0U);
        lxp_module_ctx_rollback(&ctx);
        LXP_CASE_CHECK(kernel.module_kv_count == count);
    }
    return 0;
}

static int governance_fee_keeps_key(void)
{
    uint8_t fee[149] = {0}, migration[81], signature[64];
    lxp_migration_profile profile = {
        .version = 1U,
        .module_version = 1U,
        .parameter_version = 1U,
        .activation_epoch = 2U,
        .limits = {1000000U, 16777216U, 1048576U, 1048576U, 64U, 1048576U, 4096U}
    };
    lxp_identity *governor, *other;
    lxp_authority_grant grant;
    lxp_authority_resolved authority;
    lxp_governance_authority original;
    static const uint8_t other_did[] = "did:lxp:retired-handover-other";
    const uint32_t types[] = {LXP_GOVERNANCE_PROGRAMS_FEE, LXP_GOVERNANCE_MIGRATION_BUDGET};
    LXP_CASE_CHECK(initialize(true) == 0);
    original = kernel.governance_authority;
    kernel.handover.enabled = true;
    kernel.handover.network_id = 999U;
    memcpy(kernel.handover.governance_public_key, manifest.signer_public_key, 32U);
    LXP_CASE_CHECK(lxp_identity_register(&identities, governor_did, sizeof(governor_did) - 1U,
        original.governance_public_key, &governor) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_register(&identities, other_did, sizeof(other_did) - 1U,
        manifest.signer_public_key, &other) == LXP_OK);
    memcpy(fee, "LXFG1", 5U);
    for (size_t i = 0U; i < 7U; ++i) write64(fee + 5U + i * 8U, 100U);
    memcpy(fee + 61U, asset_id, 32U);
    {
        const uint64_t policy[] = {100U, 1U, 1U, 10U, 1U, 1000U};
        for (size_t i = 0U; i < 6U; ++i) write64(fee + 93U + i * 8U, policy[i]);
    }
    write64(fee + 141U, 9U);
    LXP_CASE_CHECK(lxp_programs_fee_governance_proposal_validate(fee, sizeof(fee)) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_migration_proposal_encode(&profile, migration) == LXP_OK);
    for (size_t i = 0U; i < 2U; ++i) {
        lxp_activity activity = {0};
        activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
        activity.network_id = original.network_id;
        activity.activity_type = types[i];
        activity.actor_did = (lxp_byte_span){governor_did, sizeof(governor_did) - 1U};
        activity.authority = (lxp_byte_span){governor->primary_key, 32U};
        activity.account_sequence = governor->next_sequence;
        activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
        activity.idempotency_key[31] = (uint8_t)(i + 1U);
        activity.fee_limit = (lxp_u128){0U, 1000000U};
        activity.payload = i == 0U ? (lxp_byte_span){fee, sizeof(fee)} :
                                    (lxp_byte_span){migration, sizeof(migration)};
        LXP_CASE_CHECK(lxp_hash_payload(activity.payload.bytes, activity.payload.length,
                                        activity.payload_hash) == LXP_OK);
        LXP_CASE_CHECK(sign_activity(&activity, governor_seed, signature) == 0);
        LXP_CASE_CHECK(lxp_activity_verify_signature(&activity) == LXP_OK);
        LXP_CASE_CHECK(lxp_authority_resolve_activity(&kernel, governor, &activity,
            true, true, 10U, 100U, 1U, &grant, &authority) == LXP_OK);
        kernel.governance_authority.enabled = false;
        LXP_CASE_CHECK(dispatch_proposal(&activity, &authority, LXP_ERR_AUTH_SCOPE) == 0);
        kernel.governance_authority = original;
        activity.network_id++;
        LXP_CASE_CHECK(sign_activity(&activity, governor_seed, signature) == 0);
        LXP_CASE_CHECK(dispatch_proposal(&activity, &authority, LXP_ERR_AUTH_SCOPE) == 0);
        activity.network_id = original.network_id;
        activity.actor_did = (lxp_byte_span){other_did, sizeof(other_did) - 1U};
        activity.authority = (lxp_byte_span){other->primary_key, 32U};
        LXP_CASE_CHECK(sign_activity(&activity, sequencer_seed, signature) == 0);
        LXP_CASE_CHECK(lxp_authority_resolve_activity(&kernel, other, &activity,
            true, true, 10U, 100U, 1U, &grant, &authority) == LXP_OK);
        LXP_CASE_CHECK(dispatch_proposal(&activity, &authority, LXP_ERR_AUTH_SCOPE) == 0);
        activity.actor_did = (lxp_byte_span){governor_did, sizeof(governor_did) - 1U};
        activity.authority = (lxp_byte_span){governor->primary_key, 32U};
        LXP_CASE_CHECK(sign_activity(&activity, governor_seed, signature) == 0);
        LXP_CASE_CHECK(lxp_authority_resolve_activity(&kernel, governor, &activity,
            true, true, 10U, 100U, 1U, &grant, &authority) == LXP_OK);
        signature[0] ^= 1U;
        LXP_CASE_CHECK(dispatch_proposal(&activity, &authority, LXP_ERR_BAD_SIGNATURE) == 0);
        signature[0] ^= 1U;
        LXP_CASE_CHECK(dispatch_proposal(&activity, &authority, LXP_OK) == 0);
        LXP_CASE_CHECK(kernel.governance_authority.enabled);
        LXP_CASE_CHECK(kernel.governance_authority.network_id == original.network_id);
        LXP_CASE_CHECK(memcmp(kernel.governance_authority.governance_public_key,
                              original.governance_public_key, 32U) == 0);
        LXP_CASE_CHECK(kernel.epoch == 1U && !kernel.handover.pending);
    }
    LXP_CASE_CHECK(release() == 0);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"handover_activity_refused", handover_activity_refused},
        {"governance_fee_keeps_key", governance_fee_keeps_key}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
