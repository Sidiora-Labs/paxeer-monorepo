#include "test_asset_retirement.h"

#include "layerx/lx_asset.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_receipt.h"
#include "layerx/lxp_state.h"
#include "layerx/lxp_state_proof.h"
#include <stdlib.h>

#include <string.h>

typedef struct withdraw_fixture {
    lx_asset_registry assets;
    lx_asset_record asset_a;
    lx_asset_record asset_b;
    lx_account_registry accounts_a;
    lx_account_registry accounts_b;
    lx_account *agent_a;
    lx_account *withdrawals_a;
    lx_account *reserve_a;
    lx_account *agent_b;
    lx_account *withdrawals_b;
    lx_account *reserve_b;
    lxp_transfer_asset_state asset_states[2];
    lx_withdrawal_store store;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx ctx;
    lxp_arena arena;
    uint8_t arena_bytes[4096];
    lxp_receipt receipt;
    lxp_effect_buffer effects;
    uint64_t global_sequence;
} withdraw_fixture;

static withdraw_fixture fixture;

static int fresh_ctx(withdraw_fixture *f)
{
    ++f->global_sequence;
    if (!TEST_EQ(lxp_arena_init(&f->arena, f->arena_bytes, sizeof(f->arena_bytes)), LXP_OK) ||
        !TEST_EQ(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_ASSET, 10U, 0U,
                            f->global_sequence, 1000U, &f->arena, true), LXP_OK))
        return 1;
    if (!TEST_EQ(lxp_effect_buffer_init(&f->effects), LXP_OK) ||
        !TEST_EQ(lxp_module_ctx_bind_effects(&f->ctx, &f->effects), LXP_OK))
        return 1;
    return 0;
}

static int open_ledger(withdraw_fixture *f, const lx_asset_record *asset,
                       lx_account_registry *accounts, const char *agent_name,
                       lx_account **agent, lx_account **withdrawals,
                       lx_account **reserve)
{
    if (!TEST_EQ(lx_account_registry_init(accounts), LXP_OK) ||
        !TEST_EQ(lx_asset_account_open(&f->assets, accounts, asset->asset_id,
            (const uint8_t *)agent_name, strlen(agent_name), 1U,
            LX_ACCOUNT_OPEN_CREDIT, NULL, agent), LXP_OK) ||
        !TEST_EQ(lx_asset_account_open(&f->assets, accounts, asset->asset_id,
            (const uint8_t *)"system:paxeer-withdrawals", 25U, 1U,
            LX_ACCOUNT_OPEN_GENESIS, NULL, withdrawals), LXP_OK) ||
        !TEST_EQ(lx_asset_account_open(&f->assets, accounts, asset->asset_id,
            (const uint8_t *)"system:paxeer-reserve", 21U, 1U,
            LX_ACCOUNT_OPEN_GENESIS, NULL, reserve), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(*agent, asset->asset_id,
            (lxp_u128){ 0U, 100U }, 0U), LXP_OK))
        return 1;
    return 0;
}

static int roots(const withdraw_fixture *f, uint8_t out[96])
{
    if (!TEST_EQ(lxp_state_root(&f->kernel, out), LXP_OK) ||
        !TEST_EQ(lx_asset_state_root(&f->assets, &f->accounts_a, out + 32), LXP_OK) ||
        !TEST_EQ(lx_asset_state_root(&f->assets, &f->accounts_b, out + 64), LXP_OK))
        return 1;
    return 0;
}

static void balances(const withdraw_fixture *f, lxp_u128 out[6])
{
    out[0] = f->agent_a->balance;
    out[1] = f->withdrawals_a->balance;
    out[2] = f->reserve_a->balance;
    out[3] = f->agent_b->balance;
    out[4] = f->withdrawals_b->balance;
    out[5] = f->reserve_b->balance;
}

static int same_balances(const lxp_u128 left[6], const lxp_u128 right[6])
{
    size_t i;
    for (i = 0U; i < 6U; ++i)
        if (!TEST_EQ(left[i].hi, right[i].hi) ||
            !TEST_EQ(left[i].lo, right[i].lo)) return 1;
    return 0;
}

static void request_context(withdraw_fixture *f, lx_account *agent,
                            lx_account *withdrawals, const lx_asset_record *asset,
                            uint64_t amount,
                            lxp_transfer_source_authority *authority,
                            lx_asset_transfer_request *transfer)
{
    (void)memset(authority, 0, sizeof(*authority));
    (void)memcpy(authority->authorized_from, agent->id, 32U);
    authority->debit_authority_kind = LXP_AUTH_OWNER;
    (void)memset(transfer, 0, sizeof(*transfer));
    transfer->from = agent;
    transfer->to = withdrawals;
    transfer->asset = asset;
    transfer->amount = (lxp_u128){ 0U, amount };
    transfer->context.assets = f->asset_states;
    transfer->context.asset_count = 2U;
    transfer->context.sequence_account = agent;
    transfer->context.debit_authority_kind = LXP_AUTH_OWNER;
    (void)memcpy(transfer->context.authorized_from, agent->id, 32U);
    transfer->context.source_authorities = authority;
    transfer->context.source_authority_count = 1U;
}

static void settlement_context(const withdraw_fixture *f,
                               const lx_account *withdrawals,
                               lxp_transfer_source_authority *authority,
                               lxp_transfer_context *context)
{
    (void)memset(authority, 0, sizeof(*authority));
    (void)memcpy(authority->authorized_from, withdrawals->id, 32U);
    authority->debit_authority_kind = LXP_AUTH_PROTOCOL_MODULE;
    authority->protocol_system_capability = true;
    (void)memset(context, 0, sizeof(*context));
    context->assets = f->asset_states;
    context->asset_count = 2U;
    context->protocol_system_capability = true;
    context->debit_authority_kind = LXP_AUTH_PROTOCOL_MODULE;
    context->source_authorities = authority;
    context->source_authority_count = 1U;
}

static int native_conserved_both(withdraw_fixture *f)
{
    lx_asset_native_supply_report report;
    if (!TEST_EQ(lx_asset_native_reconcile(&f->asset_a, &f->accounts_a, &report), LXP_OK) ||
        !TEST_EQ(report.bank_units.hi, 0U) || !TEST_EQ(report.bank_units.lo, 100U) ||
        !TEST_EQ(lx_asset_native_reconcile(&f->asset_b, &f->accounts_b, &report), LXP_OK) ||
        !TEST_EQ(report.bank_units.hi, 0U) || !TEST_EQ(report.bank_units.lo, 100U)) return 1;
    return 0;
}

static void historical_request(const lx_account *agent,
    const lx_asset_record *asset, uint64_t amount, uint8_t marker,
    lx_withdrawal_request *request)
{
    memset(request, 0, sizeof(*request));
    request->network_id = 7U;
    request->withdrawal_id[0] = marker;
    memcpy(request->account_id, agent->id, 32U);
    memcpy(request->asset_id, asset->asset_id, 32U);
    request->amount.lo = amount;
    request->payout_recipient[12] = 9U;
    request->checkpoint_id[0] = 3U;
}

static int test_retired_custody_execution(withdraw_fixture *f)
{
    lx_asset_transfer_request transfer;
    lx_withdrawal_request request, crossed;
    lx_finalized_checkpoint checkpoint = {0};
    lxp_transfer_source_authority authority;
    lxp_transfer_context context;
    uint8_t nullifier[32], before[96], after[96];
    lxp_u128 balances_before[6], balances_after[6];
    lx_withdrawal_store *saved = malloc(sizeof(*saved));
    if (!TEST_NE(saved, NULL)) return 1;
    checkpoint.checkpoint_id[0] = 3U;
    checkpoint.state_root[0] = 5U;
    checkpoint.finalized = true;
    historical_request(f->agent_a, &f->asset_a, 40U, 2U, &request);
    if (!TEST_EQ(lx_withdrawal_nullifier(&request, nullifier), LXP_OK)) return 1;
    /* Empty, outstanding, settled, full and corrupt legacy stores all refuse. */
    for (unsigned mode = 0U; mode < 5U; ++mode) {
        memset(&f->store, 0, sizeof(f->store));
        if (mode != 0U) {
            f->store.count = mode == 3U ? LX_DEPOSIT_NULLIFIER_CAPACITY :
                mode == 4U ? LX_DEPOSIT_NULLIFIER_CAPACITY + 1U : 1U;
            f->store.records[0].request = request;
            memcpy(f->store.records[0].nullifier, nullifier, 32U);
            f->store.records[0].settled = mode == 2U;
        }
        *saved = f->store;
        request_context(f, f->agent_a, f->withdrawals_a, &f->asset_a,
                        40U, &authority, &transfer);
        for (unsigned attempt = 0U; attempt < 5U; ++attempt) {
            crossed = request;
            if (attempt == 1U) memcpy(crossed.asset_id, f->asset_b.asset_id, 32U);
            if (attempt == 2U) { transfer.amount.lo = 101U; crossed.amount.lo = 101U; }
            if (attempt == 3U) { transfer.amount.lo = 0U; crossed.amount.lo = 0U; }
            if (attempt == 4U) crossed.withdrawal_id[0] ^= 1U;
            if (!TEST_EQ(fresh_ctx(f), 0) || !TEST_EQ(roots(f, before), 0)) return 1;
            balances(f, balances_before);
            memset(&f->receipt, 0x5a, sizeof(f->receipt));
            lxp_receipt receipt_before = f->receipt;
            if (!TEST_EQ(lx_asset_withdraw_request(&f->ctx, &transfer, &crossed,
                    &f->store, &f->receipt), LXP_ERR_UNKNOWN_ACTIVITY) ||
                !TEST_EQ(f->ctx.transfer_applied, false) ||
                !TEST_EQ(f->effects.count, 0U) ||
                !TEST_EQ(f->ctx.ledger_receipt_present, false) ||
                !TEST_EQ(memcmp(saved, &f->store, sizeof(*saved)), 0) ||
                !TEST_EQ(memcmp(&receipt_before, &f->receipt, sizeof(f->receipt)), 0) ||
                !TEST_EQ(roots(f, after), 0) ||
                !TEST_EQ(memcmp(before, after, sizeof(before)), 0)) return 1;
            balances(f, balances_after);
            if (!TEST_EQ(same_balances(balances_before, balances_after), 0) ||
                !TEST_EQ(native_conserved_both(f), 0)) return 1;
        }
        /* Both matching assets, seven crossed combinations and both replays. */
        lx_account *withdrawals[] = {f->withdrawals_a, f->withdrawals_b};
        lx_account *reserves[] = {f->reserve_a, f->reserve_b};
        const lx_asset_record *assets[] = {&f->asset_a, &f->asset_b};
        for (unsigned w = 0U; w < 2U; ++w)
            for (unsigned r = 0U; r < 2U; ++r)
                for (unsigned a = 0U; a < 2U; ++a)
                    for (unsigned replay = 0U; replay < 2U; ++replay) {
                        if (!TEST_EQ(fresh_ctx(f), 0) || !TEST_EQ(roots(f, before), 0)) return 1;
                        balances(f, balances_before);
                        settlement_context(f, withdrawals[w], &authority, &context);
                        if (!TEST_EQ(lx_asset_withdraw_settle(&f->ctx, withdrawals[w],
                                reserves[r], assets[a], &checkpoint, nullifier,
                                &f->store, context, &f->receipt), LXP_ERR_UNKNOWN_ACTIVITY) ||
                            !TEST_EQ(f->ctx.transfer_applied, false) ||
                            !TEST_EQ(f->effects.count, 0U) ||
                            !TEST_EQ(memcmp(saved, &f->store, sizeof(*saved)), 0) ||
                            !TEST_EQ(roots(f, after), 0) ||
                            !TEST_EQ(memcmp(before, after, sizeof(before)), 0)) return 1;
                        balances(f, balances_after);
                        if (!TEST_EQ(same_balances(balances_before, balances_after), 0) ||
                            !TEST_EQ(native_conserved_both(f), 0)) return 1;
                    }
    }
    if (!TEST_EQ(lx_asset_nullifier_seen(&f->store, nullifier), true)) return 1;
    free(saved);
    memset(&f->store, 0, sizeof(f->store));
    return 0;
}

static int test_historical_withdrawal_v2_reopen(withdraw_fixture *f,
    const void *parameters)
{
    lx_withdrawal_request request;
    lx_withdrawal_record decoded, saved;
    uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES];
    uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES];
    uint8_t nullifier[32], old_nullifier[32];
    uint8_t before[32], after[32];
    uint8_t account_ids[3][32];
    memcpy(account_ids[0], f->agent_a->id, 32U);
    memcpy(account_ids[1], f->withdrawals_a->id, 32U);
    memcpy(account_ids[2], f->reserve_a->id, 32U);
    historical_request(f->agent_a, &f->asset_a, 40U, 2U, &request);
    if (!TEST_EQ(lx_asset_historical_withdrawal_nullifier_v1(&request, nullifier), LXP_OK) ||
        !TEST_EQ(lx_withdrawal_nullifier(&request, old_nullifier), LXP_OK) ||
        !TEST_EQ(memcmp(nullifier, old_nullifier, 32U), 0) ||
        !TEST_EQ(lx_asset_historical_withdrawal_encode_v2(&request, key, value), LXP_OK) ||
        !TEST_EQ(lx_asset_historical_withdrawal_decode_v2(key, sizeof(key), value,
            sizeof(value), &decoded), LXP_OK) ||
        !TEST_EQ(memcmp(decoded.nullifier, nullifier, 32U), 0) ||
        !TEST_EQ(decoded.settled, false)) return 1;
    saved = decoded;
    for (size_t n = 0U; n < sizeof(value); ++n)
        if (!TEST_EQ(lx_asset_historical_withdrawal_decode_v2(key, sizeof(key),
                value, n, &decoded), LXP_ERR_NON_CANONICAL) ||
            !TEST_EQ(memcmp(&decoded, &saved, sizeof(saved)), 0)) return 1;
    for (size_t n = 0U; n < sizeof(key); ++n)
        if (!TEST_EQ(lx_withdrawal_state_decode(key, n, value, sizeof(value),
                &decoded), LXP_ERR_NON_CANONICAL) ||
            !TEST_EQ(memcmp(&decoded, &saved, sizeof(saved)), 0)) return 1;
    value[1] = 1U;
    if (!TEST_EQ(lx_withdrawal_state_decode(key, sizeof(key), value, sizeof(value),
            &decoded), LXP_ERR_NON_CANONICAL)) return 1;
    value[1] = 2U;
    key[11] ^= 1U;
    if (!TEST_EQ(lx_withdrawal_state_decode(key, sizeof(key), value, sizeof(value),
            &decoded), LXP_ERR_NON_CANONICAL)) return 1;
    key[11] ^= 1U;
    value[102] ^= 1U;
    if (!TEST_EQ(lx_withdrawal_state_decode(key, sizeof(key), value, sizeof(value),
            &decoded), LXP_ERR_NON_CANONICAL)) return 1;
    value[102] ^= 1U;
    if (!TEST_EQ(memcmp(&decoded, &saved, sizeof(saved)), 0) ||
        !TEST_EQ(fresh_ctx(f), 0)) return 1;
    f->ctx.global_sequence = f->state.next_sequence;
    if (!TEST_EQ(lxp_state_journal_open(&f->state, f->ctx.global_sequence, &f->journal), LXP_OK) ||
        !TEST_EQ(lxp_ctx_kv_put(&f->ctx, key, sizeof(key), value, sizeof(value)), LXP_OK) ||
        !TEST_EQ(lxp_state_journal_commit(&f->journal), LXP_OK) ||
        !TEST_EQ(lxp_module_ctx_commit(&f->ctx), LXP_OK) ||
        !TEST_EQ(asset_history_proof(&f->kernel, &request, 0), 0) ||
        !TEST_EQ(lxp_state_root(&f->kernel, before), LXP_OK) ||
        !TEST_EQ(asset_history_reopen(&f->kernel, &f->state, &f->journal,
            &f->accounts_a, parameters, f->state.next_sequence - 1U), 0) ||
        !TEST_EQ(asset_history_proof(&f->kernel, &request, 0), 0) ||
        !TEST_EQ(lxp_state_root(&f->kernel, after), LXP_OK) ||
        !TEST_EQ(memcmp(before, after, 32U), 0)) return 1;
    /* Recovery preserves native balances; archived withdrawals cannot settle. */
    size_t slots[3];
    for (size_t i = 0U; i < 3U; ++i)
        if (!TEST_EQ(lx_account_registry_index_lookup(&f->accounts_a,
                account_ids[i], &slots[i]), LXP_OK)) return 1;
    f->agent_a = &f->accounts_a.accounts[slots[0]];
    f->withdrawals_a = &f->accounts_a.accounts[slots[1]];
    f->reserve_a = &f->accounts_a.accounts[slots[2]];
    if (!TEST_EQ(native_conserved_both(f), 0) ||
        !TEST_EQ(f->agent_a->balance.lo, 100U) ||
        !TEST_EQ(f->withdrawals_a->balance.lo, 0U) ||
        !TEST_EQ(f->reserve_a->balance.lo, 0U) ||
        !TEST_EQ(test_retired_custody_execution(f), 0)) return 1;
    return 0;
}

int main(void)
{
    withdraw_fixture *f = &fixture;
    uint64_t parameters = 1U;
    memset(f, 0, sizeof(*f));
    f->asset_a.asset_id[0] = 1U;
    memcpy(f->asset_a.symbol, "A", 2U);
    f->asset_a.symbol_length = 1U;
    f->asset_a.name[0] = 'A';
    f->asset_a.name_length = 1U;
    f->asset_a.issuer_kind = 2U;
    f->asset_a.issuer_did32[0] = 1U;
    f->asset_a.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    f->asset_a.custody_reference[0] = 1U;
    f->asset_a.custody_reference_length = 1U;
    f->asset_a.total_units.lo = 100U;
    f->asset_b = f->asset_a;
    f->asset_b.asset_id[0] = 2U;
    memcpy(f->asset_b.symbol, "B", 2U);
    f->asset_b.custody_reference[0] = 2U;
    if (!TEST_EQ(lx_asset_registry_init(&f->assets, 0U), LXP_OK) ||
        !TEST_EQ(lx_asset_register(&f->assets, &f->asset_a, 0U, (lxp_u128){0U, 0U}), LXP_OK) ||
        !TEST_EQ(lx_asset_register(&f->assets, &f->asset_b, 1U, (lxp_u128){0U, 0U}), LXP_OK) ||
        !TEST_EQ(open_ledger(f, &f->asset_a, &f->accounts_a, "agent:did:key:a:main",
            &f->agent_a, &f->withdrawals_a, &f->reserve_a), 0) ||
        !TEST_EQ(open_ledger(f, &f->asset_b, &f->accounts_b, "agent:did:key:b:main",
            &f->agent_b, &f->withdrawals_b, &f->reserve_b), 0) ||
        !TEST_EQ(lx_asset_transfer_state(&f->asset_a, &f->asset_states[0]), LXP_OK) ||
        !TEST_EQ(lx_asset_transfer_state(&f->asset_b, &f->asset_states[1]), LXP_OK) ||
        !TEST_EQ(lxp_state_store_init(&f->state, 0U), LXP_OK) ||
        !TEST_EQ(lxp_state_store_bind_accounts(&f->state, &f->accounts_a), LXP_OK) ||
        !TEST_EQ(lxp_state_store_require_account_root(&f->state), LXP_OK) ||
        !TEST_EQ(lxp_kernel_create(&f->kernel, &f->state, &f->journal, &parameters, 0U), LXP_OK) ||
        !TEST_EQ(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()), LXP_OK) ||
        !TEST_EQ(lxp_kernel_set_capabilities(&f->kernel, NULL,
            lxp_kernel_canonical_ledger_apply), LXP_OK) ||
        !TEST_EQ(test_retired_custody_execution(f), 0) ||
        !TEST_EQ(test_historical_withdrawal_v2_reopen(f, &parameters), 0) ||
        !TEST_EQ(lxp_state_store_destroy(&f->state), LXP_OK)) return 1;
    return 0;
}
