#include "layerx/lx_stream_read.h"
#include "layerx/lxp_kernel.h"

#include "lxp_kernel_case.h"

static lxp_state_store store;
static lxp_state_journal journal;
static lxp_kernel kernel;
static lxp_module_ctx ctx;
static lxp_arena arena;
static uint8_t arena_bytes[4096];
static const uint64_t parameters = 1U;

static int fixture_init(void)
{
    LXP_CASE_CHECK(lxp_state_store_init(&store, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &store, &journal, &parameters,
                                     0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&kernel,
                                              lx_stream_module_iface()) ==
                   LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) ==
                   LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_STREAM,
                                       1000U, 0U, 0U, 1000000U, &arena,
                                       true) == LXP_OK);
    return 0;
}

static void record_init(lx_stream_record *record)
{
    (void)memset(record, 0, sizeof(*record));
    record->stream_id[0] = 1U;
    record->payer[0] = 2U;
    record->stream_account[0] = 3U;
    record->recipient[0] = 4U;
    record->asset_id[0] = 5U;
    record->mode = LX_STREAM_MODE_TIME;
    record->rate = (lxp_u128){ 0U, 3U };
    record->rate_unit = 1000U;
    record->start_timestamp = 1000U;
    record->last_accrual_timestamp = 1000U;
    record->total_cap = (lxp_u128){ 0U, 100U };
}

static int store_record(const lx_stream_record *record)
{
    LXP_CASE_CHECK(lxp_module_ctx_set_mutable(&ctx, true) == LXP_OK);
    LXP_CASE_CHECK(lx_stream_save(&ctx, record) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(ctx.staged_count == 0U);
    return 0;
}

static int read_totals(const lx_stream_record *stored, uint64_t timestamp,
                        lxp_u128 expected_accrued, lxp_u128 expected_unsettled)
{
    uint8_t key[LX_STREAM_KEY_BYTES];
    uint8_t before[LX_STREAM_RECORD_BYTES];
    uint8_t root_before[32];
    uint8_t root_after[32];
    const uint8_t *bytes;
    size_t length;
    lxp_u128 accrued;
    lxp_u128 unsettled;

    LXP_CASE_CHECK(lx_stream_state_key(stored->stream_id, key) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&ctx, key, sizeof(key), &bytes, &length) ==
                   LXP_OK);
    LXP_CASE_CHECK(length == sizeof(before));
    (void)memcpy(before, bytes, sizeof(before));
    LXP_CASE_CHECK(lxp_state_subtree_root(&kernel, LXP_MODULE_STREAM,
                                          root_before) == LXP_OK);
    LXP_CASE_CHECK(lx_stream_accrued_read(&ctx, stored->stream_id, timestamp,
                                          &accrued, &unsettled) == LXP_OK);
    LXP_CASE_CHECK(lxp_u128_cmp(accrued, expected_accrued) == 0);
    LXP_CASE_CHECK(lxp_u128_cmp(unsettled, expected_unsettled) == 0);
    LXP_CASE_CHECK(ctx.staged_count == 0U);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&ctx, key, sizeof(key), &bytes, &length) ==
                   LXP_OK);
    LXP_CASE_CHECK(length == sizeof(before));
    LXP_CASE_CHECK(memcmp(bytes, before, sizeof(before)) == 0);
    LXP_CASE_CHECK(lxp_state_subtree_root(&kernel, LXP_MODULE_STREAM,
                                          root_after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root_before, root_after, sizeof(root_before)) == 0);
    return 0;
}

static int accrued_read_time_stream(void)
{
    lx_stream_record record;
    lxp_u128 accrued = { 0U, 99U };
    lxp_u128 unsettled = { 0U, 98U };

    LXP_CASE_CHECK(fixture_init() == 0);
    record_init(&record);
    LXP_CASE_CHECK(store_record(&record) == 0);
    LXP_CASE_CHECK(read_totals(&record, 11000U, (lxp_u128){ 0U, 30U },
                                (lxp_u128){ 0U, 30U }) == 0);
    LXP_CASE_CHECK(read_totals(&record, 11500U, (lxp_u128){ 0U, 31U },
                                (lxp_u128){ 0U, 31U }) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_set_mutable(&ctx, false) == LXP_OK);
    LXP_CASE_CHECK(read_totals(&record, 11000U, (lxp_u128){ 0U, 30U },
                                (lxp_u128){ 0U, 30U }) == 0);
    LXP_CASE_CHECK(lx_stream_accrued_read(&ctx, record.stream_id, 999U,
                                          &accrued, &unsettled) ==
                   LXP_ERR_NON_MONOTONIC_TIME);
    LXP_CASE_CHECK(accrued.hi == 0U && accrued.lo == 99U);
    LXP_CASE_CHECK(unsettled.hi == 0U && unsettled.lo == 98U);
    LXP_CASE_CHECK(lx_stream_accrued_read(&ctx, record.stream_id, 11000U,
                                          NULL, &unsettled) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lx_stream_accrued_read(&ctx, record.stream_id, 11000U,
                                          &accrued, NULL) ==
                   LXP_ERR_NON_CANONICAL);

    record.accrued_total = (lxp_u128){ 0U, 10U };
    record.settled_total = (lxp_u128){ 0U, 7U };
    record.remainder_carry = (lxp_u128){ 0U, 500U };
    record.last_accrual_timestamp = 4500U;
    LXP_CASE_CHECK(store_record(&record) == 0);
    LXP_CASE_CHECK(read_totals(&record, 5000U, (lxp_u128){ 0U, 12U },
                                (lxp_u128){ 0U, 5U }) == 0);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store) == LXP_OK);
    return 0;
}

static int accrued_read_metered_and_missing(void)
{
    lx_stream_record record;
    lx_stream_record missing_record;
    uint8_t missing_id[32] = { 99U };
    lxp_u128 accrued = { 0U, 99U };
    lxp_u128 unsettled = { 0U, 98U };
    lxp_result missing_status;

    LXP_CASE_CHECK(fixture_init() == 0);
    record_init(&record);
    record.mode = LX_STREAM_MODE_METERED;
    record.meter_authorities[0][0] = 6U;
    record.meter_authority_count = 1U;
    record.total_cap = (lxp_u128){ 2U, 0U };
    record.accrued_total = (lxp_u128){ 1U, 30U };
    record.settled_total = (lxp_u128){ 1U, 11U };
    LXP_CASE_CHECK(store_record(&record) == 0);
    LXP_CASE_CHECK(read_totals(&record, 11000U, record.accrued_total,
                                (lxp_u128){ 0U, 19U }) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_set_mutable(&ctx, false) == LXP_OK);
    LXP_CASE_CHECK(read_totals(&record, 11500U, record.accrued_total,
                                (lxp_u128){ 0U, 19U }) == 0);

    record_init(&record);
    record.accrued_total = (lxp_u128){ 0U, 40U };
    record.settled_total = (lxp_u128){ 0U, 15U };
    record.paused = true;
    LXP_CASE_CHECK(store_record(&record) == 0);
    LXP_CASE_CHECK(read_totals(&record, 11000U, record.accrued_total,
                                (lxp_u128){ 0U, 25U }) == 0);
    record.paused = false;
    record.closed = true;
    LXP_CASE_CHECK(store_record(&record) == 0);
    LXP_CASE_CHECK(read_totals(&record, 11500U, record.accrued_total,
                                (lxp_u128){ 0U, 25U }) == 0);
    record.closed = false;
    record.underfunded = true;
    LXP_CASE_CHECK(store_record(&record) == 0);
    LXP_CASE_CHECK(read_totals(&record, 11500U, record.accrued_total,
                                (lxp_u128){ 0U, 25U }) == 0);

    missing_status = lx_stream_load(&ctx, missing_id, &missing_record);
    LXP_CASE_CHECK(missing_status == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(lx_stream_accrued_read(&ctx, missing_id, 11000U,
                                          &accrued, &unsettled) ==
                   missing_status);
    LXP_CASE_CHECK(accrued.hi == 0U && accrued.lo == 99U);
    LXP_CASE_CHECK(unsettled.hi == 0U && unsettled.lo == 98U);
    LXP_CASE_CHECK(ctx.staged_count == 0U);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "accrued_read_time_stream", accrued_read_time_stream },
        { "accrued_read_metered_and_missing", accrued_read_metered_and_missing }
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
