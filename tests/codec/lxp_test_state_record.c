#include "lxp_kernel_case.h"

#include "layerx/programs.h"
#include "layerx/lxp_kernel.h"

static lxp_result count_value(const lx_programs_value_account_view *value,
                              void *user)
{
    size_t *count = (size_t *)user;
    if (value == NULL) return LXP_ERR_NON_CANONICAL;
    ++*count;
    return LXP_OK;
}

static int state_record_rejects_duplicate_account(void)
{
    static const uint8_t program_prefix[] = "program\0";
    static const uint8_t account_prefix[] = "program-account\0p";
    static uint8_t record_bytes[
        4096U * (sizeof(lx_programs_value_account_view) +
                 sizeof(lx_programs_value_account_view *)) + 65536U];
    static lxp_kernel kernel;
    static lxp_module_ctx ctx;
    static lxp_effect_buffer effects;
    static lxp_verified_receipt_index receipts;
    lxp_state_store store;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lxp_transfer_asset_state asset = {0};
    lx_programs_transfer_runtime runtime = {0};
    lx_programs_value_account_view view;
    lx_account *account;
    lxp_arena arena;
    lxp_arena record_arena;
    lxp_byte_span encoded;
    uint8_t scratch[4096];
    uint8_t program_id[32] = {1U};
    uint8_t account_id[32];
    uint8_t deploy_key[sizeof(program_prefix) - 1U + 32U];
    uint8_t deploy_record[71] = {0};
    uint8_t *sentinel;
    void *memory;
    uint64_t parameters = 1U;
    size_t mark;
    size_t primary;
    size_t count = 0U;
    bool created;

    asset.asset_id[0] = 2U;
    asset.registered = true;
    runtime.accounts = &accounts;
    runtime.assets = &asset;
    runtime.asset_count = 1U;
    deploy_record[0] = 1U;
    deploy_record[1] = 3U;
    deploy_record[33] = 4U;
    deploy_record[66] = 2U;
    deploy_record[70] = 1U;
    (void)memcpy(deploy_key, program_prefix, sizeof(program_prefix) - 1U);
    (void)memcpy(deploy_key + sizeof(program_prefix) - 1U, program_id, 32U);

    LXP_CASE_CHECK(lxp_arena_init(&arena, scratch, sizeof(scratch)) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&record_arena, record_bytes,
                                sizeof(record_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&store, 7U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &store, &journal,
                                   &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(
        &kernel, programs_module_registration_v2()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_epoch(&kernel, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(
        &kernel, LXP_MODULE_PROGRAMS, &runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_PROGRAMS,
        1U, 1U, 7U, 10000U, &arena, true) == LXP_OK);
    ctx.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    LXP_CASE_CHECK(lxp_ctx_kv_put(&ctx, deploy_key, sizeof(deploy_key),
        deploy_record, sizeof(deploy_record)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_open(&store, 7U, &journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_PROGRAMS,
        2U, 1U, 7U, 10000U, &arena, true) == LXP_OK);
    ctx.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    LXP_CASE_CHECK(lxp_effect_buffer_init(&effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&ctx, &effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_account_register(
        &ctx, program_id, (const uint8_t *)"vault", 5U, asset.asset_id,
        &account, &created) == LXP_OK);
    LXP_CASE_CHECK(created);
    (void)memcpy(account_id, account->id, 32U);
    LXP_CASE_CHECK(lxp_programs_account_register(
        &ctx, program_id, (const uint8_t *)"other", 5U, asset.asset_id,
        &account, &created) == LXP_OK);
    LXP_CASE_CHECK(created);
    LXP_CASE_CHECK(lxp_module_ctx_prepare_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_PROGRAMS,
        3U, 1U, 8U, 10000U, &arena, false) == LXP_OK);
    ctx.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    receipts.count = 1U;
    receipts.entries[0].receipt_digest[0] = 5U;
    receipts.entries[0].global_sequence = 7U;
    receipts.entries[0].timestamp = 7U;
    LXP_CASE_CHECK(lxp_state_root(
        &kernel, receipts.entries[0].resulting_state_root) == LXP_OK);
    ctx.verified_receipts = &receipts;
    LXP_CASE_CHECK(lxp_programs_value_account_read(&ctx, account_id,
        receipts.entries[0].receipt_digest, &view) == LXP_OK);
    LXP_CASE_CHECK(memcmp(view.account.id, account_id, 32U) == 0);
    LXP_CASE_CHECK(lxp_arena_alloc(&record_arena, 17U, 1U, &memory) == LXP_OK);
    sentinel = (uint8_t *)memory;
    (void)memset(sentinel, 0xa5, 17U);
    mark = lxp_arena_mark(&record_arena);
    LXP_CASE_CHECK(lxp_programs_state_record_encode(&ctx, program_id,
        receipts.entries[0].receipt_digest, &record_arena, &encoded) == LXP_OK);
    LXP_CASE_CHECK(encoded.length > 5U);
    LXP_CASE_CHECK(memcmp(encoded.bytes, "LXPS1", 5U) == 0);
    LXP_CASE_CHECK(lxp_arena_reset(&record_arena, mark) == LXP_OK);

    for (primary = 0U; primary < kernel.module_kv_count; ++primary) {
        const lxp_module_kv_entry *entry = &kernel.module_kv[primary];
        if (entry->module_id == LXP_MODULE_PROGRAMS &&
            entry->key_length >= sizeof(account_prefix) - 1U &&
            memcmp(entry->key, account_prefix,
                   sizeof(account_prefix) - 1U) == 0 &&
            entry->value_length >= 65U &&
            memcmp(entry->value + 33U, account_id, 32U) == 0)
            break;
    }
    LXP_CASE_CHECK(primary < kernel.module_kv_count);
    LXP_CASE_CHECK(kernel.module_kv_count < LXP_KERNEL_MAX_MODULE_KV);
    kernel.module_kv[kernel.module_kv_count] = kernel.module_kv[primary];
    ++kernel.module_kv_count;
    LXP_CASE_CHECK(lxp_state_root(
        &kernel, receipts.entries[0].resulting_state_root) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_value_account_read(&ctx, account_id,
        receipts.entries[0].receipt_digest, &view) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_value_account_iter(&ctx, program_id,
        receipts.entries[0].receipt_digest, count_value, &count) == LXP_OK);
    LXP_CASE_CHECK(count == 3U);
    encoded.bytes = sentinel;
    encoded.length = 17U;
    LXP_CASE_CHECK(lxp_programs_state_record_encode(&ctx, program_id,
        receipts.entries[0].receipt_digest, &record_arena, &encoded) ==
        LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(encoded.bytes == NULL && encoded.length == 0U);
    LXP_CASE_CHECK(lxp_arena_mark(&record_arena) == mark);
    for (size_t index = 0U; index < 17U; ++index)
        LXP_CASE_CHECK(sentinel[index] == 0xa5U);

    lx_account_registry_release(&accounts);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "state_record_rejects_duplicate_account",
          state_record_rejects_duplicate_account }
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
