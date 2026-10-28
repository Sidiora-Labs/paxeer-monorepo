#include "layerx/lx_asset.h"
#include "lxp_kernel_case.h"

#include <string.h>

static void state_fixture(lx_asset_registry *assets,
                          lx_account_registry *accounts,
                          lx_account records[4])
{
    size_t i;

    (void)memset(assets, 0, sizeof(*assets));
    (void)memset(accounts, 0, sizeof(*accounts));
    (void)memset(records, 0, 4U * sizeof(*records));
    assets->count = 3U;
    for (i = 0U; i < assets->count; ++i) {
        lx_asset_record *record = &assets->assets[i];
        record->asset_id[31] = (uint8_t)(i + 1U);
        (void)memcpy(record->symbol, "TOK", 4U);
        record->symbol_length = 3U;
        record->name[0] = (uint8_t)'T';
        record->name_length = 1U;
        record->decimals = 6U;
        record->issuer_kind = 2U;
        record->issuer_did32[0] = 1U;
        record->custody_kind = LX_ASSET_CUSTODY_PAXEER;
        record->custody_reference[0] = (uint8_t)(i + 1U);
        record->custody_reference_length = 1U;
    }
    accounts->accounts = records;
    accounts->count = 4U;
    accounts->capacity = 4U;
    accounts->borrowed = true;
    for (i = 0U; i < accounts->count; ++i) {
        (void)memcpy(records[i].asset_id, assets->assets[i % 3U].asset_id,
                     32U);
        records[i].id[31] = i == 3U ? 2U : 1U;
        records[i].kind = LX_ACCOUNT_AGENT_MAIN;
        records[i].has_asset = true;
        records[i].balance = (lxp_u128){ 0U, 10U + i };
    }
}

static int asset_state_root_rejects_duplicate_key(void)
{
    lx_asset_registry assets;
    lx_account_registry accounts;
    lx_account records[4];
    uint8_t root[32];
    uint8_t untouched[32];

    (void)memset(untouched, 0xa5, sizeof(untouched));
    (void)memcpy(root, untouched, sizeof(root));
    state_fixture(&assets, &accounts, records);
    (void)memcpy(assets.assets[2].asset_id, assets.assets[0].asset_id, 32U);
    LXP_CASE_CHECK(lx_asset_state_root(&assets, &accounts, root) ==
                   LXP_ERR_SEQUENCE_REUSED);
    LXP_CASE_CHECK(memcmp(root, untouched, sizeof(root)) == 0);

    state_fixture(&assets, &accounts, records);
    (void)memcpy(records[3].id, records[0].id, 32U);
    LXP_CASE_CHECK(lx_asset_state_root(&assets, &accounts, root) ==
                   LXP_ERR_SEQUENCE_REUSED);
    LXP_CASE_CHECK(memcmp(root, untouched, sizeof(root)) == 0);

    state_fixture(&assets, &accounts, records);
    (void)memset(records[0].id, 0, sizeof(records[0].id));
    LXP_CASE_CHECK(lx_asset_state_root(&assets, &accounts, root) ==
                   LXP_ERR_SEQUENCE_REUSED);
    LXP_CASE_CHECK(memcmp(root, untouched, sizeof(root)) == 0);
    return 0;
}

static int check_permutations(lx_asset_registry *assets,
                              lx_account_registry *accounts, size_t depth,
                              const uint8_t expected[32], size_t *checked)
{
    size_t i;

    if (depth == assets->count + accounts->count) {
        uint8_t root[32];
        LXP_CASE_CHECK(lx_asset_state_root(assets, accounts, root) == LXP_OK);
        LXP_CASE_CHECK(memcmp(root, expected, sizeof(root)) == 0);
        ++*checked;
        return 0;
    }
    if (depth < assets->count) {
        for (i = depth; i < assets->count; ++i) {
            lx_asset_record saved = assets->assets[depth];
            assets->assets[depth] = assets->assets[i];
            assets->assets[i] = saved;
            LXP_CASE_CHECK(check_permutations(assets, accounts, depth + 1U,
                                              expected, checked) == 0);
            assets->assets[i] = assets->assets[depth];
            assets->assets[depth] = saved;
        }
    } else {
        size_t slot = depth - assets->count;
        for (i = slot; i < accounts->count; ++i) {
            lx_account saved = accounts->accounts[slot];
            accounts->accounts[slot] = accounts->accounts[i];
            accounts->accounts[i] = saved;
            LXP_CASE_CHECK(check_permutations(assets, accounts, depth + 1U,
                                              expected, checked) == 0);
            accounts->accounts[i] = accounts->accounts[slot];
            accounts->accounts[slot] = saved;
        }
    }
    return 0;
}

static int asset_state_root_order_independent(void)
{
    lx_asset_registry assets;
    lx_account_registry accounts;
    lx_account records[4];
    uint8_t expected[32];
    size_t checked = 0U;

    state_fixture(&assets, &accounts, records);
    LXP_CASE_CHECK(lx_asset_state_root(&assets, &accounts, expected) == LXP_OK);
    LXP_CASE_CHECK(check_permutations(&assets, &accounts, 0U, expected,
                                      &checked) == 0);
    LXP_CASE_CHECK(checked == 144U);
    records[0].balance.lo += 1U;
    {
        uint8_t changed[32];
        LXP_CASE_CHECK(lx_asset_state_root(&assets, &accounts, changed) == LXP_OK);
        LXP_CASE_CHECK(memcmp(changed, expected, sizeof(changed)) != 0);
    }
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "asset_state_root_rejects_duplicate_key",
          asset_state_root_rejects_duplicate_key },
        { "asset_state_root_order_independent",
          asset_state_root_order_independent }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
