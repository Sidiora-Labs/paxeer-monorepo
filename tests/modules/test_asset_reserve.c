#define ASSET_NATIVE_HARNESS_ONLY
#include "test_asset_withdraw_activity.c"
#undef ASSET_NATIVE_HARNESS_ONLY

#include "layerx/lx_asset.h"
#include "../ledger/test_values.h"
#include "../../src/modules/asset/lx_asset_history.h"
#include "../../src/modules/asset/lx_asset_native.h"

#include <string.h>

/* Historical reserve wire format v1, retired under requirement 16961.
 * These bytes are codec vectors only, never current backing authority. */
static int test_historical_reserve_v1(void)
{
    lx_asset_reserve_report_record report = {0};
    uint8_t encoded[274];
    size_t length;
    report.asset_id[0] = 1U;
    report.agent_main.lo = 10U;
    report.reserve.lo = 90U;
    report.raw_total.lo = 100U;
    report.circulating.lo = 10U;
    report.effective_total.lo = 80U;
    report.expected_backing.lo = 80U;
    if (!TEST_EQ(lx_asset_historical_reserve_encode_v1(&report, encoded,
            sizeof(encoded), &length), LXP_OK) ||
        !TEST_EQ(length, 274U) || !TEST_EQ(encoded[47], 10U) ||
        !TEST_EQ(encoded[207], 90U) || !TEST_EQ(encoded[223], 100U) ||
        !TEST_EQ(encoded[239], 10U) || !TEST_EQ(encoded[255], 80U) ||
        !TEST_EQ(encoded[271], 80U) || !TEST_EQ(encoded[272], 0U) ||
        !TEST_EQ(encoded[273], 0U)) return 1;
    return 0;
}

static int test_native_reserve_conservation(void)
{
    native_repair_fixture *f = calloc(1U, sizeof(*f));
    lx_account *other;
    lx_asset_record *committed;
    lx_asset_native_supply_report report, saved;
    lxp_send send;
    lxp_result outcome;
    uint8_t root[32], after[32];
    const char other_name[] = "agent:did:key:native-other:main";
    NATIVE_CHECK(f != NULL);
    NATIVE_CHECK(native_fixture_init(f) == 0 && native_issue(f, 0U) == 0);
    NATIVE_CHECK(lx_asset_lookup(&f->assets, f->asset_ids[0], &committed) == LXP_OK);
    NATIVE_CHECK(lx_asset_account_open(&f->assets, &f->accounts, f->asset_ids[0],
        (const uint8_t *)other_name, sizeof(other_name) - 1U,
        f->state.next_sequence, LX_ACCOUNT_OPEN_CREDIT, NULL, &other) == LXP_OK);
    NATIVE_CHECK(native_make_send(f, 0U, 0U, f->holder_ids[0][1], 30U, 80U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 0U, false, &outcome) == 0 && outcome == LXP_OK);
    NATIVE_CHECK(lx_asset_native_reconcile(committed, &f->accounts, &report) == LXP_OK &&
        report.bank_units.lo == 100U && report.registered_units.lo == 100U &&
        report.issuance_present && report.issuance_remaining.lo == 0U);
    NATIVE_CHECK(native_make_send(f, 0U, 1U, other->id, 10U, 81U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 1U, false, &outcome) == 0 && outcome == LXP_OK);
    NATIVE_CHECK(lx_asset_native_supply_check(&f->assets, &f->accounts) == LXP_OK);
    NATIVE_CHECK(native_make_send(f, 0U, 1U, f->holder_ids[0][0], 20U, 82U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 1U, false, &outcome) == 0 && outcome == LXP_OK);
    NATIVE_CHECK(native_holder(f, 0U, 0U)->balance.lo == 90U &&
        native_holder(f, 0U, 1U)->balance.lo == 0U && other->balance.lo == 10U);
    NATIVE_CHECK(lx_asset_native_reconcile(committed, &f->accounts, &report) == LXP_OK &&
        report.bank_units.lo == 100U && report.registered_units.lo == 100U);
    saved = report;
    NATIVE_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
    ++other->balance.lo;
    NATIVE_CHECK(lx_asset_native_reconcile(committed, &f->accounts, &report) == LXP_FATAL_SUPPLY_MISMATCH);
    NATIVE_CHECK(memcmp(&report, &saved, sizeof(report)) == 0);
    --other->balance.lo;
    --committed->total_units.lo;
    NATIVE_CHECK(lx_asset_native_supply_check(&f->assets, &f->accounts) == LXP_FATAL_SUPPLY_MISMATCH);
    ++committed->total_units.lo;
    NATIVE_CHECK(lx_asset_native_supply_check(&f->assets, &f->accounts) == LXP_OK);
    NATIVE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK && memcmp(root, after, 32U) == 0);
    size_t account_count = f->accounts.count;
    size_t asset_count = f->assets.count;
    f->accounts.count = LX_ACCOUNT_REGISTRY_CAPACITY + 1U;
    NATIVE_CHECK(lx_asset_native_reconcile(committed, &f->accounts, &report) == LXP_FATAL_SUPPLY_MISMATCH);
    f->accounts.count = account_count;
    f->assets.count = LX_ASSET_REGISTRY_CAPACITY + 1U;
    NATIVE_CHECK(lx_asset_supply_check(&f->assets, &f->accounts, NULL, 0U) == LXP_FATAL_SUPPLY_MISMATCH);
    f->assets.count = asset_count;
    lx_asset_custody_attestation retired = {0};
    NATIVE_CHECK(lx_asset_supply_check(&f->assets, &f->accounts, &retired,
        LX_ASSET_REGISTRY_CAPACITY + 1U) == LXP_FATAL_SUPPLY_MISMATCH);
    NATIVE_CHECK(native_fixture_destroy(f) == 0);
    free(f);
    return 0;
}

static int test_native_supply_invariants(void)
{
    lx_asset_registry assets;
    lx_asset_record asset = {0};
    lx_account_registry accounts;
    lx_account *first, *second, *extra;
    lx_asset_native_supply_report report, unchanged;
    lx_asset_custody_attestation attestation = {0};
    lx_asset_reserve_report_record historical, saved_historical;
    uint8_t bytes[274], saved_bytes[274];
    uint8_t saved_id[32];
    size_t length = 17U;
    const char *names[] = {"agent:did:key:native-a:main",
        "agent:did:key:native-b:main", "agent:did:key:native-c:main"};
    lx_account **opened[] = {&first, &second, &extra};
    asset.asset_id[0] = 1U;
    memcpy(asset.symbol, "N", 2U);
    asset.symbol_length = 1U;
    asset.name[0] = 'N'; asset.name_length = 1U;
    asset.issuer_kind = 2U; asset.issuer_did32[0] = 1U;
    asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    asset.custody_reference[0] = 1U; asset.custody_reference_length = 1U;
    asset.total_units = (lxp_u128){UINT64_MAX, UINT64_MAX};
    if (!TEST_EQ(lx_asset_registry_init(&assets, 0U), LXP_OK) ||
        !TEST_EQ(lx_asset_register(&assets, &asset, 0U, (lxp_u128){0U, 0U}), LXP_OK) ||
        !TEST_EQ(lx_account_registry_init(&accounts), LXP_OK)) return 1;
    asset.asset_id[0] = 2U; asset.total_units = (lxp_u128){0U, 1U};
    if (!TEST_EQ(lx_asset_register(&assets, &asset, 1U, (lxp_u128){0U, 0U}), LXP_OK)) return 1;
    for (size_t i = 0U; i < 3U; ++i) {
        const uint8_t *id = assets.assets[i == 1U ? 1U : 0U].asset_id;
        if (!TEST_EQ(lx_asset_account_open(&assets, &accounts, id,
            (const uint8_t *)names[i], strlen(names[i]), i + 1U,
            LX_ACCOUNT_OPEN_CREDIT, NULL, opened[i]), LXP_OK)) return 1;
    }
    if (!TEST_EQ(lxp_ledger_bootstrap_balance(first, assets.assets[0].asset_id,
            assets.assets[0].total_units, 0U), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(second, assets.assets[1].asset_id,
            assets.assets[1].total_units, 0U), LXP_OK) ||
        !TEST_EQ(lx_asset_native_supply_check(&assets, &accounts), LXP_OK) ||
        !TEST_EQ(lx_asset_supply_check(&assets, &accounts, NULL, 0U), LXP_OK) ||
        !TEST_EQ(lx_asset_native_reconcile(&assets.assets[0], &accounts, &report), LXP_OK) ||
        !TEST_EQ(report.bank_units.hi, UINT64_MAX) ||
        !TEST_EQ(report.bank_units.lo, UINT64_MAX) ||
        !TEST_EQ(report.issuance_present, false)) return 1;
    unchanged = report;
    extra->balance.lo = 1U;
    if (!TEST_EQ(lx_asset_native_reconcile(&assets.assets[0], &accounts, &report),
            LXP_FATAL_SUPPLY_MISMATCH) ||
        !TEST_EQ(memcmp(&report, &unchanged, sizeof(report)), 0)) return 1;
    extra->balance.lo = 0U;
    --assets.assets[0].total_units.lo;
    if (!TEST_EQ(lx_asset_native_supply_check(&assets, &accounts), LXP_FATAL_SUPPLY_MISMATCH)) return 1;
    ++assets.assets[0].total_units.lo;
    assets.assets[0].supply_cap = (lxp_u128){UINT64_MAX, UINT64_MAX - 1U};
    if (!TEST_EQ(lx_asset_native_supply_check(&assets, &accounts), LXP_FATAL_SUPPLY_MISMATCH)) return 1;
    assets.assets[0].supply_cap = (lxp_u128){0U, 0U};
    second->asset_id[0] = 3U;
    if (!TEST_EQ(lx_asset_native_supply_check(&assets, &accounts), LXP_FATAL_SUPPLY_MISMATCH)) return 1;
    second->asset_id[0] = 2U;
    second->has_asset = false;
    if (!TEST_EQ(lx_asset_native_supply_check(&assets, &accounts), LXP_FATAL_SUPPLY_MISMATCH)) return 1;
    second->has_asset = true;
    memcpy(saved_id, second->id, 32U);
    memcpy(second->id, first->id, 32U);
    if (!TEST_EQ(lx_asset_native_supply_check(&assets, &accounts), LXP_FATAL_SUPPLY_MISMATCH)) return 1;
    memcpy(second->id, saved_id, 32U);
    assets.assets[1].asset_id[0] = 1U;
    if (!TEST_EQ(lx_asset_native_supply_check(&assets, &accounts), LXP_FATAL_SUPPLY_MISMATCH)) return 1;
    assets.assets[1].asset_id[0] = 2U;
    if (!TEST_EQ(lx_asset_native_supply_check(&assets, &accounts), LXP_OK)) return 1;
    attestation.finalized = true;
    attestation.custody_amount = (lxp_u128){UINT64_MAX, UINT64_MAX};
    if (!TEST_EQ(lx_asset_supply_check(&assets, &accounts, &attestation, 1U),
            LXP_FATAL_SUPPLY_MISMATCH) ||
        !TEST_EQ(lx_asset_supply_check(&assets, &accounts, NULL, 1U),
            LXP_FATAL_SUPPLY_MISMATCH)) return 1;
    memset(&historical, 0x5a, sizeof(historical));
    saved_historical = historical;
    memset(bytes, 0x5a, sizeof(bytes)); memcpy(saved_bytes, bytes, sizeof(bytes));
    if (!TEST_EQ(lx_asset_reserve_report(&accounts, &attestation, &historical), LXP_ERR_VERSION_UNSUPPORTED) ||
        !TEST_EQ(lx_asset_reserve_reconcile(&accounts, &attestation, &historical), LXP_ERR_VERSION_UNSUPPORTED) ||
        !TEST_EQ(memcmp(&historical, &saved_historical, sizeof(historical)), 0) ||
        !TEST_EQ(lx_asset_reserve_report_encode(&historical, bytes, sizeof(bytes), &length), LXP_ERR_VERSION_UNSUPPORTED) ||
        !TEST_EQ(length, 17U) || !TEST_EQ(memcmp(bytes, saved_bytes, sizeof(bytes)), 0)) return 1;
    lx_account_registry_release(&accounts);
    return 0;
}

int main(void)
{
    if (!TEST_EQ(test_native_reserve_conservation(), 0) ||
        !TEST_EQ(test_historical_reserve_v1(), 0) ||
        !TEST_EQ(test_native_supply_invariants(), 0)) return 1;
    return 0;
}
