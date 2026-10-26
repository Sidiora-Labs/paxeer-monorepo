#include "layerx/lxp_fee.h"
#include "layerx/lxp_ledger.h"
#include "lxp_kernel_case.h"

lxp_result lxp_send_store_admit(lxp_send_store *store);
lxp_result lxp_send_store_append(lxp_send_store *store,
                                 const lxp_send_store_record *record);
lxp_result lxp_send_store_lookup(const lxp_send_store *store,
                                 const uint8_t idempotency_key[32],
                                 const uint8_t activity_hash[32],
                                 lxp_send_receipt_projection *projection);

#define STORE_CHECK(condition) \
    do { \
        if (!(condition)) { \
            fprintf(stderr, "%s:%d: check failed: %s\n", \
                    __FILE__, __LINE__, #condition); \
            goto cleanup; \
        } \
    } while (0)

static lxp_send_store_record send_store_record(size_t index)
{
    lxp_send_store_record record = { 0 };
    uint64_t value = (uint64_t)index + 1U;
    uint64_t key = UINT64_MAX - value;
    size_t i;

    for (i = 0U; i < 8U; ++i) {
        unsigned int shift = (unsigned int)(7U - i) * 8U;
        record.activity_hash[i] = (uint8_t)(value >> shift);
        record.idempotency_key[i] = (uint8_t)(key >> shift);
    }
    record.receipt.from_before = (lxp_u128){ 1U, value + 100U };
    record.receipt.from_after = (lxp_u128){ 2U, value + 90U };
    record.receipt.to_before = (lxp_u128){ 3U, value + 20U };
    record.receipt.to_after = (lxp_u128){ 4U, value + 30U };
    (void)memcpy(record.receipt.transfer_set_root, record.activity_hash,
                 sizeof(record.receipt.transfer_set_root));
    return record;
}

static bool send_store_receipt_matches(
    const lxp_send_receipt_projection *actual,
    const lxp_send_receipt_projection *expected)
{
    return actual->replayed &&
        lxp_u128_cmp(actual->from_before, expected->from_before) == 0 &&
        lxp_u128_cmp(actual->from_after, expected->from_after) == 0 &&
        lxp_u128_cmp(actual->to_before, expected->to_before) == 0 &&
        lxp_u128_cmp(actual->to_after, expected->to_after) == 0 &&
        memcmp(actual->transfer_set_root, expected->transfer_set_root,
               sizeof(actual->transfer_set_root)) == 0;
}

static int send_store_spill_replays(void)
{
    const size_t capacity = (size_t)LXP_SEND_STORE_CAPACITY;
    const uint64_t history_bytes =
        (uint64_t)LXP_SEND_STORE_CAPACITY * LXP_SEND_HISTORY_RECORD_BYTES;
    lxp_send_store store = { 0 };
    lxp_meter_ctx meter;
    lxp_send_store_record next = send_store_record(capacity);
    lxp_send_receipt_projection projection;
    size_t total = 0U;
    size_t i;
    int result = 1;

    STORE_CHECK(lxp_meter_init(&meter, UINT64_MAX, history_bytes,
        (lxp_u128){ 0U, 3U }, (lxp_u128){ 0U, history_bytes * 3U },
        1U, true) == LXP_OK);
    STORE_CHECK(lxp_send_store_init(&store, &meter) == LXP_OK);
    STORE_CHECK(lxp_send_store_total(&store, &total) == LXP_OK);
    STORE_CHECK(total == 0U);
    STORE_CHECK(lxp_send_store_admit(&store) == LXP_OK);
    for (i = 0U; i < capacity; ++i) {
        lxp_send_store_record record = send_store_record(i);

        STORE_CHECK(lxp_send_store_append(&store, &record) == LXP_OK);
        STORE_CHECK(store.count == i + 1U);
    }
    STORE_CHECK(store.history == NULL);
    STORE_CHECK(meter.net_storage_bytes == 0U);
    STORE_CHECK(lxp_u128_is_zero(meter.storage_fee));
    STORE_CHECK(lxp_send_store_total(&store, &total) == LXP_OK);
    STORE_CHECK(total == capacity);
    STORE_CHECK(lxp_send_store_admit(&store) == LXP_OK);
    STORE_CHECK(store.count == 0U);
    STORE_CHECK(store.history != NULL);
    STORE_CHECK(store.history->count == capacity);
    STORE_CHECK(store.history->stored_bytes == history_bytes);
    STORE_CHECK(meter.net_storage_bytes == history_bytes);
    STORE_CHECK(lxp_u128_cmp(meter.storage_fee,
        (lxp_u128){ 0U, history_bytes * 3U }) == 0);
    STORE_CHECK(lxp_send_store_total(&store, &total) == LXP_OK);
    STORE_CHECK(total == capacity);

    for (i = 0U; i < capacity; ++i) {
        lxp_send_store_record record = send_store_record(i);

        (void)memset(&projection, 0, sizeof(projection));
        STORE_CHECK(lxp_send_store_lookup(&store, record.idempotency_key,
            next.activity_hash, &projection) == LXP_ERR_IDEMPOTENT_REPLAY);
        STORE_CHECK(send_store_receipt_matches(&projection, &record.receipt));
    }
    STORE_CHECK(lxp_send_store_lookup(&store, next.idempotency_key,
        next.activity_hash, &projection) == LXP_OK);
    STORE_CHECK(lxp_send_store_append(&store, &next) == LXP_OK);
    STORE_CHECK(store.count == 1U);
    STORE_CHECK(lxp_send_store_total(&store, &total) == LXP_OK);
    STORE_CHECK(total == capacity + 1U);
    STORE_CHECK(lxp_send_store_admit(&store) == LXP_OK);
    STORE_CHECK(store.count == 1U);
    STORE_CHECK(store.history->count == capacity);
    STORE_CHECK(store.history->stored_bytes == history_bytes);
    STORE_CHECK(meter.net_storage_bytes == history_bytes);
    STORE_CHECK(lxp_u128_cmp(meter.storage_fee,
        (lxp_u128){ 0U, history_bytes * 3U }) == 0);
    STORE_CHECK(lxp_meter_exhausted(&meter) == LXP_OK);
    for (i = 0U; i <= capacity; ++i) {
        lxp_send_store_record record = send_store_record(i);
        const uint8_t different_hash[32] = { 0 };

        (void)memset(&projection, 0, sizeof(projection));
        STORE_CHECK(lxp_send_store_lookup(&store, record.idempotency_key,
            different_hash, &projection) == LXP_ERR_IDEMPOTENT_REPLAY);
        STORE_CHECK(send_store_receipt_matches(&projection, &record.receipt));
    }
    result = 0;

cleanup:
    lxp_send_store_release(&store);
    return result;
}

static int send_store_refusals(void)
{
    const size_t capacity = (size_t)LXP_SEND_STORE_CAPACITY;
    lxp_send_store store = { 0 };
    lxp_send_store unmetered = { 0 };
    lxp_meter_ctx meter;
    lxp_send_store_record extra = send_store_record(capacity);
    lxp_send_receipt_projection projection = { 0 };
    size_t total = 0U;
    size_t i;
    int result = 1;

    STORE_CHECK(lxp_meter_init(&meter, UINT64_MAX, UINT64_MAX,
        (lxp_u128){ 0U, 1U }, (lxp_u128){ 0U, UINT64_MAX },
        1U, true) == LXP_OK);
    STORE_CHECK(lxp_send_store_init(&store, &meter) == LXP_OK);
    STORE_CHECK(lxp_send_store_init(&unmetered, NULL) == LXP_OK);
    STORE_CHECK(lxp_send_store_admit(&unmetered) == LXP_OK);
    for (i = 0U; i < capacity; ++i) {
        lxp_send_store_record record = send_store_record(i);

        STORE_CHECK(lxp_send_store_append(&store, &record) == LXP_OK);
        STORE_CHECK(lxp_send_store_append(&unmetered, &record) == LXP_OK);
        STORE_CHECK(lxp_send_store_lookup(&store, record.idempotency_key,
            record.activity_hash, &projection) == LXP_ERR_SEQUENCE_REUSED);
        STORE_CHECK(lxp_send_store_lookup(&store, extra.idempotency_key,
            record.activity_hash, &projection) == LXP_ERR_SEQUENCE_REUSED);
    }
    STORE_CHECK(lxp_send_store_append(&store, &extra) == LXP_FATAL_INVARIANT);
    STORE_CHECK(lxp_send_store_append(&unmetered, &extra) ==
        LXP_FATAL_INVARIANT);
    STORE_CHECK(store.count == capacity);
    STORE_CHECK(store.history == NULL);
    STORE_CHECK(meter.net_storage_bytes == 0U);
    STORE_CHECK(lxp_send_store_total(&store, &total) == LXP_OK);
    STORE_CHECK(total == capacity);
    STORE_CHECK(lxp_send_store_admit(&unmetered) == LXP_ERR_ARENA_EXHAUSTED);
    STORE_CHECK(unmetered.count == capacity);
    STORE_CHECK(unmetered.history == NULL);
    STORE_CHECK(lxp_send_store_total(&unmetered, &total) == LXP_OK);
    STORE_CHECK(total == capacity);
    STORE_CHECK(lxp_send_store_admit(&store) == LXP_OK);
    STORE_CHECK(store.count == 0U);
    STORE_CHECK(store.history != NULL);
    STORE_CHECK(store.history->count == capacity);
    STORE_CHECK(lxp_send_store_total(&store, &total) == LXP_OK);
    STORE_CHECK(total == capacity);
    for (i = 0U; i < capacity; ++i) {
        lxp_send_store_record record = send_store_record(i);

        STORE_CHECK(lxp_send_store_lookup(&store, record.idempotency_key,
            record.activity_hash, &projection) == LXP_ERR_SEQUENCE_REUSED);
        STORE_CHECK(lxp_send_store_lookup(&unmetered, record.idempotency_key,
            record.activity_hash, &projection) == LXP_ERR_SEQUENCE_REUSED);
        (void)memset(&projection, 0, sizeof(projection));
        STORE_CHECK(lxp_send_store_lookup(&unmetered, record.idempotency_key,
            extra.activity_hash, &projection) == LXP_ERR_IDEMPOTENT_REPLAY);
        STORE_CHECK(send_store_receipt_matches(&projection, &record.receipt));
    }
    result = 0;

cleanup:
    lxp_send_store_release(&unmetered);
    lxp_send_store_release(&store);
    return result;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "send_store_spill_replays", send_store_spill_replays },
        { "send_store_refusals", send_store_refusals }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
