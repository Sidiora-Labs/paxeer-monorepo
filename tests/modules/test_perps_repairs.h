#ifndef TEST_PERPS_REPAIRS_H
#define TEST_PERPS_REPAIRS_H

#include "layerx/lxp_snapshot.h"
#include <stdio.h>

#define REPAIR_CHECK(expression) do { \
    if (!(expression)) { \
        (void)fprintf(stderr, "perps repair: line %d: %s\n", __LINE__, #expression); \
        return 1; \
    } \
} while (0)
#define REPAIR_RESULT(expression, expected) do { \
    lxp_result repair_result = (expression); \
    if (repair_result != (expected)) { \
        (void)fprintf(stderr, "perps repair: line %d: %s = %d; expected %d\n", \
                      __LINE__, #expression, (int)repair_result, (int)(expected)); \
        return 1; \
    } \
} while (0)

static lx_account *repair_carol_main;
static lx_account *repair_keeper;

static int repair_find(const char *name, lx_account **account)
{
    uint8_t id[32];
    size_t length = strlen(name);
    return lx_account_id_from_string((const uint8_t *)name, length, id) != LXP_OK ||
        lx_account_lookup(&accounts, (const uint8_t *)name, length, id, account) != LXP_OK;
}

static int repair_reopen(void)
{
    static uint8_t snapshot_bytes[1U << 20];
    lxp_arena snapshot_arena;
    lxp_byte_span snapshot;
    lxp_snapshot_manifest_record manifest;
    uint8_t before[32];
    uint8_t after[32];
    size_t length;
    FILE *file;
    /* Module dispatch commits KV and ledger changes; the batch owner seals
     * the state journal before publishing a durable snapshot. */
    while (store.next_sequence < global_sequence) {
        REPAIR_RESULT(lxp_state_journal_open(&store, store.next_sequence,
                                              &journal), LXP_OK);
        REPAIR_RESULT(lxp_state_journal_commit(&journal), LXP_OK);
    }
    REPAIR_CHECK(store.next_sequence == global_sequence);
    REPAIR_RESULT(lxp_state_root(&kernel, before), LXP_OK);
    REPAIR_RESULT(lxp_arena_init(&snapshot_arena, snapshot_bytes,
                                 sizeof(snapshot_bytes)), LXP_OK);
    REPAIR_RESULT(lxp_snapshot_write(&kernel, store.next_sequence - 1U,
                                     &snapshot_arena, &snapshot), LXP_OK);
    REPAIR_RESULT(lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length,
        store.next_sequence - 1U, before, kernel.current_state_root, &manifest), LXP_OK);
    length = snapshot.length;
    file = tmpfile();
    REPAIR_CHECK(file != NULL);
    REPAIR_CHECK(fwrite(snapshot.bytes, 1U, length, file) == length);
    REPAIR_CHECK(fflush(file) == 0 && fseek(file, 0L, SEEK_SET) == 0);
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    lx_account_registry_release(&accounts);
    (void)memset(&kernel, 0, sizeof(kernel));
    (void)memset(&journal, 0, sizeof(journal));
    REPAIR_RESULT(lx_account_registry_init(&accounts), LXP_OK);
    REPAIR_CHECK(kernel_start() == 0);
    if (fixture_protocol == LXP_PROTOCOL_VERSION_STATE_COMMITMENT)
        REPAIR_RESULT(lxp_kernel_register_module(&kernel, lx_perps_tif_module_iface()), LXP_OK);
    (void)memset(snapshot_bytes, 0, sizeof(snapshot_bytes));
    REPAIR_CHECK(fread(snapshot_bytes, 1U, length, file) == length);
    REPAIR_CHECK(fclose(file) == 0);
    REPAIR_RESULT(lxp_snapshot_load(snapshot_bytes, length, &manifest, &kernel), LXP_OK);
    REPAIR_RESULT(lxp_snapshot_verify_root(&kernel, &manifest), LXP_OK);
    REPAIR_CHECK(store.next_sequence == global_sequence);
    REPAIR_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    REPAIR_CHECK(memcmp(before, after, 32U) == 0);
    REPAIR_CHECK(repair_find("system:liquidity:perp", &liquidity_account) == 0);
    REPAIR_CHECK(repair_find("system:funding:perp:long", &long_pool_account) == 0);
    REPAIR_CHECK(repair_find("system:funding:perp:short", &short_pool_account) == 0);
    REPAIR_CHECK(repair_find("system:insurance", &insurance_account) == 0);
    REPAIR_CHECK(repair_find("agent:did:key:alice:main", &alice_main) == 0);
    REPAIR_CHECK(repair_find("agent:did:key:alice:margin:a", &alice_margin) == 0);
    REPAIR_CHECK(repair_find("agent:did:key:bob:main", &bob_main) == 0);
    REPAIR_CHECK(repair_find("agent:did:key:bob:margin:a", &bob_margin) == 0);
    REPAIR_CHECK(repair_find("agent:did:key:carol:main", &repair_carol_main) == 0);
    REPAIR_CHECK(repair_find("agent:did:key:carol:margin:a", &carol_margin) == 0);
    REPAIR_CHECK(repair_find("agent:did:key:keeper:main", &repair_keeper) == 0);
    REPAIR_CHECK(ctx_open() == 0);
    return 0;
}

static int repair_setup(uint64_t scale, uint64_t price,
                         uint64_t liquidity, uint64_t insurance)
{
    lx_perps_market market;
    batch_timestamp = 1000U;
    global_sequence = 1U;
    activity_marker = 1U;
    fixture_protocol = LXP_PROTOCOL_VERSION_OCCUPANCY;
    REPAIR_CHECK(fixture_init() == 0 && kernel_start() == 0);
    REPAIR_RESULT(lxp_ledger_bootstrap_balance(liquidity_account, quote_asset_id,
        (lxp_u128){0U, liquidity}, 0U), LXP_OK);
    REPAIR_RESULT(lxp_ledger_bootstrap_balance(insurance_account, quote_asset_id,
        (lxp_u128){0U, insurance}, 0U), LXP_OK);
    REPAIR_CHECK(account_add("agent:did:key:alice:main", LX_ACCOUNT_OPEN_CREDIT,
                             10000U, &alice_main) == 0);
    REPAIR_CHECK(account_add("agent:did:key:alice:margin:a", LX_ACCOUNT_OPEN_CREDIT,
                             0U, &alice_margin) == 0);
    REPAIR_CHECK(account_add("agent:did:key:bob:main", LX_ACCOUNT_OPEN_CREDIT,
                             10000U, &bob_main) == 0);
    REPAIR_CHECK(account_add("agent:did:key:bob:margin:a", LX_ACCOUNT_OPEN_CREDIT,
                             0U, &bob_margin) == 0);
    REPAIR_CHECK(account_add("agent:did:key:carol:main", LX_ACCOUNT_OPEN_CREDIT,
                             10000U, &repair_carol_main) == 0);
    REPAIR_CHECK(account_add("agent:did:key:carol:margin:a", LX_ACCOUNT_OPEN_CREDIT,
                             0U, &carol_margin) == 0);
    REPAIR_CHECK(account_add("agent:did:key:keeper:main", LX_ACCOUNT_OPEN_CREDIT,
                             0U, &repair_keeper) == 0);
    market_defaults(1U, &market);
    market.price_scale = (lxp_u128){0U, scale};
    REPAIR_RESULT(market_create(&market, admin_did), LXP_OK);
    REPAIR_RESULT(oracle_push(oracle_seed, 1U, 1U, price, batch_timestamp), LXP_OK);
    return 0;
}

static int repair_position(const lx_account *margin, lx_perps_side side,
    uint64_t size, uint64_t basis, bool open)
{
    lx_perps_position position;
    if (ctx_open() != 0 || position_state(margin, &position) != LXP_OK) return 0;
    return position.side == side && position.size.hi == 0U &&
        position.size.lo == size && position.entry_notional.hi == 0U &&
        position.entry_notional.lo == basis && position.open == open;
}

static int repair_interest(uint64_t longs, uint64_t shorts)
{
    uint8_t id[32] = {1U};
    lx_perps_funding_state funding;
    return ctx_open() == 0 &&
        lx_perps_funding_state_lookup(&ctx, id, &funding) == LXP_OK &&
        funding.long_open_notional.hi == 0U && funding.long_open_notional.lo == longs &&
        funding.short_open_notional.hi == 0U && funding.short_open_notional.lo == shorts;
}

static int repair_pair(uint64_t price, uint64_t quantity, uint64_t margin)
{
    REPAIR_RESULT(position_open(alice_margin->id, alice_margin, LX_PERPS_SIDE_BUY,
        quantity, margin, "did:key:alice"), LXP_OK);
    REPAIR_RESULT(position_open(bob_margin->id, bob_margin, LX_PERPS_SIDE_SELL,
        quantity, margin, "did:key:bob"), LXP_OK);
    REPAIR_RESULT(order_place(1U, bob_margin, LX_PERPS_SIDE_SELL, price,
        quantity, "did:key:bob"), LXP_OK);
    REPAIR_RESULT(order_place(2U, alice_margin, LX_PERPS_SIDE_BUY, price,
        quantity, "did:key:alice"), LXP_OK);
    return 0;
}

static int repair_trade(uint8_t id, uint64_t price, uint64_t quantity,
                         lx_perps_side alice_side)
{
    lx_perps_side bob_side = alice_side == LX_PERPS_SIDE_BUY ?
        LX_PERPS_SIDE_SELL : LX_PERPS_SIDE_BUY;
    REPAIR_RESULT(order_place(id, bob_margin, bob_side, price,
                               quantity, "did:key:bob"), LXP_OK);
    REPAIR_RESULT(order_place((uint8_t)(id + 1U), alice_margin, alice_side,
                               price, quantity, "did:key:alice"), LXP_OK);
    return 0;
}

static lxp_result repair_cancel(uint8_t id, const char *did)
{
    lx_perps_cancel_command command;
    uint8_t payload[LX_PERPS_CANCEL_PAYLOAD_BYTES];
    (void)memset(&command, 0, sizeof(command));
    command.market_id[0] = 1U;
    command.order_id[0] = id;
    lxp_result status = lx_perps_cancel_command_encode(&command, payload);
    return status == LXP_OK ? run(LX_PERPS_ORDER_CANCEL, did, payload,
                                  sizeof(payload), NULL, NULL) : status;
}

static lxp_result repair_liquidate(const lx_account *margin)
{
    lx_perps_liquidate_command command;
    uint8_t payload[LX_PERPS_LIQUIDATE_PAYLOAD_BYTES];
    (void)memset(&command, 0, sizeof(command));
    command.market_id[0] = 1U;
    (void)memcpy(command.position_id, margin->id, 32U);
    (void)memcpy(command.liquidator_account_id, repair_keeper->id, 32U);
    lxp_result status = lx_perps_liquidate_command_encode(&command, payload);
    return status == LXP_OK ? run(LX_PERPS_LIQUIDATE, "did:key:keeper", payload,
                                  sizeof(payload), NULL, NULL) : status;
}

static uint64_t repair_total(void)
{
    uint64_t total = 0U;
    size_t i;
    for (i = 0U; i < accounts.count; ++i) {
        if (accounts.accounts[i].balance.hi != 0U ||
            UINT64_MAX - total < accounts.accounts[i].balance.lo)
            return UINT64_MAX;
        total += accounts.accounts[i].balance.lo;
    }
    return total;
}

static int test_order_price_scale(void)
{
    lx_perps_order order;
    lx_perps_market market;
    lxp_u128 notional;
    REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 500U) == 0);
    REPAIR_CHECK(repair_pair(10000U, 1U, 10U) == 0);
    REPAIR_CHECK(repair_position(alice_margin, LX_PERPS_SIDE_BUY, 1U, 100U, true));
    REPAIR_CHECK(repair_position(bob_margin, LX_PERPS_SIDE_SELL, 1U, 100U, true));
    REPAIR_CHECK(balance_is(alice_main, 9990U) && balance_is(alice_margin, 10U));
    REPAIR_CHECK(balance_is(bob_main, 9990U) && balance_is(bob_margin, 10U));
    REPAIR_CHECK(repair_interest(100U, 100U));
    REPAIR_RESULT(order_state(1U, &order), LXP_ERR_UNKNOWN_FIELD);
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_CHECK(repair_position(alice_margin, LX_PERPS_SIDE_BUY, 1U, 100U, true));
    REPAIR_RESULT(order_place(3U, carol_margin, LX_PERPS_SIDE_BUY, 10001U,
                               1U, "did:key:carol"), LXP_ERR_PARAMETER_BOUNDS);
    REPAIR_RESULT(oracle_push(oracle_seed, 1U, 2U, 10001U, batch_timestamp), LXP_OK);
    REPAIR_RESULT(position_open(carol_margin->id, carol_margin, LX_PERPS_SIDE_BUY,
        1U, 100U, "did:key:carol"), LXP_ERR_PARAMETER_BOUNDS);
    market_defaults(1U, &market);
    REPAIR_RESULT(lx_perps_fill_notional(&market,
        (lxp_u128){UINT64_MAX, UINT64_MAX}, (lxp_u128){0U, 2U}, &notional), LXP_ERR_OVERFLOW);
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    (void)puts("order-price-scale: registered reserve/place/fill and recovery passed");
    return 0;
}

static int test_close_trading_pnl(void)
{
    unsigned scenario;
    for (scenario = 0U; scenario < 2U; ++scenario) {
        uint64_t price = scenario == 0U ? 110U : 90U;
        uint64_t alice_payout = scenario == 0U ? 10100U : 9900U;
        uint64_t bob_payout = scenario == 0U ? 9900U : 10100U;
        REPAIR_CHECK(repair_setup(1U, 100U, 1000U, 500U) == 0);
        REPAIR_CHECK(repair_pair(100U, 10U, 200U) == 0);
        REPAIR_RESULT(oracle_push(oracle_seed, 1U, 2U, price, batch_timestamp), LXP_OK);
        REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_OK);
        REPAIR_CHECK(balance_is(alice_main, alice_payout) && balance_is(alice_margin, 0U));
        REPAIR_CHECK(balance_is(liquidity_account, scenario == 0U ? 900U : 1100U));
        REPAIR_CHECK(!alice_margin->has_open_reference && repair_interest(0U, 1000U));
        REPAIR_CHECK(repair_reopen() == 0);
        REPAIR_CHECK(balance_is(alice_main, alice_payout));
        REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_ERR_AGREEMENT_STATE);
        REPAIR_RESULT(position_close(bob_margin, "did:key:bob"), LXP_OK);
        REPAIR_CHECK(balance_is(bob_main, bob_payout) && balance_is(bob_margin, 0U));
        REPAIR_CHECK(balance_is(liquidity_account, 1000U) && repair_interest(0U, 0U));
        REPAIR_CHECK(repair_total() == 231500U);
        REPAIR_CHECK(repair_reopen() == 0);
        REPAIR_CHECK(balance_is(bob_main, bob_payout));
        REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    }
    (void)puts("close-trading-pnl: profitable and losing long/short closes passed");
    return 0;
}

static int test_close_rollback_recovery(void)
{
    uint8_t before[32];
    uint8_t after[32];
    REPAIR_CHECK(repair_setup(1U, 100U, 0U, 500U) == 0);
    REPAIR_CHECK(repair_pair(100U, 10U, 200U) == 0);
    REPAIR_RESULT(oracle_push(oracle_seed, 1U, 2U, 110U, batch_timestamp), LXP_OK);
    REPAIR_RESULT(lxp_state_root(&kernel, before), LXP_OK);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_ERR_INSUFFICIENT_BALANCE);
    REPAIR_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    REPAIR_CHECK(memcmp(before, after, 32U) == 0);
    REPAIR_CHECK(balance_is(alice_main, 9800U) && balance_is(alice_margin, 200U));
    REPAIR_CHECK(repair_position(alice_margin, LX_PERPS_SIDE_BUY, 10U, 1000U, true));
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_ERR_INSUFFICIENT_BALANCE);
    bob_main->frozen = true;
    REPAIR_RESULT(lxp_state_root(&kernel, before), LXP_OK);
    REPAIR_RESULT(position_close(bob_margin, "did:key:bob"), LXP_ERR_ACCOUNT_FROZEN);
    REPAIR_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    REPAIR_CHECK(memcmp(before, after, 32U) == 0);
    REPAIR_CHECK(balance_is(liquidity_account, 0U) && balance_is(bob_margin, 200U));
    bob_main->frozen = false;
    fixture_full_effects = true;
    REPAIR_RESULT(lxp_state_root(&kernel, before), LXP_OK);
    REPAIR_RESULT(position_close(bob_margin, "did:key:bob"), LXP_ERR_LENGTH_LIMIT);
    fixture_full_effects = false;
    REPAIR_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    REPAIR_CHECK(memcmp(before, after, 32U) == 0);
    REPAIR_CHECK(bob_margin->has_open_reference && balance_is(bob_margin, 200U));
    REPAIR_CHECK(repair_position(bob_margin, LX_PERPS_SIDE_SELL, 10U, 1000U, true));
    REPAIR_RESULT(position_close(bob_margin, "did:key:bob"), LXP_OK);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(balance_is(alice_main, 10100U) && balance_is(bob_main, 9900U));
    REPAIR_CHECK(repair_reopen() == 0 && repair_interest(0U, 0U));
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    (void)puts("close-trading-pnl: insufficient liquidity, atomic rollback and recovery passed");
    return 0;
}

static int test_fill_reduction_pnl(void)
{
    uint64_t total;
    REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 500U) == 0);
    total = repair_total();
    REPAIR_CHECK(repair_pair(10000U, 1U, 1000U) == 0);
    REPAIR_CHECK(repair_trade(3U, 10100U, 2U, LX_PERPS_SIDE_BUY) == 0);
    REPAIR_CHECK(repair_position(alice_margin, LX_PERPS_SIDE_BUY, 3U, 302U, true));
    REPAIR_CHECK(repair_position(bob_margin, LX_PERPS_SIDE_SELL, 3U, 302U, true));
    REPAIR_CHECK(repair_trade(5U, 10200U, 1U, LX_PERPS_SIDE_SELL) == 0);
    REPAIR_CHECK(balance_is(alice_margin, 1002U) && balance_is(bob_margin, 998U));
    REPAIR_CHECK(balance_is(liquidity_account, 1000U) && repair_total() == total);
    REPAIR_CHECK(repair_position(alice_margin, LX_PERPS_SIDE_BUY, 2U, 202U, true));
    REPAIR_CHECK(repair_position(bob_margin, LX_PERPS_SIDE_SELL, 2U, 202U, true));
    REPAIR_CHECK(repair_interest(202U, 202U));
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_CHECK(balance_is(alice_margin, 1002U) && balance_is(bob_margin, 998U));
    REPAIR_CHECK(repair_trade(7U, 9900U, 2U, LX_PERPS_SIDE_SELL) == 0);
    REPAIR_CHECK(balance_is(alice_main, 9998U) && balance_is(bob_main, 10002U));
    REPAIR_CHECK(balance_is(alice_margin, 0U) && balance_is(bob_margin, 0U));
    REPAIR_CHECK(balance_is(liquidity_account, 1000U) && repair_total() == total);
    REPAIR_CHECK(repair_interest(0U, 0U) && repair_reopen() == 0);
    REPAIR_CHECK(balance_is(alice_main, 9998U) && balance_is(bob_main, 10002U));
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    (void)puts("fill-reduction-pnl: mixed basis, both partial reductions, full offsets and recovery passed");
    return 0;
}

static int test_fill_reversal_pnl(void)
{
    uint64_t total;
    REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 500U) == 0);
    total = repair_total();
    REPAIR_CHECK(repair_pair(10000U, 2U, 1000U) == 0);
    REPAIR_CHECK(repair_trade(3U, 11000U, 3U, LX_PERPS_SIDE_SELL) == 0);
    REPAIR_CHECK(repair_position(alice_margin, LX_PERPS_SIDE_SELL, 1U, 110U, true));
    REPAIR_CHECK(repair_position(bob_margin, LX_PERPS_SIDE_BUY, 1U, 110U, true));
    REPAIR_CHECK(balance_is(alice_margin, 1020U) && balance_is(bob_margin, 980U));
    REPAIR_CHECK(repair_total() == total && repair_interest(110U, 110U));
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_CHECK(repair_trade(5U, 9000U, 1U, LX_PERPS_SIDE_BUY) == 0);
    REPAIR_CHECK(balance_is(alice_main, 10040U) && balance_is(bob_main, 9960U));
    REPAIR_CHECK(balance_is(alice_margin, 0U) && balance_is(bob_margin, 0U));
    REPAIR_CHECK(balance_is(liquidity_account, 1000U) && repair_total() == total);
    REPAIR_CHECK(repair_reopen() == 0 && repair_interest(0U, 0U));
    REPAIR_CHECK(balance_is(alice_main, 10040U) && balance_is(bob_main, 9960U));
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    (void)puts("fill-reduction-pnl: both reversals, opposite offsets and recovery passed");
    return 0;
}

static int test_liquidation_equity(void)
{
    unsigned scenario;
    REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 500U) == 0);
    REPAIR_CHECK(repair_pair(10000U, 1U, 10U) == 0);
    REPAIR_RESULT(oracle_push(oracle_seed, 1U, 2U, 20000U, batch_timestamp), LXP_OK);
    REPAIR_RESULT(repair_liquidate(alice_margin), LXP_ERR_MARGIN_INSUFFICIENT);
    REPAIR_CHECK(repair_position(alice_margin, LX_PERPS_SIDE_BUY, 1U, 100U, true));
    REPAIR_RESULT(repair_liquidate(bob_margin), LXP_OK);
    REPAIR_CHECK(balance_is(liquidity_account, 1100U) && balance_is(insurance_account, 410U));
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_CHECK(repair_position(bob_margin, LX_PERPS_SIDE_SELL, 1U, 100U, false));
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    for (scenario = 0U; scenario < 2U; ++scenario) {
        REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 500U) == 0);
        REPAIR_CHECK(repair_pair(10000U, 1U, 10U) == 0);
        REPAIR_RESULT(oracle_push(oracle_seed, 1U, 2U,
            scenario == 0U ? 9500U : 10400U, batch_timestamp), LXP_OK);
        REPAIR_RESULT(repair_liquidate(scenario == 0U ? alice_margin : bob_margin),
                       LXP_ERR_MARGIN_INSUFFICIENT);
        REPAIR_RESULT(oracle_push(oracle_seed, 1U, 3U,
            scenario == 0U ? 9400U : 10500U, batch_timestamp), LXP_OK);
        REPAIR_RESULT(repair_liquidate(scenario == 0U ? alice_margin : bob_margin), LXP_OK);
        REPAIR_CHECK(repair_reopen() == 0);
        REPAIR_CHECK(balance_is(scenario == 0U ? alice_margin : bob_margin, 0U));
        REPAIR_RESULT(repair_liquidate(scenario == 0U ? alice_margin : bob_margin),
                       LXP_ERR_AGREEMENT_STATE);
        REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    }
    (void)puts("liquidation-equity: profitable-long refusal, adverse long/short and exact boundaries passed");
    return 0;
}

static int repair_enable_tif(void)
{
    uint8_t key[38];
    uint8_t bytes[512];
    size_t length;
    asset_record.issuer_kind = 2U;
    asset_record.name_length = 5U;
    (void)memcpy(asset_record.name, "Quote", 5U);
    (void)memcpy(asset_record.issuer_did32, administrator, 32U);
    asset_record.total_units = (lxp_u128){0U, repair_total()};
    REPAIR_RESULT(lx_asset_record_encode(&asset_record, bytes, sizeof(bytes), &length), LXP_OK);
    (void)memcpy(key, "asset:", 6U);
    (void)memcpy(key + 6U, quote_asset_id, 32U);
    REPAIR_RESULT(lxp_kernel_register_module(&kernel, lx_asset_module_iface()), LXP_OK);
    REPAIR_CHECK(ctx_open() == 0);
    REPAIR_RESULT(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_ASSET,
        batch_timestamp, 0U, global_sequence, 1000000U, &arena, true), LXP_OK);
    REPAIR_RESULT(lxp_ctx_kv_put(&ctx, key, sizeof(key), bytes, length), LXP_OK);
    REPAIR_RESULT(lxp_module_ctx_commit(&ctx), LXP_OK);
    ++global_sequence;
    REPAIR_RESULT(lxp_kernel_register_module(&kernel, lx_perps_tif_module_iface()), LXP_OK);
    fixture_protocol = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    return 0;
}

static lxp_result repair_ioc(void)
{
    lx_perps_order_command command;
    uint8_t payload[LX_PERPS_ORDER_PAYLOAD_TIF_BYTES];
    size_t length;
    (void)memset(&command, 0, sizeof(command));
    command.market_id[0] = 1U;
    command.order_id[0] = 9U;
    (void)memcpy(command.owner_account_id, alice_margin->id, 32U);
    command.side = LX_PERPS_SIDE_BUY;
    command.price = (lxp_u128){0U, 10000U};
    command.quantity = (lxp_u128){0U, 1U};
    command.time_in_force = LX_PERPS_TIF_IMMEDIATE_OR_CANCEL;
    lxp_result status = lx_perps_order_command_encode_versioned(&command,
        payload, sizeof(payload), &length);
    return status == LXP_OK ? run(LX_PERPS_ORDER_PLACE, "did:key:alice", payload,
                                  length, NULL, NULL) : status;
}

static int test_unfilled_margin_release(void)
{
    lx_perps_position position;
    lx_perps_order order;
    REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 500U) == 0);
    REPAIR_RESULT(position_open(alice_margin->id, alice_margin, LX_PERPS_SIDE_BUY,
        1U, 100U, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_RESULT(position_close(alice_margin, "did:key:bob"), LXP_ERR_UNAUTHORIZED_DEBIT);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(balance_is(alice_main, 10000U) && balance_is(alice_margin, 0U));
    REPAIR_CHECK(!alice_margin->has_open_reference);
    REPAIR_RESULT(position_state(alice_margin, &position), LXP_ERR_UNKNOWN_FIELD);
    REPAIR_RESULT(position_open(alice_margin->id, alice_margin, LX_PERPS_SIDE_BUY,
        1U, 100U, "did:key:alice"), LXP_OK);
    REPAIR_RESULT(order_place(3U, alice_margin, LX_PERPS_SIDE_BUY, 10000U,
        1U, "did:key:alice"), LXP_OK);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_ERR_ACCOUNT_NOT_EMPTY);
    REPAIR_RESULT(repair_cancel(3U, "did:key:bob"), LXP_ERR_UNAUTHORIZED_DEBIT);
    REPAIR_RESULT(repair_cancel(3U, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(balance_is(alice_main, 10000U));
    REPAIR_CHECK(repair_pair(10000U, 1U, 100U) == 0);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_OK);
    REPAIR_RESULT(position_open(alice_margin->id, alice_margin, LX_PERPS_SIDE_BUY,
        1U, 100U, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(balance_is(alice_main, 10000U) && balance_is(alice_margin, 0U));
    REPAIR_CHECK(repair_position(bob_margin, LX_PERPS_SIDE_SELL, 1U, 100U, true));
    REPAIR_CHECK(bob_margin->has_open_reference && balance_is(bob_margin, 100U));
    REPAIR_RESULT(position_close(bob_margin, "did:key:bob"), LXP_OK);
    REPAIR_RESULT(position_open(alice_margin->id, alice_margin, LX_PERPS_SIDE_BUY,
        1U, 100U, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(repair_enable_tif() == 0);
    REPAIR_RESULT(repair_ioc(), LXP_OK);
    REPAIR_CHECK(ctx_open() == 0);
    REPAIR_RESULT(order_state(9U, &order), LXP_OK);
    REPAIR_CHECK(!order.active && lxp_u128_is_zero(order.remaining));
    REPAIR_CHECK(repair_reopen() == 0);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(balance_is(alice_main, 10000U) && balance_is(alice_margin, 0U));
    REPAIR_CHECK(!alice_margin->has_open_reference && repair_interest(0U, 0U));
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    (void)puts("unfilled-margin-release: unused, cancelled, IOC, reopened and durable reservations passed");
    return 0;
}

static int repair_funding_tick(bool negative, uint64_t intervals)
{
    lx_account *bid;
    lx_account *ask;
    lx_perps_tick_command command;
    uint8_t payload[LX_PERPS_TICK_PAYLOAD_BYTES];
    REPAIR_CHECK(account_add("agent:did:key:dave:main", LX_ACCOUNT_OPEN_CREDIT,
                             0U, NULL) == 0);
    REPAIR_CHECK(account_add("agent:did:key:dave:margin:a", LX_ACCOUNT_OPEN_CREDIT,
                             1000U, &bid) == 0);
    REPAIR_CHECK(account_add("agent:did:key:erin:main", LX_ACCOUNT_OPEN_CREDIT,
                             0U, NULL) == 0);
    REPAIR_CHECK(account_add("agent:did:key:erin:margin:a", LX_ACCOUNT_OPEN_CREDIT,
                             1000U, &ask) == 0);
    REPAIR_RESULT(order_place(20U, bid, LX_PERPS_SIDE_BUY,
        negative ? 8000U : 10000U, 1U, "did:key:dave"), LXP_OK);
    REPAIR_RESULT(order_place(21U, ask, LX_PERPS_SIDE_SELL,
        negative ? 10000U : 12000U, 1U, "did:key:erin"), LXP_OK);
    (void)memset(&command, 0, sizeof(command));
    command.market_id[0] = 1U;
    REPAIR_RESULT(lx_perps_tick_command_encode(&command, payload), LXP_OK);
    batch_timestamp += intervals * 100U;
    REPAIR_RESULT(run(LX_PERPS_FUNDING_TICK, admin_did, payload,
                       sizeof(payload), NULL, NULL), LXP_OK);
    return 0;
}

static int test_close_with_funding(void)
{
    uint64_t total;
    REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 500U) == 0);
    REPAIR_CHECK(repair_pair(10000U, 1U, 20U) == 0);
    REPAIR_CHECK(repair_funding_tick(false, 1U) == 0);
    total = repair_total();
    REPAIR_RESULT(oracle_push(oracle_seed, 1U, 2U, 11000U, batch_timestamp), LXP_OK);
    REPAIR_RESULT(position_close(alice_margin, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(balance_is(alice_main, 10000U) && balance_is(alice_margin, 0U));
    REPAIR_CHECK(balance_is(liquidity_account, 990U) && balance_is(long_pool_account, 100000U));
    REPAIR_RESULT(position_close(bob_margin, "did:key:bob"), LXP_OK);
    REPAIR_CHECK(balance_is(bob_main, 10000U) && balance_is(bob_margin, 0U));
    REPAIR_CHECK(balance_is(liquidity_account, 1000U) && balance_is(short_pool_account, 100000U));
    REPAIR_CHECK(repair_total() == total && repair_reopen() == 0);
    REPAIR_CHECK(balance_is(alice_main, 10000U) && balance_is(bob_main, 10000U));
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    (void)puts("close-trading-pnl: atomic trading gains/losses with both funding signs passed");
    return 0;
}

static int repair_funding_debt(const lx_account *pool, uint64_t expected)
{
    uint8_t key[77];
    const uint8_t *value;
    size_t length;
    lxp_u128 amount;
    (void)memset(key, 0, sizeof(key));
    (void)memcpy(key, "funding-debt:", 13U);
    key[13] = 1U;
    (void)memcpy(key + 45U, pool->id, 32U);
    REPAIR_CHECK(ctx_open() == 0);
    REPAIR_RESULT(lxp_ctx_kv_get(&ctx, key, sizeof(key), &value, &length), LXP_OK);
    REPAIR_CHECK(length == 16U);
    REPAIR_RESULT(lxp_u128_from_be(value, &amount), LXP_OK);
    REPAIR_CHECK(amount.hi == 0U && amount.lo == expected);
    return 0;
}

static int test_liquidation_funding(void)
{
    unsigned scenario;
    for (scenario = 0U; scenario < 4U; ++scenario) {
        lx_account *debtor;
        lx_account *pool;
        uint64_t total;
        REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 5U) == 0);
        REPAIR_CHECK(repair_pair(10000U, 1U, 10U) == 0);
        REPAIR_CHECK(repair_funding_tick((scenario % 2U) != 0U, 3U) == 0);
        if (scenario >= 2U)
            REPAIR_RESULT(oracle_push(oracle_seed, 1U, 2U,
                (scenario % 2U) == 0U ? 11000U : 9000U, batch_timestamp), LXP_OK);
        total = repair_total();
        debtor = (scenario % 2U) == 0U ? alice_margin : bob_margin;
        pool = (scenario % 2U) == 0U ? long_pool_account : short_pool_account;
        REPAIR_RESULT(repair_liquidate(debtor), LXP_OK);
        REPAIR_CHECK(balance_is(pool, scenario < 2U ? 99985U : 99995U) && balance_is(insurance_account, 0U));
        REPAIR_CHECK(balance_is(debtor, 0U) && !debtor->has_open_reference);
        REPAIR_CHECK(balance_is(liquidity_account, scenario < 2U ? 1000U : 990U) && repair_total() == total);
        REPAIR_CHECK(repair_funding_debt(pool, scenario < 2U ? 15U : 5U) == 0);
        REPAIR_CHECK(repair_reopen() == 0);
        debtor = (scenario % 2U) == 0U ? alice_margin : bob_margin;
        pool = (scenario % 2U) == 0U ? long_pool_account : short_pool_account;
        REPAIR_CHECK(repair_funding_debt(pool, scenario < 2U ? 15U : 5U) == 0);
        REPAIR_RESULT(repair_liquidate(debtor), LXP_ERR_AGREEMENT_STATE);
        REPAIR_CHECK(balance_is(pool, scenario < 2U ? 99985U : 99995U) && repair_total() == total);
        REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    }
    for (scenario = 0U; scenario < 2U; ++scenario) {
        lx_account *creditor;
        lx_account *pool;
        uint64_t total;
        REPAIR_CHECK(repair_setup(100U, 10000U, 1000U, 500U) == 0);
        REPAIR_CHECK(repair_pair(10000U, 1U, 10U) == 0);
        REPAIR_CHECK(repair_funding_tick(scenario != 0U, 1U) == 0);
        REPAIR_RESULT(oracle_push(oracle_seed, 1U, 2U,
            scenario == 0U ? 13000U : 7000U, batch_timestamp), LXP_OK);
        total = repair_total();
        creditor = scenario == 0U ? bob_margin : alice_margin;
        pool = scenario == 0U ? short_pool_account : long_pool_account;
        REPAIR_RESULT(repair_liquidate(creditor), LXP_OK);
        REPAIR_CHECK(balance_is(pool, 100000U));
        REPAIR_CHECK(balance_is(liquidity_account, 1030U) && balance_is(insurance_account, 490U));
        REPAIR_CHECK(balance_is(creditor, 0U) && repair_total() == total);
        REPAIR_CHECK(repair_reopen() == 0);
        creditor = scenario == 0U ? bob_margin : alice_margin;
        pool = scenario == 0U ? short_pool_account : long_pool_account;
        REPAIR_RESULT(repair_liquidate(creditor), LXP_ERR_AGREEMENT_STATE);
        REPAIR_CHECK(balance_is(pool, 100000U) && repair_total() == total);
        REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    }
    (void)puts("liquidation-funding: both tick signs, funding-only insolvency, credits, deficits and recovery passed");
    return 0;
}

#undef REPAIR_CHECK
#undef REPAIR_RESULT
#endif
