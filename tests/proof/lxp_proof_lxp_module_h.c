/* Proves: signed asset activities -> include/layerx/lxp_module.h staging and receipt facts -> asset_execute_typed and layerx_programs_call_arbiter_authority_begin.
 * lxp_ctx_asset_issuance_stage/lxp_ctx_asset_account_stage and
 * lxp_ctx_verified_receipt_authority_facts ->
 * src/modules/asset/lx_asset_execution.h asset_execute_typed
 * and src/modules/programs/call.c layerx_programs_call_arbiter_authority_begin.
 * Repairs asset_execute_typed's NULL authority-byte dereference and confines
 * the Programs authority view to a Programs context (including cache clearing).
 * src/modules/asset/lx_asset_registry.c module_decode deliberately refuses the
 * retired grant activities; lxp_ctx_asset_grant_state_stage was never built
 * (tests/codec/lxp_test_receive_retired.c). Assert absence, never resurrect it.
 * State crosses destruction/recreation through lxp_snapshot_write/load; the
 * receipt index is rebuilt from the actual account-open consumer's receipt.
 */
#include "../codec/lxp_kernel_case.h"
#include "layerx/lx_asset.h"
#include "layerx/lxp_identity.h"
#include "layerx/lxp_snapshot.h"

/* Use the real C host bridge and its private application state, as in
 * tests/codec/lxp_test_receipt_authority_v2.c; no transport is replaced. */
#include "../../src/modules/programs/call.c"
#include <openssl/evp.h>

extern void lxp_ctx_asset_grant_state_stage(void) __attribute__((weak));

static const uint8_t proof_did[] = "did:key:module-proof";
static const uint8_t proof_seed[32] = {17U};

typedef struct proof_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lx_asset_runtime runtime;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_activity activity;
    lxp_authority_resolved authority;
    lxp_arena arena;
    lxp_verified_receipt_index index;
    lxp_programs_call_activity call;
    lxp_receipt receipt;
    uint8_t asset_id[32];
    uint8_t account_id[32];
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[89];
    uint8_t receipt_bytes[4096];
    size_t receipt_length;
    uint8_t receipt_digest[32];
    uint8_t work[1048576];
    uint8_t wire[LXP_MAX_ACTIVITY_BYTES];
} proof_fixture;

static int proof_open(proof_fixture *f)
{
    static const uint64_t parameters = 1U;
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                     &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                              lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                      programs_module_registration()) == LXP_OK);
    (void)memset(&f->runtime, 0, sizeof(f->runtime));
    return 0;
}

static int proof_bind_runtime(proof_fixture *f)
{
    size_t i;
    /* Bootstrap with signed native registration, then use its emitted record
     * for the runtime. An empty runtime is rejected by the kernel. */
    for (i = 0U; i < f->kernel.module_kv_count; ++i) {
        const lxp_module_kv_entry *entry = &f->kernel.module_kv[i];
        if (entry->module_id == LXP_MODULE_ASSET && entry->key_length == 38U &&
            memcmp(entry->key, "asset:", 6U) == 0 &&
            memcmp(entry->key + 6U, f->asset_id, 32U) == 0) {
            LXP_CASE_CHECK(lx_asset_record_decode(entry->value, entry->value_length,
                                                   &f->asset) == LXP_OK);
            break;
        }
    }
    LXP_CASE_CHECK(i < f->kernel.module_kv_count);
    LXP_CASE_CHECK(lx_asset_transfer_state(&f->asset, &f->transfer_asset) == LXP_OK);
    f->runtime = (lx_asset_runtime){&f->accounts, &f->asset, 1U,
        &f->transfer_asset, 1U, 7U, LXP_PROTOCOL_VERSION_OCCUPANCY, false};
    LXP_CASE_CHECK(!f->runtime.binding_authority);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET,
                                                   &f->runtime) == LXP_OK);
    return 0;
}

static int proof_close(proof_fixture *f)
{
    LXP_CASE_CHECK(!f->journal.open);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    return 0;
}

static int proof_restart(proof_fixture *f)
{
    uint8_t *storage = malloc(sizeof(f->work));
    uint8_t before[32], after[32];
    lxp_snapshot_manifest_record manifest;
    lxp_byte_span snapshot;
    lxp_arena arena;
    uint64_t sequence = f->state.next_sequence - 1U;
    LXP_CASE_CHECK(storage != NULL);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, storage, sizeof(f->work)) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_write(&f->kernel, sequence, &arena, &snapshot) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_manifest(snapshot.bytes, snapshot.length,
                         sequence, before, f->receipt_length == 0U ? before :
                         f->receipt_digest, &manifest) == LXP_OK);
    LXP_CASE_CHECK(proof_close(f) == 0);
    (void)memset(&f->kernel, 0, sizeof(f->kernel));
    (void)memset(&f->ctx, 0, sizeof(f->ctx));
    (void)memset(&f->journal, 0, sizeof(f->journal));
    (void)memset(&f->index, 0, sizeof(f->index));
    (void)memset(&f->call, 0, sizeof(f->call));
    LXP_CASE_CHECK(proof_open(f) == 0);
    LXP_CASE_CHECK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
                                     &manifest, &f->kernel) == LXP_OK);
    LXP_CASE_CHECK(proof_bind_runtime(f) == 0);
    LXP_CASE_CHECK(lxp_snapshot_verify_root(&f->kernel, &manifest) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(f->state.next_sequence == sequence + 1U);
    free(storage);
    return 0;
}

static int proof_prepare(proof_fixture *f, uint32_t type)
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                 proof_seed, 32U);
    EVP_MD_CTX *signer = EVP_MD_CTX_new();
    uint8_t digest[32];
    size_t key_length = 32U, signature_length = 64U;
    lxp_arena wire_arena;
    lxp_byte_span wire;
    LXP_CASE_CHECK(key != NULL && signer != NULL);
    LXP_CASE_CHECK(EVP_PKEY_get_raw_public_key(key, f->public_key, &key_length) == 1);
    LXP_CASE_CHECK(key_length == 32U);
    (void)memset(&f->authority, 0, sizeof(f->authority));
    f->authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    LXP_CASE_CHECK(lxp_did_id_derive(proof_did, sizeof(proof_did) - 1U,
                                     f->authority.actor) == LXP_OK);
    (void)memcpy(f->authority.principal, f->authority.actor, 32U);
    (void)memset(f->payload, 0, sizeof(f->payload));
    f->payload[1] = 1U;
    if (type == LX_ASSET_REGISTER) {
        lxp_hash_context hash;
        lxp_hash_init(&hash);
        LXP_CASE_CHECK(lxp_hash_update(&hash, (const uint8_t *)"LX:ASSET:v1", 11U) == LXP_OK);
        LXP_CASE_CHECK(lxp_hash_update(&hash, f->authority.actor, 32U) == LXP_OK);
        LXP_CASE_CHECK(lxp_hash_update(&hash, f->payload + 34U, 32U) == LXP_OK);
        LXP_CASE_CHECK(lxp_hash_final(&hash, f->asset_id) == LXP_OK);
        f->payload[66] = 1U; f->payload[67] = 'P';
        f->payload[68] = 1U; f->payload[69] = 'P';
        f->payload[86] = 100U; f->payload[87] = 1U;
    }
    (void)memcpy(f->payload + 2U, f->asset_id, 32U);
    (void)memset(&f->activity, 0, sizeof(f->activity));
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    f->activity.network_id = 7U;
    f->activity.activity_type = type;
    f->activity.actor_did = (lxp_byte_span){proof_did, sizeof(proof_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.payload = (lxp_byte_span){f->payload,
        type == LX_ASSET_REGISTER ? sizeof(f->payload) : 34U};
    f->activity.timestamp_bound.not_after = 100U;
    f->activity.idempotency_key[0] = (uint8_t)type;
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, f->activity.payload.length,
                                    f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    LXP_CASE_CHECK(EVP_DigestSignInit(signer, NULL, NULL, NULL, key) == 1);
    LXP_CASE_CHECK(EVP_DigestSign(signer, f->signature, &signature_length, digest, 32U) == 1);
    EVP_MD_CTX_free(signer);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(signature_length == 64U);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&wire_arena, f->wire, sizeof(f->wire)) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_encode(&f->activity, &wire_arena, &wire) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->work, sizeof(f->work)) == LXP_OK);
    LXP_CASE_CHECK(lxp_effect_buffer_init(&f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_open(&f->state, f->state.next_sequence,
                                          &f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_ASSET,
          10U, 0U, f->state.next_sequence, 1000000U, &f->arena, true) == LXP_OK);
    f->ctx.protocol_version = f->activity.protocol_version;
    f->ctx.activity = &f->activity;
    LXP_CASE_CHECK(lxp_activity_id(wire.bytes, wire.length, f->ctx.activity_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&f->ctx, &f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_ledger_admission(&f->ctx, &f->authority, type) == LXP_OK);
    return 0;
}

static int proof_discard(proof_fixture *f)
{
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(lxp_state_journal_rollback(&f->journal) == LXP_OK);
    return 0;
}

static int proof_submit(proof_fixture *f, uint32_t type, lxp_result expected)
{
    const lxp_module_registration *registration;
    lxp_result result;
    uint8_t before[32], after[32];
    lxp_ledger_receipt_input input;
    lxp_byte_span encoded;
    LXP_CASE_CHECK(proof_prepare(f, type) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&f->kernel, type, 0U,
                                                  &registration) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_dispatch(registration, &f->ctx, &f->activity,
                         &f->authority, &f->effects, &result) == LXP_OK);
    LXP_CASE_CHECK(result == expected);
    if (expected != LXP_OK) {
        LXP_CASE_CHECK(proof_discard(f) == 0);
        LXP_CASE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
        LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
        return 0;
    }
    LXP_CASE_CHECK(f->ctx.staged_account_count == 1U);
    LXP_CASE_CHECK(f->effects.count == 1U);
    if (type == LX_ASSET_ACCOUNT_OPEN) {
        LXP_CASE_CHECK(f->ctx.ledger_receipt_present);
        input = f->ctx.ledger_receipt;
        (void)memcpy(f->account_id, f->ctx.staged_accounts[0].account.id, 32U);
    }
    LXP_CASE_CHECK(lxp_module_ctx_prepare_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    if (type == LX_ASSET_REGISTER)
        LXP_CASE_CHECK(proof_bind_runtime(f) == 0);
    if (type == LX_ASSET_ACCOUNT_OPEN) {
        (void)memcpy(input.previous_state_root, before, 32U);
        LXP_CASE_CHECK(lxp_state_root(&f->kernel, input.resulting_state_root) == LXP_OK);
        LXP_CASE_CHECK(lxp_ledger_receipt_build(&f->receipt, &input) == LXP_OK);
        /* Commit has consumed the decoded activity. Receipt encoding reserves
         * LXP_MAX_ACTIVITY_BYTES, so release that activity's scratch first. */
        LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
        LXP_CASE_CHECK(lxp_receipt_sign(&f->receipt, proof_seed, &f->arena) == LXP_OK);
        LXP_CASE_CHECK(lxp_receipt_verify(&f->receipt, f->public_key, &f->arena) == LXP_OK);
        LXP_CASE_CHECK(lxp_receipt_digest(&f->receipt, &f->arena, f->receipt_digest) == LXP_OK);
        LXP_CASE_CHECK(lxp_receipt_encode(&f->receipt, true, &f->arena, &encoded) == LXP_OK);
        LXP_CASE_CHECK(encoded.length <= sizeof(f->receipt_bytes));
        (void)memcpy(f->receipt_bytes, encoded.bytes, encoded.length);
        f->receipt_length = encoded.length;
    }
    return 0;
}

static int proof_consumer(proof_fixture *f)
{
    lxp_byte_span canonical = {f->receipt_bytes, f->receipt_length};
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->work, sizeof(f->work)) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_init(&f->index) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(&f->index, 7U) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
                                  &f->index, canonical, 2U, &f->arena) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
                                  &f->index, canonical, 2U, &f->arena) == LXP_OK);
    LXP_CASE_CHECK(f->index.authority_count == 1U);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_PROGRAMS,
                  10U, 0U, f->state.next_sequence, 1000000U, &f->arena, false) == LXP_OK);
    f->ctx.batch_number = 3U;
    f->ctx.verified_receipts = &f->index;
    (void)memset(&f->call, 0, sizeof(f->call));
    f->call.ctx = &f->ctx;
    f->call.authority = &f->authority;
    f->call.network_id = 7U;
    return 0;
}

static lxp_result proof_begin(proof_fixture *f, uint64_t height)
{
    return layerx_programs_call_arbiter_authority_begin((uint64_t)(uintptr_t)&f->call,
        read_u64(f->receipt_digest), read_u64(f->receipt_digest + 8U),
        read_u64(f->receipt_digest + 16U), read_u64(f->receipt_digest + 24U), height);
}

static int proof_lxp_module_h_happy_path(void)
{
    proof_fixture *f = calloc(1U, sizeof(*f));
    lx_account *account = NULL;
    uint8_t issuance_name[LX_ASSET_ISSUANCE_NAME_BYTES], issuance_id[32];
    uint8_t expected[151] = {0};
    lxp_verified_receipt_authority_facts facts;
    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(proof_open(f) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_REGISTER, LXP_OK) == 0);
    LXP_CASE_CHECK(proof_restart(f) == 0);
    LXP_CASE_CHECK(proof_prepare(f, LX_ASSET_REGISTER) == 0);
    LXP_CASE_CHECK(lxp_ctx_asset_issuance_stage(&f->ctx, &f->activity,
                     &f->authority, &account) == LXP_ERR_ASSET_ALREADY_REGISTERED);
    LXP_CASE_CHECK(account == NULL && f->ctx.staged_account_count == 0U);
    LXP_CASE_CHECK(proof_discard(f) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_REGISTER, LXP_ERR_ASSET_ALREADY_REGISTERED) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_ACCOUNT_OPEN, LXP_OK) == 0);
    LXP_CASE_CHECK(proof_restart(f) == 0);
    LXP_CASE_CHECK(proof_prepare(f, LX_ASSET_ACCOUNT_OPEN) == 0);
    LXP_CASE_CHECK(lxp_ctx_asset_account_stage(&f->ctx, &f->activity,
                          &f->authority, &account) == LXP_ERR_CONTEXT_MISMATCH);
    LXP_CASE_CHECK(account == NULL && f->ctx.staged_account_count == 0U);
    LXP_CASE_CHECK(lxp_ctx_account_find(&f->ctx, f->account_id, &account) == LXP_OK);
    LXP_CASE_CHECK(account->kind == LX_ACCOUNT_AGENT_ASSET && account->has_asset);
    LXP_CASE_CHECK(memcmp(account->asset_id, f->asset_id, 32U) == 0);
    LXP_CASE_CHECK(account->has_authority_key && memcmp(account->authority_key, f->public_key, 32U) == 0);
    LXP_CASE_CHECK(lxp_u128_is_zero(account->balance) && account->created_at_sequence == 2U);
    LXP_CASE_CHECK(lx_asset_issuance_name(f->asset_id, issuance_name, issuance_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_account_find(&f->ctx, issuance_id, &account) == LXP_OK);
    LXP_CASE_CHECK(account->kind == LX_ACCOUNT_MODULE_VALUE && account->balance.hi == 0U && account->balance.lo == 100U);
    LXP_CASE_CHECK(f->accounts.count == 2U);
    LXP_CASE_CHECK(proof_discard(f) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_ACCOUNT_OPEN, LXP_ERR_CONTEXT_MISMATCH) == 0);
    LXP_CASE_CHECK(proof_consumer(f) == 0);
    LXP_CASE_CHECK(lxp_ctx_verified_receipt_authority_facts(&f->ctx, f->receipt_digest, &facts) == LXP_OK);
    LXP_CASE_CHECK(facts.version == 2U && facts.network_id == 7U && facts.block_height == 2U);
    LXP_CASE_CHECK(facts.module_id == LXP_MODULE_ASSET && facts.operation == 4U && facts.result_code == LXP_OK);
    LXP_CASE_CHECK(memcmp(facts.activity_id, f->receipt.activity_id, 32U) == 0);
    expected[1] = 2U; expected[5] = 7U;
    expected[6] = (uint8_t)(f->receipt.protocol_version >> 8U);
    expected[7] = (uint8_t)f->receipt.protocol_version;
    expected[9] = LXP_MODULE_ASSET; expected[10] = 4U; expected[22] = 2U;
    (void)memcpy(expected + 23U, f->receipt_digest, 32U);
    (void)memcpy(expected + 55U, f->receipt.activity_id, 32U);
    (void)memcpy(expected + 87U, f->receipt.previous_state_root, 32U);
    (void)memcpy(expected + 119U, f->receipt.resulting_state_root, 32U);
    for (unsigned int attempt = 0U; attempt < 2U; ++attempt) {
        LXP_CASE_CHECK(proof_begin(f, 2U) == LXP_OK);
        for (uint32_t i = 0U; i < sizeof(expected); ++i)
            LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
                          (uint64_t)(uintptr_t)&f->call, i) == (lxp_result)expected[i]);
        LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
                          (uint64_t)(uintptr_t)&f->call, 151U) == LXP_ERR_TRUNCATED);
    }
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(proof_close(f) == 0);
    free(f);
    return 0;
}

static int proof_lxp_module_h_refusal(void)
{
    proof_fixture *f = calloc(1U, sizeof(*f));
    const lxp_module_iface *iface = lx_asset_module_iface();
    const uint32_t retired[] = {LX_ASSET_RECEIVE, LX_ASSET_GRANT_ISSUE, LX_ASSET_GRANT_REVOKE};
    const lxp_module_registration *registration = NULL;
    void *decoded = NULL;
    uint8_t before[32], after[32];
    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(proof_open(f) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_REGISTER, LXP_OK) == 0);
    LXP_CASE_CHECK(proof_restart(f) == 0);
    LXP_CASE_CHECK(proof_prepare(f, LX_ASSET_ACCOUNT_OPEN) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    LXP_CASE_CHECK(iface->decode(&f->ctx, 4U, f->payload, 34U, &decoded) == LXP_OK);
    f->activity.authority.bytes = NULL;
    LXP_CASE_CHECK(iface->execute(&f->ctx, &f->activity, &f->authority, decoded,
                                 &f->effects) == LXP_ERR_UNAUTHORIZED_DEBIT);
    f->activity.authority.bytes = f->public_key;
    f->authority.verified_key[0] ^= 1U;
    LXP_CASE_CHECK(iface->execute(&f->ctx, &f->activity, &f->authority, decoded,
                                 &f->effects) == LXP_ERR_UNAUTHORIZED_DEBIT);
    LXP_CASE_CHECK(f->ctx.staged_account_count == 0U && f->effects.count == 0U);
    LXP_CASE_CHECK(proof_discard(f) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(lxp_ctx_asset_grant_state_stage == NULL);
    for (size_t i = 0U; i < sizeof(retired) / sizeof(retired[0]); ++i) {
        LXP_CASE_CHECK(lxp_kernel_module_for_activity(&f->kernel, retired[i], 0U,
                                     &registration) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(proof_prepare(f, retired[i]) == 0);
        decoded = NULL;
        LXP_CASE_CHECK(iface->decode(&f->ctx, lxp_activity_type_ordinal(retired[i]),
                            f->payload, 34U, &decoded) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(decoded == NULL && f->effects.count == 0U);
        LXP_CASE_CHECK(proof_discard(f) == 0);
    }
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_ACCOUNT_OPEN, LXP_OK) == 0);
    LXP_CASE_CHECK(proof_restart(f) == 0);
    LXP_CASE_CHECK(proof_consumer(f) == 0);
    LXP_CASE_CHECK(proof_begin(f, 2U) == LXP_OK);
    f->ctx.module_id = LXP_MODULE_ASSET;
    LXP_CASE_CHECK(proof_begin(f, 2U) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
                      (uint64_t)(uintptr_t)&f->call, 0U) == LXP_ERR_UNKNOWN_FIELD);
    for (size_t i = 0U; i < sizeof(f->call.arbiter_authority); ++i)
        LXP_CASE_CHECK(f->call.arbiter_authority[i] == 0U);
    f->ctx.module_id = LXP_MODULE_PROGRAMS;
    LXP_CASE_CHECK(proof_begin(f, 2U) == LXP_OK);
    LXP_CASE_CHECK(proof_begin(f, 1U) == LXP_ERR_ROOT_MISMATCH);
    f->call.network_id = 8U;
    LXP_CASE_CHECK(proof_begin(f, 2U) == LXP_ERR_WRONG_NETWORK);
    f->call.network_id = 7U;
    f->ctx.batch_number = 2U;
    LXP_CASE_CHECK(proof_begin(f, 2U) == LXP_ERR_NOT_YET_VALID);
    f->ctx.batch_number = 3U;
    f->receipt_digest[0] ^= 1U;
    LXP_CASE_CHECK(proof_begin(f, 2U) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
                      (uint64_t)(uintptr_t)&f->call, 0U) == LXP_ERR_UNKNOWN_FIELD);
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(proof_close(f) == 0);
    free(f);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"proof_lxp_module_h_happy_path", proof_lxp_module_h_happy_path},
        {"proof_lxp_module_h_refusal", proof_lxp_module_h_refusal}
    };
    char *all[] = {argv[0], (char *)cases[0].name, (char *)cases[1].name};
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc == 1 ? 3 : argc, argc == 1 ? all : argv);
}
