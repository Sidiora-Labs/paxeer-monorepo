#include "layerx/programs.h"

#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_module_ctx.h"

#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static const char *account_names[] = {
    "agent:did:lxp:capacity-payer:main",
    "agent:did:lxp:capacity-debtor:main", "system:fees"};
static const uint8_t asset_id[32] = {1U};

typedef struct fixture {
    lxp_state_store store;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lx_programs_transfer_runtime *runtime;
    lxp_module_ctx ctx;
    lxp_arena seed_arena;
    uint8_t seed_arena_bytes[65536];
    uint64_t parameters;
    uint32_t blob_serial;
} fixture;

static fixture fx;
static size_t passed;

#define CHECK(condition, name) \
    do { \
        if (!(condition)) { \
            (void)fprintf(stderr, "BLOB_ADMISSION_FAIL %s line %d: %s\n", \
                          name, __LINE__, #condition); \
            return 1; \
        } \
    } while (0)

static int pass(const char *name)
{
    ++passed;
    (void)printf("BLOB_ADMISSION_CASE %s\n", name);
    return 0;
}

static void put_u32(uint8_t *p, uint32_t v)
{
    p[0] = (uint8_t)(v >> 24U);
    p[1] = (uint8_t)(v >> 16U);
    p[2] = (uint8_t)(v >> 8U);
    p[3] = (uint8_t)v;
}

static int fixture_init(void)
{
    lxp_transfer_asset_state *asset_state;
    uint8_t id[32];
    size_t i;
    (void)memset(&fx, 0, sizeof(fx));
    fx.parameters = 1U;
    if (lxp_state_store_init(&fx.store, 1U) != LXP_OK ||
        lxp_kernel_create(&fx.kernel, &fx.store, &fx.journal, &fx.parameters,
                          0U) != LXP_OK ||
        lxp_kernel_register_module(
            &fx.kernel, programs_module_registration_v4_storage_retirement()) !=
            LXP_OK)
        return 1;
    fx.runtime = calloc(1U, sizeof(*fx.runtime));
    asset_state = calloc(1U, sizeof(*asset_state));
    if (fx.runtime == NULL || asset_state == NULL) {
        free(asset_state);
        return 1;
    }
    fx.runtime->accounts = calloc(1U, sizeof(*fx.runtime->accounts));
    (void)memcpy(asset_state->asset_id, asset_id, 32U);
    asset_state->registered = true;
    fx.runtime->assets = asset_state;
    fx.runtime->asset_count = 1U;
    (void)memcpy(fx.runtime->occupancy_asset_id, asset_id, 32U);
    if (fx.runtime->accounts == NULL ||
        lx_account_registry_init(fx.runtime->accounts) != LXP_OK ||
        lxp_state_store_bind_accounts(&fx.store, fx.runtime->accounts) != LXP_OK)
        return 1;
    for (i = 0U; i < sizeof(account_names) / sizeof(account_names[0]); ++i) {
        lx_account *account;
        const bool treasury = i == 2U;
        if (lx_account_id_from_string((const uint8_t *)account_names[i],
                                      strlen(account_names[i]), id) != LXP_OK ||
            lx_account_open(fx.runtime->accounts,
                            (const uint8_t *)account_names[i],
                            strlen(account_names[i]), id, treasury ? 2U : 1U,
                            LX_ACCOUNT_OPEN_GENESIS, NULL, &account) != LXP_OK ||
            lxp_ledger_bootstrap_balance(account, asset_id,
                treasury ? (lxp_u128){0U, 0U} :
                           (lxp_u128){0U, UINT64_C(1000000000)}, 0U) != LXP_OK)
            return 1;
    }
    if (lxp_state_store_require_account_root(&fx.store) != LXP_OK ||
        lxp_kernel_bind_module_runtime(&fx.kernel, LXP_MODULE_PROGRAMS,
                                       fx.runtime) != LXP_OK ||
        lxp_programs_bind_fee_transaction(&fx.kernel) != LXP_OK ||
        lxp_kernel_set_capabilities(&fx.kernel, NULL,
                                    lxp_kernel_canonical_ledger_apply) != LXP_OK ||
        lxp_state_root(&fx.kernel, fx.kernel.current_state_root) != LXP_OK)
        return 1;

    return 0;
}

static int fixture_destroy(void)
{
    while (fx.kernel.blob_count != 0U)
        free(fx.kernel.blobs[--fx.kernel.blob_count].bytes);
    if (fx.runtime != NULL) {
        free((void *)fx.runtime->assets);
        lx_account_registry_release(fx.runtime->accounts);
        free(fx.runtime->accounts);
        free(fx.runtime);
    }
    return lxp_state_store_destroy(&fx.store) != LXP_OK;
}

static int seed_open(void)
{
    return lxp_arena_init(&fx.seed_arena, fx.seed_arena_bytes,
                          sizeof(fx.seed_arena_bytes)) != LXP_OK ||
        lxp_module_ctx_init(&fx.ctx, &fx.kernel, LXP_MODULE_PROGRAMS, 10U,
                            fx.kernel.epoch, fx.store.next_sequence,
                            UINT64_C(100000000), &fx.seed_arena,
                            true) != LXP_OK;
}

static int seed_close(lxp_result status)
{
    if (status != LXP_OK) {
        lxp_module_ctx_rollback(&fx.ctx);
        return 1;
    }
    return lxp_module_ctx_commit(&fx.ctx) != LXP_OK ||
        lxp_state_root(&fx.kernel, fx.kernel.current_state_root) != LXP_OK;
}

static lxp_result blob_stage(uint8_t key[32])
{
    uint8_t bytes[8];
    lxp_result status;
    (void)memcpy(bytes, "capblob", 4U);
    put_u32(bytes + 4U, ++fx.blob_serial);
    status = lxp_hash_sha256(bytes, sizeof(bytes), key);
    return status == LXP_OK ? lxp_ctx_blob_put(&fx.ctx, key, bytes,
                                               sizeof(bytes)) : status;
}

static int blobs_commit_until(size_t target)
{
    uint8_t key[32];
    while (fx.kernel.blob_count < target) {
        size_t batch = target - fx.kernel.blob_count;
        lxp_result status = LXP_OK;
        size_t i;
        if (batch > LXP_KERNEL_MAX_STAGED_BLOBS)
            batch = LXP_KERNEL_MAX_STAGED_BLOBS;
        if (seed_open() != 0) return 1;
        for (i = 0U; i < batch && status == LXP_OK; ++i)
            status = blob_stage(key);
        if (seed_close(status) != 0) return 1;
    }
    return fx.kernel.blob_count == target ? 0 : 1;
}

static int case_full_store(void)
{
    uint8_t key[32];
    size_t committed_blobs;
    lxp_result status;
    CHECK(blobs_commit_until(LXP_KERNEL_MAX_BLOBS) == 0, "fill_to_limit");
    CHECK(fx.kernel.blob_count == LXP_KERNEL_MAX_BLOBS, "full_live_store");
    committed_blobs = fx.kernel.blob_count;
    CHECK(seed_open() == 0, "deletion_context");
    CHECK(lxp_ctx_blob_del(&fx.ctx, fx.kernel.blobs[0].key) == LXP_OK,
          "staged_deletion");
    status = blob_stage(key);
    if (status != LXP_OK) {
        lxp_module_ctx_rollback(&fx.ctx);
        CHECK(false, "staged_addition");
    }
    status = lxp_module_ctx_prepare_commit(&fx.ctx);
    lxp_module_ctx_rollback(&fx.ctx);
    CHECK(status == LXP_ERR_ARENA_EXHAUSTED, "native_prepare_refuses_addition");
    CHECK(fx.kernel.blob_count == committed_blobs, "nothing_deleted");
    return pass("full_live_store_refused");
}

int main(void)
{
    int failed;
    if (fixture_init() != 0) {
        (void)fprintf(stderr, "BLOB_ADMISSION_FAIL fixture\n");
        (void)fixture_destroy();
        return 1;
    }
    failed = case_full_store() != 0;
    if (fixture_destroy() != 0) failed = 1;
    if (failed != 0) return 1;
    (void)printf("PASSED %zu\n", passed);
    return 0;
}
