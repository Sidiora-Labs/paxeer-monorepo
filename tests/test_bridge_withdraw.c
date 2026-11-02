#include "layerx/lxp_bridge.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_receipt.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_state.h"

#include <stdint.h>
#include <stdio.h>
#include <string.h>

static int bridge_failure(int line)
{
    (void)fprintf(stderr, "%s:%d: bridge check failed\n", __FILE__, line);
    return 1;
}

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
    lxp_effect_buffer effects;
    uint64_t global_sequence;
} bridge_fixture;

static int fresh_ctx(bridge_fixture *f)
{
    ++f->global_sequence;
    if (lxp_arena_init(f->arena, f->arena_bytes, f->arena_capacity) != LXP_OK ||
        lxp_module_ctx_init(f->ctx, f->kernel, LXP_MODULE_ASSET, 10U, 0U,
                            f->global_sequence, 1000U, f->arena, true) !=
            LXP_OK ||
        lxp_effect_buffer_init(&f->effects) != LXP_OK ||
        lxp_module_ctx_bind_effects(f->ctx, &f->effects) != LXP_OK)
        return bridge_failure(__LINE__);
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
        return bridge_failure(__LINE__);
    return 0;
}

static int roots(const bridge_fixture *f, uint8_t out[96])
{
    if (lxp_state_root(f->kernel, out) != LXP_OK ||
        lx_asset_state_root(f->assets, f->accounts_a, out + 32) != LXP_OK ||
        lx_asset_state_root(f->assets, f->accounts_b, out + 64) != LXP_OK)
        return bridge_failure(__LINE__);
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
            return bridge_failure(__LINE__);
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
        return bridge_failure(__LINE__);
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
        return bridge_failure(__LINE__);
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
        return bridge_failure(__LINE__);
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

/* Reopen only from the real producer's snapshot, discarding its cache. */
static int bridge_withdraw_reopen(bridge_fixture *f)
{
    static uint8_t snapshot_bytes[1048576];
    static const char *const names[] = {
        "agent:did:key:a:main", "system:paxeer-withdrawals",
        "system:paxeer-reserve"
    };
    lxp_state_store *state = f->kernel->state;
    lxp_state_journal *journal = f->kernel->journal;
    const void *parameters = f->kernel->parameter_set;
    lxp_snapshot_manifest_record manifest;
    lxp_byte_span snapshot;
    lxp_arena arena;
    lx_withdrawal_record produced = f->store->records[0];
    lx_withdrawal_record observed;
    lx_asset_record *asset;
    lx_asset_transfer_request transfer;
    lxp_transfer_source_authority authority;
    uint8_t ids[3][32];
    uint8_t before[96];
    uint8_t after[96];
    uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES];
    uint8_t expected[LX_WITHDRAWAL_STATE_VALUE_BYTES];
    uint8_t leaf_before[32];
    uint8_t leaf_after[32];
    const uint8_t *stored;
    size_t stored_length;
    size_t i;
    uint64_t sequence = state->next_sequence;

    for (i = 0U; i < 3U; ++i)
        (void)memcpy(ids[i], f->ledger[i]->id, 32U);
    if (sequence != f->ctx->global_sequence ||
        lxp_module_ctx_commit(f->ctx) != LXP_OK ||
        lxp_state_journal_open(state, sequence, journal) != LXP_OK ||
        lxp_state_journal_commit(journal) != LXP_OK ||
        roots(f, before) != 0 ||
        lx_withdrawal_state_encode(&produced.request, key, expected) != LXP_OK ||
        lxp_withdrawal_leaf(&produced.request, leaf_before) != LXP_OK ||
        lxp_arena_init(&arena, snapshot_bytes, sizeof(snapshot_bytes)) != LXP_OK ||
        lxp_snapshot_write(f->kernel, sequence, &arena, &snapshot) != LXP_OK ||
        lxp_snapshot_manifest(snapshot.bytes, snapshot.length, sequence,
                              before, before, &manifest) != LXP_OK ||
        lxp_state_store_destroy(state) != LXP_OK)
        return bridge_failure(__LINE__);
    lx_account_registry_release(f->accounts_a);
    (void)memset(f->store, 0, sizeof(*f->store));
    if (lx_account_registry_init(f->accounts_a) != LXP_OK ||
        lxp_state_store_init(state, 0U) != LXP_OK ||
        lxp_state_store_bind_accounts(state, f->accounts_a) != LXP_OK ||
        lxp_state_store_require_account_root(state) != LXP_OK ||
        lxp_kernel_create(f->kernel, state, journal, parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(f->kernel, lx_asset_module_iface()) != LXP_OK ||
        lxp_kernel_set_capabilities(f->kernel, NULL,
                                     lxp_kernel_canonical_ledger_apply) != LXP_OK ||
        lxp_snapshot_load(snapshot.bytes, snapshot.length, &manifest,
                           f->kernel) != LXP_OK ||
        lxp_snapshot_verify_root(f->kernel, &manifest) != LXP_OK ||
        state->next_sequence != sequence + 1U)
        return bridge_failure(__LINE__);
    for (i = 0U; i < 3U; ++i)
        if (lx_account_lookup(f->accounts_a, (const uint8_t *)names[i],
                              strlen(names[i]), ids[i], &f->ledger[i]) != LXP_OK)
            return bridge_failure(__LINE__);
    if (fresh_ctx(f) != 0 || roots(f, after) != 0 ||
        memcmp(before, after, sizeof(before)) != 0 ||
        f->ledger[0]->balance.hi != 0U || f->ledger[0]->balance.lo != 60U ||
        f->ledger[1]->balance.hi != 0U || f->ledger[1]->balance.lo != 40U ||
        !lxp_u128_is_zero(f->ledger[2]->balance) ||
        lxp_ctx_kv_get(f->ctx, key, sizeof(key), &stored, &stored_length) != LXP_OK ||
        stored_length != sizeof(expected) ||
        memcmp(stored, expected, sizeof(expected)) != 0 ||
        lx_withdrawal_state_decode(key, sizeof(key), stored, stored_length,
                                    &observed) != LXP_OK ||
        observed.settled ||
        memcmp(observed.nullifier, produced.nullifier, 32U) != 0 ||
        lxp_withdrawal_leaf(&observed.request, leaf_after) != LXP_OK ||
        memcmp(leaf_before, leaf_after, 32U) != 0 ||
        lx_asset_lookup(f->assets, observed.request.asset_id, &asset) != LXP_OK)
        return bridge_failure(__LINE__);
    request_context(f, f->ledger[0], f->ledger[1], asset,
                    observed.request.amount, &authority, &transfer);
    /* An empty process-local cache must still refuse the persisted duplicate. */
    if (f->store->count != 0U ||
        lxp_bridge_withdraw_request(f->ctx, &transfer, &observed.request,
                                     f->store, f->receipt) !=
            LXP_ERR_WITHDRAWAL_ALREADY_SETTLED ||
        f->store->count != 0U || f->ctx->staged_count != 0U ||
        f->ctx->transfer_applied ||
        roots(f, after) != 0 || memcmp(before, after, sizeof(before)) != 0 ||
        state->next_sequence != sequence + 1U)
        return bridge_failure(__LINE__);
    f->store->records[0] = observed;
    f->store->count = 1U;
    return 0;
}

static int bridge_withdraw_regression(bool refusal_only)
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
        lxp_state_store_init(&state, 1U) != LXP_OK ||
        lxp_state_store_bind_accounts(&state, &accounts) != LXP_OK ||
        lxp_state_store_require_account_root(&state) != LXP_OK ||
        lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(&kernel, lx_asset_module_iface()) != LXP_OK ||
        lxp_kernel_set_capabilities(&kernel, NULL,
                                     lxp_kernel_canonical_ledger_apply) != LXP_OK)
        return bridge_failure(__LINE__);
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
        return bridge_failure(__LINE__);

    (void)memset(&withdrawal, 0, sizeof(withdrawal));
    withdrawal.network_id = 7U;
    withdrawal.withdrawal_id[0] = 2U;
    (void)memcpy(withdrawal.account_id, agent->id, 32U);
    (void)memcpy(withdrawal.asset_id, asset.asset_id, 32U);
    withdrawal.amount = (lxp_u128){0U, 40U};
    withdrawal.payout_recipient[31] = 0xaaU;
    if (lxp_withdrawal_leaf(&withdrawal, leaf_hash) != LXP_OK) return bridge_failure(__LINE__);

    (void)memcpy(withdrawal.checkpoint_id, checkpoint_id, 32U);
    if (lxp_withdrawal_nullifier(&withdrawal, nullifier) != LXP_OK) return bridge_failure(__LINE__);

    request_context(&fixture, agent, withdrawals, &asset, withdrawal.amount,
                    &authority, &transfer);
    if (refusal_only) {
        lx_withdrawal_request malformed = withdrawal;
        (void)memset(malformed.payout_recipient, 0, 32U);
        if (roots(&fixture, before) != 0 ||
            lxp_withdrawal_nullifier(&malformed, nullifier) !=
                LXP_ERR_NON_CANONICAL ||
            lxp_withdrawal_leaf(&malformed, leaf_hash) != LXP_ERR_NON_CANONICAL ||
            lxp_bridge_withdraw_request(&module_ctx, &transfer, &malformed,
                                         &store, &receipt) !=
                LXP_ERR_GRANT_SCOPE_VIOLATION ||
            roots(&fixture, after) != 0 ||
            memcmp(before, after, sizeof(before)) != 0 ||
            module_ctx.transfer_applied || module_ctx.staged_count != 0U ||
            store.count != 0U || agent->balance.lo != 100U ||
            !lxp_u128_is_zero(withdrawals->balance) ||
            conserved_both(&fixture, asset.asset_id, asset_b.asset_id,
                           0U, 0U, 0U, 0U) != 0 ||
            lxp_state_store_destroy(&state) != LXP_OK)
            return bridge_failure(__LINE__);
        lx_account_registry_release(&accounts);
        lx_account_registry_release(&accounts_b);
        return 0;
    }
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
        return bridge_failure(__LINE__);

    if (bridge_withdraw_reopen(&fixture) != 0) return bridge_failure(__LINE__);
    agent = fixture.ledger[0];
    withdrawals = fixture.ledger[1];

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
            return bridge_failure(__LINE__);
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
        return bridge_failure(__LINE__);
    if (lxp_state_store_destroy(&state) != LXP_OK) return bridge_failure(__LINE__);
    lx_account_registry_release(&accounts);
    lx_account_registry_release(&accounts_b);
    return 0;
}

#ifndef LXP_BRIDGE_WITHDRAW_PROOF
int main(void)
{
    return bridge_withdraw_regression(false);
}
#endif
