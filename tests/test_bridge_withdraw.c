#include "layerx/lxp_bridge.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_receipt.h"
#include "layerx/lxp_state.h"

#include <stdint.h>
#include <string.h>

typedef struct bridge_fixture {
    lx_asset_registry *assets;
    lx_account_registry *accounts_a;
    lx_account_registry *accounts_b;
    lx_account *ledger[6];
    lxp_transfer_asset_state asset_states[2];
    lx_withdrawal_store *store;
    lxp_kernel *kernel;
    lxp_module_ctx *ctx;
    lxp_arena *arena;
    uint8_t *arena_bytes;
    size_t arena_capacity;
    lxp_receipt *receipt;
    uint64_t global_sequence;
} bridge_fixture;

static lxp_result apply_capability(lxp_kernel *kernel,
                                   const lxp_transfer_set *set,
                                   lxp_receipt *receipt)
{
    lxp_transfer_set_result result;
    lxp_transfer_context context = set->context;
    lxp_result status;
    (void)kernel;
    status = lxp_apply_transfer_set((lxp_transfer_leg *)set->legs,
                                    set->leg_count, &context, &result);
    if (status == LXP_OK)
        (void)memcpy(receipt->transfer_set_root, result.transfer_set_root, 32U);
    return status;
}

static int fresh_ctx(bridge_fixture *f)
{
    ++f->global_sequence;
    if (lxp_arena_init(f->arena, f->arena_bytes, f->arena_capacity) != LXP_OK ||
        lxp_module_ctx_init(f->ctx, f->kernel, LXP_MODULE_ASSET, 10U, 0U,
                            f->global_sequence, 1000U, f->arena, true) !=
            LXP_OK)
        return 1;
    return 0;
}

static int open_ledger(lx_asset_registry *assets, const lx_asset_record *asset,
                       lx_account_registry *accounts, const char *agent_name,
                       lx_account **agent, lx_account **withdrawals,
                       lx_account **reserve)
{
    if (lx_account_registry_init(accounts) != LXP_OK ||
        lx_asset_account_open(assets, accounts, asset->asset_id,
            (const uint8_t *)agent_name, strlen(agent_name), 1U,
            LX_ACCOUNT_OPEN_CREDIT, NULL, agent) != LXP_OK ||
        lx_asset_account_open(assets, accounts, asset->asset_id,
            (const uint8_t *)"system:paxeer-withdrawals", 25U, 1U,
            LX_ACCOUNT_OPEN_GENESIS, NULL, withdrawals) != LXP_OK ||
        lx_asset_account_open(assets, accounts, asset->asset_id,
            (const uint8_t *)"system:paxeer-reserve", 21U, 1U,
            LX_ACCOUNT_OPEN_GENESIS, NULL, reserve) != LXP_OK ||
        lxp_ledger_bootstrap_balance(*agent, asset->asset_id,
            (lxp_u128){0U, 100U}, 0U) != LXP_OK)
        return 1;
    return 0;
}

static int roots(const bridge_fixture *f, uint8_t out[96])
{
    if (lxp_state_root(f->kernel, out) != LXP_OK ||
        lx_asset_state_root(f->assets, f->accounts_a, out + 32) != LXP_OK ||
        lx_asset_state_root(f->assets, f->accounts_b, out + 64) != LXP_OK)
        return 1;
    return 0;
}

static int withdrawal_totals(const lx_withdrawal_store *store,
                             const uint8_t asset_id[32],
                             lxp_u128 *outstanding, lxp_u128 *settled)
{
    size_t i;
    *outstanding = (lxp_u128){0U, 0U};
    *settled = (lxp_u128){0U, 0U};
    for (i = 0U; i < store->count; ++i) {
        const lx_withdrawal_record *record = &store->records[i];
        lxp_u128 *bucket = record->settled ? settled : outstanding;
        if (memcmp(record->request.asset_id, asset_id, 32U) != 0) continue;
        if (lxp_u128_add(*bucket, record->request.amount, bucket) != LXP_OK)
            return 1;
    }
    return 0;
}

static int conserved(const bridge_fixture *f,
                     const lx_account_registry *accounts,
                     const lx_account *agent, const uint8_t asset_id[32],
                     uint64_t outstanding_expected, uint64_t settled_expected)
{
    lx_asset_custody_attestation attestation;
    lx_asset_reserve_report_record report;
    lxp_u128 outstanding;
    lxp_u128 settled;
    lxp_u128 total;
    lxp_u128 custody;
    if (withdrawal_totals(f->store, asset_id, &outstanding, &settled) != 0 ||
        outstanding.hi != 0U || outstanding.lo != outstanding_expected ||
        settled.hi != 0U || settled.lo != settled_expected)
        return 1;
    (void)memset(&attestation, 0, sizeof(attestation));
    (void)memcpy(attestation.asset_id, asset_id, 32U);
    attestation.custody_amount = (lxp_u128){0U, 100U};
    attestation.settled_out = settled;
    attestation.checkpoint_id[0] = 3U;
    attestation.state_root[0] = 5U;
    attestation.finalized = true;
    if (lx_asset_reserve_reconcile(accounts, &attestation, &report) != LXP_OK ||
        lx_asset_total_units(f->assets, accounts, asset_id, &total) != LXP_OK ||
        total.hi != 0U || total.lo != 100U ||
        lxp_u128_cmp(report.raw_total, total) != 0 ||
        lxp_u128_cmp(report.withdrawals, outstanding) != 0 ||
        lxp_u128_cmp(report.reserve, settled) != 0 ||
        lxp_u128_add(agent->balance, outstanding, &custody) != LXP_OK ||
        lxp_u128_add(custody, settled, &custody) != LXP_OK ||
        lxp_u128_cmp(custody, total) != 0)
        return 1;
    return 0;
}

static int conserved_both(const bridge_fixture *f,
                          const uint8_t asset_a[32], const uint8_t asset_b[32],
                          uint64_t outstanding_a, uint64_t settled_a,
                          uint64_t outstanding_b, uint64_t settled_b)
{
    if (conserved(f, f->accounts_a, f->ledger[0], asset_a,
                  outstanding_a, settled_a) != 0 ||
        conserved(f, f->accounts_b, f->ledger[3], asset_b,
                  outstanding_b, settled_b) != 0)
        return 1;
    return 0;
}

static void request_context(const bridge_fixture *f, lx_account *agent,
                            lx_account *withdrawals,
                            const lx_asset_record *asset, lxp_u128 amount,
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
    transfer->amount = amount;
    transfer->context.assets = f->asset_states;
    transfer->context.asset_count = 2U;
    transfer->context.sequence_account = agent;
    transfer->context.debit_authority_kind = LXP_AUTH_OWNER;
    (void)memcpy(transfer->context.authorized_from, agent->id, 32U);
    transfer->context.source_authorities = authority;
    transfer->context.source_authority_count = 1U;
}

int main(void)
{
    static lx_asset_registry assets;
    static lx_account_registry accounts;
    static lx_account_registry accounts_b;
    lx_asset_record asset;
    lx_asset_record asset_b;
    lx_account *agent;
    lx_account *withdrawals;
    lx_account *reserve;
    lx_account *agent_b;
    lx_account *withdrawals_b;
    lx_account *reserve_b;
    lx_asset_transfer_request transfer;
    lxp_transfer_source_authority authority;
    lx_withdrawal_request withdrawal;
    lx_withdrawal_request withdrawal_b;
    lx_withdrawal_store store;
    lxp_challenge_window_state challenged;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx module_ctx;
    lxp_receipt receipt;
    uint8_t module_arena_bytes[4096];
    lxp_arena module_arena;
    bridge_fixture fixture;
    uint64_t parameters = 1U;
    uint8_t leaf_hash[32];
    uint8_t checkpoint_id[32] = {3U};
    uint8_t nullifier[32];
    uint8_t nullifier_b[32];
    uint8_t before[96];
    uint8_t after[96];

    (void)memset(&asset, 0, sizeof(asset));
    asset.asset_id[0] = 1U;
    (void)memcpy(asset.symbol, "A", 2U);
    asset.symbol_length = 1U;
    asset.name[0] = (uint8_t)'A';
    asset.name_length = 1U;
    asset.issuer_kind = 2U;
    asset.issuer_did32[0] = 1U;
    asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    asset.custody_reference[0] = 1U;
    asset.custody_reference_length = 1U;
    asset_b = asset;
    asset_b.asset_id[0] = 2U;
    (void)memcpy(asset_b.symbol, "B", 2U);
    asset_b.custody_reference[0] = 2U;
    (void)memset(&fixture, 0, sizeof(fixture));
    if (lx_asset_registry_init(&assets, 0U) != LXP_OK ||
        lx_asset_register(&assets, &asset, 0U, (lxp_u128){0U, 0U}) != LXP_OK ||
        lx_asset_register(&assets, &asset_b, 1U, (lxp_u128){0U, 0U}) != LXP_OK ||
        open_ledger(&assets, &asset, &accounts, "agent:did:key:a:main",
                    &agent, &withdrawals, &reserve) != 0 ||
        open_ledger(&assets, &asset_b, &accounts_b, "agent:did:key:b:main",
                    &agent_b, &withdrawals_b, &reserve_b) != 0 ||
        lx_asset_transfer_state(&asset, &fixture.asset_states[0]) != LXP_OK ||
        lx_asset_transfer_state(&asset_b, &fixture.asset_states[1]) != LXP_OK ||
        lxp_state_store_init(&state, 0U) != LXP_OK ||
        lxp_state_store_bind_accounts(&state, &accounts) != LXP_OK ||
        lxp_state_store_require_account_root(&state) != LXP_OK ||
        lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(&kernel, lx_asset_module_iface()) != LXP_OK ||
        lxp_kernel_set_capabilities(&kernel, NULL, apply_capability) != LXP_OK)
        return 1;
    fixture.assets = &assets;
    fixture.accounts_a = &accounts;
    fixture.accounts_b = &accounts_b;
    fixture.ledger[0] = agent;
    fixture.ledger[1] = withdrawals;
    fixture.ledger[2] = reserve;
    fixture.ledger[3] = agent_b;
    fixture.ledger[4] = withdrawals_b;
    fixture.ledger[5] = reserve_b;
    fixture.store = &store;
    fixture.kernel = &kernel;
    fixture.ctx = &module_ctx;
    fixture.arena = &module_arena;
    fixture.arena_bytes = module_arena_bytes;
    fixture.arena_capacity = sizeof(module_arena_bytes);
    fixture.receipt = &receipt;
    (void)memset(&store, 0, sizeof(store));
    if (fresh_ctx(&fixture) != 0 ||
        conserved_both(&fixture, asset.asset_id, asset_b.asset_id,
                       0U, 0U, 0U, 0U) != 0)
        return 1;

    (void)memset(&withdrawal, 0, sizeof(withdrawal));
    withdrawal.network_id = 7U;
    withdrawal.withdrawal_id[0] = 2U;
    (void)memcpy(withdrawal.account_id, agent->id, 32U);
    (void)memcpy(withdrawal.asset_id, asset.asset_id, 32U);
    withdrawal.amount = (lxp_u128){0U, 40U};
    withdrawal.payout_recipient[31] = 0xaaU;
    if (lxp_withdrawal_leaf(&withdrawal, leaf_hash) != LXP_OK) return 1;

    (void)memcpy(withdrawal.checkpoint_id, checkpoint_id, 32U);
    if (lxp_withdrawal_nullifier(&withdrawal, nullifier) != LXP_OK) return 1;

    request_context(&fixture, agent, withdrawals, &asset, withdrawal.amount,
                    &authority, &transfer);
    if (roots(&fixture, before) != 0 ||
        lxp_bridge_withdraw_request(
            &module_ctx, &transfer, &withdrawal, &store, &receipt) != LXP_OK ||
        agent->balance.lo != 60U || withdrawals->balance.lo != 40U ||
        store.count != 1U ||
        memcmp(store.records[0].nullifier, nullifier, 32U) != 0 ||
        roots(&fixture, after) != 0 || memcmp(before, after, 32U) == 0 ||
        memcmp(before + 32, after + 32, 32U) == 0 ||
        memcmp(before + 64, after + 64, 32U) != 0 ||
        conserved_both(&fixture, asset.asset_id, asset_b.asset_id,
                       40U, 0U, 0U, 0U) != 0)
        return 1;

    (void)memset(&withdrawal_b, 0, sizeof(withdrawal_b));
    withdrawal_b.network_id = 7U;
    withdrawal_b.withdrawal_id[0] = 3U;
    (void)memcpy(withdrawal_b.account_id, agent_b->id, 32U);
    (void)memcpy(withdrawal_b.asset_id, asset_b.asset_id, 32U);
    withdrawal_b.amount = (lxp_u128){0U, 30U};
    withdrawal_b.payout_recipient[31] = 0xbbU;
    (void)memcpy(withdrawal_b.checkpoint_id, checkpoint_id, 32U);
    {
        lx_asset_transfer_request transfer_b;
        lxp_transfer_source_authority authority_b;
        request_context(&fixture, agent_b, withdrawals_b, &asset_b,
                        withdrawal_b.amount, &authority_b, &transfer_b);
        if (lxp_withdrawal_nullifier(&withdrawal_b, nullifier_b) != LXP_OK ||
            fresh_ctx(&fixture) != 0 || roots(&fixture, before) != 0 ||
            lxp_bridge_withdraw_request(
                &module_ctx, &transfer_b, &withdrawal_b, &store, &receipt) !=
                LXP_OK ||
            agent_b->balance.lo != 70U || withdrawals_b->balance.lo != 30U ||
            agent->balance.lo != 60U || withdrawals->balance.lo != 40U ||
            store.count != 2U ||
            memcmp(store.records[1].nullifier, nullifier_b, 32U) != 0 ||
            roots(&fixture, after) != 0 || memcmp(before, after, 64U) != 0 ||
            memcmp(before + 64, after + 64, 32U) == 0 ||
            conserved_both(&fixture, asset.asset_id, asset_b.asset_id,
                           40U, 0U, 30U, 0U) != 0)
            return 1;
    }

    (void)memset(&challenged, 0, sizeof(challenged));
    challenged.checkpoint_id[0] = 0x44U;
    challenged.opened_at_ms = 100U;
    challenged.closes_at_ms = 200U;
    if (lxp_paxeer_challenge_window(
            &challenged, 120U, LXP_CHALLENGE_PENDING, 3U) !=
            LXP_ERR_CHALLENGE_WINDOW_OPEN ||
        lxp_paxeer_challenge_window(
            &challenged, 220U, LXP_CHALLENGE_SUCCEEDED, 3U) !=
            LXP_ERR_WITHDRAWAL_CANCELLED ||
        !challenged.payouts_cancelled ||
        challenged.slashed_attester_count != 3U ||
        lxp_paxeer_challenge_window(
            &challenged, 230U, LXP_CHALLENGE_NONE, 3U) !=
            LXP_ERR_WITHDRAWAL_CANCELLED)
        return 1;
    return lxp_state_store_destroy(&state) == LXP_OK ? 0 : 1;
}
