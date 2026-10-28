#include "layerx/lxp_kernel.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/lx_asset.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_state.h"

#include <openssl/evp.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define REQUIRE(condition) do { if (!(condition)) { \
    (void)fprintf(stderr, "idempotency lifecycle check failed at line %d\n", __LINE__); \
    return 1; } } while (0)

#define ARENA_BYTES ((size_t)64U * 1024U * 1024U)
#define ALICE_START 1000000U
#define CAROL_START 10U

typedef struct fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lx_account_registry accounts;
    lx_asset_runtime runtime;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lxp_arena arena;
    uint8_t *arena_bytes;
    uint64_t parameters;
    uint8_t alice_key[32];
    uint8_t carol_key[32];
    lxp_kernel_execution execution;
    lxp_fee_params fees;
} fixture;

static const uint8_t alice_seed[32] = {1U};
static const uint8_t carol_seed[32] = {2U};
static const uint8_t alice_did[] = "did:key:alice";
static const uint8_t carol_did[] = "did:key:carol";
static const char alice_name[] = "agent:did:key:alice:main";
static const char bob_name[] = "agent:did:key:bob:main";
static const char carol_name[] = "agent:did:key:carol:main";

static unsigned cases;

static void case_ok(const char *name)
{
    ++cases;
    (void)printf("IDEMPOTENCY_CASE %s ok\n", name);
    (void)fflush(stdout);
}

static int sign_digest(const uint8_t seed[32], const uint8_t digest[32],
                       uint8_t signature[64], uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, 32U);
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

static int open_account(fixture *f, const char *name, uint64_t units,
                        const uint8_t *authority_key)
{
    uint8_t id[32];
    uint8_t asset[32] = {3U};
    lx_account *opened;
    REQUIRE(lx_account_id_from_string((const uint8_t *)name, strlen(name), id) == LXP_OK);
    REQUIRE(lx_account_open(&f->accounts, (const uint8_t *)name, strlen(name),
                            id, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &opened) == LXP_OK);
    REQUIRE(lxp_ledger_bootstrap_balance(opened, asset, (lxp_u128){0U, units}, 0U) == LXP_OK);
    if (authority_key != NULL) {
        opened->has_authority_key = true;
        (void)memcpy(opened->authority_key, authority_key, 32U);
    }
    return 0;
}

static int fixture_init(fixture *f)
{
    uint8_t digest[32] = {0};
    uint8_t signature[64];
    lxp_identity *identity;
    (void)memset(f, 0, sizeof(*f));
    REQUIRE(sign_digest(alice_seed, digest, signature, f->alice_key) == 0);
    REQUIRE(sign_digest(carol_seed, digest, signature, f->carol_key) == 0);
    f->arena_bytes = (uint8_t *)malloc(ARENA_BYTES);
    REQUIRE(f->arena_bytes != NULL);
    REQUIRE(lxp_arena_init(&f->arena, f->arena_bytes, ARENA_BYTES) == LXP_OK);
    REQUIRE(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    f->parameters = 1U;
    REQUIRE(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                              &f->parameters, 0U) == LXP_OK);
    REQUIRE(lxp_kernel_set_capabilities(&f->kernel, NULL,
                                        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    REQUIRE(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()) == LXP_OK);
    REQUIRE(lxp_identity_register(&f->identities, alice_did, sizeof(alice_did) - 1U,
                                  f->alice_key, &identity) == LXP_OK);
    REQUIRE(lxp_identity_register(&f->identities, carol_did, sizeof(carol_did) - 1U,
                                  f->carol_key, &identity) == LXP_OK);
    REQUIRE(lx_account_registry_init(&f->accounts) == LXP_OK);
    REQUIRE(open_account(f, alice_name, ALICE_START, f->alice_key) == 0);
    REQUIRE(open_account(f, bob_name, 0U, NULL) == 0);
    REQUIRE(open_account(f, carol_name, CAROL_START, f->carol_key) == 0);
    f->asset.asset_id[0] = 3U;
    f->transfer_asset.asset_id[0] = 3U;
    f->transfer_asset.registered = true;
    f->runtime = (lx_asset_runtime){&f->accounts, &f->asset, 1U,
        &f->transfer_asset, 1U, 7U, LXP_PROTOCOL_VERSION_STATE_COMMITMENT, false};
    REQUIRE(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET, &f->runtime) == LXP_OK);
    f->fees.version = 1U;
    f->fees.multiplier_basis_points = 10000U;
    f->execution.network_id = 7U;
    f->execution.batch_number = 1U;
    f->execution.batch_timestamp_ms = 10U;
    f->execution.maximum_timestamp_window = 100U;
    f->execution.recorded_module_version = 1U;
    f->execution.parameter_version = 1U;
    f->execution.signature_valid = true;
    f->execution.identities = &f->identities;
    f->execution.fee_parameters = &f->fees;
    f->execution.gas_limit = 10000U;
    f->execution.arena = &f->arena;
    f->execution.sequencer_private_key = alice_seed;
    f->execution.batch_id[0] = 5U;
    REQUIRE(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    return 0;
}

static void fixture_destroy(fixture *f)
{
    (void)lxp_state_store_destroy(&f->state);
    lx_account_registry_release(&f->accounts);
    free(f->arena_bytes);
    free(f);
}

static void raw_key(uint32_t index, uint8_t marker, uint8_t key[32])
{
    (void)memset(key, 0, 32U);
    key[0] = (uint8_t)(index >> 24);
    key[1] = (uint8_t)(index >> 16);
    key[2] = (uint8_t)(index >> 8);
    key[3] = (uint8_t)index;
    key[30] = marker;
    key[31] = 1U;
}

static int journal_single_entry(void)
{
    static const uint8_t actor[] = "did:lxp:alice";
    static const uint8_t first_receipt[] = { 1U, 2U, 3U, 4U };
    uint8_t idempotency_key[32] = { 9U };
    uint8_t balance_key[32] = { 5U };
    lxp_state_store store;
    lxp_state_journal journal;
    lxp_u128 value;
    bool found;
    REQUIRE(lxp_state_store_init(&store, 0U) == LXP_OK);
    REQUIRE(lxp_state_journal_open(&store, 0U, &journal) == LXP_OK);
    REQUIRE(lxp_state_journal_set(&journal, balance_key, (lxp_u128){ 0U, 90U }) == LXP_OK);
    REQUIRE(lxp_idempotency_record(&journal, actor, sizeof(actor) - 1U,
                                   idempotency_key, first_receipt,
                                   sizeof(first_receipt)) == LXP_OK);
    REQUIRE(lxp_state_journal_commit(&journal) == LXP_OK);
    REQUIRE(store.next_sequence == 1U && store.idempotency_count == 1U);
    REQUIRE(lxp_state_store_get(&store, balance_key, &value, &found) == LXP_OK);
    REQUIRE(found && value.hi == 0U && value.lo == 90U);
    REQUIRE(lxp_state_store_destroy(&store) == LXP_OK);
    case_ok("journal_single_entry");
    return 0;
}

static const uint8_t retention_actor[] = "did:lxp:retention";
static const uint8_t retention_receipt[] = { 7U, 8U, 9U };

enum { RETENTION_WINDOW = 1000, RETENTION_KEY_MARKER = 0xB0 };

static int retention_commit(lxp_state_journal *journal, lxp_state_store *store,
                            uint32_t key_index, lxp_result expected)
{
    uint8_t key[32];
    raw_key(key_index, RETENTION_KEY_MARKER, key);
    REQUIRE(lxp_state_journal_open(store, store->next_sequence, journal) == LXP_OK);
    REQUIRE(lxp_idempotency_record(journal, retention_actor, sizeof(retention_actor) - 1U,
                                   key, retention_receipt, sizeof(retention_receipt)) == LXP_OK);
    REQUIRE(lxp_state_journal_commit(journal) == expected);
    if (expected != LXP_OK) REQUIRE(lxp_state_journal_rollback(journal) == LXP_OK);
    return 0;
}

static int retention_lookup(lxp_state_store *store, uint32_t key_index, lxp_result expected)
{
    uint8_t key[32];
    const uint8_t *bytes = NULL;
    size_t length = 0U;
    raw_key(key_index, RETENTION_KEY_MARKER, key);
    REQUIRE(lxp_idempotency_lookup(store, retention_actor, sizeof(retention_actor) - 1U,
                                   key, &bytes, &length) == expected);
    REQUIRE(bytes == NULL && length == 0U);
    return 0;
}

static int retention_advance(lxp_state_journal *journal, lxp_state_store *store,
                             uint64_t target)
{
    while (store->next_sequence < target) {
        REQUIRE(lxp_state_journal_open(store, store->next_sequence, journal) == LXP_OK);
        REQUIRE(lxp_state_journal_commit(journal) == LXP_OK);
    }
    REQUIRE(store->next_sequence == target);
    return 0;
}

static int retention_fill(lxp_state_store *store, uint64_t first_recorded, uint64_t stride)
{
    size_t i;
    REQUIRE(store->idempotency_count == 0U);
    REQUIRE(lxp_idempotency_reserve(store) == LXP_OK);
    for (i = 0U; i < LXP_STATE_MAX_IDEMPOTENCY; ++i) {
        lxp_idempotency_key_state *entry = &store->idempotency[i];
        (void)memset(entry->key_hash, 0, 32U);
        entry->key_hash[0] = (uint8_t)(i >> 8);
        entry->key_hash[1] = (uint8_t)i;
        entry->key_hash[31] = 0xA5U;
        entry->receipt_length = 1U;
        entry->receipt[0] = 0x5AU;
        entry->canonical_length = 0U;
        entry->recorded_sequence = first_recorded + (uint64_t)i * stride;
    }
    store->idempotency_count = LXP_STATE_MAX_IDEMPOTENCY;
    return 0;
}

static int retention_records_sequence(void)
{
    lxp_state_store store;
    lxp_state_journal journal;
    REQUIRE(lxp_state_store_init(&store, 900U) == LXP_OK);
    store.idempotency_retention = RETENTION_WINDOW;
    REQUIRE(retention_commit(&journal, &store, 1U, LXP_OK) == 0);
    REQUIRE(store.next_sequence == 901U && store.idempotency_count == 1U);
    REQUIRE(store.idempotency[0].recorded_sequence == 900U);
    REQUIRE(retention_commit(&journal, &store, 2U, LXP_OK) == 0);
    REQUIRE(store.next_sequence == 902U && store.idempotency_count == 2U);
    REQUIRE(store.idempotency[0].recorded_sequence == 900U);
    REQUIRE(store.idempotency[1].recorded_sequence == 901U);
    REQUIRE(lxp_state_store_destroy(&store) == LXP_OK);
    case_ok("retention_records_sequence");
    return 0;
}

static int retention_admits_after_window(void)
{
    lxp_state_store store;
    lxp_state_journal journal;
    REQUIRE(lxp_state_store_init(&store, 9000U) == LXP_OK);
    store.idempotency_retention = RETENTION_WINDOW;
    REQUIRE(retention_fill(&store, 1U, 1U) == 0);
    REQUIRE(lxp_idempotency_live_count(&store, 9000U) == 192U);
    REQUIRE(retention_commit(&journal, &store, 1U, LXP_OK) == 0);
    REQUIRE(store.idempotency_count == 193U && store.next_sequence == 9001U);
    REQUIRE(store.idempotency[0].recorded_sequence == 8001U);
    REQUIRE(store.idempotency[191].recorded_sequence == 8192U);
    REQUIRE(store.idempotency[192].recorded_sequence == 9000U);
    REQUIRE(retention_advance(&journal, &store, 9200U) == 0);
    REQUIRE(retention_commit(&journal, &store, 2U, LXP_OK) == 0);
    REQUIRE(store.idempotency_count == 2U);
    REQUIRE(store.idempotency[0].recorded_sequence == 9000U);
    REQUIRE(store.idempotency[1].recorded_sequence == 9200U);
    REQUIRE(lxp_state_store_destroy(&store) == LXP_OK);
    case_ok("retention_admits_after_window");
    return 0;
}

static int retention_zero_window(void)
{
    lxp_state_store store;
    lxp_state_journal journal;
    uint32_t i;
    REQUIRE(lxp_state_store_init(&store, 0U) == LXP_OK);
    REQUIRE(store.idempotency_retention == 0U);
    for (i = 0U; i < LXP_STATE_MAX_IDEMPOTENCY; ++i)
        REQUIRE(retention_commit(&journal, &store, i, LXP_OK) == 0);
    REQUIRE(store.idempotency_count == LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(retention_advance(&journal, &store,
                              LXP_STATE_MAX_IDEMPOTENCY + 2U * RETENTION_WINDOW) == 0);
    REQUIRE(store.idempotency_count == LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(lxp_idempotency_live_count(&store, store.next_sequence) ==
            LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(store.idempotency[0].recorded_sequence == 0U);
    REQUIRE(retention_commit(&journal, &store, LXP_STATE_MAX_IDEMPOTENCY,
                             LXP_ERR_ARENA_EXHAUSTED) == 0);
    REQUIRE(store.idempotency_count == LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(store.next_sequence == LXP_STATE_MAX_IDEMPOTENCY + 2U * RETENTION_WINDOW);
    REQUIRE(retention_lookup(&store, LXP_STATE_MAX_IDEMPOTENCY, LXP_OK) == 0);
    REQUIRE(lxp_state_store_destroy(&store) == LXP_OK);
    case_ok("retention_zero_window");
    return 0;
}

static int retention_preview(fixture *f, lxp_module_ctx *ctx, uint32_t key_index,
                             lxp_result expected)
{
    uint8_t key[32];
    uint8_t preview[32];
    uint8_t committed[32];
    uint64_t sequence = f->state.next_sequence;
    raw_key(key_index, RETENTION_KEY_MARKER, key);
    REQUIRE(lxp_state_journal_open(&f->state, sequence, &f->journal) == LXP_OK);
    REQUIRE(lxp_idempotency_record(&f->journal, retention_actor, sizeof(retention_actor) - 1U,
                                   key, retention_receipt, sizeof(retention_receipt)) == LXP_OK);
    REQUIRE(lxp_module_ctx_init(ctx, &f->kernel, LXP_MODULE_ASSET, 10U, f->kernel.epoch,
                                sequence, 100U, &f->arena, true) == LXP_OK);
    REQUIRE(lxp_module_ctx_prepare_commit(ctx) == LXP_OK);
    REQUIRE(lxp_module_ctx_preview_state_root(ctx, &f->journal, preview) == expected);
    if (expected != LXP_OK) {
        lxp_module_ctx_rollback(ctx);
        REQUIRE(lxp_state_journal_rollback(&f->journal) == LXP_OK);
        return 0;
    }
    REQUIRE(lxp_state_journal_commit(&f->journal) == LXP_OK);
    REQUIRE(lxp_module_ctx_commit(ctx) == LXP_OK);
    REQUIRE(lxp_state_root(&f->kernel, committed) == LXP_OK);
    REQUIRE(memcmp(preview, committed, 32U) == 0);
    return 0;
}

static int retention_preview_bound(void)
{
    fixture *f = (fixture *)malloc(sizeof(*f));
    lxp_module_ctx *ctx = (lxp_module_ctx *)malloc(sizeof(*ctx));
    REQUIRE(f != NULL && ctx != NULL);
    REQUIRE(fixture_init(f) == 0);
    f->state.idempotency_retention = RETENTION_WINDOW;
    REQUIRE(retention_advance(&f->journal, &f->state, 1500U) == 0);
    REQUIRE(retention_fill(&f->state, 1000U, 0U) == 0);
    REQUIRE(lxp_idempotency_live_count(&f->state, 1500U) == LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(retention_preview(f, ctx, 1U, LXP_ERR_ARENA_EXHAUSTED) == 0);
    REQUIRE(f->state.idempotency_count == LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(f->state.next_sequence == 1500U);
    REQUIRE(retention_advance(&f->journal, &f->state, 2000U) == 0);
    REQUIRE(lxp_idempotency_live_count(&f->state, 2000U) == 0U);
    REQUIRE(retention_preview(f, ctx, 1U, LXP_OK) == 0);
    REQUIRE(f->state.idempotency_count == 1U && f->state.next_sequence == 2001U);
    REQUIRE(f->state.idempotency[0].recorded_sequence == 2000U);
    free(ctx);
    fixture_destroy(f);
    case_ok("retention_preview_bound");
    return 0;
}

static int test_retention_window_prunes(void)
{
    lxp_state_store store;
    lxp_state_journal journal;
    uint8_t oldest_hash[32];
    size_t i;
    REQUIRE(lxp_state_store_init(&store, LXP_STATE_MAX_IDEMPOTENCY + 1U) == LXP_OK);
    store.idempotency_retention = LXP_STATE_MAX_IDEMPOTENCY;
    REQUIRE(retention_fill(&store, 1U, 1U) == 0);
    (void)memcpy(oldest_hash, store.idempotency[0].key_hash, 32U);
    REQUIRE(lxp_idempotency_live_count(&store, store.next_sequence) ==
            LXP_STATE_MAX_IDEMPOTENCY - 1U);
    REQUIRE(retention_commit(&journal, &store, 1U, LXP_OK) == 0);
    REQUIRE(store.idempotency_count == LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(store.next_sequence == LXP_STATE_MAX_IDEMPOTENCY + 2U);
    REQUIRE(store.idempotency[0].recorded_sequence == 2U);
    REQUIRE(store.idempotency[LXP_STATE_MAX_IDEMPOTENCY - 2U].recorded_sequence ==
            LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(store.idempotency[LXP_STATE_MAX_IDEMPOTENCY - 1U].recorded_sequence ==
            LXP_STATE_MAX_IDEMPOTENCY + 1U);
    for (i = 0U; i < store.idempotency_count; ++i)
        REQUIRE(memcmp(store.idempotency[i].key_hash, oldest_hash, 32U) != 0);
    REQUIRE(retention_commit(&journal, &store, 2U, LXP_OK) == 0);
    REQUIRE(store.idempotency_count == LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(store.idempotency[0].recorded_sequence == 3U);
    REQUIRE(store.idempotency[LXP_STATE_MAX_IDEMPOTENCY - 1U].recorded_sequence ==
            LXP_STATE_MAX_IDEMPOTENCY + 2U);
    REQUIRE(retention_commit(&journal, &store, 3U, LXP_OK) == 0);
    REQUIRE(store.idempotency_count == LXP_STATE_MAX_IDEMPOTENCY);
    REQUIRE(store.idempotency[0].recorded_sequence == 4U);
    REQUIRE(lxp_state_store_destroy(&store) == LXP_OK);
    case_ok("test_retention_window_prunes");
    return 0;
}

int main(void)
{
    REQUIRE(journal_single_entry() == 0);
    REQUIRE(retention_records_sequence() == 0);
    REQUIRE(retention_admits_after_window() == 0);
    REQUIRE(retention_zero_window() == 0);
    REQUIRE(retention_preview_bound() == 0);
    REQUIRE(test_retention_window_prunes() == 0);
    (void)printf("IDEMPOTENCY_LIFECYCLE cases=%u skipped=0\n", cases);
    return 0;
}
