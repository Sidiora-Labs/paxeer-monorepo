/* Proves: include/layerx/lx_asset.h signed registration/mint/pause -> lxp_kernel_dispatch and lxp_snapshot_write/load -> lx_asset_runtime_restore, lx_asset_state_root and budget_asset_state.
 * Repairs unchecked state-key/record-ID binding in lx_asset_execution.h
 * asset_load, lx_budget_module.c budget_asset_state and lxp_module_ctx.c
 * asset_staged_record using lx_asset_record_decode_for_id. The host in
 * modules/lxphost/host.go uses the same inventory restoration as this proof.
 * The historical lx_asset_payer_grant_read and lx_asset_payer_grant_encode
 * inventory entries conflict with receive-grant retirement (req 11043).
 * tests/codec/lxp_test_receive_retired.c grant_store_symbols_absent requires
 * both symbols absent; assert that behaviour and real module refusals here.
 * No retired producer or transport is recreated. The four live producers
 * exercised below are registration, account opening, minting and pausing.
 */
#include "../codec/lxp_kernel_case.h"
#include "layerx/lx_asset.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_identity.h"
#include "layerx/lxp_snapshot.h"

#include <openssl/evp.h>
#include <stdlib.h>

/* Compile the actual budget consumer to reach its private asset-state reader;
 * its public module registration also serves this kernel. No double is used. */
#include "../../src/modules/budget/lx_budget_module.c"

struct lxp_grant_state;
extern lxp_result lx_asset_payer_grant_read(const lxp_kernel *,
    const uint8_t[32], struct lxp_grant_state *) __attribute__((weak));
extern void lx_asset_payer_grant_encode(void) __attribute__((weak));

static const uint8_t proof_did[] = "did:key:asset-proof";
static const uint8_t proof_seed[32] = {29U};

typedef struct proof_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lx_asset_registry assets;
    lxp_transfer_asset_state transfer_assets[LX_ASSET_REGISTRY_CAPACITY];
    lx_asset_runtime runtime;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_activity activity;
    lxp_authority_resolved authority;
    lxp_arena arena;
    uint8_t asset_id[32];
    uint8_t account_id[32];
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[89];
    uint8_t work[1048576];
    uint8_t wire[LXP_MAX_ACTIVITY_BYTES];
} proof_fixture;

static int proof_open(proof_fixture *f)
{
    static const uint64_t parameters = 1U;
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_registry_init(&f->assets, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                    &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                             lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                             lx_budget_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
                                   lxp_kernel_canonical_ledger_apply) == LXP_OK);
    return 0;
}

static int proof_restore(proof_fixture *f)
{
    f->runtime.network_id = 7U;
    f->runtime.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    f->runtime.binding_authority = false;
    LXP_CASE_CHECK(lx_asset_runtime_restore(&f->kernel, &f->assets,
                              f->transfer_assets, &f->runtime) == LXP_OK);
    LXP_CASE_CHECK(f->assets.count == 1U && f->runtime.asset_count == 1U);
    LXP_CASE_CHECK(f->runtime.accounts == &f->accounts);
    LXP_CASE_CHECK(f->runtime.assets == f->assets.assets);
    LXP_CASE_CHECK(f->runtime.transfer_assets == f->transfer_assets);
    LXP_CASE_CHECK(f->runtime.transfer_asset_count == 1U);
    LXP_CASE_CHECK(memcmp(f->assets.assets[0].asset_id, f->asset_id, 32U) == 0);
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
    uint8_t before[32], after[32], asset_before[32], asset_after[32];
    uint8_t receipt_root[32] = {0};
    lxp_snapshot_manifest_record manifest;
    lxp_byte_span snapshot;
    lxp_arena arena;
    uint64_t sequence = f->state.next_sequence - 1U;
    LXP_CASE_CHECK(storage != NULL);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_state_root(&f->assets, &f->accounts, asset_before) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, storage, sizeof(f->work)) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_write(&f->kernel, sequence, &arena, &snapshot) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_manifest(snapshot.bytes, snapshot.length,
                      sequence, before, receipt_root, &manifest) == LXP_OK);
    LXP_CASE_CHECK(proof_close(f) == 0);
    (void)memset(&f->kernel, 0, sizeof(f->kernel));
    (void)memset(&f->journal, 0, sizeof(f->journal));
    (void)memset(&f->ctx, 0, sizeof(f->ctx));
    (void)memset(&f->runtime, 0, sizeof(f->runtime));
    (void)memset(f->transfer_assets, 0, sizeof(f->transfer_assets));
    LXP_CASE_CHECK(proof_open(f) == 0);
    LXP_CASE_CHECK(f->kernel.module_kv_count == 0U && f->accounts.count == 0U);
    LXP_CASE_CHECK(f->assets.count == 0U);
    LXP_CASE_CHECK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
                                    &manifest, &f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_verify_root(&f->kernel, &manifest) == LXP_OK);
    LXP_CASE_CHECK(proof_restore(f) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_state_root(&f->assets, &f->accounts, asset_after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(memcmp(asset_before, asset_after, 32U) == 0);
    LXP_CASE_CHECK(f->state.next_sequence == sequence + 1U);
    free(storage);
    return 0;
}

static int proof_prepare(proof_fixture *f, uint32_t type, const uint8_t id[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                proof_seed, 32U);
    EVP_MD_CTX *signer = EVP_MD_CTX_new();
    uint8_t digest[32];
    size_t key_length = 32U, signature_length = 64U, payload_length = 34U;
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
        f->payload[66] = 1U;
        f->payload[67] = 'P';
        f->payload[68] = 1U;
        f->payload[69] = 'P';
        f->payload[86] = 100U;
        f->payload[87] = 1U;
        payload_length = sizeof(f->payload);
    } else if (type == LX_ASSET_MINT) {
        (void)memcpy(f->payload + 34U, f->account_id, 32U);
        LXP_CASE_CHECK(lxp_u128_to_be((lxp_u128){0U, 7U}, f->payload + 66U) == LXP_OK);
        payload_length = 82U;
    }
    (void)memcpy(f->payload + 2U, id, 32U);
    (void)memset(&f->activity, 0, sizeof(f->activity));
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    f->activity.network_id = 7U;
    f->activity.activity_type = type;
    f->activity.actor_did = (lxp_byte_span){proof_did, sizeof(proof_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.payload = (lxp_byte_span){f->payload, payload_length};
    f->activity.timestamp_bound.not_after = 100U;
    f->activity.idempotency_key[0] = (uint8_t)type;
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, payload_length, f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    LXP_CASE_CHECK(EVP_DigestSignInit(signer, NULL, NULL, NULL, key) == 1);
    LXP_CASE_CHECK(EVP_DigestSign(signer, f->signature, &signature_length, digest, 32U) == 1);
    EVP_MD_CTX_free(signer);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(signature_length == 64U);
    LXP_CASE_CHECK(lxp_arena_init(&wire_arena, f->wire, sizeof(f->wire)) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_encode(&f->activity, &wire_arena, &wire) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_decode(wire.bytes, wire.length, &f->activity) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->work, sizeof(f->work)) == LXP_OK);
    LXP_CASE_CHECK(lxp_effect_buffer_init(&f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_open(&f->state, f->state.next_sequence, &f->journal) == LXP_OK);
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

static int proof_submit_id(proof_fixture *f, uint32_t type,
                           const uint8_t id[32], lxp_result expected)
{
    const lxp_module_registration *registration;
    lxp_result result;
    uint8_t before[32], after[32];
    LXP_CASE_CHECK(proof_prepare(f, type, id) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&f->kernel, type, 0U, &registration) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_dispatch(registration, &f->ctx, &f->activity,
                         &f->authority, &f->effects, &result) == LXP_OK);
    LXP_CASE_CHECK(result == expected);
    if (expected != LXP_OK) {
        LXP_CASE_CHECK(proof_discard(f) == 0);
        LXP_CASE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
        LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
        return 0;
    }
    if (type == LX_ASSET_ACCOUNT_OPEN) {
        LXP_CASE_CHECK(f->ctx.staged_account_count == 1U);
        (void)memcpy(f->account_id, f->ctx.staged_accounts[0].account.id, 32U);
    }
    LXP_CASE_CHECK(lxp_module_ctx_prepare_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(proof_restore(f) == 0);
    return 0;
}

static int proof_submit(proof_fixture *f, uint32_t type, lxp_result expected)
{
    return proof_submit_id(f, type, f->asset_id, expected);
}

static int proof_budget_observes(proof_fixture *f, bool paused)
{
    const uint16_t versions[] = {LXP_PROTOCOL_VERSION_OCCUPANCY,
                                LXP_PROTOCOL_VERSION_STATE_COMMITMENT};
    lxp_transfer_asset_state observed;
    for (size_t i = 0U; i < sizeof(versions) / sizeof(versions[0]); ++i) {
        LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->work, sizeof(f->work)) == LXP_OK);
        LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_BUDGET,
            10U, 0U, f->state.next_sequence, 1000000U, &f->arena, false) == LXP_OK);
        f->ctx.protocol_version = versions[i];
        LXP_CASE_CHECK(budget_asset_state(&f->ctx, f->asset_id, &observed) == LXP_OK);
        LXP_CASE_CHECK(observed.registered && observed.paused == paused);
        LXP_CASE_CHECK(memcmp(observed.asset_id, f->asset_id, 32U) == 0);
    }
    return 0;
}

static int proof_lx_asset_h_happy_path(void)
{
    proof_fixture *f = calloc(1U, sizeof(*f));
    uint8_t empty_supply_root[32], minted_root[32];
    lxp_u128 balance;
    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(proof_open(f) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_REGISTER, LXP_OK) == 0);
    LXP_CASE_CHECK(!f->runtime.binding_authority);
    LXP_CASE_CHECK(proof_restart(f) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_REGISTER, LXP_ERR_ASSET_ALREADY_REGISTERED) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_ACCOUNT_OPEN, LXP_OK) == 0);
    LXP_CASE_CHECK(lx_asset_state_root(&f->assets, &f->accounts, empty_supply_root) == LXP_OK);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_MINT, LXP_OK) == 0);
    LXP_CASE_CHECK(lx_asset_state_root(&f->assets, &f->accounts, minted_root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(empty_supply_root, minted_root, 32U) != 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_PAUSE, LXP_OK) == 0);
    LXP_CASE_CHECK(proof_restart(f) == 0);
    LXP_CASE_CHECK(f->accounts.count == 2U);
    LXP_CASE_CHECK(f->assets.assets[0].total_units.hi == 0U &&
                   f->assets.assets[0].total_units.lo == 7U);
    LXP_CASE_CHECK(lx_asset_balance_get(&f->accounts, f->account_id,
                                       f->asset_id, &balance) == LXP_OK);
    LXP_CASE_CHECK(balance.hi == 0U && balance.lo == 7U);
    LXP_CASE_CHECK(proof_budget_observes(f, true) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_ACCOUNT_OPEN, LXP_ERR_ASSET_PAUSED) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_UNPAUSE, LXP_OK) == 0);
    LXP_CASE_CHECK(proof_budget_observes(f, false) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_ACCOUNT_OPEN, LXP_ERR_CONTEXT_MISMATCH) == 0);
    LXP_CASE_CHECK(proof_restart(f) == 0);
    LXP_CASE_CHECK(lx_asset_state_root(&f->assets, &f->accounts, empty_supply_root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(empty_supply_root, minted_root, 32U) == 0);
    LXP_CASE_CHECK(proof_close(f) == 0);
    free(f);
    return 0;
}

static int proof_lx_asset_h_refusal(void)
{
    proof_fixture *f = calloc(1U, sizeof(*f));
    uint8_t unknown[32] = {99U};
    lxp_module_kv_entry *entry = NULL;
    lx_asset_record decoded, unchanged;
    lxp_transfer_asset_state observed;
    const lxp_module_registration *registration = NULL;
    const uint32_t retired[] = {LX_ASSET_RECEIVE, LX_ASSET_GRANT_ISSUE, LX_ASSET_GRANT_REVOKE};
    void *payload = NULL;
    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(proof_open(f) == 0);
    LXP_CASE_CHECK(proof_submit(f, LX_ASSET_REGISTER, LXP_OK) == 0);
    LXP_CASE_CHECK(proof_restart(f) == 0);
    LXP_CASE_CHECK(proof_submit_id(f, LX_ASSET_PAUSE, unknown, LXP_ERR_ASSET_MISMATCH) == 0);
    LXP_CASE_CHECK(lx_asset_payer_grant_read == NULL);
    LXP_CASE_CHECK(lx_asset_payer_grant_encode == NULL);
    LXP_CASE_CHECK(proof_prepare(f, LX_ASSET_PAUSE, f->asset_id) == 0);
    for (size_t i = 0U; i < sizeof(retired) / sizeof(retired[0]); ++i) {
        LXP_CASE_CHECK(lxp_kernel_module_for_activity(&f->kernel, retired[i], 0U,
                           &registration) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(lx_asset_module_iface()->decode(&f->ctx,
            lxp_activity_type_ordinal(retired[i]), f->payload, 34U,
            &payload) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(payload == NULL);
    }
    LXP_CASE_CHECK(proof_discard(f) == 0);
    /* Corrupt only the transport key; the value is the real registration's
     * encoded output, never a fabricated asset record. All readers must fail. */
    for (size_t i = 0U; i < f->kernel.module_kv_count; ++i) {
        lxp_module_kv_entry *candidate = &f->kernel.module_kv[i];
        if (candidate->module_id == LXP_MODULE_ASSET && candidate->key_length == 38U &&
            memcmp(candidate->key, "asset:", 6U) == 0) entry = candidate;
    }
    LXP_CASE_CHECK(entry != NULL);
    (void)memset(&decoded, 0x5a, sizeof(decoded));
    unchanged = decoded;
    LXP_CASE_CHECK(lx_asset_record_decode_for_id(entry->value, entry->value_length,
                         unknown, &decoded) == LXP_ERR_ASSET_MISMATCH);
    LXP_CASE_CHECK(memcmp(&decoded, &unchanged, sizeof(decoded)) == 0);
    LXP_CASE_CHECK(lx_asset_record_decode_for_id(entry->value, entry->value_length - 1U,
                         f->asset_id, &decoded) == LXP_ERR_NON_CANONICAL);
    (void)memcpy(entry->key + 6U, unknown, 32U);
    LXP_CASE_CHECK(proof_submit_id(f, LX_ASSET_PAUSE, unknown, LXP_ERR_ASSET_MISMATCH) == 0);
    LXP_CASE_CHECK(proof_prepare(f, LX_ASSET_PAUSE, unknown) == 0);
    LXP_CASE_CHECK(lxp_ctx_bind_asset_supply(&f->ctx, unknown,
        (lxp_u128){0U, 0U}, (lxp_u128){0U, 0U}) == LXP_FATAL_SUPPLY_MISMATCH);
    LXP_CASE_CHECK(proof_discard(f) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->work, sizeof(f->work)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_BUDGET,
        10U, 0U, f->state.next_sequence, 1000000U, &f->arena, false) == LXP_OK);
    f->ctx.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    LXP_CASE_CHECK(budget_asset_state(&f->ctx, unknown, &observed) == LXP_ERR_ASSET_MISMATCH);
    f->ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    LXP_CASE_CHECK(budget_asset_state(&f->ctx, unknown, &observed) == LXP_ERR_ASSET_MISMATCH);
    LXP_CASE_CHECK(lx_asset_runtime_restore(&f->kernel, &f->assets,
        f->transfer_assets, &f->runtime) == LXP_ERR_ASSET_MISMATCH);
    (void)memcpy(entry->key + 6U, f->asset_id, 32U);
    /* The kernel must reject an invalid account view before binding it. */
    size_t count = f->accounts.count;
    f->accounts.count = LX_ACCOUNT_REGISTRY_CAPACITY + 1U;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET,
        &f->runtime) == LXP_ERR_NON_CANONICAL);
    f->accounts.count = count;
    LXP_CASE_CHECK(proof_close(f) == 0);
    free(f);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"proof_lx_asset_h_happy_path", proof_lx_asset_h_happy_path},
        {"proof_lx_asset_h_refusal", proof_lx_asset_h_refusal}
    };
    if (argc == 1) {
        /* tests/codec/lxp_kernel_case.h lxp_kernel_case_main requires names;
         * the declared gate has none, so explicitly run every registered case. */
        for (size_t i = 0U; i < sizeof(cases) / sizeof(cases[0]); ++i) {
            char *args[] = {argv[0], (char *)cases[i].name};
            if (lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), 2, args) != 0)
                return 1;
            (void)printf("%s passed\n", cases[i].name);
        }
        return 0;
    }
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
