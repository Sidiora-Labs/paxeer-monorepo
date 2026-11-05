#include "layerx/lxp_kernel_host.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"
#include "layerx/lx_stream.h"
#include "layerx/programs.h"
#include "lxp_kernel_case.h"

#include <openssl/evp.h>
#include <stdlib.h>

lxp_result lxp_kernel_module_kv_load(lxp_kernel *kernel,
                                     const lxp_module_kv_entry *rows,
                                     size_t count);
lxp_result lxp_kernel_claim_writer(lxp_kernel *kernel);
lxp_result lxp_kernel_execute_activity(lxp_kernel *kernel,
                                       const lxp_activity *activity,
                                       const lxp_kernel_execution *execution,
                                       lxp_receipt *receipt);

enum { INTEGRATED_ACTIVITIES = 64, INTEGRATED_STREAMS = 2,
       INTEGRATED_ARENA_BYTES = 2 * LXP_MAX_ACTIVITY_BYTES + 65536 };

static const uint8_t integrated_seed[32] = {0x31U};
static const uint8_t integrated_asset[32] = {3U};
static const uint8_t integrated_did[] = "did:key:integrated";
static const char *const integrated_names[] = {
    "agent:did:key:integrated:asset:0300000000000000000000000000000000000000000000000000000000000000",
    "agent:did:key:recipient:asset:0300000000000000000000000000000000000000000000000000000000000000",
    "agent:did:key:integrated:main",
    "agent:did:key:integrated:stream:s0",
    "agent:did:key:integrated:stream:s1"
};

typedef struct integrated_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lxp_identity_store identities;
    lxp_identity *identity;
    uint8_t account_ids[5][32];
    uint8_t public_key[32];
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lx_asset_runtime runtime;
    lxp_arena arena;
    uint8_t arena_bytes[INTEGRATED_ARENA_BYTES];
} integrated_fixture;

typedef struct integrated_work {
    integrated_fixture *fixture;
    const lxp_activity *activity;
    const lxp_kernel_execution *execution;
    lxp_receipt receipt;
    lxp_result claim;
    lxp_result owner;
    lxp_result result;
    bool writer_is_self;
} integrated_work;

static int integrated_sign(const uint8_t digest[32], uint8_t signature[64],
                            uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                  integrated_seed, 32U);
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    size_t key_length = 32U, signature_length = 64U;
    int ok = key != NULL && ctx != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &key_length) == 1 &&
        key_length == 32U &&
        EVP_DigestSignInit(ctx, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(ctx, signature, &signature_length, digest, 32U) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(ok);
    return 0;
}

static int integrated_genesis(lxp_genesis_manifest *manifest)
{
    lx_programs_metering_schedule metering = {0};
    lx_programs_fee_genesis_parameters fees = {0};
    uint8_t signature[64];
    size_t index;

    manifest->protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    manifest->network_id = 7U;
    manifest->genesis_timestamp_ms = 1U;
    manifest->parameter_count = 2U;
    manifest->parameters[0].module_id = LXP_MODULE_GOVERNANCE;
    LXP_CASE_CHECK(lxp_genesis_module_enable_key(LXP_MODULE_STREAM,
        manifest->parameters[0].key) == LXP_OK);
    manifest->parameters[0].value[31] = 1U;
    manifest->parameters[1].module_id = LXP_MODULE_GOVERNANCE;
    (void)memcpy(manifest->parameters[1].key, "parameter-version", 17U);
    manifest->parameters[1].value[31] = 1U;
    manifest->guarantor_count = 1U;
    manifest->guarantors[0].guarantor_id[0] = 1U;
    (void)memset(manifest->guarantors[0].public_key, 1, 33U);
    LXP_CASE_CHECK(integrated_sign((const uint8_t[32]){0}, signature,
                                   manifest->signer_public_key) == 0);
    metering.version = 1U;
    for (index = 0U; index < 5U; ++index)
        metering.coefficients[index] = 1U;
    metering.coefficients[5] = 8U;
    metering.coefficients[6] = 8U;
    metering.coefficients[7] = 64U;
    metering.coefficients[8] = 8U;
    metering.activation_batch = 1U;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    LXP_CASE_CHECK(lxp_hash_payload(manifest->signer_public_key, 32U,
                                    metering.authority_digest) == LXP_OK);
    fees.schedule = (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U};
    (void)memcpy(fees.occupancy_asset_id, integrated_asset, 32U);
    fees.target_occupancy_byte_batches = 100U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 10U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 1000U;
    LXP_CASE_CHECK(lxp_genesis_fresh_empty_accounts(manifest,
                                                  integrated_asset) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_append(manifest,
                                                       &metering) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(manifest, &fees) == LXP_OK);
    return 0;
}

static int integrated_fixture_init(integrated_fixture *fixture,
                                    const lxp_genesis_manifest *manifest)
{
    lxp_genesis_module_plan plan;
    uint8_t signature[64];
    size_t index;

    LXP_CASE_CHECK(lxp_arena_init(&fixture->arena, fixture->arena_bytes,
                                  sizeof(fixture->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&fixture->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&fixture->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&fixture->state,
                                                &fixture->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&fixture->kernel, &fixture->state,
        &fixture->journal, manifest, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_resolve(manifest, &plan) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_register(&plan,
                                                   &fixture->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_materialize(manifest, &fixture->arena,
                                          &fixture->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&fixture->kernel,
                                  fixture->kernel.current_state_root) == LXP_OK);
    LXP_CASE_CHECK(integrated_sign((const uint8_t[32]){0}, signature,
                                   fixture->public_key) == 0);
    LXP_CASE_CHECK(lxp_identity_register(&fixture->identities, integrated_did,
        sizeof(integrated_did) - 1U, fixture->public_key,
        &fixture->identity) == LXP_OK);
    for (index = 0U; index < 5U; ++index) {
        lx_account *account;
        size_t length = strlen(integrated_names[index]);
        uint64_t balance = index == 0U ? 10000U : index >= 3U ? 1000U : 0U;
        LXP_CASE_CHECK(lx_account_id_from_string(
            (const uint8_t *)integrated_names[index], length,
            fixture->account_ids[index]) == LXP_OK);
        LXP_CASE_CHECK(lx_account_open(&fixture->accounts,
            (const uint8_t *)integrated_names[index], length,
            fixture->account_ids[index], 1U, LX_ACCOUNT_OPEN_GENESIS,
            NULL, &account) == LXP_OK);
        LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(account, integrated_asset,
            (lxp_u128){0U, balance}, 0U) == LXP_OK);
        if (index == 0U) {
            account->has_authority_key = true;
            (void)memcpy(account->authority_key, fixture->public_key, 32U);
        }
    }
    (void)memcpy(fixture->asset.asset_id, integrated_asset, 32U);
    (void)memcpy(fixture->asset.symbol, "INT", 4U);
    fixture->asset.symbol_length = 3U;
    fixture->asset.name[0] = (uint8_t)'I';
    fixture->asset.name_length = 1U;
    fixture->asset.issuer_kind = 2U;
    fixture->asset.issuer_did32[0] = 1U;
    fixture->asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    fixture->asset.custody_reference[0] = 1U;
    fixture->asset.custody_reference_length = 1U;
    fixture->asset.total_units = (lxp_u128){0U, 12000U};
    LXP_CASE_CHECK(lx_asset_transfer_state(&fixture->asset,
                                          &fixture->transfer_asset) == LXP_OK);
    fixture->runtime = (lx_asset_runtime){&fixture->accounts, &fixture->asset,
        1U, &fixture->transfer_asset, 1U, 7U,
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT, false};
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&fixture->kernel,
        LXP_MODULE_ASSET, &fixture->runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&fixture->kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    return 0;
}

static uint64_t integrated_random(uint64_t *seed)
{
    *seed ^= *seed << 13U;
    *seed ^= *seed >> 7U;
    *seed ^= *seed << 17U;
    return *seed;
}

static void integrated_permute(lxp_module_kv_entry *rows, size_t count,
                                uint64_t seed)
{
    size_t index;
    for (index = count; index > 1U; --index) {
        size_t other = (size_t)(integrated_random(&seed) % index);
        lxp_module_kv_entry held = rows[index - 1U];
        rows[index - 1U] = rows[other];
        rows[other] = held;
    }
}

static int integrated_load(integrated_fixture *left,
                            integrated_fixture *right, uint64_t seed)
{
    size_t count = left->kernel.module_kv_count + 1U + INTEGRATED_STREAMS;
    lxp_module_kv_entry *rows = calloc(count, sizeof(*rows));
    lxp_module_kv_entry *permuted = calloc(count, sizeof(*permuted));
    lxp_module_kv_entry *entry;
    size_t index, length;

    LXP_CASE_CHECK(rows != NULL && permuted != NULL);
    LXP_CASE_CHECK(count <= LXP_KERNEL_MAX_MODULE_KV);
    LXP_CASE_CHECK(left->kernel.module_kv_count == right->kernel.module_kv_count);
    LXP_CASE_CHECK(memcmp(left->kernel.module_kv, right->kernel.module_kv,
        left->kernel.module_kv_count * sizeof(*rows)) == 0);
    (void)memcpy(rows, left->kernel.module_kv,
                 left->kernel.module_kv_count * sizeof(*rows));
    entry = &rows[left->kernel.module_kv_count];
    entry->module_id = LXP_MODULE_ASSET;
    entry->key_length = 38U;
    (void)memcpy(entry->key, "asset:", 6U);
    (void)memcpy(entry->key + 6U, integrated_asset, 32U);
    LXP_CASE_CHECK(lx_asset_record_encode(&left->asset, entry->value,
        sizeof(entry->value), &length) == LXP_OK);
    entry->value_length = (uint32_t)length;
    for (index = 0U; index < INTEGRATED_STREAMS; ++index) {
        lx_stream_record record = {0};
        entry = &rows[left->kernel.module_kv_count + 1U + index];
        record.stream_id[0] = (uint8_t)(index + 1U);
        (void)memcpy(record.payer, left->account_ids[2], 32U);
        (void)memcpy(record.recipient, left->account_ids[1], 32U);
        (void)memcpy(record.stream_account, left->account_ids[3U + index], 32U);
        (void)memcpy(record.asset_id, integrated_asset, 32U);
        record.mode = LX_STREAM_MODE_TIME;
        record.rate = (lxp_u128){0U, 1U};
        record.rate_unit = 1U;
        record.start_timestamp = 1U;
        record.last_accrual_timestamp = 1U;
        record.total_cap = (lxp_u128){0U, 100000U};
        entry->module_id = LXP_MODULE_STREAM;
        entry->key_length = LX_STREAM_KEY_BYTES;
        entry->value_length = LX_STREAM_RECORD_BYTES;
        LXP_CASE_CHECK(lx_stream_state_key(record.stream_id, entry->key) == LXP_OK);
        LXP_CASE_CHECK(lx_stream_record_encode(&record, entry->value) == LXP_OK);
    }
    (void)memcpy(permuted, rows, count * sizeof(*rows));
    integrated_permute(rows, count, seed);
    integrated_permute(permuted, count, seed ^ UINT64_C(0x9e3779b97f4a7c15));
    LXP_CASE_CHECK(memcmp(rows, permuted, count * sizeof(*rows)) != 0);
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&left->kernel, rows, count) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&right->kernel, permuted, count) == LXP_OK);
    LXP_CASE_CHECK(memcmp(left->kernel.module_kv, right->kernel.module_kv,
                          count * sizeof(*rows)) == 0);
    free(permuted);
    free(rows);
    LXP_CASE_CHECK(lxp_state_root(&left->kernel,
                                  left->kernel.current_state_root) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&right->kernel,
                                  right->kernel.current_state_root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(left->kernel.current_state_root,
                          right->kernel.current_state_root, 32U) == 0);
    return 0;
}

static int integrated_activity(integrated_fixture *fixture, size_t index,
                               const lxp_kernel_execution *execution,
                               lxp_activity *activity, uint8_t payload[512],
                               uint8_t signature[64])
{
    uint8_t digest[32], public_key[32];
    size_t length;

    (void)memset(activity, 0, sizeof(*activity));
    activity->protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity->network_id = execution->network_id;
    activity->actor_did = (lxp_byte_span){integrated_did,
                                        sizeof(integrated_did) - 1U};
    activity->authority = (lxp_byte_span){fixture->public_key, 32U};
    activity->signature = (lxp_byte_span){signature, 64U};
    activity->account_sequence = execution->global_sequence - 1U;
    activity->idempotency_key[31] = (uint8_t)execution->global_sequence;
    activity->timestamp_bound = (lxp_timestamp_bound){
        execution->batch_timestamp_ms, execution->batch_timestamp_ms + 100U};
    if (index % 2U == 0U) {
        lxp_send send = {0};
        uint8_t material[144], message[512];
        size_t message_length;
        activity->activity_type = LX_ASSET_SEND;
        (void)memcpy(send.from, fixture->account_ids[0], 32U);
        (void)memcpy(send.to, fixture->account_ids[1], 32U);
        (void)memcpy(send.asset, integrated_asset, 32U);
        send.amount = (lxp_u128){0U, 1U};
        send.sequence = (execution->global_sequence - 1U) / 2U;
        send.expires_at = activity->timestamp_bound.not_after;
        (void)memcpy(send.idempotency_key, activity->idempotency_key, 32U);
        send.authorization.kind = LXP_AUTH_OWNER;
        send.authorization.network_id = execution->network_id;
        send.authorization.protocol_version = activity->protocol_version;
        (void)memcpy(send.authorization.controller, send.from, 32U);
        (void)memcpy(send.authorization.public_key, fixture->public_key, 32U);
        (void)memcpy(material, send.from, 32U);
        (void)memcpy(material + 32U, send.to, 32U);
        (void)memcpy(material + 64U, send.asset, 32U);
        LXP_CASE_CHECK(lxp_u128_to_be(send.amount, material + 96U) == LXP_OK);
        (void)memcpy(material + 112U, send.idempotency_key, 32U);
        LXP_CASE_CHECK(lxp_hash_context_value(material, sizeof(material),
                                              send.context_hash) == LXP_OK);
        (void)memcpy(send.authorization.signed_context_hash, send.context_hash, 32U);
        LXP_CASE_CHECK(lxp_send_authorization_message(&send, message,
            sizeof(message), &message_length) == LXP_OK);
        LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message,
                                       message_length, digest) == LXP_OK);
        LXP_CASE_CHECK(integrated_sign(digest, send.authorization.signature,
                                       public_key) == 0);
        LXP_CASE_CHECK(lxp_send_encode(&send, payload, 512U, &length) == LXP_OK);
    } else {
        lx_stream_id_payload id = {0};
        id.stream_id[0] = (uint8_t)(1U + (index / 4U) % INTEGRATED_STREAMS);
        activity->activity_type = index % 4U == 1U ?
            LX_STREAM_PAUSE : LX_STREAM_RESUME;
        LXP_CASE_CHECK(lx_stream_id_encode(&id, payload, 512U, &length) == LXP_OK);
    }
    activity->payload = (lxp_byte_span){payload, length};
    LXP_CASE_CHECK(lxp_hash_payload(payload, length, activity->payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(activity, digest) == LXP_OK);
    LXP_CASE_CHECK(integrated_sign(digest, signature, public_key) == 0);
    LXP_CASE_CHECK(lxp_activity_verify_signature(activity) == LXP_OK);
    return 0;
}

static void *integrated_worker(void *argument)
{
    integrated_work *work = argument;
    work->claim = lxp_kernel_claim_writer(&work->fixture->kernel);
    work->owner = lxp_state_writer_assert_owner(&work->fixture->state);
    work->writer_is_self =
        pthread_equal(work->fixture->state.writer, pthread_self()) != 0;
    if (work->claim == LXP_OK && work->owner == LXP_OK)
        work->result = lxp_kernel_execute_activity(&work->fixture->kernel,
            work->activity, work->execution, &work->receipt);
    return NULL;
}

static int integrated_stream_check(const integrated_fixture *fixture,
                                    size_t stream, uint64_t timestamp,
                                    uint64_t accrued, bool paused)
{
    uint8_t id[32] = {0}, key[LX_STREAM_KEY_BYTES];
    size_t index;
    id[0] = (uint8_t)(stream + 1U);
    LXP_CASE_CHECK(lx_stream_state_key(id, key) == LXP_OK);
    for (index = 0U; index < fixture->kernel.module_kv_count; ++index) {
        const lxp_module_kv_entry *entry = &fixture->kernel.module_kv[index];
        lx_stream_record record;
        if (entry->module_id != LXP_MODULE_STREAM ||
            entry->key_length != sizeof(key) ||
            memcmp(entry->key, key, sizeof(key)) != 0) continue;
        LXP_CASE_CHECK(lx_stream_record_decode(entry->value,
                                               entry->value_length, &record) == LXP_OK);
        LXP_CASE_CHECK(record.paused == paused);
        LXP_CASE_CHECK(record.last_accrual_timestamp == timestamp);
        LXP_CASE_CHECK(record.accrued_total.hi == 0U);
        LXP_CASE_CHECK(record.accrued_total.lo == accrued);
        return 0;
    }
    LXP_CASE_CHECK(false);
    return 1;
}

static int integrated_run(bool handoff, uint64_t seed)
{
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    integrated_fixture *fixtures = calloc(2U, sizeof(*fixtures));
    lxp_fee_params fees = {.version = 1U, .multiplier_basis_points = 10000U};
    uint64_t last[INTEGRATED_STREAMS] = {1U, 1U};
    uint64_t accrued[INTEGRATED_STREAMS] = {0U, 0U};
    size_t index, side;
    size_t sends = 0U, stream_activities = 0U;

    LXP_CASE_CHECK(manifest != NULL && fixtures != NULL);
    LXP_CASE_CHECK(integrated_genesis(manifest) == 0);
    for (side = 0U; side < 2U; ++side)
        LXP_CASE_CHECK(integrated_fixture_init(&fixtures[side], manifest) == 0);
    LXP_CASE_CHECK(memcmp(fixtures[0].kernel.current_state_root,
                          fixtures[1].kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(integrated_load(&fixtures[0], &fixtures[1], seed) == 0);
    for (index = 0U; index < INTEGRATED_ACTIVITIES; ++index) {
        lxp_kernel_execution execution = {0}, executions[2];
        lxp_authority_resolved authority = {0};
        lxp_activity activity;
        uint8_t payload[512], signature[64], before[32];
        lxp_byte_span encoded[2];
        integrated_work work[2] = {0};

        execution.network_id = 7U;
        execution.batch_number = 1U + index / 8U;
        execution.batch_timestamp_ms = 1000U + index * 37U;
        execution.maximum_timestamp_window = 100U;
        execution.global_sequence = index + 1U;
        execution.recorded_module_version = 1U;
        execution.parameter_version = 1U;
        execution.signature_valid = true;
        execution.fee_parameters = &fees;
        execution.gas_limit = 1000000U;
        execution.sequencer_private_key = integrated_seed;
        execution.batch_id[31] = (uint8_t)execution.batch_number;
        LXP_CASE_CHECK(integrated_activity(&fixtures[0], index, &execution,
                                           &activity, payload, signature) == 0);
        authority.kind = LXP_AUTHORITY_OWNER;
        (void)memcpy(authority.actor, fixtures[0].identity->did_id, 32U);
        (void)memcpy(authority.verified_key, fixtures[0].public_key, 32U);
        (void)memcpy(authority.principal,
            fixtures[0].account_ids[index % 2U == 0U ? 0U : 2U], 32U);
        execution.authority = &authority;
        (void)memcpy(before, fixtures[0].kernel.current_state_root, 32U);
        for (side = 0U; side < 2U; ++side) {
            integrated_fixture *fixture = &fixtures[side];
            LXP_CASE_CHECK(lxp_arena_reset(&fixture->arena, 0U) == LXP_OK);
            executions[side] = execution;
            executions[side].identities = &fixture->identities;
            executions[side].arena = &fixture->arena;
            work[side].fixture = fixture;
            work[side].activity = &activity;
            work[side].execution = &executions[side];
            work[side].result = LXP_FATAL_INVARIANT;
            if (handoff && side == 1U) {
                pthread_t thread;
                LXP_CASE_CHECK(lxp_state_writer_assert_owner(&fixture->state) == LXP_OK);
                LXP_CASE_CHECK(pthread_create(&thread, NULL, integrated_worker,
                                              &work[side]) == 0);
                LXP_CASE_CHECK(pthread_join(thread, NULL) == 0);
                LXP_CASE_CHECK(work[side].claim == LXP_OK);
                LXP_CASE_CHECK(work[side].owner == LXP_OK);
                LXP_CASE_CHECK(work[side].writer_is_self);
                LXP_CASE_CHECK(lxp_state_writer_assert_owner(&fixture->state) ==
                                LXP_FATAL_INVARIANT);
                LXP_CASE_CHECK(lxp_kernel_claim_writer(&fixture->kernel) == LXP_OK);
            } else {
                work[side].result = lxp_kernel_execute_activity(&fixture->kernel,
                    &activity, &executions[side], &work[side].receipt);
            }
            if (work[side].result != LXP_OK || work[side].receipt.result_code != LXP_OK)
                (void)fprintf(stderr, "seed %llu, sequence %llu, side %zu: execution %d, receipt %d\n",
                    (unsigned long long)seed, (unsigned long long)execution.global_sequence,
                    side, (int)work[side].result, (int)work[side].receipt.result_code);
            LXP_CASE_CHECK(work[side].result == LXP_OK);
            LXP_CASE_CHECK(work[side].receipt.result_code == LXP_OK);
            LXP_CASE_CHECK(work[side].receipt.global_sequence == execution.global_sequence);
            LXP_CASE_CHECK(memcmp(work[side].receipt.previous_state_root, before, 32U) == 0);
            LXP_CASE_CHECK(fixture->state.next_sequence == execution.global_sequence + 1U);
            LXP_CASE_CHECK(fixture->identity->next_sequence == execution.global_sequence);
            LXP_CASE_CHECK(!fixture->journal.open);
            {
                uint8_t root[32];
                LXP_CASE_CHECK(lxp_state_root(&fixture->kernel, root) == LXP_OK);
                LXP_CASE_CHECK(memcmp(root, fixture->kernel.current_state_root, 32U) == 0);
                LXP_CASE_CHECK(memcmp(root, work[side].receipt.resulting_state_root, 32U) == 0);
                LXP_CASE_CHECK(memcmp(root, before, 32U) != 0);
            }
            LXP_CASE_CHECK(lxp_receipt_encode(&work[side].receipt, true,
                &fixture->arena, &encoded[side]) == LXP_OK);
            LXP_CASE_CHECK(lxp_receipt_verify(&work[side].receipt,
                fixture->public_key, &fixture->arena) == LXP_OK);
        }
        LXP_CASE_CHECK(encoded[0].length == encoded[1].length);
        LXP_CASE_CHECK(memcmp(encoded[0].bytes, encoded[1].bytes, encoded[0].length) == 0);
        LXP_CASE_CHECK(memcmp(fixtures[0].kernel.current_state_root,
                              fixtures[1].kernel.current_state_root, 32U) == 0);
        if (index % 2U == 0U) {
            ++sends;
        } else {
            size_t stream = (index / 4U) % INTEGRATED_STREAMS;
            bool paused = index % 4U == 1U;
            if (paused) accrued[stream] += execution.batch_timestamp_ms - last[stream];
            last[stream] = execution.batch_timestamp_ms;
            for (side = 0U; side < 2U; ++side)
                LXP_CASE_CHECK(integrated_stream_check(&fixtures[side], stream,
                    last[stream], accrued[stream], paused) == 0);
            ++stream_activities;
        }
    }
    LXP_CASE_CHECK(sends == 32U && stream_activities == 32U);
    for (side = 0U; side < 2U; ++side) {
        const lx_account *from, *to;
        size_t from_slot, to_slot;
        LXP_CASE_CHECK(lx_account_registry_index_lookup(&fixtures[side].accounts,
            fixtures[side].account_ids[0], &from_slot) == LXP_OK);
        LXP_CASE_CHECK(lx_account_registry_index_lookup(&fixtures[side].accounts,
            fixtures[side].account_ids[1], &to_slot) == LXP_OK);
        from = &fixtures[side].accounts.accounts[from_slot];
        to = &fixtures[side].accounts.accounts[to_slot];
        LXP_CASE_CHECK(from->balance.hi == 0U && from->balance.lo == 9968U);
        LXP_CASE_CHECK(to->balance.hi == 0U && to->balance.lo == 32U);
        LXP_CASE_CHECK(from->next_sequence == 32U);
        LXP_CASE_CHECK(lxp_state_store_destroy(&fixtures[side].state) == LXP_OK);
        lx_account_registry_release(&fixtures[side].accounts);
    }
    (void)printf("%s: seed %llu, 64 equal receipts and roots\n",
        handoff ? "integrated_writer_handoff_equal_roots" :
                  "integrated_permuted_load_equal_roots",
        (unsigned long long)seed);
    free(fixtures);
    free(manifest);
    return 0;
}

static int integrated_permuted_load_equal_roots(void)
{
    static const uint64_t seeds[] = {UINT64_C(0x1003), UINT64_C(0xabcdef),
                                     UINT64_C(0x736565646564)};
    size_t index;
    for (index = 0U; index < sizeof(seeds) / sizeof(seeds[0]); ++index)
        LXP_CASE_CHECK(integrated_run(false, seeds[index]) == 0);
    return 0;
}

static int integrated_writer_handoff_equal_roots(void)
{
    static const uint64_t seeds[] = {UINT64_C(0x1003), UINT64_C(0xabcdef),
                                     UINT64_C(0x736565646564)};
    size_t index;
    for (index = 0U; index < sizeof(seeds) / sizeof(seeds[0]); ++index)
        LXP_CASE_CHECK(integrated_run(true, seeds[index]) == 0);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"integrated_permuted_load_equal_roots", integrated_permuted_load_equal_roots},
        {"integrated_writer_handoff_equal_roots", integrated_writer_handoff_equal_roots}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
