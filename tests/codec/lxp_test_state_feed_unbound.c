#define main unbound_fixture_reference_main
static int unbound_fixture_reference_main(int argc, char **argv) __attribute__((unused));
#include "../programs/test_call_activity.c"
#undef main
#include "layerx/lxp_batch_identity.h"
#include "../../src/modules/programs/storage.h"

#include "lxp_kernel_case.h"
#include "layerx/lxp_history.h"

typedef struct unbound_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lx_account_registry accounts;
    lxp_transfer_asset_state asset;
    lx_programs_transfer_runtime runtime;
    lxp_fee_params fees;
    lxp_arena arena;
    uint8_t arena_bytes[4U * LXP_MAX_ACTIVITY_BYTES];
    lxp_identity *identity;
    lx_account *actor;
    lx_account *payee;
    lx_account *source;
    uint64_t parameters;
    uint8_t owner_key[32];
    uint8_t delegate_key[32];
    uint8_t program[32];
    uint8_t source_id[32];
    uint8_t grant_id[32];
    uint8_t call[1024];
    size_t call_length;
    lxp_authority_grant grant;
    lxp_authority_resolved authority;
    lxp_transfer_allowance allowance;
    lxp_kernel_execution execution;
    lxp_activity activity;
    lxp_receipt receipt;
    uint8_t signature[64];
    lxp_u128 signed_fee_limit;
    unsigned recipient_mode;
} unbound_fixture;

static const uint8_t unbound_owner_seed[32] = {0x33U};
static const uint8_t unbound_delegate_seed[32] = {0x34U};
static const uint8_t unbound_did[] = "did:lxp:metered-call";
static int unbound_account(unbound_fixture *f, const char *name,
                            uint64_t balance, lx_account **account)
{
    uint8_t id[32];
    LXP_CASE_CHECK(lx_account_id_from_string((const uint8_t *)name,
                                            strlen(name), id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, (const uint8_t *)name,
                                   strlen(name), id, 1U,
                                   LX_ACCOUNT_OPEN_GENESIS, NULL, account) ==
                  LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(*account, f->asset.asset_id,
                                              (lxp_u128){0U, balance}, 0U) ==
                  LXP_OK);
    return 0;
}

static int unbound_activity(unbound_fixture *f, uint32_t type,
                             const uint8_t *payload, size_t length,
                             uint8_t marker, bool delegated)
{
    const uint8_t *key = delegated ? f->delegate_key : f->owner_key;
    const uint8_t *seed = delegated ? unbound_delegate_seed : unbound_owner_seed;
    uint8_t digest[32];
    lxp_byte_span encoded;
    EVP_PKEY *signer_key;
    EVP_MD_CTX *signer_context;
    size_t signature_length = sizeof(f->signature);
    LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    fill_activity(&f->activity, type, payload, length, unbound_did,
                   sizeof(unbound_did) - 1U, key);
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->activity.account_sequence = f->identity->next_sequence;
    write_u64(f->activity.idempotency_key + 23U, f->state.next_sequence);
    f->activity.idempotency_key[31] = marker;
    f->activity.fee_limit = lxp_u128_is_zero(f->signed_fee_limit) ?
        (lxp_u128){0U, 67108864U} : f->signed_fee_limit;
    f->activity.signature = (lxp_byte_span){f->signature, sizeof(f->signature)};
    LXP_CASE_CHECK(lxp_hash_payload(payload, length, f->activity.payload_hash) ==
                  LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    signer_key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, 32U);
    signer_context = EVP_MD_CTX_new();
    LXP_CASE_CHECK(signer_key != NULL && signer_context != NULL);
    LXP_CASE_CHECK(EVP_DigestSignInit(signer_context, NULL, NULL, NULL,
                                     signer_key) == 1);
    LXP_CASE_CHECK(EVP_DigestSign(signer_context, f->signature, &signature_length,
                                digest, sizeof(digest)) == 1);
    EVP_MD_CTX_free(signer_context);
    EVP_PKEY_free(signer_key);
    LXP_CASE_CHECK(signature_length == sizeof(f->signature));
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    LXP_CASE_CHECK(lxp_authority_resolve_activity(&f->kernel, f->identity,
        &f->activity, !delegated, true, 10U, 100U, f->state.next_sequence,
        &f->grant, &f->authority) == LXP_OK);
    lxp_authority_allowance_bind(&f->grant, &f->authority, &f->allowance);
    (void)memset(&f->execution, 0, sizeof(f->execution));
    f->execution.network_id = 7U;
    f->execution.batch_number = 1U;
    f->execution.batch_timestamp_ms = 10U;
    f->execution.maximum_timestamp_window = 100U;
    f->execution.global_sequence = f->state.next_sequence;
    f->execution.recorded_module_version = LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION;
    f->execution.recorded_metering_schedule_version = 1U;
    f->execution.recorded_fee_schedule_version = 1U;
    f->execution.parameter_version = 1U;
    f->execution.signature_valid = true;
    f->execution.sequencer_private_key = executed_sequencer_seed;
    f->execution.identities = &f->identities;
    f->execution.authority = &f->authority;
    f->execution.allowance = &f->allowance;
    f->execution.fee_parameters = &f->fees;
    f->execution.fee_balance = f->actor->balance;
    f->execution.gas_limit = 1000000U;
    f->execution.arena = &f->arena;
    LXP_CASE_CHECK(lxp_activity_encode(&f->activity, &f->arena, &encoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_id(encoded.bytes, encoded.length, digest) == LXP_OK);
    LXP_CASE_CHECK(lxp_batch_identity_activity(f->kernel.current_state_root,
        digest, f->state.next_sequence, 1U, f->execution.batch_id) == LXP_OK);
    return 0;
}

static int unbound_fixture_init(unbound_fixture *f)
{
    lx_account *treasury;
    lxp_genesis_manifest manifest = {0};
    lx_programs_fee_genesis_parameters fees = {0};
    LXP_CASE_CHECK(executed_public_key(unbound_owner_seed, f->owner_key) == 0);
    LXP_CASE_CHECK(executed_public_key(unbound_delegate_seed, f->delegate_key) == 0);
    f->asset.asset_id[0] = 9U;
    f->asset.registered = true;
    f->program[0] = 0x47U;
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(unbound_account(f, "agent:did:lxp:metered-call:main",
                                   UINT64_C(1000000000), &f->actor) == 0);
    LXP_CASE_CHECK(unbound_account(f, "agent:did:lxp:metered-payee:main", 0U,
                                   &f->payee) == 0);
    LXP_CASE_CHECK(unbound_account(f, "system:fees", 0U, &treasury) == 0);
    if (f->recipient_mode == 1U) f->payee = f->actor;
    if (f->recipient_mode == 2U) f->payee = treasury;
    f->actor->has_authority_key = true;
    (void)memcpy(f->actor->authority_key, f->owner_key, 32U);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                     &f->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(install_metering_v1(&f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                    programs_module_registration_v4()) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    f->runtime.assets = &f->asset;
    f->runtime.asset_count = 1U;
    f->runtime.fee_schedule = (lx_programs_fee_schedule){
        1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U
    };
    (void)memcpy(f->runtime.occupancy_asset_id, f->asset.asset_id, 32U);
    f->runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;
    f->runtime.metering_schedule_context = &f->kernel;
    f->runtime.resolve_occupancy_parameters = lxp_programs_fee_governance_resolve_runtime;
    f->runtime.occupancy_parameter_context = &f->kernel;
    (void)memcpy(manifest.signer_public_key, f->owner_key, 32U);
    fees.schedule = f->runtime.fee_schedule;
    (void)memcpy(fees.occupancy_asset_id, f->asset.asset_id, 32U);
    fees.target_occupancy_byte_batches = 3U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 1U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 10U;
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(&manifest, &fees) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_materialize(&manifest, &f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_PROGRAMS,
                                                  &f->runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_bind_fee_transaction(&f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
                    lxp_kernel_canonical_ledger_apply) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_register(&f->identities, unbound_did,
        sizeof(unbound_did) - 1U, f->owner_key, &f->identity) == LXP_OK);
    f->identity->revocation_sequence = 1U;
    f->fees.version = 1U;
    f->fees.multiplier_basis_points = 10000U;
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
                                 sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    return 0;
}

static size_t unbound_storage_guest(uint8_t *out)
{
    static const uint8_t header[] = {0, 0x61, 0x73, 0x6d, 1, 0, 0, 0};
    static const uint8_t functions[] = {2, 1, 2};
    static const uint8_t memory[] = {1, 1, 1, 1};
    static const uint8_t storage_body[] = {
        0, 0x41, 1, 0x20, 0, 0x2d, 0, 0, 0x3a, 0, 0,
        0x41, 0, 0x41, 1, 0x41, 1, 0x41, 1, 0x10, 0, 0x0b
    };
    uint8_t section[256];
    size_t cursor = 0, length = 0;
    append_bytes(out, &cursor, header, sizeof(header));
    section[length++] = 3; section[length++] = 0x60;
    section[length++] = 4;
    for (unsigned i = 0; i < 4U; ++i) section[length++] = 0x7f;
    section[length++] = 1; section[length++] = 0x7f;
    section[length++] = 0x60; section[length++] = 1; section[length++] = 0x7f;
    section[length++] = 1; section[length++] = 0x7f;
    section[length++] = 0x60; section[length++] = 2;
    section[length++] = 0x7f; section[length++] = 0x7f;
    section[length++] = 1; section[length++] = 0x7f;
    append_section(out, &cursor, 1, section, length);
    length = 0; section[length++] = 1;
    append_name(section, &length, "layerx_v1");
    append_name(section, &length, "storage_write");
    section[length++] = 0; section[length++] = 0;
    append_section(out, &cursor, 2, section, length);
    append_section(out, &cursor, 3, functions, sizeof(functions));
    append_section(out, &cursor, 5, memory, sizeof(memory));
    length = 0; section[length++] = 3;
    append_name(section, &length, "layerx_reserve");
    section[length++] = 0; section[length++] = 1;
    append_name(section, &length, "layerx_call");
    section[length++] = 0; section[length++] = 2;
    append_name(section, &length, "memory");
    section[length++] = 2; section[length++] = 0;
    append_section(out, &cursor, 7, section, length);
    length = 0; section[length++] = 2;
    section[length++] = 5; section[length++] = 0;
    section[length++] = 0x41; section[length++] = 0x80;
    section[length++] = 8; section[length++] = 0x0b;
    append_u32_leb(section, &length, (uint32_t)sizeof(storage_body));
    append_bytes(section, &length, storage_body, sizeof(storage_body));
    append_section(out, &cursor, 10, section, length);
    { static const uint8_t data[] = {1, 0, 0x41, 0, 0x0b, 2, 'k', 'v'};
      append_section(out, &cursor, 11, data, sizeof(data)); }
    return cursor;
}

typedef struct unbound_publication {
    lxp_log receipts;
    lxp_arena arena;
    uint8_t storage[2U * LXP_MAX_ACTIVITY_BYTES];
    uint64_t last_offset;
    size_t count;
    char directory[128];
} unbound_publication;

typedef struct unbound_feed_store {
    lx_programs_state_feed_store store;
    lxp_log feed_log;
    lxp_log canonical_log;
    lxp_history history;
    pthread_mutex_t mutex;
    lxp_arena arena;
    uint8_t storage[2U * LXP_MAX_ACTIVITY_BYTES];
    char directory[128];
} unbound_feed_store;

static lxp_result unbound_publish(void *context, const lxp_kernel *kernel,
                                  const lxp_activity *activity,
                                  const lxp_receipt *receipt)
{
    unbound_publication *publication = context;
    lxp_byte_span encoded;
    lxp_result status;
    if (memcmp(kernel->current_state_root, receipt->resulting_state_root,
               32U) != 0 || activity->activity_type == 0U)
        return LXP_FATAL_INVARIANT;
    status = lxp_arena_reset(&publication->arena, 0U);
    if (status == LXP_OK)
        status = lxp_receipt_encode(receipt, true, &publication->arena, &encoded);
    if (status == LXP_OK && encoded.length > UINT32_MAX)
        status = LXP_ERR_LENGTH_LIMIT;
    if (status == LXP_OK)
        status = lxp_log_append(&publication->receipts, LXP_LOG_RECEIPT,
            receipt->global_sequence, encoded.bytes, (uint32_t)encoded.length,
            &publication->last_offset);
    if (status == LXP_OK)
        status = lxp_log_write_boundary(&publication->receipts);
    if (status == LXP_OK) ++publication->count;
    return status;
}

static int unbound_publication_open(unbound_fixture *f,
                                    unbound_publication *publication)
{
    char path[192];
    strcpy(publication->directory, "/tmp/lxp-unbound-receipts-XXXXXX");
    LXP_CASE_CHECK(mkdtemp(publication->directory) != NULL);
    LXP_CASE_CHECK(snprintf(path, sizeof(path), "%s/receipts.log",
                           publication->directory) > 0);
    LXP_CASE_CHECK(lxp_log_open_or_create(&publication->receipts, path,
                                         LXP_MAX_BATCH_BODY_BYTES) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&publication->arena, publication->storage,
                                 sizeof(publication->storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_commit_observer(&f->kernel, unbound_publish,
                                                 publication) == LXP_OK);
    return 0;
}

static int unbound_feed_open(unbound_fixture *f, unbound_feed_store *feed)
{
    char path[192], database[192];
    strcpy(feed->directory, "/tmp/lxp-unbound-feed-XXXXXX");
    LXP_CASE_CHECK(mkdtemp(feed->directory) != NULL);
    LXP_CASE_CHECK(pthread_mutex_init(&feed->mutex, NULL) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&feed->arena, feed->storage,
                                 sizeof(feed->storage)) == LXP_OK);
    LXP_CASE_CHECK(snprintf(path, sizeof(path), "%s/feed.log",
                           feed->directory) > 0);
    LXP_CASE_CHECK(lxp_log_open_or_create(&feed->feed_log, path,
                                         LXP_MAX_BATCH_BODY_BYTES) == LXP_OK);
    LXP_CASE_CHECK(snprintf(path, sizeof(path), "%s/canonical.log",
                           feed->directory) > 0);
    LXP_CASE_CHECK(lxp_log_open_or_create(&feed->canonical_log, path,
                                         LXP_MAX_BATCH_BODY_BYTES) == LXP_OK);
    LXP_CASE_CHECK(snprintf(database, sizeof(database), "%s/history.db",
                           feed->directory) > 0);
    LXP_CASE_CHECK(lxp_history_open(&feed->history, &feed->canonical_log,
        database, "migrations/0007_history_index.sql") == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_state_feed_store_open(&feed->store,
        &feed->feed_log, &feed->canonical_log, &feed->history, &feed->arena,
        &feed->mutex) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_state_feed_store_anchor(&feed->store,
        f->state.next_sequence, f->kernel.current_state_root) == LXP_OK);
    f->runtime.state_feed = &feed->store.feed;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_PROGRAMS, &f->runtime) == LXP_OK);
    return 0;
}

static int unbound_remove_file(const char *directory, const char *name)
{
    char path[192];
    LXP_CASE_CHECK(snprintf(path, sizeof(path), "%s/%s", directory, name) > 0);
    LXP_CASE_CHECK(unlink(path) == 0);
    return 0;
}

static int unbound_publication_close(unbound_publication *publication)
{
    LXP_CASE_CHECK(lxp_log_close(&publication->receipts) == LXP_OK);
    LXP_CASE_CHECK(unbound_remove_file(publication->directory, "receipts.log") == 0);
    LXP_CASE_CHECK(rmdir(publication->directory) == 0);
    return 0;
}

static int unbound_feed_close(unbound_feed_store *feed)
{
    LXP_CASE_CHECK(lxp_history_close(&feed->history) == LXP_OK);
    LXP_CASE_CHECK(lxp_log_close(&feed->feed_log) == LXP_OK);
    LXP_CASE_CHECK(lxp_log_close(&feed->canonical_log) == LXP_OK);
    LXP_CASE_CHECK(pthread_mutex_destroy(&feed->mutex) == 0);
    LXP_CASE_CHECK(unbound_remove_file(feed->directory, "feed.log") == 0);
    LXP_CASE_CHECK(unbound_remove_file(feed->directory, "canonical.log") == 0);
    LXP_CASE_CHECK(unbound_remove_file(feed->directory, "history.db") == 0);
    LXP_CASE_CHECK(rmdir(feed->directory) == 0);
    return 0;
}

static int unbound_deploy(unbound_fixture *f)
{
    uint8_t wasm[512], payload[1024], code_hash[32];
    size_t wasm_length = unbound_storage_guest(wasm);
    size_t length = program_spend_deploy_payload(payload, f->program,
        f->identity->did_id, wasm, wasm_length, code_hash);
    LXP_CASE_CHECK(unbound_activity(f, LX_PROGRAMS_DEPLOY, payload, length,
                                   1U, false) == 0);
    LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt) == LXP_OK);
    LXP_CASE_CHECK(f->receipt.result_code == LXP_OK);
    return 0;
}

static int unbound_call(unbound_fixture *f, uint8_t value)
{
    static const uint8_t capabilities[] = {0U, 1U, 2U};
    static const uint8_t access[] = "LayerX/programs/access-declaration/v1\0";
    uint8_t payload[1024], namespace_bytes[65] = {0}, root[32];
    uint8_t storage[65536];
    lxp_arena arena;
    lxp_module_ctx context;
    const uint8_t *key, *stored_value;
    uint16_t key_length;
    uint32_t value_length, cell_count;
    size_t length = call_payload_with_data(payload, f->program,
        capabilities, sizeof(capabilities), access, sizeof(access), &value, 1U);
    write_u16(payload + 32U, LX_PROGRAMS_ACCOUNT_ABI_VERSION);
    LXP_CASE_CHECK(unbound_activity(f, LX_PROGRAMS_CALL, payload, length,
                                   value, false) == 0);
    LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt) == LXP_OK);
    LXP_CASE_CHECK(f->receipt.result_code == LXP_OK);
    LXP_CASE_CHECK(f->receipt.program_outcome.present);
    LXP_CASE_CHECK(f->receipt.program_outcome.terminal_kind ==
                   LXP_PROGRAM_TERMINAL_SUCCESS);
    LXP_CASE_CHECK(f->receipt.program_outcome.storage_write_bytes > 0U);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root, f->kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(root, f->receipt.resulting_state_root, 32U) == 0);
    memcpy(namespace_bytes, f->program, 32U);
    memcpy(namespace_bytes + 33U, f->identity->did_id, 32U);
    LXP_CASE_CHECK(lxp_arena_init(&arena, storage, sizeof(storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&context, &f->kernel, LXP_MODULE_PROGRAMS,
        10U, 0U, f->state.next_sequence, 1000000U, &arena, false) == LXP_OK);
    context.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    lxp_result status = lxp_programs_storage_cell_at(&context, namespace_bytes,
        sizeof(namespace_bytes), 0U, &key, &key_length, &stored_value,
        &value_length, &cell_count);
    bool exact = status == LXP_OK && cell_count == 1U && key_length == 1U &&
        key[0] == 'k' && value_length == 1U && stored_value[0] == value;
    lxp_module_ctx_rollback(&context);
    LXP_CASE_CHECK(exact);
    return 0;
}

static int unbound_fixture_close(unbound_fixture *f)
{
    while (f->kernel.blob_count != 0U)
        free(f->kernel.blobs[--f->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    free(f);
    return 0;
}

static int programs_runtime_without_feed(void)
{
    unbound_fixture *f = calloc(1U, sizeof(*f));
    unbound_publication *publication = calloc(1U, sizeof(*publication));
    lxp_kernel_prepared_batch *batch = NULL;
    uint8_t registration[74], account_id[32];
    LXP_CASE_CHECK(f != NULL && publication != NULL);
    LXP_CASE_CHECK(unbound_fixture_init(f) == 0);
    LXP_CASE_CHECK(f->runtime.state_feed == NULL);
    LXP_CASE_CHECK(f->kernel.observe_commit == NULL);
    LXP_CASE_CHECK(unbound_publication_open(f, publication) == 0);
    LXP_CASE_CHECK(unbound_deploy(f) == 0);
    memcpy(registration, f->program, 32U);
    memcpy(registration + 32U, "LXPA1", 5U);
    memcpy(registration + 37U, f->asset.asset_id, 32U);
    write_u32(registration + 69U, 1U);
    registration[73] = 's';
    LXP_CASE_CHECK(unbound_activity(f, LX_PROGRAMS_ACCOUNT, registration,
        sizeof(registration), 2U, false) == 0);
    LXP_CASE_CHECK(lxp_kernel_prepare_serial_activity_batch(&f->kernel,
        &f->activity, &f->execution, &batch) == LXP_OK);
    LXP_CASE_CHECK(batch != NULL && lxp_kernel_prepared_batch_count(batch) == 1U);
    LXP_CASE_CHECK(lxp_kernel_prepared_batch_receipts(batch)[0].result_code == LXP_OK);
    LXP_CASE_CHECK(publication->count == 1U);
    lxp_kernel_prepared_batch_destroy(batch);
    LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt) == LXP_OK);
    LXP_CASE_CHECK(f->receipt.result_code == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_account_derive(f->program, registration + 73U,
        1U, account_id) == LXP_OK);
    LXP_CASE_CHECK(program_spend_account(&f->accounts, account_id) != NULL);
    LXP_CASE_CHECK(unbound_call(f, 3U) == 0);
    LXP_CASE_CHECK(f->runtime.state_feed == NULL);
    LXP_CASE_CHECK(publication->count == 3U);
    LXP_CASE_CHECK(unbound_publication_close(publication) == 0);
    free(publication);
    LXP_CASE_CHECK(unbound_fixture_close(f) == 0);
    return 0;
}

static int feed_does_not_change_roots(void)
{
    unbound_fixture *plain = calloc(1U, sizeof(*plain));
    unbound_fixture *bound = calloc(1U, sizeof(*bound));
    unbound_publication *plain_log = calloc(1U, sizeof(*plain_log));
    unbound_publication *bound_log = calloc(1U, sizeof(*bound_log));
    unbound_feed_store *feed = calloc(1U, sizeof(*feed));
    uint8_t prior_root[32];
    uint64_t feed_offset, canonical_offset;
    LXP_CASE_CHECK(plain != NULL && bound != NULL && plain_log != NULL &&
                   bound_log != NULL && feed != NULL);
    LXP_CASE_CHECK(unbound_fixture_init(plain) == 0);
    LXP_CASE_CHECK(unbound_fixture_init(bound) == 0);
    LXP_CASE_CHECK(unbound_publication_open(plain, plain_log) == 0);
    LXP_CASE_CHECK(unbound_publication_open(bound, bound_log) == 0);
    LXP_CASE_CHECK(unbound_feed_open(bound, feed) == 0);
    LXP_CASE_CHECK(plain->runtime.state_feed == NULL);
    LXP_CASE_CHECK(bound->runtime.state_feed == &feed->store.feed);
    LXP_CASE_CHECK(bound->kernel.observe_commit == unbound_publish);
    LXP_CASE_CHECK(bound->kernel.commit_observer_context == bound_log);
    feed_offset = feed->feed_log.write_offset;
    canonical_offset = feed->canonical_log.write_offset;
    LXP_CASE_CHECK(unbound_deploy(plain) == 0);
    LXP_CASE_CHECK(unbound_deploy(bound) == 0);
    LXP_CASE_CHECK(memcmp(plain->kernel.current_state_root,
                          bound->kernel.current_state_root, 32U) == 0);
    for (uint8_t call = 1U; call <= 16U; ++call) {
        lxp_log_record_header plain_header, bound_header;
        uint8_t plain_bytes[LXP_MAX_ACTIVITY_BYTES];
        uint8_t bound_bytes[LXP_MAX_ACTIVITY_BYTES];
        memcpy(prior_root, plain->kernel.current_state_root, 32U);
        LXP_CASE_CHECK(unbound_call(plain, call) == 0);
        LXP_CASE_CHECK(unbound_call(bound, call) == 0);
        LXP_CASE_CHECK(memcmp(prior_root, plain->kernel.current_state_root, 32U) != 0);
        LXP_CASE_CHECK(memcmp(plain->kernel.current_state_root,
                              bound->kernel.current_state_root, 32U) == 0);
        LXP_CASE_CHECK(lxp_log_read(&plain_log->receipts, plain_log->last_offset,
            &plain_header, plain_bytes, sizeof(plain_bytes)) == LXP_OK);
        LXP_CASE_CHECK(lxp_log_read(&bound_log->receipts, bound_log->last_offset,
            &bound_header, bound_bytes, sizeof(bound_bytes)) == LXP_OK);
        LXP_CASE_CHECK(plain_header.record_kind == LXP_LOG_RECEIPT &&
                       bound_header.record_kind == LXP_LOG_RECEIPT);
        LXP_CASE_CHECK(plain_header.body_length == bound_header.body_length);
        LXP_CASE_CHECK(plain_header.body_length > 0U);
        LXP_CASE_CHECK(memcmp(plain_bytes, bound_bytes, plain_header.body_length) == 0);
    }
    LXP_CASE_CHECK(plain_log->count == 17U && bound_log->count == 17U);
    LXP_CASE_CHECK(feed->feed_log.write_offset == feed_offset);
    LXP_CASE_CHECK(feed->canonical_log.write_offset == canonical_offset);
    LXP_CASE_CHECK(unbound_feed_close(feed) == 0);
    LXP_CASE_CHECK(unbound_publication_close(plain_log) == 0);
    LXP_CASE_CHECK(unbound_publication_close(bound_log) == 0);
    free(feed);
    free(plain_log);
    free(bound_log);
    LXP_CASE_CHECK(unbound_fixture_close(plain) == 0);
    LXP_CASE_CHECK(unbound_fixture_close(bound) == 0);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "programs_runtime_without_feed", programs_runtime_without_feed },
        { "feed_does_not_change_roots", feed_does_not_change_roots }
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
