#include "layerx/lxp_transfer.h"
#include "lxp_kernel_case.h"

extern lxp_result lxp_ledger_apply_bridge_credit(lx_account *reserve,
                                               const uint8_t asset_id[32],
                                               lxp_u128 amount);

#pragma weak lxp_ledger_apply_bridge_credit

static int ledger_bridge_credit_absent(void)
{
    lx_account account = { 0 };
    const uint8_t asset_id[32] = { 0x31U };
    lxp_u128 balance;

    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(&account, asset_id,
                       (lxp_u128){ 2U, 100U }, 17U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_balance_get(&account, asset_id, &balance) ==
                   LXP_OK);
    LXP_CASE_CHECK(balance.hi == 2U && balance.lo == 100U);
    LXP_CASE_CHECK(lxp_ledger_apply_bridge_credit == NULL);
    return 0;
}

static int ledger_restore_snapshot_kept(void)
{
    lx_account accounts[2] = { 0 };
    lxp_transfer_leg leg = { 0 };
    lxp_transfer_result result;
    lxp_ledger_journal journal;
    const uint8_t asset_id[32] = { 0x31U };
    const lxp_ledger_journal_entry *entry;
    lxp_u128 balance;
    size_t i;

    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(&accounts[0], asset_id,
                       (lxp_u128){ 2U, 100U }, UINT64_MAX) == LXP_OK);
    accounts[0].frozen = true;
    accounts[1].next_sequence = 23U;
    leg.from = &accounts[0];
    leg.to = &accounts[1];
    (void)memcpy(leg.asset_id, asset_id, 32U);
    leg.amount = (lxp_u128){ 0U, 7U };

    LXP_CASE_CHECK(lxp_journal_open(&leg, 1U, &journal) == LXP_OK);
    LXP_CASE_CHECK(journal.open && journal.count == 2U);
    LXP_CASE_CHECK(lxp_balance_apply_leg(&leg, &result) == LXP_OK);
    LXP_CASE_CHECK(accounts[0].balance.hi == 2U);
    LXP_CASE_CHECK(accounts[0].balance.lo == 93U);
    LXP_CASE_CHECK(accounts[1].balance.hi == 0U);
    LXP_CASE_CHECK(accounts[1].balance.lo == 7U);
    LXP_CASE_CHECK(accounts[1].has_asset);

    (void)memset(accounts[0].asset_id, 0x72, 32U);
    accounts[0].has_asset = false;
    accounts[0].next_sequence = 1U;
    accounts[1].next_sequence = 24U;
    entry = &journal.entries[0];
    LXP_CASE_CHECK(entry->account == &accounts[0]);
    LXP_CASE_CHECK(lxp_ledger_restore_account_snapshot(NULL,
                       entry->balance_before, entry->asset_id,
                       entry->has_asset, entry->next_sequence) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_ledger_restore_account_snapshot(entry->account,
                       entry->balance_before, NULL, entry->has_asset,
                       entry->next_sequence) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(accounts[0].balance.hi == 2U);
    LXP_CASE_CHECK(accounts[0].balance.lo == 93U);
    LXP_CASE_CHECK(accounts[0].asset_id[0] == 0x72U);
    LXP_CASE_CHECK(!accounts[0].has_asset);
    LXP_CASE_CHECK(accounts[0].next_sequence == 1U);
    LXP_CASE_CHECK(lxp_ledger_restore_account_snapshot(entry->account,
                       entry->balance_before, entry->asset_id,
                       entry->has_asset, entry->next_sequence) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_balance_get(&accounts[0], asset_id, &balance) ==
                   LXP_OK);
    LXP_CASE_CHECK(balance.hi == 2U && balance.lo == 100U);
    LXP_CASE_CHECK(accounts[0].has_asset);
    LXP_CASE_CHECK(accounts[0].next_sequence == UINT64_MAX);
    LXP_CASE_CHECK(accounts[0].frozen);

    LXP_CASE_CHECK(lxp_journal_rollback(&journal) == LXP_OK);
    LXP_CASE_CHECK(!journal.open);
    for (i = 0U; i < 2U; ++i) {
        entry = &journal.entries[i];
        LXP_CASE_CHECK(entry->account == &accounts[i]);
        LXP_CASE_CHECK(accounts[i].balance.hi == entry->balance_before.hi);
        LXP_CASE_CHECK(accounts[i].balance.lo == entry->balance_before.lo);
        LXP_CASE_CHECK(memcmp(accounts[i].asset_id, entry->asset_id, 32U) == 0);
        LXP_CASE_CHECK(accounts[i].has_asset == entry->has_asset);
        LXP_CASE_CHECK(accounts[i].next_sequence == entry->next_sequence);
    }
    LXP_CASE_CHECK(!accounts[1].has_asset);
    LXP_CASE_CHECK(lxp_state_balance_get(&accounts[1], asset_id, &balance) ==
                   LXP_ERR_ASSET_MISMATCH);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "ledger_bridge_credit_absent", ledger_bridge_credit_absent },
        { "ledger_restore_snapshot_kept", ledger_restore_snapshot_kept }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
