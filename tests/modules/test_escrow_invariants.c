#define ASSET_NATIVE_HARNESS_ONLY
#include "test_asset_withdraw_activity.c"
#undef ASSET_NATIVE_HARNESS_ONLY

#include "layerx/lx_escrow.h"
#include "layerx/lxp_kernel.h"

#include <string.h>

#include "test_escrow_audit.h"

typedef struct hold_scan {
    uint8_t previous[32];
    size_t count;
    size_t settled;
} hold_scan;

static lxp_result apply_capability(lxp_kernel *kernel,
                                   const lxp_transfer_set *set,
                                   lxp_receipt *receipt)
{
    lxp_transfer_set_result result;
    lxp_transfer_context context = set->context;
    lxp_result status;
    (void)kernel;
    if (set->context.source_authorities == NULL ||
        set->context.source_authority_count != 1U ||
        set->context.origin_module_id != LXP_MODULE_ESCROW)
        return LXP_ERR_NON_CANONICAL;
    status = lxp_apply_transfer_set((lxp_transfer_leg *)set->legs,
                                    set->leg_count, &context, &result);
    if (status == LXP_OK)
        (void)memcpy(receipt->transfer_set_root, result.transfer_set_root, 32U);
    return status;
}

static uint64_t next_random(uint64_t *state)
{
    *state ^= *state << 13U;
    *state ^= *state >> 7U;
    *state ^= *state << 17U;
    return *state;
}

static void tagged_key(uint8_t key[32], uint8_t tag, size_t iteration)
{
    (void)memset(key, 0, 32U);
    key[0] = tag;
    key[1] = (uint8_t)(iteration >> 8U);
    key[2] = (uint8_t)iteration;
}

static void record_init(lx_escrow_record *record, size_t iteration,
                        const lx_account *owner,
                        const lx_account *escrow_account,
                        const lx_account *beneficiary,
                        const lx_asset_record *asset, uint64_t amount,
                        uint64_t clock)
{
    (void)memset(record, 0, sizeof(*record));
    tagged_key(record->escrow_id, 7U, iteration);
    (void)memcpy(record->owner, owner->id, 32U);
    (void)memcpy(record->escrow_account, escrow_account->id, 32U);
    (void)memcpy(record->beneficiary, beneficiary->id, 32U);
    record->arbiter[0] = 8U;
    (void)memcpy(record->asset_id, asset->asset_id, 32U);
    record->locked_amount = (lxp_u128){ 0U, amount };
    record->state = LX_ESCROW_STATE_OPEN;
    record->expiry = clock + 50U;
    record->dispute_window = clock + 1000U;
}

static bool settled_hold(lx_escrow_status state)
{
    return state == LX_ESCROW_STATE_CAPTURED ||
           state == LX_ESCROW_STATE_RELEASED ||
           state == LX_ESCROW_STATE_RESOLVED ||
           state == LX_ESCROW_STATE_TIMED_OUT;
}

static lxp_result scan_holds(const lx_escrow_record *record, void *user)
{
    hold_scan *scan = (hold_scan *)user;
    if (scan->count != 0U &&
        memcmp(scan->previous, record->escrow_id, 32U) >= 0)
        return LXP_ERR_UNSORTED_SEQUENCE;
    (void)memcpy(scan->previous, record->escrow_id, 32U);
    ++scan->count;
    if (settled_hold(record->state) && lxp_u128_is_zero(record->locked_amount))
        ++scan->settled;
    return LXP_OK;
}

static int locked_funds_are_unspendable(lx_account *owner,
                                        lx_account *beneficiary,
                                        lx_account *escrow_account,
                                        const lx_asset_record *asset,
                                        lxp_transfer_asset_state *asset_state)
{
    lxp_transfer_leg ordinary;
    lxp_transfer_context context;
    lxp_transfer_result result;
    (void)memset(&ordinary, 0, sizeof(ordinary));
    (void)memset(&context, 0, sizeof(context));
    ordinary.from = owner;
    ordinary.to = beneficiary;
    (void)memcpy(ordinary.asset_id, asset->asset_id, 32U);
    ordinary.amount = (lxp_u128){ 0U, 100U };
    ordinary.reason = LXP_REASON_PAYMENT;
    context.assets = asset_state;
    context.asset_count = 1U;
    context.actor_sequence = owner->next_sequence;
    context.sequence_account = owner;
    context.debit_authority_kind = LXP_AUTH_OWNER;
    (void)memcpy(context.authorized_from, owner->id, 32U);
    if (lxp_apply_transfer(&ordinary, &context, &result) !=
            LXP_ERR_INSUFFICIENT_BALANCE ||
        owner->balance.lo != 0U || escrow_account->balance.lo != 100U)
        return 1;
    return 0;
}

static int property_sequences(lxp_kernel *kernel, lxp_arena *arena,
                              lx_asset_record *asset,
                              lxp_transfer_asset_state *asset_state)
{
    lx_account owner;
    lx_account escrow_account;
    lx_account beneficiary;
    lx_escrow_record record;
    lx_escrow_record stored;
    lx_escrow_open_request open;
    lx_escrow_capture_request capture;
    lx_escrow_release_request release;
    lx_escrow_dispute_request dispute;
    lxp_authority_resolved beneficiary_authority;
    lxp_authority_resolved owner_authority;
    lxp_authority_resolved arbiter_authority;
    lxp_module_ctx ctx;
    lxp_receipt receipt;
    hold_scan scan;
    uint64_t random_state = UINT64_C(0x91e10da5c79e7b1d);
    uint64_t clock = 500U;
    uint64_t sequence = 0U;
    size_t iteration;

    (void)memset(&owner, 0, sizeof(owner));
    (void)memset(&escrow_account, 0, sizeof(escrow_account));
    (void)memset(&beneficiary, 0, sizeof(beneficiary));
    (void)memset(&scan, 0, sizeof(scan));
    owner.id[0] = 1U;
    owner.kind = LX_ACCOUNT_AGENT_MAIN;
    escrow_account.id[0] = 2U;
    escrow_account.kind = LX_ACCOUNT_AGENT_ESCROW;
    beneficiary.id[0] = 3U;
    beneficiary.kind = LX_ACCOUNT_AGENT_MAIN;
    (void)memset(&beneficiary_authority, 0, sizeof(beneficiary_authority));
    (void)memset(&owner_authority, 0, sizeof(owner_authority));
    (void)memset(&arbiter_authority, 0, sizeof(arbiter_authority));
    (void)memcpy(beneficiary_authority.principal, beneficiary.id, 32U);
    (void)memcpy(owner_authority.principal, owner.id, 32U);
    arbiter_authority.principal[0] = 8U;

    for (iteration = 0U; iteration < 64U; ++iteration) {
        uint64_t locked = iteration == 0U ? 100U :
            next_random(&random_state) % 100U + 1U;
        uint64_t mode = next_random(&random_state) % 4U;
        clock += 100U;
        if (lxp_ledger_bootstrap_balance(&owner, asset->asset_id,
                                         (lxp_u128){ 0U, 100U }, 0U) != LXP_OK ||
            lxp_ledger_bootstrap_balance(&escrow_account, asset->asset_id,
                                         (lxp_u128){ 0U, 0U }, 0U) != LXP_OK ||
            lxp_ledger_bootstrap_balance(&beneficiary, asset->asset_id,
                                         (lxp_u128){ 0U, 0U }, 0U) != LXP_OK ||
            lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_ESCROW, clock, 0U,
                                ++sequence, 100000U, arena, true) != LXP_OK)
            return 1;
        record_init(&record, iteration, &owner, &escrow_account, &beneficiary,
                    asset, locked, clock);
        (void)memset(&open, 0, sizeof(open));
        open.owner = &owner;
        open.escrow_account = &escrow_account;
        open.asset = asset;
        open.amount = (lxp_u128){ 0U, locked };
        open.record = record;
        if (lx_escrow_open_execute(&ctx, &open, &receipt) != LXP_OK ||
            lx_escrow_lookup(&ctx, record.escrow_id, &stored) != LXP_OK ||
            lx_escrow_invariant_check(&stored, &escrow_account) != LXP_OK ||
            lx_escrow_open_execute(&ctx, &open, &receipt) !=
                LXP_ERR_ESCROW_STATE ||
            lxp_module_ctx_commit(&ctx) != LXP_OK)
            return 1;
        if (iteration == 0U &&
            locked_funds_are_unspendable(&owner, &beneficiary,
                                         &escrow_account, asset,
                                         asset_state) != 0)
            return 1;

        (void)memset(&release, 0, sizeof(release));
        release.escrow_id = record.escrow_id;
        release.escrow_account = &escrow_account;
        release.owner_account = &owner;
        release.asset = asset;
        release.authority = &owner_authority;
        tagged_key(release.idempotency_key, 2U, iteration);
        if (mode == 0U && locked > 1U) {
            uint64_t part = next_random(&random_state) % (locked - 1U) + 1U;
            if (lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_ESCROW, clock, 0U,
                                    ++sequence, 100000U, arena, true) != LXP_OK)
                return 1;
            (void)memset(&capture, 0, sizeof(capture));
            capture.escrow_id = record.escrow_id;
            capture.escrow_account = &escrow_account;
            capture.beneficiary_account = &beneficiary;
            capture.owner_account = &owner;
            capture.asset = asset;
            capture.amount = (lxp_u128){ 0U, part };
            capture.authority = &beneficiary_authority;
            tagged_key(capture.idempotency_key, 1U, iteration);
            if (lx_escrow_partial_capture_execute(&ctx, &capture,
                                                  &receipt) != LXP_OK ||
                lx_escrow_lookup(&ctx, record.escrow_id, &stored) != LXP_OK ||
                stored.captured_amount.lo != part ||
                stored.state != LX_ESCROW_STATE_PARTIALLY_CAPTURED ||
                lx_escrow_invariant_check(&stored, &escrow_account) != LXP_OK ||
                lxp_module_ctx_commit(&ctx) != LXP_OK ||
                lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_ESCROW, clock, 0U,
                                    ++sequence, 100000U, arena, true) != LXP_OK ||
                lx_escrow_release_execute(&ctx, &release, &receipt) != LXP_OK)
                return 1;
        } else if (mode == 1U || (mode == 0U && locked == 1U)) {
            if (lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_ESCROW, clock, 0U,
                                    ++sequence, 100000U, arena, true) != LXP_OK ||
                lx_escrow_release_execute(&ctx, &release, &receipt) != LXP_OK)
                return 1;
        } else if (mode == 2U) {
            clock = record.expiry;
            if (lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_ESCROW, clock, 0U,
                                    ++sequence, 100000U, arena, true) != LXP_OK ||
                lx_escrow_timeout_execute(&ctx, &release, &receipt) != LXP_OK)
                return 1;
        } else {
            if (lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_ESCROW, clock, 0U,
                                    ++sequence, 100000U, arena, true) != LXP_OK)
                return 1;
            (void)memset(&dispute, 0, sizeof(dispute));
            dispute.escrow_id = record.escrow_id;
            dispute.escrow_account = &escrow_account;
            dispute.beneficiary_account = &beneficiary;
            dispute.owner_account = &owner;
            dispute.asset = asset;
            dispute.authority = &beneficiary_authority;
            dispute.beneficiary_basis_points =
                (uint32_t)(next_random(&random_state) % 10001U);
            tagged_key(dispute.idempotency_key, 3U, iteration);
            if (lx_escrow_dispute_open_execute(&ctx, &dispute) != LXP_OK ||
                lx_escrow_lookup(&ctx, record.escrow_id, &stored) != LXP_OK ||
                stored.state != LX_ESCROW_STATE_DISPUTED ||
                lx_escrow_dispute_resolve_execute(&ctx, &dispute, &receipt) !=
                    LXP_ERR_UNAUTHORIZED_DEBIT)
                return 1;
            dispute.authority = &arbiter_authority;
            if (lx_escrow_dispute_resolve_execute(&ctx, &dispute,
                                                  &receipt) != LXP_OK)
                return 1;
        }
        if (lx_escrow_lookup(&ctx, record.escrow_id, &stored) != LXP_OK ||
            lx_escrow_invariant_check(&stored, &escrow_account) != LXP_OK ||
            !settled_hold(stored.state) ||
            !lxp_u128_is_zero(stored.locked_amount) ||
            owner.balance.lo + escrow_account.balance.lo +
                beneficiary.balance.lo != 100U ||
            lxp_module_ctx_commit(&ctx) != LXP_OK)
            return 1;
    }
    if (lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_ESCROW, clock, 0U,
                            ++sequence, 100000U, arena, true) != LXP_OK ||
        lx_escrow_state_iter(&ctx, scan_holds, &scan) != LXP_OK ||
        scan.count != 64U || scan.settled != 64U ||
        lxp_module_ctx_commit(&ctx) != LXP_OK)
        return 1;
    return 0;
}

typedef struct native_hold_scan {
    lx_account_registry *accounts;
    uint8_t previous[32];
    uint8_t custody_ids[2][32];
    lxp_u128 total;
    size_t count;
} native_hold_scan;

static lxp_result scan_native_holds(const lx_escrow_record *record, void *opaque)
{
    native_hold_scan *scan = opaque;
    size_t slot;
    uint64_t expected = scan->count == 0U ? 20U : 30U;
    if (scan->count >= 2U ||
        (scan->count != 0U && memcmp(scan->previous, record->escrow_id, 32U) >= 0) ||
        memcmp(record->escrow_account, scan->custody_ids[scan->count], 32U) != 0 ||
        record->locked_amount.hi != 0U || record->locked_amount.lo != expected ||
        lx_account_registry_index_lookup(scan->accounts, record->escrow_account, &slot) != LXP_OK ||
        lx_escrow_invariant_check(record, &scan->accounts->accounts[slot]) != LXP_OK ||
        lxp_u128_add(scan->total, record->locked_amount, &scan->total) != LXP_OK)
        return LXP_FATAL_INVARIANT;
    memcpy(scan->previous, record->escrow_id, 32U);
    ++scan->count;
    return LXP_OK;
}

static int reserve_lines(void)
{
    native_repair_fixture *f = calloc(1U, sizeof(*f));
    lx_account *main_account;
    lx_account *holds[2];
    lx_asset_record *asset;
    lx_escrow_runtime runtime;
    lx_asset_native_supply_report report;
    lx_asset_reserve_report_record historical_v1 = {0};
    native_hold_scan scan = {0};
    lxp_send send;
    lxp_result outcome;
    lxp_module_ctx context;
    lxp_arena memory;
    uint8_t bytes[4096], encoded_v1[370], idempotency[32] = {90U};
    uint8_t burn[82] = {0U, 1U};
    const uint8_t seed[32] = {31U};
    const char *owner_name = "agent:did:key:native-a:main";
    size_t length;
    NATIVE_CHECK(f != NULL && native_fixture_init(f) == 0 && native_issue(f, 0U) == 0);
    NATIVE_CHECK(lx_account_registry_reserve(&f->accounts, f->accounts.count + 3U) == LXP_OK);
    NATIVE_CHECK(lx_asset_lookup(&f->assets, f->asset_ids[0], &asset) == LXP_OK);
    NATIVE_CHECK(lx_asset_account_open(&f->assets, &f->accounts, asset->asset_id,
        (const uint8_t *)owner_name, strlen(owner_name), f->state.next_sequence,
        LX_ACCOUNT_OPEN_CREDIT, NULL, &main_account) == LXP_OK);
    NATIVE_CHECK(native_make_send(f, 0U, 0U, main_account->id, 60U, 90U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 0U, false, &outcome) == 0 && outcome == LXP_OK);
    memcpy(burn + 2U, asset->asset_id, 32U);
    memcpy(burn + 34U, f->holder_ids[0][0], 32U);
    burn[81] = 40U;
    ++idempotency[0];
    NATIVE_CHECK(native_submit(f, "did:key:native-a", seed, LX_ASSET_BURN,
        burn, sizeof(burn), idempotency, native_holder(f, 0U, 0U)->next_sequence,
        f->holder_ids[0][0], false, &outcome) == 0 && outcome == LXP_OK);
    NATIVE_CHECK(asset->total_units.lo == 60U);
    NATIVE_CHECK(lxp_kernel_register_module(&f->kernel, lx_escrow_module_iface()) == LXP_OK);
    runtime.accounts = &f->accounts;
    runtime.assets = &f->assets;
    NATIVE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ESCROW, &runtime) == LXP_OK);
    /* Insert in reverse identifier order; the committed iterator must sort. */
    for (size_t reverse = 2U; reverse > 0U; --reverse) {
        size_t i = reverse - 1U;
        lx_escrow_record record;
        uint8_t payload[LX_ESCROW_OPEN_PAYLOAD_BYTES];
        char name[LX_ACCOUNT_NAME_MAX];
        int count = snprintf(name, sizeof(name),
            "agent:did:key:native-a:escrow:%02x%062u", (unsigned)(i + 1U), 0U);
        NATIVE_CHECK(count > 0 && (size_t)count < sizeof(name));
        NATIVE_CHECK(lx_asset_account_open(&f->assets, &f->accounts, asset->asset_id,
            (const uint8_t *)name, (size_t)count, f->state.next_sequence,
            LX_ACCOUNT_OPEN_CREDIT, NULL, &holds[i]) == LXP_OK);
        record_init(&record, i, main_account, holds[i], main_account, asset,
                    i == 0U ? 20U : 30U, 10U);
        memset(record.escrow_id, 0, 32U);
        record.escrow_id[0] = (uint8_t)(i + 1U);
        escrow_audit_open_payload(&record, payload);
        ++idempotency[0];
        NATIVE_CHECK(native_submit(f, "did:key:native-a", seed, LX_ESCROW_OPEN,
            payload, sizeof(payload), idempotency, main_account->next_sequence,
            main_account->id, false, &outcome) == 0);
        NATIVE_CHECK(TEST_EQ(outcome, LXP_OK));
        NATIVE_CHECK(holds[i]->balance.lo == (i == 0U ? 20U : 30U));
    }
    NATIVE_CHECK(lxp_arena_init(&memory, bytes, sizeof(bytes)) == LXP_OK);
    NATIVE_CHECK(lxp_module_ctx_init(&context, &f->kernel, LXP_MODULE_ESCROW, 10U,
        0U, f->state.next_sequence, 100000U, &memory, true) == LXP_OK);
    context.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    scan.accounts = &f->accounts;
    memcpy(scan.custody_ids[0], holds[0]->id, 32U);
    memcpy(scan.custody_ids[1], holds[1]->id, 32U);
    NATIVE_CHECK(lx_escrow_state_iter(&context, scan_native_holds, &scan) == LXP_OK);
    NATIVE_CHECK(scan.count == 2U && scan.total.hi == 0U && scan.total.lo == 50U &&
        main_account->balance.lo == 10U && main_account->balance.hi == 0U);
    NATIVE_CHECK(lx_asset_native_reconcile(asset, &f->accounts, &report) == LXP_OK &&
        report.bank_units.hi == 0U && report.bank_units.lo == 60U &&
        report.registered_units.lo == 60U && report.issuance_remaining.lo == 40U);
    NATIVE_CHECK(lx_asset_native_supply_check(&f->assets, &f->accounts) == LXP_OK);
    /* Retired reserve-report encoding v1 only; no custody attestation is an
     * input to the live reconciliation above. */
    memcpy(historical_v1.asset_id, asset->asset_id, 32U);
    historical_v1.agent_main.lo = 10U;
    historical_v1.escrow = scan.total;
    historical_v1.raw_total.lo = 60U;
    historical_v1.escrow_line_count = 2U;
    for (size_t i = 0U; i < 2U; ++i) {
        memcpy(historical_v1.escrow_lines[i].account_id, holds[i]->id, 32U);
        historical_v1.escrow_lines[i].balance = holds[i]->balance;
    }
    NATIVE_CHECK(lx_asset_historical_reserve_encode_v1(&historical_v1,
        encoded_v1, sizeof(encoded_v1), &length) == LXP_OK && length == 370U);
    NATIVE_CHECK(memcmp(encoded_v1 + 274U, holds[0]->id, 32U) == 0 &&
        encoded_v1[321] == 20U && memcmp(encoded_v1 + 322U, holds[1]->id, 32U) == 0 &&
        encoded_v1[369] == 30U);
    NATIVE_CHECK(native_fixture_destroy(f) == 0);
    free(f);
    return 0;
}

int main(void)
{
    lx_asset_record asset;
    lxp_transfer_asset_state asset_state;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_arena arena;
    uint8_t arena_bytes[4096];
    uint64_t parameters = 1U;
    lx_account terminal_account;
    lx_escrow_record terminal_record;

    if (audit_escrow_findings() != 0) return 1;
    (void)puts("escrow audit findings 8, 9, 10 and 11: passed");

    (void)memset(&asset, 0, sizeof(asset));
    asset.asset_id[0] = 5U;
    if (lx_asset_transfer_state(&asset, &asset_state) != LXP_OK ||
        lxp_state_store_init(&state, 0U) != LXP_OK ||
        lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(&kernel, lx_escrow_module_iface()) != LXP_OK ||
        lxp_kernel_set_capabilities(&kernel, NULL, apply_capability) != LXP_OK ||
        lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK ||
        property_sequences(&kernel, &arena, &asset, &asset_state) != 0 ||
        reserve_lines() != 0)
        return 1;

    (void)memset(&terminal_account, 0, sizeof(terminal_account));
    (void)memset(&terminal_record, 0, sizeof(terminal_record));
    terminal_account.id[0] = 9U;
    terminal_account.kind = LX_ACCOUNT_AGENT_ESCROW;
    if (lxp_ledger_bootstrap_balance(&terminal_account, asset.asset_id,
                                     (lxp_u128){ 0U, 1U }, 0U) != LXP_OK)
        return 1;
    (void)memcpy(terminal_record.escrow_account, terminal_account.id, 32U);
    (void)memcpy(terminal_record.asset_id, asset.asset_id, 32U);
    terminal_record.state = LX_ESCROW_STATE_RELEASED;
    if (lx_escrow_invariant_check(&terminal_record, &terminal_account) !=
            LXP_FATAL_INVARIANT ||
        lx_escrow_authority_check(&terminal_account, LXP_AUTH_OWNER, 0U,
                                  LXP_REASON_PAYMENT) !=
            LXP_ERR_UNAUTHORIZED_DEBIT ||
        lx_escrow_authority_check(&terminal_account, LXP_AUTH_SESSION_KEY, 0U,
                                  LXP_REASON_PAYMENT) !=
            LXP_ERR_UNAUTHORIZED_DEBIT ||
        lx_escrow_authority_check(&terminal_account, LXP_AUTH_ESCROW,
                                  LXP_MODULE_ESCROW, LXP_REASON_PAYMENT) !=
            LXP_ERR_UNAUTHORIZED_ESCROW_SPEND ||
        lx_escrow_authority_check(&terminal_account, LXP_AUTH_ESCROW,
                                  LXP_MODULE_ESCROW,
                                  LXP_REASON_ESCROW_CAPTURE) != LXP_OK ||
        lxp_state_store_destroy(&state) != LXP_OK)
        return 1;
    return 0;
}
