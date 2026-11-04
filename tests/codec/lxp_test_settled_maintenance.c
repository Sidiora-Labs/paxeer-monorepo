#define _POSIX_C_SOURCE 200809L

#include "lxp_kernel_case.h"

#include "layerx/lx_asset.h"
#include "layerx/lxp_batch_identity.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_fee.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_maintenance.h"
#include "layerx/lxp_storage.h"
#include "layerx/programs.h"

#include <openssl/evp.h>
#include <stdlib.h>
#include <unistd.h>

static const uint8_t settled_sequencer_seed[32] = {0x45U};
static const uint8_t settled_actor_seed[32] = {0x51U};
static const uint8_t settled_did[] = "did:lxp:terminal-rejection";
static const uint8_t settled_actor_name[] = "agent:did:lxp:terminal-rejection:main";
static const uint8_t settled_treasury_name[] = "system:fees";
static const uint8_t settled_recipient_name[] = "agent:did:lxp:terminal-recipient:main";

typedef struct settled_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lxp_identity *identity;
    lx_account_registry accounts;
    lx_account *actor;
    lx_account *treasury;
    lx_account *recipient;
    lxp_transfer_asset_state asset;
    lx_asset_record asset_record;
    lx_asset_runtime asset_runtime;
    lx_programs_transfer_runtime runtime;
    lxp_authority_scope scope;
    lxp_authority_resolved authority;
    lxp_fee_params fees;
    lxp_arena arena;
    uint8_t *storage;
    uint8_t actor_public_key[32];
    uint8_t actor_id[32];
    uint8_t recipient_id[32];
    lxp_log publication;
    lxp_arena publication_arena;
    uint8_t publication_bytes[LXP_BATCH_MAINTENANCE_MAX_BYTES];
    uint8_t published_root[32];
    uint64_t published_sequence;
    size_t published_count;
    char directory[128];
} settled_fixture;

static int settled_public_key(const uint8_t seed[32], uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, 32U);
    size_t length = 32U;
    int ok = key != NULL && EVP_PKEY_get_raw_public_key(key, public_key, &length) == 1 &&
             length == 32U;
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static void write_u32(uint8_t *out, uint32_t value)
{
    out[0] = (uint8_t)(value >> 24U);
    out[1] = (uint8_t)(value >> 16U);
    out[2] = (uint8_t)(value >> 8U);
    out[3] = (uint8_t)value;
}

static void write_u64(uint8_t *out, uint64_t value)
{
    size_t index;
    for (index = 0U; index < 8U; ++index)
        out[index] = (uint8_t)(value >> ((7U - index) * 8U));
}

static lxp_result install_metering_v1(lxp_kernel *kernel)
{
    static const uint8_t active_key[] = "progmet/active/v1";
    static const uint8_t history_key[] = {
        'p', 'r', 'o', 'g', 'm', 'e', 't', '/', 'h', 'i', 's', 't', 'o',
        'r', 'y', '/', 'v', '1', '/', 0U, 0U, 0U, 1U
    };
    static const uint64_t coefficients[9] = {1U, 1U, 1U, 1U, 1U,
                                            8U, 8U, 64U, 8U};
    uint8_t record[LX_PROGRAMS_METERING_RECORD_BYTES] = {0U};
    size_t offset = 0U;
    size_t index;
    if (kernel == NULL || kernel->module_kv_count != 0U)
        return LXP_ERR_NON_CANONICAL;
    (void)memcpy(record + offset, "LXMR1", 5U);
    offset += 5U;
    write_u32(record + offset, 1U);
    offset += 4U;
    for (index = 0U; index < 9U; ++index) {
        write_u64(record + offset, coefficients[index]);
        offset += 8U;
    }
    write_u64(record + offset, 1U);
    offset += 8U;
    record[offset++] = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    (void)memset(record + offset, 0xa5, 32U);
    if (offset + 32U != sizeof(record)) return LXP_FATAL_INVARIANT;
    kernel->module_kv[0].module_id = LXP_MODULE_PROGRAMS;
    kernel->module_kv[0].key_length = sizeof(active_key) - 1U;
    kernel->module_kv[0].value_length = sizeof(record);
    (void)memcpy(kernel->module_kv[0].key, active_key,
                 sizeof(active_key) - 1U);
    (void)memcpy(kernel->module_kv[0].value, record, sizeof(record));
    kernel->module_kv[1].module_id = LXP_MODULE_PROGRAMS;
    kernel->module_kv[1].key_length = sizeof(history_key);
    kernel->module_kv[1].value_length = sizeof(record);
    (void)memcpy(kernel->module_kv[1].key, history_key, sizeof(history_key));
    (void)memcpy(kernel->module_kv[1].value, record, sizeof(record));
    kernel->module_kv_count = 2U;
    return LXP_OK;
}

static int settled_sign(const uint8_t seed[32], const uint8_t *message,
                        size_t length, uint8_t signature[64])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, 32U);
    EVP_MD_CTX *context = EVP_MD_CTX_new();
    size_t signature_length = 64U;
    int ok = key != NULL && context != NULL &&
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(context, signature, &signature_length, message, length) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static lxp_result settled_publish_receipt(void *context, const lxp_kernel *kernel,
    const lxp_activity *activity, const lxp_receipt *receipt)
{
    settled_fixture *f = context;
    lxp_byte_span encoded;
    lxp_result status;
    uint64_t offset;
    if (!kernel->batch_publication_pending ||
        receipt->global_sequence != f->published_sequence + 1U ||
        memcmp(receipt->previous_state_root, f->published_root, 32U) != 0 ||
        activity->activity_type != LX_ASSET_SEND)
        return LXP_ERR_CONTEXT_MISMATCH;
    status = lxp_arena_reset(&f->publication_arena, 0U);
    if (status == LXP_OK)
        status = lxp_receipt_encode(receipt, true, &f->publication_arena, &encoded);
    if (status == LXP_OK && encoded.length > UINT32_MAX)
        status = LXP_ERR_LENGTH_LIMIT;
    if (status == LXP_OK)
        status = lxp_log_append(&f->publication, LXP_LOG_RECEIPT,
            receipt->global_sequence, encoded.bytes, (uint32_t)encoded.length, &offset);
    if (status == LXP_OK) status = lxp_log_write_boundary(&f->publication);
    if (status == LXP_OK) {
        memcpy(f->published_root, receipt->resulting_state_root, 32U);
        f->published_sequence = receipt->global_sequence;
        ++f->published_count;
    }
    return status;
}

static lxp_result settled_publish_maintenance(void *context, const lxp_kernel *kernel,
    lxp_byte_span encoded, uint64_t timestamp_ms)
{
    settled_fixture *f = context;
    lxp_batch_maintenance envelope;
    lxp_programs_occupancy_receipt sweep;
    uint64_t offset;
    lxp_result status = lxp_batch_maintenance_decode(encoded.bytes, encoded.length,
        &envelope);
    if (status == LXP_OK)
        status = lxp_programs_occupancy_receipt_decode(envelope.occupancy.bytes,
            envelope.occupancy.length, &sweep);
    if (status != LXP_OK) return status;
    if (envelope.timestamp_ms != timestamp_ms ||
        sweep.global_sequence != f->published_sequence + 1U ||
        memcmp(sweep.previous_state_root, f->published_root, 32U) != 0 ||
        memcmp(sweep.resulting_state_root, kernel->current_state_root, 32U) != 0)
        return LXP_ERR_CONTEXT_MISMATCH;
    if (encoded.length > UINT32_MAX) return LXP_ERR_LENGTH_LIMIT;
    status = lxp_log_append(&f->publication, LXP_LOG_BATCH_BODY,
        sweep.global_sequence, encoded.bytes, (uint32_t)encoded.length, &offset);
    if (status == LXP_OK) status = lxp_log_write_boundary(&f->publication);
    if (status == LXP_OK) {
        memcpy(f->published_root, sweep.resulting_state_root, 32U);
        f->published_sequence = sweep.global_sequence;
        ++f->published_count;
    }
    return status;
}

static int settled_fixture_open(settled_fixture *f)
{
    static const uint64_t parameters = 1U;
    uint8_t treasury_id[32], grant[32] = {0};
    lxp_genesis_manifest manifest = {0};
    lx_programs_fee_genesis_parameters fees = {0};
    char path[192];
    memset(f, 0, sizeof(*f));
    f->storage = malloc(4U * LXP_MAX_BATCH_BODY_BYTES);
    LXP_CASE_CHECK(f->storage != NULL);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->storage, 4U * LXP_MAX_BATCH_BODY_BYTES) == LXP_OK);
    LXP_CASE_CHECK(settled_public_key(settled_actor_seed, f->actor_public_key) == 0);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(settled_actor_name,
        sizeof(settled_actor_name) - 1U, f->actor_id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(settled_treasury_name,
        sizeof(settled_treasury_name) - 1U, treasury_id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(settled_recipient_name,
        sizeof(settled_recipient_name) - 1U, f->recipient_id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, settled_actor_name, sizeof(settled_actor_name) - 1U,
        f->actor_id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL, &f->actor) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, settled_treasury_name, sizeof(settled_treasury_name) - 1U,
        treasury_id, 2U, LX_ACCOUNT_OPEN_GENESIS, NULL, &f->treasury) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, settled_recipient_name, sizeof(settled_recipient_name) - 1U,
        f->recipient_id, 3U, LX_ACCOUNT_OPEN_GENESIS, NULL, &f->recipient) == LXP_OK);
    f->asset.asset_id[0] = 9U;
    f->asset.registered = true;
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(f->actor, f->asset.asset_id,
        (lxp_u128){0U, 1000U}, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(f->treasury, f->asset.asset_id,
        (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(f->recipient, f->asset.asset_id,
        (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_register(&f->identities, settled_did, sizeof(settled_did) - 1U,
        f->actor_public_key, &f->identity) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_consume_sequence(f->identity, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal, &parameters, 1U) == LXP_OK);
    LXP_CASE_CHECK(install_metering_v1(&f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel, programs_module_registration_v4()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()) == LXP_OK);
    memcpy(f->asset_record.asset_id, f->asset.asset_id, 32U);
    f->asset_runtime = (lx_asset_runtime){&f->accounts, &f->asset_record, 1U,
        &f->asset, 1U, 7U, 3U, false};
    f->actor->has_authority_key = true;
    memcpy(f->actor->authority_key, f->actor_public_key, 32U);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET, &f->asset_runtime) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    f->runtime.assets = &f->asset;
    f->runtime.asset_count = 1U;
    f->runtime.fee_schedule = (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U};
    memcpy(f->runtime.occupancy_asset_id, f->asset.asset_id, 32U);
    f->runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;
    f->runtime.metering_schedule_context = &f->kernel;
    f->runtime.resolve_occupancy_parameters = lxp_programs_fee_governance_resolve_runtime;
    f->runtime.occupancy_parameter_context = &f->kernel;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_PROGRAMS, &f->runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL, lxp_kernel_canonical_ledger_apply) == LXP_OK);
    memcpy(manifest.signer_public_key, f->actor_public_key, 32U);
    fees.schedule = f->runtime.fee_schedule;
    memcpy(fees.occupancy_asset_id, f->asset.asset_id, 32U);
    fees.target_occupancy_byte_batches = 3U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 1U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 10U;
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(&manifest, &fees) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_materialize(&manifest, &f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    f->scope.module_mask = UINT64_C(1) << LXP_MODULE_ASSET;
    f->scope.activity_ordinal_min = 1U;
    f->scope.activity_ordinal_max = 10U;
    f->scope.maximum_per_activity = (lxp_u128){UINT64_MAX, UINT64_MAX};
    f->scope.maximum_total = f->scope.maximum_per_activity;
    f->scope.maximum_per_period = f->scope.maximum_per_activity;
    f->authority.scope = &f->scope;
    f->authority.kind = LXP_AUTHORITY_OWNER;
    memcpy(f->authority.actor, f->identity->did_id, 32U);
    memcpy(f->authority.principal, f->actor_id, 32U);
    memcpy(f->authority.verified_key, f->actor_public_key, 32U);
    LXP_CASE_CHECK(lxp_authority_hash(f->authority.kind, grant, f->actor_public_key,
        f->authority.authority_hash) == LXP_OK);
    f->fees.version = 1U;
    f->fees.multiplier_basis_points = 10000U;
    strcpy(f->directory, "/tmp/lxp-settled-maintenance-XXXXXX");
    LXP_CASE_CHECK(mkdtemp(f->directory) != NULL);
    LXP_CASE_CHECK(snprintf(path, sizeof(path), "%s/publication.log", f->directory) > 0);
    LXP_CASE_CHECK(lxp_log_open_or_create(&f->publication, path,
        LXP_MAX_BATCH_BODY_BYTES) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->publication_arena, f->publication_bytes,
        sizeof(f->publication_bytes)) == LXP_OK);
    memcpy(f->published_root, f->kernel.current_state_root, 32U);
    LXP_CASE_CHECK(lxp_kernel_set_commit_observer(&f->kernel,
        settled_publish_receipt, f) == LXP_OK);
    f->kernel.observe_maintenance = settled_publish_maintenance;
    return 0;
}

static int settled_build_send(settled_fixture *f, lxp_activity *activity,
                              uint8_t *payload, size_t *payload_length)
{
    lxp_send send;
    uint8_t material[144], message[512], preimage[32];
    size_t message_length;
    memset(&send, 0, sizeof(send));
    memcpy(send.from, f->actor_id, 32U);
    memcpy(send.to, f->recipient_id, 32U);
    memcpy(send.asset, f->asset.asset_id, 32U);
    send.amount.lo = 4U;
    send.sequence = f->actor->next_sequence;
    send.expires_at = 100U;
    send.idempotency_key[0] = 0x2AU;
    send.authorization.kind = LXP_AUTH_OWNER;
    send.authorization.network_id = 7U;
    send.authorization.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    memcpy(send.authorization.controller, send.from, 32U);
    memcpy(send.authorization.public_key, f->actor_public_key, 32U);
    memcpy(material, send.from, 32U);
    memcpy(material + 32U, send.to, 32U);
    memcpy(material + 64U, send.asset, 32U);
    LXP_CASE_CHECK(lxp_u128_to_be(send.amount, material + 96U) == LXP_OK);
    memcpy(material + 112U, send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_context_value(material, sizeof(material), send.context_hash) == LXP_OK);
    memcpy(send.authorization.signed_context_hash, send.context_hash, 32U);
    LXP_CASE_CHECK(lxp_send_authorization_message(&send, message, sizeof(message), &message_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message, message_length, preimage) == LXP_OK);
    LXP_CASE_CHECK(settled_sign(settled_actor_seed, preimage, 32U, send.authorization.signature) == 0);
    LXP_CASE_CHECK(lxp_send_encode(&send, payload, 512U, payload_length) == LXP_OK);
    memset(activity, 0, sizeof(*activity));
    activity->protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity->network_id = 7U;
    activity->activity_type = LX_ASSET_SEND;
    activity->actor_did = (lxp_byte_span){settled_did, sizeof(settled_did) - 1U};
    activity->authority = (lxp_byte_span){f->actor_public_key, 32U};
    activity->timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    activity->account_sequence = send.sequence;
    activity->payload = (lxp_byte_span){payload, *payload_length};
    memcpy(activity->idempotency_key, send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_payload(payload, *payload_length, activity->payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(activity, preimage) == LXP_OK);
    {
        static uint8_t signature[64];
        LXP_CASE_CHECK(settled_sign(settled_actor_seed, preimage, 32U, signature) == 0);
        activity->signature = (lxp_byte_span){signature, 64U};
    }
    LXP_CASE_CHECK(lxp_activity_verify_signature(activity) == LXP_OK);
    return 0;
}

static int settled_execution(settled_fixture *f, const lxp_activity *activity,
                             lxp_kernel_execution *execution,
                             uint64_t global_sequence, uint64_t batch_number)
{
    lxp_byte_span encoded;
    uint8_t activity_id[32];
    memset(execution, 0, sizeof(*execution));
    execution->network_id = 7U;
    execution->batch_number = batch_number;
    execution->batch_timestamp_ms = 10U;
    execution->maximum_timestamp_window = 100U;
    execution->epoch = 1U;
    execution->global_sequence = global_sequence;
    execution->recorded_module_version = lx_asset_module_iface()->abi_version;
    execution->parameter_version = 1U;
    execution->signature_valid = true;
    execution->identities = &f->identities;
    execution->authority = &f->authority;
    execution->fee_parameters = &f->fees;
    execution->fee_balance = f->actor->balance;
    execution->gas_limit = UINT64_MAX;
    execution->arena = &f->arena;
    execution->sequencer_private_key = settled_sequencer_seed;
    LXP_CASE_CHECK(lxp_activity_encode(activity, &f->arena, &encoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_id(encoded.bytes, encoded.length, activity_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_batch_identity_activity(f->kernel.current_state_root, activity_id,
        global_sequence, batch_number, execution->batch_id) == LXP_OK);
    return 0;
}

static int settled_first_batch(settled_fixture *f)
{
    lxp_activity activity;
    lxp_kernel_execution execution;
    lxp_kernel_prepared_batch *prepared = NULL;
    uint8_t payload[512], digest[32];
    size_t payload_length;

    LXP_CASE_CHECK(f->state.next_sequence == 1U);
    LXP_CASE_CHECK(f->state.count == 0U);
    LXP_CASE_CHECK(!f->journal.open);
    LXP_CASE_CHECK(f->kernel.observe_commit == settled_publish_receipt);
    LXP_CASE_CHECK(f->published_count == 0U);
    LXP_CASE_CHECK(settled_build_send(f, &activity, payload, &payload_length) == 0);
    LXP_CASE_CHECK(settled_execution(f, &activity, &execution, 1U, 1U) == 0);
    LXP_CASE_CHECK(lxp_kernel_prepare_terminal_rejection(&f->kernel, &activity,
        &execution, LXP_ERR_SEQUENCE_MISMATCH, &prepared) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_prepared_batch_count(prepared) == 1U);
    LXP_CASE_CHECK(lxp_kernel_prepared_batch_receipts(prepared)[0].result_code ==
                   LXP_ERR_SEQUENCE_MISMATCH);
    LXP_CASE_CHECK(lxp_kernel_prepare_batch_maintenance(prepared, &activity,
        &execution) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_prepared_batch_maintenance(prepared).length != 0U);
    memcpy(digest, lxp_kernel_prepared_batch_publication_digest(prepared), 32U);
    LXP_CASE_CHECK(lxp_kernel_commit_prepared_batch(&f->kernel, &f->identities,
        prepared, digest) == LXP_OK);
    LXP_CASE_CHECK(f->state.next_sequence == 3U);
    LXP_CASE_CHECK(memcmp(f->kernel.current_state_root,
        lxp_kernel_prepared_batch_final_root(prepared), 32U) == 0);
    LXP_CASE_CHECK(f->state.count == 0U);
    LXP_CASE_CHECK(!f->journal.open);
    LXP_CASE_CHECK(lxp_kernel_finalize_prepared_batch_publication(&f->kernel,
        &activity, prepared, digest) == LXP_OK);
    LXP_CASE_CHECK(!f->kernel.publication_poisoned);
    LXP_CASE_CHECK(!f->kernel.batch_publication_pending);
    LXP_CASE_CHECK(f->published_count == 2U && f->published_sequence == 2U);
    LXP_CASE_CHECK(memcmp(f->published_root, f->kernel.current_state_root, 32U) == 0);
    lxp_kernel_prepared_batch_destroy(prepared);
    return 0;
}

static int settled_fixture_close(settled_fixture *f)
{
    char path[192];
    LXP_CASE_CHECK(lxp_log_close(&f->publication) == LXP_OK);
    LXP_CASE_CHECK(snprintf(path, sizeof(path), "%s/publication.log", f->directory) > 0);
    LXP_CASE_CHECK(unlink(path) == 0);
    LXP_CASE_CHECK(rmdir(f->directory) == 0);
    while (f->kernel.blob_count != 0U)
        free(f->kernel.blobs[--f->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f->storage);
    free(f);
    return 0;
}

static int fresh_genesis_maintenance_batch_commits(void)
{
    settled_fixture *f = calloc(1U, sizeof(*f));

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(settled_fixture_open(f) == 0);
    LXP_CASE_CHECK(settled_first_batch(f) == 0);
    return settled_fixture_close(f);
}

static int second_maintenance_batch_chains(void)
{
    settled_fixture *f = calloc(1U, sizeof(*f));
    lxp_activity activity;
    lxp_kernel_execution execution;
    lxp_kernel_prepared_batch *prepared = NULL;
    const lxp_receipt *receipts;
    lxp_batch_maintenance envelope;
    lxp_programs_occupancy_receipt sweep;
    lxp_byte_span maintenance;
    uint8_t first_root[32], payload[512], digest[32];
    size_t payload_length;

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(settled_fixture_open(f) == 0);
    LXP_CASE_CHECK(settled_first_batch(f) == 0);
    memcpy(first_root, f->kernel.current_state_root, 32U);
    LXP_CASE_CHECK(settled_build_send(f, &activity, payload, &payload_length) == 0);
    LXP_CASE_CHECK(activity.account_sequence == f->identity->next_sequence);
    LXP_CASE_CHECK(settled_execution(f, &activity, &execution,
        f->state.next_sequence, 2U) == 0);
    LXP_CASE_CHECK(execution.global_sequence == 3U);
    LXP_CASE_CHECK(lxp_kernel_prepare_serial_activity_batch(&f->kernel, &activity,
        &execution, &prepared) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_prepared_batch_count(prepared) == 1U);
    LXP_CASE_CHECK(lxp_kernel_prepare_batch_maintenance(prepared, &activity,
        &execution) == LXP_OK);
    memcpy(digest, lxp_kernel_prepared_batch_publication_digest(prepared), 32U);
    LXP_CASE_CHECK(lxp_kernel_commit_prepared_batch(&f->kernel, &f->identities,
        prepared, digest) == LXP_OK);
    maintenance = lxp_kernel_prepared_batch_maintenance(prepared);
    LXP_CASE_CHECK(maintenance.bytes != NULL && maintenance.length != 0U);
    LXP_CASE_CHECK(lxp_batch_maintenance_decode(maintenance.bytes,
        maintenance.length, &envelope) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_occupancy_receipt_decode(envelope.occupancy.bytes,
        envelope.occupancy.length, &sweep) == LXP_OK);
    receipts = lxp_kernel_prepared_batch_receipts(prepared);
    LXP_CASE_CHECK(receipts != NULL);
    LXP_CASE_CHECK(receipts[0].result_code == LXP_OK);
    LXP_CASE_CHECK(memcmp(receipts[0].previous_state_root, first_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(sweep.previous_state_root,
        receipts[0].resulting_state_root, 32U) == 0);
    LXP_CASE_CHECK(sweep.global_sequence == receipts[0].global_sequence + 1U);
    LXP_CASE_CHECK(sweep.global_sequence == 4U);
    LXP_CASE_CHECK(f->state.next_sequence == 5U);
    LXP_CASE_CHECK(memcmp(f->kernel.current_state_root,
        sweep.resulting_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(f->kernel.current_state_root,
        lxp_kernel_prepared_batch_final_root(prepared), 32U) == 0);
    LXP_CASE_CHECK(f->state.count == 0U);
    LXP_CASE_CHECK(!f->journal.open);
    LXP_CASE_CHECK(lxp_kernel_finalize_prepared_batch_publication(&f->kernel,
        &activity, prepared, digest) == LXP_OK);
    LXP_CASE_CHECK(f->published_count == 4U && f->published_sequence == 4U);
    LXP_CASE_CHECK(memcmp(f->published_root, sweep.resulting_state_root, 32U) == 0);
    lxp_kernel_prepared_batch_destroy(prepared);
    return settled_fixture_close(f);
}

static int maintenance_refusal_is_atomic(void)
{
    settled_fixture *f = calloc(1U, sizeof(*f));
    lxp_activity activity;
    lxp_kernel_execution execution;
    lxp_kernel_prepared_batch *prepared = NULL;
    uint8_t payload[512], root[32], after[32], digest[32], wrong_digest[32];
    size_t payload_length;
    uint64_t identity_sequence, account_sequence, log_offset;

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(settled_fixture_open(f) == 0);
    memcpy(root, f->kernel.current_state_root, 32U);
    identity_sequence = f->identity->next_sequence;
    account_sequence = f->actor->next_sequence;
    log_offset = f->publication.write_offset;
    LXP_CASE_CHECK(settled_build_send(f, &activity, payload, &payload_length) == 0);
    LXP_CASE_CHECK(settled_execution(f, &activity, &execution, 1U, 1U) == 0);
    LXP_CASE_CHECK(lxp_kernel_prepare_terminal_rejection(&f->kernel, &activity,
        &execution, LXP_ERR_SEQUENCE_MISMATCH, &prepared) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_prepare_batch_maintenance(prepared, &activity,
        &execution) == LXP_OK);
    memcpy(digest, lxp_kernel_prepared_batch_publication_digest(prepared), 32U);
    memcpy(wrong_digest, digest, 32U);
    wrong_digest[0] ^= 1U;
    LXP_CASE_CHECK(lxp_kernel_commit_prepared_batch(&f->kernel, &f->identities,
        prepared, wrong_digest) == LXP_ERR_CONTEXT_MISMATCH);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root, after, 32U) == 0);
    LXP_CASE_CHECK(memcmp(root, f->kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(f->state.next_sequence == 1U && !f->journal.open);
    LXP_CASE_CHECK(f->identity->next_sequence == identity_sequence);
    LXP_CASE_CHECK(f->actor->next_sequence == account_sequence);
    LXP_CASE_CHECK(f->publication.write_offset == log_offset && f->published_count == 0U);
    LXP_CASE_CHECK(!f->kernel.publication_poisoned && !f->kernel.batch_publication_pending);
    LXP_CASE_CHECK(lxp_kernel_commit_prepared_batch(&f->kernel, &f->identities,
        prepared, digest) == LXP_OK);
    log_offset = f->publication.write_offset;
    LXP_CASE_CHECK(lxp_kernel_finalize_prepared_batch_publication(&f->kernel,
        &activity, prepared, wrong_digest) == LXP_ERR_CONTEXT_MISMATCH);
    LXP_CASE_CHECK(f->publication.write_offset == log_offset && f->published_count == 0U);
    LXP_CASE_CHECK(f->kernel.publication_poisoned && f->kernel.batch_publication_pending);
    LXP_CASE_CHECK(lxp_kernel_finalize_prepared_batch_publication(&f->kernel,
        &activity, prepared, digest) == LXP_OK);
    LXP_CASE_CHECK(f->published_count == 2U && f->state.next_sequence == 3U);
    lxp_kernel_prepared_batch_destroy(prepared);
    return settled_fixture_close(f);
}

static int expired_activity_leaves_maintenance_state_unchanged(void)
{
    settled_fixture *f = calloc(1U, sizeof(*f));
    lxp_activity activity;
    lxp_kernel_execution execution;
    lxp_receipt receipt;
    uint8_t payload[512], signature[64], digest[32], before[32], after[32];
    size_t payload_length;
    uint64_t identity_sequence, account_sequence, log_offset;
    lxp_u128 actor_balance, recipient_balance;

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(settled_fixture_open(f) == 0);
    LXP_CASE_CHECK(settled_first_batch(f) == 0);
    memcpy(before, f->kernel.current_state_root, 32U);
    identity_sequence = f->identity->next_sequence;
    account_sequence = f->actor->next_sequence;
    actor_balance = f->actor->balance;
    recipient_balance = f->recipient->balance;
    log_offset = f->publication.write_offset;
    LXP_CASE_CHECK(settled_build_send(f, &activity, payload, &payload_length) == 0);
    activity.timestamp_bound.not_after = 9U;
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&activity, digest) == LXP_OK);
    LXP_CASE_CHECK(settled_sign(settled_actor_seed, digest, sizeof(digest), signature) == 0);
    activity.signature = (lxp_byte_span){signature, sizeof(signature)};
    LXP_CASE_CHECK(lxp_activity_verify_signature(&activity) == LXP_OK);
    LXP_CASE_CHECK(settled_execution(f, &activity, &execution,
        f->state.next_sequence, 2U) == 0);
    LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity,
        &execution, &receipt) == LXP_ERR_EXPIRED);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(memcmp(before, f->kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(f->state.next_sequence == 3U && !f->journal.open);
    LXP_CASE_CHECK(f->identity->next_sequence == identity_sequence);
    LXP_CASE_CHECK(f->actor->next_sequence == account_sequence);
    LXP_CASE_CHECK(lxp_u128_cmp(f->actor->balance, actor_balance) == 0);
    LXP_CASE_CHECK(lxp_u128_cmp(f->recipient->balance, recipient_balance) == 0);
    LXP_CASE_CHECK(f->publication.write_offset == log_offset && f->published_count == 2U);
    LXP_CASE_CHECK(!f->kernel.publication_poisoned && !f->kernel.batch_publication_pending);
    return settled_fixture_close(f);
}

typedef struct settled_recovery {
    uint64_t sequence;
    uint8_t root[32];
} settled_recovery;

static lxp_result settled_recover_publication(void *context,
    const lxp_log_record_header *header, const uint8_t *body)
{
    settled_recovery *recovery = context;
    lxp_result status;
    const uint8_t *previous_root;
    const uint8_t *resulting_root;
    uint64_t sequence;
    lxp_receipt receipt;
    lxp_batch_maintenance envelope;
    lxp_programs_occupancy_receipt sweep;

    if (header->record_kind == LXP_LOG_RECEIPT) {
        status = lxp_receipt_decode(body, header->body_length, true, &receipt);
        if (status != LXP_OK) return status;
        sequence = receipt.global_sequence;
        previous_root = receipt.previous_state_root;
        resulting_root = receipt.resulting_state_root;
    } else if (header->record_kind == LXP_LOG_BATCH_BODY) {
        status = lxp_batch_maintenance_decode(body, header->body_length, &envelope);
        if (status == LXP_OK)
            status = lxp_programs_occupancy_receipt_decode(envelope.occupancy.bytes,
                envelope.occupancy.length, &sweep);
        if (status != LXP_OK) return status;
        sequence = sweep.global_sequence;
        previous_root = sweep.previous_state_root;
        resulting_root = sweep.resulting_state_root;
    } else {
        return LXP_ERR_CONTEXT_MISMATCH;
    }
    if (sequence != recovery->sequence + 1U || sequence != header->global_sequence ||
        memcmp(previous_root, recovery->root, 32U) != 0)
        return LXP_ERR_CONTEXT_MISMATCH;
    recovery->sequence = sequence;
    memcpy(recovery->root, resulting_root, 32U);
    return LXP_OK;
}

static int maintenance_publication_survives_reopen(void)
{
    settled_fixture *f = calloc(1U, sizeof(*f));
    lxp_log_record_header header;
    lxp_receipt receipt;
    lxp_batch_maintenance envelope;
    lxp_programs_occupancy_receipt sweep;
    uint64_t offset, durable_offset;
    settled_recovery recovery = {0};
    char path[192];

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(settled_fixture_open(f) == 0);
    LXP_CASE_CHECK(settled_first_batch(f) == 0);
    durable_offset = f->publication.durable_offset;
    LXP_CASE_CHECK(durable_offset == f->publication.write_offset);
    LXP_CASE_CHECK(lxp_log_close(&f->publication) == LXP_OK);
    LXP_CASE_CHECK(snprintf(path, sizeof(path), "%s/publication.log", f->directory) > 0);
    LXP_CASE_CHECK(lxp_log_open(&f->publication, path) == LXP_OK);
    LXP_CASE_CHECK(f->publication.durable_offset == durable_offset);
    LXP_CASE_CHECK(lxp_log_read(&f->publication, 0U, &header,
        f->publication_bytes, sizeof(f->publication_bytes)) == LXP_OK);
    LXP_CASE_CHECK(header.record_kind == LXP_LOG_RECEIPT && header.global_sequence == 1U);
    LXP_CASE_CHECK(lxp_receipt_decode(f->publication_bytes, header.body_length,
        true, &receipt) == LXP_OK);
    LXP_CASE_CHECK(receipt.result_code == LXP_ERR_SEQUENCE_MISMATCH);
    memcpy(recovery.root, receipt.previous_state_root, 32U);
    LXP_CASE_CHECK(lxp_log_recover_complete_records(&f->publication,
        settled_recover_publication, &recovery) == LXP_OK);
    LXP_CASE_CHECK(f->publication.write_offset == durable_offset);
    LXP_CASE_CHECK(recovery.sequence == 2U);
    LXP_CASE_CHECK(memcmp(recovery.root, f->kernel.current_state_root, 32U) == 0);
    offset = LXP_LOG_HEADER_BYTES + header.body_length;
    LXP_CASE_CHECK(lxp_log_read(&f->publication, offset, &header,
        f->publication_bytes, sizeof(f->publication_bytes)) == LXP_OK);
    LXP_CASE_CHECK(header.record_kind == LXP_LOG_BATCH_BODY && header.global_sequence == 2U);
    LXP_CASE_CHECK(lxp_batch_maintenance_decode(f->publication_bytes,
        header.body_length, &envelope) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_occupancy_receipt_decode(envelope.occupancy.bytes,
        envelope.occupancy.length, &sweep) == LXP_OK);
    LXP_CASE_CHECK(memcmp(sweep.previous_state_root, receipt.resulting_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(sweep.resulting_state_root, f->kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(offset + LXP_LOG_HEADER_BYTES + header.body_length == durable_offset);
    return settled_fixture_close(f);
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "fresh_genesis_maintenance_batch_commits", fresh_genesis_maintenance_batch_commits },
        { "second_maintenance_batch_chains", second_maintenance_batch_chains },
        { "maintenance_refusal_is_atomic", maintenance_refusal_is_atomic },
        { "expired_activity_leaves_maintenance_state_unchanged", expired_activity_leaves_maintenance_state_unchanged },
        { "maintenance_publication_survives_reopen", maintenance_publication_survives_reopen }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
