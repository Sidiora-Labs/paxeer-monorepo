#include "lxp_kernel_case.h"

#include "layerx/lx_asset.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/programs.h"

#include <openssl/evp.h>
#include <stdlib.h>

typedef struct signer_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lx_account_registry accounts;
    lx_asset_runtime runtime;
    lx_programs_transfer_runtime programs_runtime;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lxp_arena arena;
    uint8_t arena_bytes[4U * 1024U * 1024U];
    uint64_t parameters;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[512];
    lxp_activity activity;
    lxp_kernel_execution execution;
    lxp_authority_resolved authority;
    lxp_fee_params fees;
    lxp_receipt receipt;
} signer_fixture;

static const uint8_t signer_seed[32] = {1U};
static const uint8_t signer_did[] = "did:key:alice";

static int signer_sign(const uint8_t digest[32], uint8_t signature[64],
                       uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, signer_seed, 32U);
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    size_t key_length = 32U;
    size_t signature_length = 64U;
    int ok = key != NULL && ctx != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &key_length) == 1 &&
        key_length == 32U && EVP_DigestSignInit(ctx, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(ctx, signature, &signature_length, digest, 32U) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static int signer_fixture_prepare(signer_fixture *f)
{
    const uint16_t version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    static const uint8_t from_name[] = "agent:did:key:alice:main";
    static const uint8_t to_name[] = "agent:did:key:bob:main";
    uint8_t digest[32] = {0};
    uint8_t material[144];
    uint8_t message[512];
    size_t message_length;
    size_t payload_length;
    lx_account *from;
    lx_account *to;
    lxp_identity *identity;
    lxp_send send;
    (void)memset(&send, 0, sizeof(send));
    LXP_CASE_CHECK(signer_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes, sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                              &f->parameters, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
                                        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_register(&f->identities, signer_did, sizeof(signer_did) - 1U,
                                  f->public_key, &identity) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(from_name, sizeof(from_name) - 1U, send.from) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(to_name, sizeof(to_name) - 1U, send.to) == LXP_OK);
    send.asset[0] = 3U;
    send.amount.lo = 1U;
    send.expires_at = 100U;
    send.idempotency_key[0] = 7U;
    send.authorization.kind = LXP_AUTH_OWNER;
    send.authorization.network_id = 7U;
    send.authorization.protocol_version = version;
    (void)memcpy(send.authorization.controller, send.from, 32U);
    (void)memcpy(material, send.from, 32U);
    (void)memcpy(material + 32U, send.to, 32U);
    (void)memcpy(material + 64U, send.asset, 32U);
    LXP_CASE_CHECK(lxp_u128_to_be(send.amount, material + 96U) == LXP_OK);
    (void)memcpy(material + 112U, send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_context_value(material, sizeof(material), send.context_hash) == LXP_OK);
    (void)memcpy(send.authorization.signed_context_hash, send.context_hash, 32U);
    (void)memcpy(send.authorization.public_key, f->public_key, 32U);
    LXP_CASE_CHECK(lxp_send_authorization_message(&send, message, sizeof(message), &message_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message, message_length, digest) == LXP_OK);
    LXP_CASE_CHECK(signer_sign(digest, send.authorization.signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_send_encode(&send, f->payload, sizeof(f->payload), &payload_length) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, from_name, sizeof(from_name) - 1U,
                            send.from, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &from) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, to_name, sizeof(to_name) - 1U,
                            send.to, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &to) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(from, send.asset, (lxp_u128){0U, 10U}, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(to, send.asset, (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    from->has_authority_key = true;
    (void)memcpy(from->authority_key, f->public_key, 32U);
    (void)memcpy(f->asset.asset_id, send.asset, 32U);
    (void)memcpy(f->transfer_asset.asset_id, send.asset, 32U);
    f->transfer_asset.registered = true;
    f->runtime = (lx_asset_runtime){&f->accounts, &f->asset, 1U,
        &f->transfer_asset, 1U, 7U, version, false};
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET, &f->runtime) == LXP_OK);
    f->activity.protocol_version = version;
    f->activity.network_id = 7U;
    f->activity.activity_type = LX_ASSET_SEND;
    f->activity.actor_did = (lxp_byte_span){signer_did, sizeof(signer_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    f->activity.payload = (lxp_byte_span){f->payload, payload_length};
    (void)memcpy(f->activity.idempotency_key, send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, payload_length, f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    LXP_CASE_CHECK(signer_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(f->authority.principal, send.from, 32U);
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    (void)memcpy(f->authority.actor, identity->did_id, 32U);
    f->fees.version = 1U;
    f->fees.multiplier_basis_points = 10000U;
    f->execution.network_id = 7U;
    f->execution.epoch = f->kernel.epoch;
    f->execution.batch_number = 1U;
    f->execution.batch_timestamp_ms = 10U;
    f->execution.maximum_timestamp_window = 100U;
    f->execution.global_sequence = 1U;
    f->execution.recorded_module_version = 1U;
    f->execution.parameter_version = 1U;
    f->execution.signature_valid = true;
    f->execution.identities = &f->identities;
    f->execution.authority = &f->authority;
    f->execution.fee_parameters = &f->fees;
    f->execution.gas_limit = 10000U;
    f->execution.arena = &f->arena;
    f->execution.sequencer_private_key = signer_seed;
    f->execution.batch_id[0] = 5U;
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    return 0;
}

static int signer_activity_sign(signer_fixture *f)
{
    uint8_t digest[32];
    LXP_CASE_CHECK(lxp_hash_payload(f->activity.payload.bytes,
        f->activity.payload.length, f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    LXP_CASE_CHECK(signer_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    return 0;
}

static int signer_fixture_open(signer_fixture *f)
{
    static const uint8_t treasury_name[] = "system:fees";
    uint8_t treasury_id[32];
    lx_account *treasury;
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    lx_programs_fee_genesis_parameters fees = {0};
    lx_programs_metering_schedule metering = {0};
    LXP_CASE_CHECK(manifest != NULL);
    LXP_CASE_CHECK(signer_fixture_prepare(f) == 0);
    f->programs_runtime.accounts = &f->accounts;
    f->programs_runtime.assets = &f->transfer_asset;
    f->programs_runtime.asset_count = 1U;
    f->programs_runtime.fee_schedule =
        (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U};
    (void)memcpy(f->programs_runtime.occupancy_asset_id,
                 f->asset.asset_id, 32U);
    f->programs_runtime.resolve_occupancy_parameters =
        lxp_programs_fee_governance_resolve_runtime;
    f->programs_runtime.occupancy_parameter_context = &f->kernel;
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
        programs_module_registration()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_PROGRAMS, &f->programs_runtime) == LXP_OK);
    (void)memcpy(manifest->signer_public_key, f->public_key, 32U);
    fees.schedule = f->programs_runtime.fee_schedule;
    (void)memcpy(fees.occupancy_asset_id, f->asset.asset_id, 32U);
    fees.target_occupancy_byte_batches = 3U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 1U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 10U;
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(manifest, &fees) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_materialize(manifest,
        &f->kernel) == LXP_OK);
    metering.version = 1U;
    metering.activation_batch = 1U;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    for (size_t i = 0U; i < 9U; ++i)
        metering.coefficients[i] = i < 5U ? 1U : (i == 7U ? 64U : 8U);
    LXP_CASE_CHECK(lxp_hash_payload(f->public_key, 32U,
        metering.authority_digest) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_append(manifest,
        &metering) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_materialize(manifest,
        &f->kernel) == LXP_OK);
    f->programs_runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;
    f->programs_runtime.metering_schedule_context = &f->kernel;
    f->execution.recorded_metering_schedule_version = 1U;
    (void)memcpy(f->authority.principal, f->authority.actor, 32U);
    f->activity.activity_type = LX_PROGRAMS_CALL;
    f->activity.payload = (lxp_byte_span){f->payload, 1U};
    free(manifest);
    LXP_CASE_CHECK(lx_account_id_from_string(treasury_name,
        sizeof(treasury_name) - 1U, treasury_id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, treasury_name,
        sizeof(treasury_name) - 1U, treasury_id, 1U,
        LX_ACCOUNT_OPEN_GENESIS, NULL, &treasury) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(treasury, f->asset.asset_id,
        (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_bind_fee_transaction(&f->kernel) == LXP_OK);
    f->fees.base_fee = (lxp_u128){0U, 3U};
    f->fees.per_encoded_byte.lo = 1U;
    f->fees.per_execution_unit.lo = 1U;
    f->fees.per_storage_unit.lo = 1U;
    f->activity.fee_limit = (lxp_u128){0U, 10U};
    f->execution.fee_balance = (lxp_u128){0U, 10U};
    f->execution.recorded_fee_schedule_version = 1U;
    LXP_CASE_CHECK(signer_activity_sign(f) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel,
                                  f->kernel.current_state_root) == LXP_OK);
    return 0;
}

static int signer_fixture_close(signer_fixture *f)
{
    while (f->kernel.blob_count != 0U)
        free(f->kernel.blobs[--f->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int signer_state_unchanged(signer_fixture *f,
                                  const uint8_t expected_root[32])
{
    uint8_t root[32];
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root, expected_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(f->kernel.current_state_root,
                         expected_root, 32U) == 0);
    LXP_CASE_CHECK(f->state.next_sequence == 1U);
    LXP_CASE_CHECK(f->state.idempotency_count == 0U);
    LXP_CASE_CHECK(!f->journal.open && f->journal.count == 0U);
    LXP_CASE_CHECK(f->identities.count == 1U);
    LXP_CASE_CHECK(f->identities.identities[0].next_sequence == 0U);
    for (size_t i = 0U; i < f->accounts.count; ++i)
        LXP_CASE_CHECK(f->accounts.accounts[i].next_sequence == 0U);
    return 0;
}

static int signer_prepare_and_apply(bool unsigned_activity)
{
    signer_fixture *f = calloc(1U, sizeof(*f));
    lxp_kernel_batch_snapshot *snapshot = NULL;
    lxp_prepared_transition *prepared = NULL;
    lxp_kernel_batch_boundary before, after;
    uint8_t root[32];
    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(signer_fixture_open(f) == 0);
    if (unsigned_activity) {
        f->activity.signature = (lxp_byte_span){NULL, 0U};
        f->execution.binding_authority = true;
    }
    (void)memcpy(root, f->kernel.current_state_root, 32U);
    LXP_CASE_CHECK(lxp_kernel_batch_snapshot_create(&f->kernel, &f->identities,
        NULL, &f->execution, &snapshot) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_batch_snapshot_begin_level(snapshot) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_batch_snapshot_boundary(snapshot, &before) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_prepare_activity(snapshot, &f->activity,
        &f->execution, &f->arena, &prepared) == LXP_OK);
    LXP_CASE_CHECK(prepared != NULL);
    LXP_CASE_CHECK(signer_state_unchanged(f, root) == 0);
    if (unsigned_activity) {
        f->execution.binding_authority = false;
        LXP_CASE_CHECK(lxp_kernel_snapshot_apply_prepared(snapshot, &f->activity,
            &f->execution, prepared, &f->receipt, NULL) == LXP_ERR_CONTEXT_MISMATCH);
        LXP_CASE_CHECK(signer_state_unchanged(f, root) == 0);
        LXP_CASE_CHECK(lxp_kernel_batch_snapshot_boundary(snapshot, &after) == LXP_OK);
        LXP_CASE_CHECK(before.next_sequence == after.next_sequence);
        LXP_CASE_CHECK(memcmp(before.receipt_state_root,
            after.receipt_state_root, 32U) == 0);
        LXP_CASE_CHECK(memcmp(before.canonical_state_root,
            after.canonical_state_root, 32U) == 0);
        f->execution.binding_authority = true;
    }
    LXP_CASE_CHECK(lxp_kernel_snapshot_apply_prepared(snapshot, &f->activity,
        &f->execution, prepared, &f->receipt, NULL) == LXP_OK);
    LXP_CASE_CHECK(f->receipt.result_code == LXP_ERR_TRUNCATED);
    LXP_CASE_CHECK(signer_state_unchanged(f, root) == 0);
    lxp_prepared_transition_destroy(prepared);
    lxp_kernel_batch_snapshot_destroy(snapshot);
    LXP_CASE_CHECK(signer_fixture_close(f) == 0);
    return 0;
}

static int signer_execute(bool unsigned_activity, bool binding_authority,
                          lxp_result *status, lxp_result *result_code)
{
    signer_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(signer_fixture_open(f) == 0);
    if (unsigned_activity)
        f->activity.signature = (lxp_byte_span){NULL, 0U};
    f->execution.binding_authority = binding_authority;
    *status = lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt);
    *result_code = f->receipt.result_code;
    LXP_CASE_CHECK(signer_fixture_close(f) == 0);
    return 0;
}

static int bound_unsigned_program_passes_signer_binding(void)
{
    lxp_result signed_status, signed_result, unsigned_status, unsigned_result;
    LXP_CASE_CHECK(signer_prepare_and_apply(false) == 0);
    LXP_CASE_CHECK(signer_prepare_and_apply(true) == 0);
    LXP_CASE_CHECK(signer_execute(false, false, &signed_status, &signed_result) == 0);
    LXP_CASE_CHECK(signer_execute(true, true, &unsigned_status, &unsigned_result) == 0);
    LXP_CASE_CHECK(signed_status == LXP_OK);
    LXP_CASE_CHECK(signed_result == LXP_ERR_TRUNCATED);
    LXP_CASE_CHECK(unsigned_status == signed_status);
    LXP_CASE_CHECK(unsigned_result == signed_result);
    LXP_CASE_CHECK(unsigned_status != LXP_ERR_BAD_SIGNATURE);
    LXP_CASE_CHECK(unsigned_result != LXP_ERR_BAD_SIGNATURE);
    (void)puts("bound_unsigned_program_passes_signer_binding passed");
    return 0;
}

static int unbound_unsigned_program_refused(void)
{
    for (size_t variant = 0U; variant < 3U; ++variant) {
        signer_fixture *f = calloc(1U, sizeof(*f));
        lxp_kernel_batch_snapshot *snapshot = NULL;
        lxp_prepared_transition *prepared = NULL;
        lxp_kernel_batch_boundary before, after;
        lxp_result expected = variant == 2U ? LXP_ERR_AUTH_SCOPE : LXP_ERR_BAD_SIGNATURE;
        uint8_t root[32];
        LXP_CASE_CHECK(f != NULL);
        LXP_CASE_CHECK(signer_fixture_open(f) == 0);
        f->activity.signature = (lxp_byte_span){NULL, 0U};
        f->execution.binding_authority = variant != 0U;
        f->execution.signature_valid = variant != 1U;
        if (variant == 2U) f->authority.verified_key[0] ^= 1U;
        (void)memcpy(root, f->kernel.current_state_root, 32U);
        LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
            &f->execution, &f->receipt) == expected);
        LXP_CASE_CHECK(signer_state_unchanged(f, root) == 0);
        LXP_CASE_CHECK(lxp_kernel_batch_snapshot_create(&f->kernel, &f->identities,
            NULL, &f->execution, &snapshot) == LXP_OK);
        LXP_CASE_CHECK(lxp_kernel_batch_snapshot_begin_level(snapshot) == LXP_OK);
        LXP_CASE_CHECK(lxp_kernel_batch_snapshot_boundary(snapshot, &before) == LXP_OK);
        LXP_CASE_CHECK(lxp_kernel_prepare_activity(snapshot, &f->activity,
            &f->execution, &f->arena, &prepared) == expected);
        LXP_CASE_CHECK(prepared == NULL);
        LXP_CASE_CHECK(signer_state_unchanged(f, root) == 0);
        LXP_CASE_CHECK(lxp_kernel_batch_snapshot_boundary(snapshot, &after) == LXP_OK);
        LXP_CASE_CHECK(before.next_sequence == after.next_sequence);
        LXP_CASE_CHECK(memcmp(before.receipt_state_root,
            after.receipt_state_root, 32U) == 0);
        LXP_CASE_CHECK(memcmp(before.canonical_state_root,
            after.canonical_state_root, 32U) == 0);
        lxp_kernel_batch_snapshot_destroy(snapshot);
        LXP_CASE_CHECK(signer_fixture_close(f) == 0);
    }
    (void)puts("unbound_unsigned_program_refused passed");
    return 0;
}

static int bound_signed_program_still_verified(void)
{
    lxp_result bound_status, bound_result, unbound_status, unbound_result;
    for (size_t variant = 0U; variant < 2U; ++variant) {
        signer_fixture *f = calloc(1U, sizeof(*f));
        uint8_t root[32];
        LXP_CASE_CHECK(f != NULL);
        LXP_CASE_CHECK(signer_fixture_open(f) == 0);
        f->execution.binding_authority = variant != 0U;
        f->signature[0] ^= 1U;
        (void)memcpy(root, f->kernel.current_state_root, 32U);
        LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
            &f->execution, &f->receipt) == LXP_ERR_BAD_SIGNATURE);
        LXP_CASE_CHECK(signer_state_unchanged(f, root) == 0);
        LXP_CASE_CHECK(signer_fixture_close(f) == 0);
    }
    LXP_CASE_CHECK(signer_execute(false, true, &bound_status, &bound_result) == 0);
    LXP_CASE_CHECK(signer_execute(false, false, &unbound_status, &unbound_result) == 0);
    LXP_CASE_CHECK(unbound_status == LXP_OK);
    LXP_CASE_CHECK(unbound_result == LXP_ERR_TRUNCATED);
    LXP_CASE_CHECK(bound_status == unbound_status);
    LXP_CASE_CHECK(bound_result == unbound_result);
    (void)puts("bound_signed_program_still_verified passed");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"bound_unsigned_program_passes_signer_binding",
         bound_unsigned_program_passes_signer_binding},
        {"unbound_unsigned_program_refused", unbound_unsigned_program_refused},
        {"bound_signed_program_still_verified",
         bound_signed_program_still_verified}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
