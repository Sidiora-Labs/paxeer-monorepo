#include "layerx/lx_budget.h"
#include <stdio.h>
#include <string.h>

#define CHECK(x) do { if (!(x)) { fprintf(stderr, "caps vector line %d\n", __LINE__); return 1; } } while (0)

static void field(const char *name, const uint8_t *bytes, size_t length)
{
    printf("\"%s\":\"", name);
    for (size_t i = 0; i < length; ++i) printf("%02x", bytes[i]);
    printf("\",");
}

static int budget_vectors(void)
{
    lx_budget_record record = {0}, decoded;
    uint8_t bytes[1024], key[LX_BUDGET_STATE_KEY_BYTES];
    size_t length;
    memset(record.budget_id, 1, 32);
    memset(record.owner, 2, 32);
    memset(record.budget_account, 3, 32);
    memset(record.asset_id, 4, 32);
    memset(record.purpose_hash, 5, 32);
    record.per_period_limit = (lxp_u128){0, 100};
    record.configured_period_limit = record.per_period_limit;
    record.spent_this_period = (lxp_u128){0, 23};
    record.period_length = 1000;
    record.period_start = 1000;
    record.expiry = 9000;
    record.revocation_sequence = 31;
    record.rollover_policy = LX_BUDGET_ROLLOVER_NONE;
    record.delegate_count = 2;
    memset(record.delegates[0], 6, 32);
    memset(record.delegates[1], 7, 32);
    CHECK(lx_budget_state_key(record.budget_id, key) == LXP_OK);
    field("budget_key", key, sizeof(key));
    CHECK(lx_budget_record_encode(&record, bytes, sizeof(bytes), &length) == LXP_OK);
    CHECK(lx_budget_record_decode(bytes, length, &decoded) == LXP_OK);
    field("budget_v1", bytes, length);
    record.native_source = true;
    memset(record.source_account, 8, 32);
    CHECK(lx_budget_record_encode(&record, bytes, sizeof(bytes), &length) == LXP_OK);
    CHECK(lx_budget_record_decode(bytes, length, &decoded) == LXP_OK);
    field("budget_v2", bytes, length);
    record.delegate_count = 16;
    for (size_t i = 0; i < 16; ++i) memset(record.delegates[i], (int)i + 1, 32);
    record.per_period_limit = (lxp_u128){UINT64_MAX, UINT64_MAX};
    record.configured_period_limit = record.per_period_limit;
    record.spent_this_period = record.per_period_limit;
    record.carry_cap = record.per_period_limit;
    record.carried = record.per_period_limit;
    record.rollover_policy = LX_BUDGET_ROLLOVER_CAPPED;
    record.period_start = 0;
    record.period_length = UINT64_MAX;
    record.expiry = UINT64_MAX;
    record.revocation_sequence = UINT64_MAX;
    record.closed = true;
    record.revoked = true;
    CHECK(lx_budget_record_encode(&record, bytes, sizeof(bytes), &length) == LXP_OK);
    CHECK(lx_budget_record_decode(bytes, length, &decoded) == LXP_OK);
    field("budget_extreme", bytes, length);
    return 0;
}

int main(void)
{
    printf("{");
    CHECK(budget_vectors() == 0);
    printf("\"producer\":\"native-budget-codec-and-grant-save\"}\n");
    return 0;
}
