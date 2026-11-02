#include "../ledger/test_values.h"

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

static int withdrawal_totals(const lx_withdrawal_store *store,
                             const uint8_t asset_id[32],
                             lxp_u128 *outstanding, lxp_u128 *settled)
{
    size_t i;
    *outstanding = (lxp_u128){ 0U, 0U };
    *settled = (lxp_u128){ 0U, 0U };
    for (i = 0U; i < store->count; ++i) {
        const lx_withdrawal_record *record = &store->records[i];
        lxp_u128 *bucket = record->settled ? settled : outstanding;
        if (memcmp(record->request.asset_id, asset_id, 32U) != 0) continue;
        if (!TEST_EQ(lxp_u128_add(*bucket, record->request.amount, bucket), LXP_OK))
            return 1;
    }
    return 0;
}

static int conserved(withdraw_fixture *f, const lx_account_registry *accounts,
                     const lx_account *agent, const uint8_t asset_id[32],
                     uint64_t outstanding_expected, uint64_t settled_expected)
{
    lx_asset_custody_attestation attestation;
    lx_asset_reserve_report_record report;
    lxp_u128 outstanding;
    lxp_u128 settled;
    lxp_u128 total;
    lxp_u128 custody;
    if (!TEST_EQ(withdrawal_totals(&f->store, asset_id, &outstanding, &settled), 0) ||
        !TEST_EQ(outstanding.hi, 0U) ||
        !TEST_EQ(outstanding.lo, outstanding_expected) ||
        !TEST_EQ(settled.hi, 0U) ||
        !TEST_EQ(settled.lo, settled_expected))
        return 1;
    (void)memset(&attestation, 0, sizeof(attestation));
    (void)memcpy(attestation.asset_id, asset_id, 32U);
    attestation.custody_amount = (lxp_u128){ 0U, 100U };
    attestation.settled_out = settled;
    attestation.checkpoint_id[0] = 3U;
    attestation.state_root[0] = 5U;
    attestation.finalized = true;
    if (!TEST_EQ(lx_asset_reserve_reconcile(accounts, &attestation, &report), LXP_OK) ||
        !TEST_EQ(lx_asset_total_units(&f->assets, accounts, asset_id, &total), LXP_OK) ||
        !TEST_EQ(total.hi, 0U) ||
        !TEST_EQ(total.lo, 100U) ||
        !TEST_EQ(lxp_u128_cmp(report.raw_total, total), 0) ||
        !TEST_EQ(lxp_u128_cmp(report.withdrawals, outstanding), 0) ||
        !TEST_EQ(lxp_u128_cmp(report.reserve, settled), 0) ||
        !TEST_EQ(lxp_u128_add(agent->balance, outstanding, &custody), LXP_OK) ||
        !TEST_EQ(lxp_u128_add(custody, settled, &custody), LXP_OK) ||
        !TEST_EQ(lxp_u128_cmp(custody, total), 0))
        return 1;
    return 0;
}

static int conserved_both(withdraw_fixture *f, uint64_t outstanding_a,
                          uint64_t settled_a, uint64_t outstanding_b,
                          uint64_t settled_b)
{
    if (!TEST_EQ(conserved(f, &f->accounts_a, f->agent_a, f->asset_a.asset_id,
                  outstanding_a, settled_a), 0) ||
        !TEST_EQ(conserved(f, &f->accounts_b, f->agent_b, f->asset_b.asset_id,
                  outstanding_b, settled_b), 0))
        return 1;
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

static int refused_settle(withdraw_fixture *f, lx_account *withdrawals,
                          lx_account *reserve, const lx_asset_record *asset,
                          const lx_finalized_checkpoint *checkpoint,
                          const uint8_t nullifier[32], size_t record_index,
                          uint64_t outstanding_a, uint64_t settled_a,
                          uint64_t outstanding_b, uint64_t settled_b)
{
    uint8_t before[96];
    uint8_t after[96];
    lxp_u128 balances_before[6];
    lxp_u128 balances_after[6];
    lxp_transfer_source_authority authority;
    lxp_transfer_context context;
    if (!TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(roots(f, before), 0)) return 1;
    balances(f, balances_before);
    settlement_context(f, withdrawals, &authority, &context);
    if (!TEST_EQ(lx_asset_withdraw_settle(&f->ctx, withdrawals, reserve, asset,
                                 checkpoint, nullifier, &f->store, context,
                                 &f->receipt), LXP_ERR_WITHDRAWAL_ASSET_MISMATCH) ||
        !TEST_EQ(!!(f->ctx.transfer_applied), 0) ||
        !TEST_EQ(!!(f->store.records[record_index].settled), 0) ||
        !TEST_EQ(memcmp(f->store.records[record_index].nullifier, nullifier, 32U), 0) ||
        !TEST_EQ(lx_asset_nullifier_seen(&f->store, nullifier), 1) ||
        !TEST_EQ(roots(f, after), 0) ||
        !TEST_EQ(memcmp(before, after, sizeof(before)), 0))
        return 1;
    balances(f, balances_after);
    if (!TEST_EQ(same_balances(balances_before, balances_after), 0) ||
        !TEST_EQ(conserved_both(f, outstanding_a, settled_a, outstanding_b, settled_b), 0))
        return 1;
    return 0;
}

static int matched_settle(withdraw_fixture *f, lx_account *withdrawals,
                          lx_account *reserve, const lx_asset_record *asset,
                          const lx_finalized_checkpoint *checkpoint,
                          const uint8_t nullifier[32], size_t record_index,
                          uint64_t amount, uint64_t outstanding_a,
                          uint64_t settled_a, uint64_t outstanding_b,
                          uint64_t settled_b)
{
    uint8_t before[96];
    uint8_t after[96];
    lxp_transfer_source_authority authority;
    lxp_transfer_context context;
    if (!TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(roots(f, before), 0)) return 1;
    settlement_context(f, withdrawals, &authority, &context);
    if (!TEST_EQ(lx_asset_withdraw_settle(&f->ctx, withdrawals, reserve, asset,
                                 checkpoint, nullifier, &f->store, context,
                                 &f->receipt), LXP_OK) ||
        !TEST_EQ(f->ctx.transfer_applied, 1) ||
        !TEST_EQ(f->store.records[record_index].settled, 1) ||
        !TEST_EQ(withdrawals->balance.hi, 0U) ||
        !TEST_EQ(withdrawals->balance.lo, 0U) ||
        !TEST_EQ(reserve->balance.hi, 0U) ||
        !TEST_EQ(reserve->balance.lo, amount) ||
        !TEST_EQ(roots(f, after), 0) ||
        !TEST_NE(memcmp(before, after, sizeof(before)), 0) ||
        !TEST_EQ(conserved_both(f, outstanding_a, settled_a, outstanding_b, settled_b), 0))
        return 1;
    if (!TEST_EQ(fresh_ctx(f), 0)) return 1;
    settlement_context(f, withdrawals, &authority, &context);
    if (!TEST_EQ(lx_asset_withdraw_settle(&f->ctx, withdrawals, reserve, asset,
                                 checkpoint, nullifier, &f->store, context,
                                 &f->receipt), LXP_ERR_WITHDRAWAL_ALREADY_SETTLED) ||
        !TEST_EQ(!!(f->ctx.transfer_applied), 0) ||
        !TEST_EQ(reserve->balance.lo, amount) ||
        !TEST_EQ(conserved_both(f, outstanding_a, settled_a, outstanding_b, settled_b), 0))
        return 1;
    return 0;
}

int main(void)
{
    withdraw_fixture *f = &fixture;
    lx_asset_transfer_request transfer;
    lxp_transfer_source_authority authority;
    lx_withdrawal_request withdrawal_a;
    lx_withdrawal_request withdrawal_b;
    lx_withdrawal_request crossed;
    lx_finalized_checkpoint checkpoint;
    uint8_t nullifier_a[32];
    uint8_t nullifier_b[32];
    uint8_t before[96];
    uint8_t after[96];
    uint64_t parameters = 1U;
    lxp_u128 total;

    (void)memset(f, 0, sizeof(*f));
    f->asset_a.asset_id[0] = 1U;
    (void)memcpy(f->asset_a.symbol, "A", 2U); f->asset_a.symbol_length = 1U;
    f->asset_a.name[0] = (uint8_t)'A'; f->asset_a.name_length = 1U;
    f->asset_a.issuer_kind = 2U; f->asset_a.issuer_did32[0] = 1U;
    f->asset_a.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    f->asset_a.custody_reference[0] = 1U; f->asset_a.custody_reference_length = 1U;
    f->asset_b = f->asset_a;
    f->asset_b.asset_id[0] = 2U;
    (void)memcpy(f->asset_b.symbol, "B", 2U);
    f->asset_b.custody_reference[0] = 2U;
    if (!TEST_EQ(lx_asset_registry_init(&f->assets, 0U), LXP_OK) ||
        !TEST_EQ(lx_asset_register(&f->assets, &f->asset_a, 0U, (lxp_u128){ 0U, 0U }), LXP_OK) ||
        !TEST_EQ(lx_asset_register(&f->assets, &f->asset_b, 1U, (lxp_u128){ 0U, 0U }), LXP_OK) ||
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
        !TEST_EQ(lxp_kernel_set_capabilities(&f->kernel, NULL, lxp_kernel_canonical_ledger_apply), LXP_OK) ||
        !TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(roots(f, before), 0) ||
        !TEST_EQ(conserved_both(f, 0U, 0U, 0U, 0U), 0)) return 1;

    request_context(f, f->agent_a, f->withdrawals_a, &f->asset_a, 40U,
                    &authority, &transfer);
    (void)memset(&withdrawal_a, 0, sizeof(withdrawal_a));
    withdrawal_a.network_id = 7U; withdrawal_a.withdrawal_id[0] = 2U;
    (void)memcpy(withdrawal_a.account_id, f->agent_a->id, 32U);
    (void)memcpy(withdrawal_a.asset_id, f->asset_a.asset_id, 32U);
    withdrawal_a.payout_recipient[12] = 9U;
    withdrawal_a.amount = transfer.amount; withdrawal_a.checkpoint_id[0] = 3U;
    if (!TEST_EQ(lx_withdrawal_nullifier(&withdrawal_a, nullifier_a), LXP_OK) ||
        !TEST_EQ(lx_asset_withdraw_request(&f->ctx, &transfer, &withdrawal_a, &f->store,
                                  &f->receipt), LXP_OK) ||
        !TEST_EQ(f->agent_a->balance.lo, 60U) ||
        !TEST_EQ(f->withdrawals_a->balance.lo, 40U) ||
        !TEST_EQ(f->reserve_a->balance.lo, 0U) ||
        !TEST_EQ(f->store.count, 1U) ||
        !TEST_EQ(lx_asset_nullifier_seen(&f->store, nullifier_a), 1) ||
        !TEST_EQ(lx_asset_total_units(&f->assets, &f->accounts_a, f->asset_a.asset_id,
                             &total), LXP_OK) ||
        !TEST_EQ(total.lo, 100U) ||
        !TEST_EQ(roots(f, after), 0) ||
        !TEST_NE(memcmp(before, after, 32U), 0) ||
        !TEST_NE(memcmp(before + 32, after + 32, 32U), 0) ||
        !TEST_EQ(memcmp(before + 64, after + 64, 32U), 0) ||
        !TEST_EQ(conserved_both(f, 40U, 0U, 0U, 0U), 0)) return 1;

    {
        uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES];
        uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES];
        lx_withdrawal_record decoded;
        lxp_state_witness *proof = malloc(sizeof(*proof));
        uint8_t committed[32];
        if (!TEST_EQ(!!(proof == NULL), 0) ||
        !TEST_EQ(lx_withdrawal_state_encode(&withdrawal_a, key, value), LXP_OK) ||
        !TEST_EQ(lx_withdrawal_state_decode(key, sizeof(key), value, sizeof(value), &decoded), LXP_OK) ||
        !TEST_EQ(memcmp(decoded.nullifier, nullifier_a, 32U), 0) ||
        !TEST_EQ(!!(decoded.settled), 0) ||
        !TEST_EQ(lxp_module_ctx_commit(&f->ctx), LXP_OK) ||
        !TEST_EQ(lxp_state_root(&f->kernel, committed), LXP_OK) ||
        !TEST_EQ(lxp_state_proof_build(&f->kernel, LXP_MODULE_ASSET,
                (lxp_byte_span){key, sizeof(key)}, proof), LXP_OK) ||
        !TEST_EQ(lxp_state_proof_verify(proof, committed), LXP_OK) ||
        !TEST_EQ(memcmp(proof->value, value, sizeof(value)), 0)) return 1;
        for (size_t n = 0U; n < sizeof(value); ++n)
            if (!TEST_NE(lx_withdrawal_state_decode(key, sizeof(key), value, n, &decoded), LXP_OK)) return 1;
        value[1] = 1U;
        if (!TEST_NE(lx_withdrawal_state_decode(key, sizeof(key), value, sizeof(value), &decoded), LXP_OK)) return 1;
        value[1] = 2U;
        key[11] ^= 1U;
        if (!TEST_NE(lx_withdrawal_state_decode(key, sizeof(key), value, sizeof(value), &decoded), LXP_OK)) return 1;
        key[11] ^= 1U;
        value[102] ^= 1U;
        if (!TEST_NE(lx_withdrawal_state_decode(key, sizeof(key), value, sizeof(value), &decoded), LXP_OK)) return 1;
        free(proof);
    }

    crossed = withdrawal_a;
    crossed.withdrawal_id[0] = 9U;
    (void)memcpy(crossed.asset_id, f->asset_b.asset_id, 32U);
    if (!TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(roots(f, before), 0) ||
        !TEST_EQ(lx_asset_withdraw_request(&f->ctx, &transfer, &crossed, &f->store,
                                  &f->receipt), LXP_ERR_GRANT_SCOPE_VIOLATION) ||
        !TEST_EQ(!!(f->ctx.transfer_applied), 0) ||
        !TEST_EQ(f->store.count, 1U) ||
        !TEST_EQ(f->agent_a->balance.lo, 60U) ||
        !TEST_EQ(f->withdrawals_a->balance.lo, 40U) ||
        !TEST_EQ(roots(f, after), 0) ||
        !TEST_EQ(memcmp(before, after, sizeof(before)), 0) ||
        !TEST_EQ(conserved_both(f, 40U, 0U, 0U, 0U), 0)) return 1;

    if (!TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(lx_asset_withdraw_request(&f->ctx, &transfer, &withdrawal_a, &f->store,
                                  &f->receipt), LXP_ERR_WITHDRAWAL_ALREADY_SETTLED) ||
        !TEST_EQ(f->agent_a->balance.lo, 60U))
        return 1;
    withdrawal_a.withdrawal_id[0] = 4U;
    transfer.amount = (lxp_u128){ 0U, 70U };
    withdrawal_a.payout_recipient[12] = 9U;
    withdrawal_a.amount = transfer.amount;
    transfer.context.actor_sequence = 1U;
    if (!TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(lx_asset_withdraw_request(&f->ctx, &transfer, &withdrawal_a, &f->store,
                                  &f->receipt), LXP_ERR_INSUFFICIENT_BALANCE) ||
        !TEST_EQ(f->agent_a->balance.lo, 60U) ||
        !TEST_EQ(f->withdrawals_a->balance.lo, 40U) ||
        !TEST_EQ(f->store.count, 1U) ||
        !TEST_EQ(conserved_both(f, 40U, 0U, 0U, 0U), 0))
        return 1;

    {
        lx_asset_transfer_request transfer_b;
        lxp_transfer_source_authority authority_b;
        request_context(f, f->agent_b, f->withdrawals_b, &f->asset_b, 30U,
                        &authority_b, &transfer_b);
        (void)memset(&withdrawal_b, 0, sizeof(withdrawal_b));
        withdrawal_b.network_id = 7U; withdrawal_b.withdrawal_id[0] = 5U;
        (void)memcpy(withdrawal_b.account_id, f->agent_b->id, 32U);
        (void)memcpy(withdrawal_b.asset_id, f->asset_b.asset_id, 32U);
        withdrawal_b.payout_recipient[12] = 10U;
        withdrawal_b.amount = transfer_b.amount;
        withdrawal_b.checkpoint_id[0] = 3U;
        if (!TEST_EQ(lx_withdrawal_nullifier(&withdrawal_b, nullifier_b), LXP_OK) ||
        !TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(roots(f, before), 0) ||
        !TEST_EQ(lx_asset_withdraw_request(&f->ctx, &transfer_b, &withdrawal_b,
                                      &f->store, &f->receipt), LXP_OK) ||
        !TEST_EQ(f->agent_b->balance.lo, 70U) ||
        !TEST_EQ(f->withdrawals_b->balance.lo, 30U) ||
        !TEST_EQ(f->reserve_b->balance.lo, 0U) ||
        !TEST_EQ(f->store.count, 2U) ||
        !TEST_EQ(f->agent_a->balance.lo, 60U) ||
        !TEST_EQ(f->withdrawals_a->balance.lo, 40U) ||
        !TEST_EQ(lx_asset_nullifier_seen(&f->store, nullifier_b), 1) ||
        !TEST_EQ(memcmp(f->store.records[1].nullifier, nullifier_b, 32U), 0) ||
        !TEST_EQ(roots(f, after), 0) ||
        !TEST_EQ(memcmp(before, after, 32U), 0) ||
        !TEST_EQ(memcmp(before + 32, after + 32, 32U), 0) ||
        !TEST_NE(memcmp(before + 64, after + 64, 32U), 0) ||
        !TEST_EQ(conserved_both(f, 40U, 0U, 30U, 0U), 0)) return 1;
    }

    (void)memset(&checkpoint, 0, sizeof(checkpoint));
    checkpoint.checkpoint_id[0] = 3U; checkpoint.state_root[0] = 5U;
    checkpoint.finalized = true;
    if (!TEST_EQ(refused_settle(f, f->withdrawals_b, f->reserve_b, &f->asset_b,
                       &checkpoint, nullifier_a, 0U, 40U, 0U, 30U, 0U), 0) ||
        !TEST_EQ(refused_settle(f, f->withdrawals_a, f->reserve_a, &f->asset_b,
                       &checkpoint, nullifier_a, 0U, 40U, 0U, 30U, 0U), 0) ||
        !TEST_EQ(refused_settle(f, f->withdrawals_b, f->reserve_b, &f->asset_a,
                       &checkpoint, nullifier_a, 0U, 40U, 0U, 30U, 0U), 0) ||
        !TEST_EQ(refused_settle(f, f->withdrawals_b, f->reserve_a, &f->asset_a,
                       &checkpoint, nullifier_a, 0U, 40U, 0U, 30U, 0U), 0) ||
        !TEST_EQ(refused_settle(f, f->withdrawals_a, f->reserve_b, &f->asset_a,
                       &checkpoint, nullifier_a, 0U, 40U, 0U, 30U, 0U), 0) ||
        !TEST_EQ(refused_settle(f, f->withdrawals_a, f->reserve_a, &f->asset_a,
                       &checkpoint, nullifier_b, 1U, 40U, 0U, 30U, 0U), 0) ||
        !TEST_EQ(refused_settle(f, f->withdrawals_a, f->reserve_a, &f->asset_b,
                       &checkpoint, nullifier_b, 1U, 40U, 0U, 30U, 0U), 0))
        return 1;

    if (!TEST_EQ(matched_settle(f, f->withdrawals_a, f->reserve_a, &f->asset_a,
                       &checkpoint, nullifier_a, 0U, 40U, 0U, 40U, 30U, 0U), 0) ||
        !TEST_EQ(f->withdrawals_b->balance.lo, 30U) ||
        !TEST_EQ(f->reserve_b->balance.lo, 0U) ||
        !TEST_EQ(refused_settle(f, f->withdrawals_a, f->reserve_a, &f->asset_a,
                       &checkpoint, nullifier_b, 1U, 0U, 40U, 30U, 0U), 0) ||
        !TEST_EQ(refused_settle(f, f->withdrawals_b, f->reserve_b, &f->asset_a,
                       &checkpoint, nullifier_b, 1U, 0U, 40U, 30U, 0U), 0) ||
        !TEST_EQ(matched_settle(f, f->withdrawals_b, f->reserve_b, &f->asset_b,
                       &checkpoint, nullifier_b, 1U, 30U, 0U, 40U, 0U, 30U), 0) ||
        !TEST_EQ(f->reserve_a->balance.lo, 40U) ||
        !TEST_EQ(f->withdrawals_a->balance.lo, 0U) ||
        !TEST_EQ(lx_asset_total_units(&f->assets, &f->accounts_a, f->asset_a.asset_id,
                             &total), LXP_OK) ||
        !TEST_EQ(total.lo, 100U) ||
        !TEST_EQ(lx_asset_total_units(&f->assets, &f->accounts_b, f->asset_b.asset_id,
                             &total), LXP_OK) ||
        !TEST_EQ(total.lo, 100U))
        return 1;

    f->store.count = LX_DEPOSIT_NULLIFIER_CAPACITY + 1U;
    if (!TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(lx_asset_nullifier_seen(&f->store, nullifier_a), 1) ||
        !TEST_EQ(lx_asset_withdraw_request(&f->ctx, &transfer, &withdrawal_a, &f->store,
                                  &f->receipt), LXP_ERR_NON_CANONICAL))
        return 1;
    {
        lxp_transfer_context settlement = { 0 };
        if (!TEST_EQ(fresh_ctx(f), 0) ||
        !TEST_EQ(lx_asset_withdraw_settle(&f->ctx, f->withdrawals_a, f->reserve_a,
                                     &f->asset_a, &checkpoint, nullifier_a,
                                     &f->store, settlement, &f->receipt), LXP_ERR_NON_CANONICAL))
            return 1;
    }
    if (!TEST_EQ(lxp_state_store_destroy(&f->state), LXP_OK)) return 1;
    return 0;
}
