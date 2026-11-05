#include "layerx/lxp_transfer.h"
#include "lxp_kernel_case.h"

static int ledger_journal_rollback_restores(void)
{
    lx_account accounts[2] = { 0 };
    lx_account before[2];
    lxp_transfer_leg leg = { 0 };
    lxp_transfer_result result;
    lxp_ledger_journal journal;
    size_t i;

    accounts[0].balance = (lxp_u128){ 2U, 100U };
    (void)memset(accounts[0].asset_id, 0x31, 32U);
    accounts[0].has_asset = true;
    accounts[0].next_sequence = 11U;
    accounts[1].next_sequence = 23U;
    before[0] = accounts[0];
    before[1] = accounts[1];
    leg.from = &accounts[0];
    leg.to = &accounts[1];
    (void)memcpy(leg.asset_id, accounts[0].asset_id, 32U);
    leg.amount = (lxp_u128){ 0U, 7U };

    LXP_CASE_CHECK(lxp_journal_open(&leg, 1U, &journal) == LXP_OK);
    LXP_CASE_CHECK(journal.open);
    LXP_CASE_CHECK(journal.count == 2U);
    LXP_CASE_CHECK(lxp_balance_apply_leg(&leg, &result) == LXP_OK);
    LXP_CASE_CHECK(accounts[0].balance.hi == 2U);
    LXP_CASE_CHECK(accounts[0].balance.lo == 93U);
    LXP_CASE_CHECK(accounts[1].balance.hi == 0U);
    LXP_CASE_CHECK(accounts[1].balance.lo == 7U);
    LXP_CASE_CHECK(accounts[1].has_asset);
    LXP_CASE_CHECK(memcmp(accounts[1].asset_id, leg.asset_id, 32U) == 0);

    ++accounts[0].next_sequence;
    ++accounts[1].next_sequence;
    (void)memset(accounts[0].asset_id, 0x72, 32U);
    accounts[0].has_asset = false;

    LXP_CASE_CHECK(lxp_journal_rollback(&journal) == LXP_OK);
    LXP_CASE_CHECK(!journal.open);
    for (i = 0U; i < 2U; ++i) {
        LXP_CASE_CHECK(accounts[i].balance.hi == before[i].balance.hi);
        LXP_CASE_CHECK(accounts[i].balance.lo == before[i].balance.lo);
        LXP_CASE_CHECK(memcmp(accounts[i].asset_id,
                              before[i].asset_id, 32U) == 0);
        LXP_CASE_CHECK(accounts[i].has_asset == before[i].has_asset);
        LXP_CASE_CHECK(accounts[i].next_sequence == before[i].next_sequence);
    }
    return 0;
}

static int ledger_journal_state_machine(void)
{
    lx_account from = { 0 };
    lx_account to = { 0 };
    lxp_transfer_leg leg = { 0 };
    lxp_transfer_result result;
    lxp_ledger_journal journal = { 0 };

    from.balance = (lxp_u128){ 0U, 20U };
    (void)memset(from.asset_id, 0x31, 32U);
    from.has_asset = true;
    leg.from = &from;
    leg.to = &to;
    (void)memcpy(leg.asset_id, from.asset_id, 32U);
    leg.amount = (lxp_u128){ 0U, 7U };

    LXP_CASE_CHECK(lxp_journal_commit(NULL) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_journal_rollback(NULL) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_journal_commit(&journal) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_journal_rollback(&journal) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_journal_open(&leg, 0U, &journal) ==
                   LXP_ERR_TOO_MANY_LEGS);
    LXP_CASE_CHECK(!journal.open);
    LXP_CASE_CHECK(lxp_journal_open(&leg, LXP_MAX_TRANSFER_SET_LEGS + 1U,
                                   &journal) == LXP_ERR_TOO_MANY_LEGS);
    LXP_CASE_CHECK(lxp_journal_open(NULL, 1U, &journal) ==
                   LXP_ERR_TOO_MANY_LEGS);
    LXP_CASE_CHECK(lxp_journal_open(&leg, 1U, NULL) ==
                   LXP_ERR_TOO_MANY_LEGS);

    leg.from = NULL;
    LXP_CASE_CHECK(lxp_journal_open(&leg, 1U, &journal) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(!journal.open);
    LXP_CASE_CHECK(lxp_journal_commit(&journal) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_journal_rollback(&journal) == LXP_ERR_NON_CANONICAL);
    leg.from = &from;
    leg.to = NULL;
    LXP_CASE_CHECK(lxp_journal_open(&leg, 1U, &journal) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(!journal.open);
    LXP_CASE_CHECK(lxp_journal_commit(&journal) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_journal_rollback(&journal) == LXP_ERR_NON_CANONICAL);
    leg.to = &to;

    LXP_CASE_CHECK(lxp_journal_open(&leg, 1U, &journal) == LXP_OK);
    LXP_CASE_CHECK(journal.open);
    LXP_CASE_CHECK(lxp_balance_apply_leg(&leg, &result) == LXP_OK);
    LXP_CASE_CHECK(lxp_journal_commit(&journal) == LXP_OK);
    LXP_CASE_CHECK(!journal.open);
    LXP_CASE_CHECK(lxp_journal_commit(&journal) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_journal_rollback(&journal) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(from.balance.hi == 0U && from.balance.lo == 13U);
    LXP_CASE_CHECK(to.balance.hi == 0U && to.balance.lo == 7U);
    LXP_CASE_CHECK(to.has_asset);
    LXP_CASE_CHECK(memcmp(to.asset_id, leg.asset_id, 32U) == 0);

    LXP_CASE_CHECK(lxp_journal_open(&leg, 1U, &journal) == LXP_OK);
    LXP_CASE_CHECK(journal.open);
    LXP_CASE_CHECK(lxp_balance_apply_leg(&leg, &result) == LXP_OK);
    LXP_CASE_CHECK(from.balance.lo == 6U && to.balance.lo == 14U);
    LXP_CASE_CHECK(lxp_journal_rollback(&journal) == LXP_OK);
    LXP_CASE_CHECK(!journal.open);
    LXP_CASE_CHECK(from.balance.hi == 0U && from.balance.lo == 13U);
    LXP_CASE_CHECK(to.balance.hi == 0U && to.balance.lo == 7U);
    LXP_CASE_CHECK(lxp_journal_commit(&journal) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_journal_rollback(&journal) == LXP_ERR_NON_CANONICAL);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "ledger_journal_rollback_restores", ledger_journal_rollback_restores },
        { "ledger_journal_state_machine", ledger_journal_state_machine }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
