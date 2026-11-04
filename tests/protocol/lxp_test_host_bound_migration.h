#ifndef LXP_TEST_HOST_BOUND_MIGRATION_H
#define LXP_TEST_HOST_BOUND_MIGRATION_H

#include "layerx/lxp_crypto.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/programs.h"

#include <openssl/evp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MIGRATION_CHECK(condition) \
    do { \
        if (!(condition)) { \
            fprintf(stderr, "%s:%d: %s\n", __FILE__, __LINE__, #condition); \
            return 1; \
        } \
    } while (0)

typedef struct migration_binding_fixture {
    lxp_kernel kernel;
    lxp_state_store store;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lx_account_registry accounts;
    lx_programs_transfer_runtime runtime;
    lxp_transfer_asset_state asset;
    lxp_module_ctx ctx;
    lxp_arena arena;
    uint8_t arena_bytes[4U * LXP_MAX_ACTIVITY_BYTES];
    lxp_arena wire_arena;
    uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
    size_t wire_length;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[115];
    lxp_activity activity;
    lxp_authority_resolved authority;
    lxp_kernel_execution execution;
    lxp_fee_params fees;
    uint64_t parameters;
} migration_binding_fixture;

static const uint8_t migration_binding_seed[32] = {1U};
static const uint8_t migration_binding_did[] = "did:key:migration";

static int migration_binding_sign(migration_binding_fixture *f)
{
    uint8_t digest[32];
    EVP_PKEY *key;
    EVP_MD_CTX *context;
    size_t key_length = 32U, signature_length = 64U;
    int ok;
    key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                      migration_binding_seed, 32U);
    context = EVP_MD_CTX_new();
    ok = key != NULL && context != NULL &&
        EVP_PKEY_get_raw_public_key(key, f->public_key, &key_length) == 1 &&
        key_length == 32U &&
        lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK &&
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(context, f->signature, &signature_length,
                       digest, sizeof(digest)) == 1 && signature_length == 64U;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    MIGRATION_CHECK(ok);
    return 0;
}

static int migration_binding_context(migration_binding_fixture *f)
{
    MIGRATION_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel,
        LXP_MODULE_PROGRAMS, f->execution.batch_timestamp_ms, f->execution.epoch,
        f->execution.global_sequence, f->execution.gas_limit, &f->arena,
        false) == LXP_OK);
    f->ctx.protocol_version = f->activity.protocol_version;
    f->ctx.batch_number = f->execution.batch_number;
    f->ctx.identities = &f->identities;
    return 0;
}

static int migration_binding_open(migration_binding_fixture *f,
                                   bool native_signed)
{
    static const uint8_t wasm[] = {0U, 97U, 115U, 109U, 1U, 0U, 0U, 0U};
    static const char *names[] = {"agent:did:key:migration:main", "system:fees"};
    lx_programs_fee_genesis_parameters fees = {0};
    lx_programs_metering_schedule metering = {0};
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    lxp_identity *identity;
    lxp_byte_span encoded;
    uint8_t id[32];
    MIGRATION_CHECK(manifest != NULL);
    f->parameters = 1U;
    f->asset.asset_id[0] = 3U;
    f->asset.registered = true;
    MIGRATION_CHECK(lxp_state_store_init(&f->store, 1U) == LXP_OK);
    MIGRATION_CHECK(lxp_kernel_create(&f->kernel, &f->store, &f->journal,
        &f->parameters, 0U) == LXP_OK);
    MIGRATION_CHECK(lxp_kernel_register_module(&f->kernel,
        programs_module_registration()) == LXP_OK);
    MIGRATION_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
        sizeof(f->arena_bytes)) == LXP_OK);
    MIGRATION_CHECK(lxp_arena_init(&f->wire_arena, f->wire_bytes,
        sizeof(f->wire_bytes)) == LXP_OK);
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->activity.network_id = 7U;
    f->activity.activity_type = LX_PROGRAMS_UPGRADE;
    f->activity.actor_did = (lxp_byte_span){migration_binding_did,
        sizeof(migration_binding_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    f->activity.idempotency_key[0] = 9U;
    f->activity.fee_limit = (lxp_u128){0U, 100U};
    f->payload[0] = 1U;
    f->payload[33] = 1U;
    f->payload[34] = 1U;
    f->payload[36] = 2U;
    MIGRATION_CHECK(lxp_hash_sha256(wasm, sizeof(wasm), f->payload + 68U) == LXP_OK);
    f->payload[101] = 1U;
    f->payload[105] = (uint8_t)sizeof(wasm);
    f->payload[106] = 0U;
    (void)memcpy(f->payload + 107U, wasm, sizeof(wasm));
    f->activity.payload = (lxp_byte_span){f->payload, sizeof(f->payload)};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    MIGRATION_CHECK(lxp_hash_payload(f->payload, sizeof(f->payload),
        f->activity.payload_hash) == LXP_OK);
    MIGRATION_CHECK(migration_binding_sign(f) == 0);
    MIGRATION_CHECK(lxp_identity_register(&f->identities,
        migration_binding_did, sizeof(migration_binding_did) - 1U,
        f->public_key, &identity) == LXP_OK);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(f->authority.actor, identity->did_id, 32U);
    (void)memcpy(f->authority.principal, identity->did_id, 32U);
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    MIGRATION_CHECK(lxp_hash_payload(f->public_key, 32U,
        f->authority.authority_hash) == LXP_OK);
    MIGRATION_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    for (size_t i = 0U; i < 2U; ++i) {
        lx_account *account;
        MIGRATION_CHECK(lx_account_id_from_string((const uint8_t *)names[i],
            strlen(names[i]), id) == LXP_OK);
        MIGRATION_CHECK(lx_account_open(&f->accounts, (const uint8_t *)names[i],
            strlen(names[i]), id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL,
            &account) == LXP_OK);
        MIGRATION_CHECK(lxp_ledger_bootstrap_balance(account, f->asset.asset_id,
            (lxp_u128){0U, i == 0U ? 1000U : 0U}, 0U) == LXP_OK);
    }
    f->runtime.accounts = &f->accounts;
    f->runtime.assets = &f->asset;
    f->runtime.asset_count = 1U;
    f->runtime.fee_schedule =
        (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U};
    (void)memcpy(f->runtime.occupancy_asset_id, f->asset.asset_id, 32U);
    f->runtime.resolve_occupancy_parameters = lxp_programs_fee_governance_resolve_runtime;
    f->runtime.occupancy_parameter_context = &f->kernel;
    f->runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;
    f->runtime.metering_schedule_context = &f->kernel;
    MIGRATION_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_PROGRAMS, &f->runtime) == LXP_OK);
    MIGRATION_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    MIGRATION_CHECK(lxp_programs_bind_fee_transaction(&f->kernel) == LXP_OK);
    (void)memcpy(manifest->signer_public_key, f->public_key, 32U);
    fees.schedule = f->runtime.fee_schedule;
    (void)memcpy(fees.occupancy_asset_id, f->asset.asset_id, 32U);
    fees.target_occupancy_byte_batches = 3U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 1U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 10U;
    MIGRATION_CHECK(lxp_programs_fee_genesis_append(manifest, &fees) == LXP_OK);
    MIGRATION_CHECK(lxp_programs_fee_genesis_materialize(manifest,
        &f->kernel) == LXP_OK);
    metering.version = 1U;
    metering.activation_batch = 1U;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    for (size_t i = 0U; i < 9U; ++i)
        metering.coefficients[i] = i < 5U ? 1U : (i == 7U ? 64U : 8U);
    MIGRATION_CHECK(lxp_hash_payload(f->public_key, 32U,
        metering.authority_digest) == LXP_OK);
    MIGRATION_CHECK(lxp_programs_metering_genesis_append(manifest,
        &metering) == LXP_OK);
    MIGRATION_CHECK(lxp_programs_metering_genesis_materialize(manifest,
        &f->kernel) == LXP_OK);
    free(manifest);
    if (!native_signed) f->activity.signature = (lxp_byte_span){NULL, 0U};
    MIGRATION_CHECK(lxp_activity_encode(&f->activity, &f->wire_arena,
        &encoded) == LXP_OK);
    MIGRATION_CHECK(lxp_activity_decode(encoded.bytes, encoded.length,
        &f->activity) == LXP_OK);
    f->wire_length = encoded.length;
    f->fees.version = 1U;
    f->fees.base_fee = (lxp_u128){0U, 3U};
    f->fees.multiplier_basis_points = 10000U;
    f->execution.network_id = 7U;
    f->execution.batch_number = 1U;
    f->execution.batch_timestamp_ms = 10U;
    f->execution.maximum_timestamp_window = 100U;
    f->execution.global_sequence = 1U;
    f->execution.recorded_module_version = programs_module_registration()->abi_version;
    f->execution.parameter_version = 1U;
    f->execution.recorded_fee_schedule_version = 1U;
    f->execution.recorded_metering_schedule_version = 1U;
    f->execution.signature_valid = true;
    f->execution.binding_authority = !native_signed;
    f->execution.identities = &f->identities;
    f->execution.authority = &f->authority;
    f->execution.fee_parameters = &f->fees;
    f->execution.fee_balance = (lxp_u128){0U, 1000U};
    f->execution.gas_limit = 1000000U;
    f->execution.arena = &f->arena;
    f->execution.sequencer_private_key = migration_binding_seed;
    f->execution.batch_id[0] = 5U;
    MIGRATION_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    return migration_binding_context(f);
}

static int migration_binding_close(migration_binding_fixture *f)
{
    MIGRATION_CHECK(!f->journal.open);
    MIGRATION_CHECK(lxp_state_store_destroy(&f->store) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int host_bound_migration_admission(void)
{
    for (size_t mode = 0U; mode < 2U; ++mode) {
        migration_binding_fixture *f = calloc(1U, sizeof(*f));
        lxp_receipt receipt;
        uint8_t activity_id[32];
        MIGRATION_CHECK(f != NULL);
        MIGRATION_CHECK(migration_binding_open(f, mode == 0U) == 0);
        MIGRATION_CHECK(lxp_activity_id(f->wire_bytes, f->wire_length,
            activity_id) == LXP_OK);
        (void)memcpy(f->ctx.activity_id, activity_id, 32U);
        MIGRATION_CHECK(lxp_kernel_bind_migration_admission(&f->ctx,
            &f->activity, &f->execution) == LXP_OK);
        MIGRATION_CHECK(memcmp(f->ctx.activity_id, activity_id, 32U) == 0);
        MIGRATION_CHECK(f->ctx.migration_admission.module_version ==
            f->execution.recorded_module_version);
        MIGRATION_CHECK(f->ctx.migration_admission.parameter_version == 1U);
        MIGRATION_CHECK(f->ctx.migration_admission.fee_schedule_version == 1U);
        MIGRATION_CHECK(f->ctx.migration_admission.metering_schedule_version == 1U);
        MIGRATION_CHECK(lxp_u128_cmp(f->ctx.migration_admission.validation_fee,
            (lxp_u128){0U, 3U}) == 0);
        MIGRATION_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
            &f->execution, &receipt) == LXP_OK);
        MIGRATION_CHECK(receipt.result_code == LXP_ERR_UNKNOWN_FIELD);
        MIGRATION_CHECK(memcmp(receipt.activity_id, activity_id, 32U) == 0);
        MIGRATION_CHECK(lxp_u128_cmp(receipt.fee_charged, (lxp_u128){0U, 3U}) == 0);
        MIGRATION_CHECK(f->accounts.accounts[0].balance.lo == 997U);
        MIGRATION_CHECK(f->accounts.accounts[1].balance.lo == 3U);
        MIGRATION_CHECK(f->store.next_sequence == 2U);
        MIGRATION_CHECK(f->identities.identities[0].next_sequence == 1U);
        MIGRATION_CHECK(migration_binding_close(f) == 0);
    }
    (void)puts("host_bound_migration_admission passed");
    return 0;
}

static int host_bound_migration_refusals(void)
{
    for (size_t variant = 0U; variant < 17U; ++variant) {
        migration_binding_fixture *f = calloc(1U, sizeof(*f));
        lxp_receipt receipt;
        lxp_result expected = LXP_ERR_AUTH_SCOPE;
        uint8_t before[32], after[32];
        bool execute_refusal = true;
        MIGRATION_CHECK(f != NULL);
        MIGRATION_CHECK(migration_binding_open(f, variant == 6U) == 0);
        (void)memcpy(before, f->kernel.current_state_root, 32U);
        switch (variant) {
        case 0U: f->authority.actor[0] ^= 1U; break;
        case 1U: f->authority.principal[0] ^= 1U; break;
        case 2U: f->authority.verified_key[0] ^= 1U; break;
        case 3U:
            f->activity.payload_hash[0] ^= 1U;
            expected = LXP_ERR_PAYLOAD_HASH_MISMATCH;
            break;
        case 4U:
            f->execution.binding_authority = false;
            expected = LXP_ERR_BAD_SIGNATURE;
            break;
        case 5U:
            f->execution.signature_valid = false;
            expected = LXP_ERR_CONTEXT_MISMATCH;
            execute_refusal = false;
            break;
        case 6U:
            f->execution.binding_authority = true;
            f->activity.signature.bytes = f->signature;
            f->signature[0] ^= 1U;
            expected = LXP_ERR_BAD_SIGNATURE;
            break;
        case 7U:
            f->ctx.activity_id[0] = 1U;
            expected = LXP_ERR_CONTEXT_MISMATCH;
            execute_refusal = false;
            break;
        case 8U:
            ++f->ctx.epoch;
            expected = LXP_ERR_CONTEXT_MISMATCH;
            execute_refusal = false;
            break;
        case 9U:
            ++f->ctx.batch_number;
            expected = LXP_ERR_CONTEXT_MISMATCH;
            execute_refusal = false;
            break;
        case 10U:
            ++f->ctx.global_sequence;
            expected = LXP_ERR_CONTEXT_MISMATCH;
            execute_refusal = false;
            break;
        case 11U:
            f->execution.recorded_fee_schedule_version = 2U;
            expected = LXP_ERR_CONTEXT_MISMATCH;
            execute_refusal = false;
            break;
        case 12U:
            f->execution.recorded_metering_schedule_version = 2U;
            expected = LXP_ERR_CONTEXT_MISMATCH;
            execute_refusal = false;
            break;
        case 13U:
            f->identities.identities[0].primary_key[0] ^= 1U;
            break;
        case 14U: {
            static const uint8_t other_did[] = "did:key:other";
            lxp_identity *other;
            MIGRATION_CHECK(lxp_identity_register(&f->identities, other_did,
                sizeof(other_did) - 1U, f->public_key, &other) == LXP_OK);
            f->activity.actor_did = (lxp_byte_span){other_did,
                sizeof(other_did) - 1U};
            break;
        }
        case 15U:
            f->public_key[0] ^= 1U;
            f->activity.authority.bytes = f->public_key;
            break;
        default:
            f->activity.authority.bytes = NULL;
            break;
        }
        MIGRATION_CHECK(lxp_kernel_bind_migration_admission(&f->ctx,
            &f->activity, &f->execution) == expected);
        MIGRATION_CHECK(!f->ctx.migration_admission.present);
        MIGRATION_CHECK(f->ctx.migration_admission.module_version == 0U);
        if (execute_refusal)
            MIGRATION_CHECK(lxp_kernel_execute_activity(&f->kernel,
                &f->activity, &f->execution, &receipt) == expected);
        MIGRATION_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
        MIGRATION_CHECK(memcmp(before, after, 32U) == 0);
        MIGRATION_CHECK(memcmp(before, f->kernel.current_state_root, 32U) == 0);
        MIGRATION_CHECK(f->store.next_sequence == 1U);
        MIGRATION_CHECK(f->identities.identities[0].next_sequence == 0U);
        MIGRATION_CHECK(f->accounts.accounts[0].balance.lo == 1000U);
        MIGRATION_CHECK(f->accounts.accounts[1].balance.lo == 0U);
        MIGRATION_CHECK(migration_binding_close(f) == 0);
    }
    (void)puts("host_bound_migration_refusals passed");
    return 0;
}

static int host_bound_migration_transport(void)
{
    MIGRATION_CHECK(system("CGO_ENABLED=1 go test -count=1 -run '^TestHostBoundMigration' "
        "./tests/protocol/hostmigration") == 0);
    return 0;
}

#undef MIGRATION_CHECK

#endif
