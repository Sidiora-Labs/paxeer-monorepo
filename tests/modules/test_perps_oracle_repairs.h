#ifndef TEST_PERPS_ORACLE_REPAIRS_H
#define TEST_PERPS_ORACLE_REPAIRS_H

#include "../../src/modules/perps/lx_perps_internal.h"

static lxp_result repair_halt(bool halted)
{
    lx_perps_halt_command command;
    uint8_t payload[LX_PERPS_HALT_PAYLOAD_BYTES];
    (void)memset(&command, 0, sizeof(command));
    command.market_id[0] = 1U;
    command.halted = halted;
    lxp_result status = lx_perps_halt_command_encode(&command, payload);
    return status == LXP_OK ? run(LX_PERPS_MARKET_HALT, admin_did,
        payload, sizeof(payload), NULL, NULL) : status;
}

static lxp_result repair_transport_push(uint64_t sequence, uint64_t price,
    uint64_t timestamp, bool corrupt_signature)
{
    lx_perps_oracle_command command;
    uint8_t payload[LX_ORACLE_TRANSPORT_BYTES];
    (void)memset(&command, 0, sizeof(command));
    command.market_id[0] = 1U;
    command.observation_sequence = sequence;
    command.price = (lxp_u128){ 0U, price };
    command.observed_at = timestamp;
    command.source_identifier = 5U;
    lxp_result status = lx_perps_oracle_command_sign(&command, oracle_seed);
    if (status == LXP_OK) status = lx_perps_oracle_transport_encode(&command, payload);
    if (status != LXP_OK) return status;
    if (corrupt_signature) payload[73] ^= 1U;
    return run(LX_PERPS_ORACLE_PUSH, admin_did, payload, sizeof(payload),
        command.oracle_public_key, command.signature);
}

static int repair_epoch(void)
{
    while (store.next_sequence < global_sequence) {
        REPAIR_RESULT(lxp_state_journal_open(&store, store.next_sequence, &journal), LXP_OK);
        REPAIR_RESULT(lxp_state_journal_commit(&journal), LXP_OK);
    }
    REPAIR_RESULT(lxp_arena_init(&arena, arena_storage.bytes,
        sizeof(arena_storage.bytes)), LXP_OK);
    REPAIR_RESULT(lxp_kernel_epoch_transition(&kernel, kernel.epoch + 1U,
        batch_timestamp, &arena), LXP_OK);
    global_sequence = store.next_sequence;
    return 0;
}

static int repair_health(bool expected, uint64_t sequence, uint64_t price)
{
    uint8_t market_id[32] = { 1U };
    uint8_t key[44];
    const uint8_t *value;
    size_t length;
    lx_perps_oracle_state state;
    lx_oracle_committed committed;
    bool halted;
    REPAIR_CHECK(ctx_open() == 0);
    REPAIR_RESULT(lx_perps_oracle_halt_read(&ctx, market_id, &halted), LXP_OK);
    REPAIR_CHECK(halted == expected);
    (void)memcpy(key, "oracle-halt:", 12U);
    (void)memcpy(key + 12U, market_id, 32U);
    REPAIR_RESULT(lxp_ctx_kv_get(&ctx, key, sizeof(key), &value, &length), LXP_OK);
    REPAIR_CHECK(length == 1U && value[0] == (expected ? 1U : 0U));
    REPAIR_RESULT(lx_perps_oracle_state_lookup(&ctx, market_id, &state), LXP_OK);
    REPAIR_CHECK(state.observation_sequence == sequence && state.price.hi == 0U &&
        state.price.lo == price && state.source_identifier == 5U &&
        memcmp(state.oracle_public_key, oracle_key, 32U) == 0);
    REPAIR_RESULT(lx_oracle_committed_read(&ctx, market_id, &committed),
        expected ? LXP_ERR_MARKET_HALTED : LXP_OK);
    if (!expected) {
        REPAIR_CHECK(committed.observation_sequence == sequence &&
            committed.price.hi == 0U && committed.price.lo == price &&
            committed.observed_at == state.observed_at);
    }
    /* The production cross-module consumer sees the same committed policy
     * and restores its caller's context even on refusal. */
    ctx.module_id = LXP_MODULE_PROGRAMS;
    REPAIR_RESULT(lx_oracle_committed_read(&ctx, market_id, &committed),
        expected ? LXP_ERR_MARKET_HALTED : LXP_OK);
    REPAIR_CHECK(ctx.module_id == LXP_MODULE_PROGRAMS && ctx.staged_count == 0U);
    return 0;
}

static int repair_exposure_records(uint8_t position_bytes[LX_PERPS_POSITION_BYTES],
    uint8_t funding_bytes[LX_PERPS_FUNDING_BYTES])
{
    uint8_t position_key[LX_PERPS_POSITION_KEY_BYTES] = { 0U };
    uint8_t funding_key[LX_PERPS_FUNDING_KEY_BYTES] = { 0U };
    const uint8_t *value;
    size_t length;
    lx_perps_position position;
    lx_perps_funding_state funding;
    uint8_t market_id[32] = { 1U };
    REPAIR_CHECK(ctx_open() == 0);
    REPAIR_RESULT(position_state(alice_margin, &position), LXP_OK);
    REPAIR_RESULT(lx_perps_funding_state_lookup(&ctx, market_id, &funding), LXP_OK);
    (void)memcpy(position_key, "position:", 9U);
    (void)memcpy(position_key + 9U, market_id, 32U);
    (void)memcpy(position_key + 41U, alice_margin->id, 32U);
    REPAIR_RESULT(lxp_ctx_kv_get(&ctx, position_key, sizeof(position_key), &value, &length), LXP_OK);
    REPAIR_CHECK(length == LX_PERPS_POSITION_BYTES);
    (void)memcpy(position_bytes, value, length);
    (void)memcpy(funding_key, "funding:", 8U);
    (void)memcpy(funding_key + 8U, market_id, 32U);
    REPAIR_RESULT(lxp_ctx_kv_get(&ctx, funding_key, sizeof(funding_key), &value, &length), LXP_OK);
    REPAIR_CHECK(length == LX_PERPS_FUNDING_BYTES);
    (void)memcpy(funding_bytes, value, length);
    return 0;
}

static int repair_topup_codec(void)
{
    lx_perps_increase_command command;
    lx_perps_increase_command decoded;
    uint8_t payload[LX_PERPS_INCREASE_PAYLOAD_BYTES];
    (void)memset(&command, 0, sizeof(command));
    command.market_id[0] = 1U;
    (void)memcpy(command.position_id, alice_margin->id, 32U);
    command.margin_amount = (lxp_u128){ 0U, 1U };
    REPAIR_RESULT(lx_perps_increase_command_encode(&command, payload), LXP_OK);
    REPAIR_RESULT(lx_perps_increase_command_decode(payload, sizeof(payload), &decoded), LXP_OK);
    REPAIR_CHECK(lxp_u128_is_zero(decoded.size_delta) &&
        lxp_u128_is_zero(decoded.notional_delta) && decoded.margin_amount.lo == 1U);
    command.margin_amount.lo = 0U;
    REPAIR_RESULT(lx_perps_increase_command_encode(&command, payload), LXP_ERR_NON_CANONICAL);
    command.margin_amount.lo = 1U;
    command.notional_delta.lo = 1U;
    REPAIR_RESULT(lx_perps_increase_command_encode(&command, payload), LXP_ERR_NON_CANONICAL);
    return 0;
}

static int test_halted_margin_policy(void)
{
    unsigned scenario;
    for (scenario = 0U; scenario < 3U; ++scenario) {
        lx_perps_order order;
        uint8_t position_before[LX_PERPS_POSITION_BYTES];
        uint8_t position_after[LX_PERPS_POSITION_BYTES];
        uint8_t funding_before[LX_PERPS_FUNDING_BYTES];
        uint8_t funding_after[LX_PERPS_FUNDING_BYTES];
        uint8_t root_before[32];
        uint8_t root_after[32];
        uint64_t total;
        uint64_t sequence;
        lxp_result exposure_error = scenario == 1U ? LXP_ERR_ORACLE_STALE : LXP_ERR_MARKET_HALTED;
        REPAIR_CHECK(repair_setup(1U, 100U, 1000U, 500U) == 0);
        REPAIR_CHECK(repair_pair(100U, 1U, 100U) == 0);
        REPAIR_CHECK(repair_topup_codec() == 0);
        REPAIR_RESULT(order_place(3U, bob_margin, LX_PERPS_SIDE_SELL, 110U,
            1U, "did:key:bob"), LXP_OK);
        REPAIR_CHECK(ctx_open() == 0);
        REPAIR_RESULT(order_state(3U, &order), LXP_OK);
        REPAIR_CHECK(order.active && order.remaining.hi == 0U && order.remaining.lo == 1U);
        REPAIR_CHECK(repair_enable_tif() == 0);
        if (scenario == 0U) REPAIR_RESULT(repair_halt(true), LXP_OK);
        else {
            batch_timestamp += 100001U;
            if (scenario == 2U) REPAIR_CHECK(repair_epoch() == 0);
        }
        REPAIR_CHECK(repair_exposure_records(position_before, funding_before) == 0);
        total = repair_total();
        sequence = alice_main->next_sequence;
        REPAIR_RESULT(position_increase(alice_margin, 1U, 0U, 7U, "did:key:alice"), exposure_error);
        REPAIR_RESULT(order_place(4U, alice_margin, LX_PERPS_SIDE_BUY, 100U,
            1U, "did:key:alice"), exposure_error);
        REPAIR_RESULT(position_open(carol_margin->id, carol_margin,
            LX_PERPS_SIDE_BUY, 1U, 100U, "did:key:carol"), exposure_error);
        REPAIR_RESULT(lxp_state_root(&kernel, root_before), LXP_OK);
        REPAIR_RESULT(position_increase(alice_margin, 0U, 0U, 7U, "did:key:bob"),
            LXP_ERR_UNAUTHORIZED_DEBIT);
        REPAIR_RESULT(position_increase(alice_margin, 0U, 0U, 20000U, "did:key:alice"),
            LXP_ERR_INSUFFICIENT_BALANCE);
        REPAIR_RESULT(lxp_state_root(&kernel, root_after), LXP_OK);
        REPAIR_CHECK(memcmp(root_before, root_after, 32U) == 0);
        REPAIR_RESULT(position_increase(alice_margin, 0U, 0U, 7U, "did:key:alice"), LXP_OK);
        REPAIR_CHECK(balance_is(alice_main, 9893U) && balance_is(alice_margin, 107U));
        REPAIR_CHECK(alice_main->next_sequence == sequence + 1U && repair_total() == total);
        REPAIR_CHECK(repair_exposure_records(position_after, funding_after) == 0);
        REPAIR_CHECK(memcmp(position_before, position_after, sizeof(position_before)) == 0 &&
            memcmp(funding_before, funding_after, sizeof(funding_before)) == 0);
        REPAIR_CHECK(ctx_open() == 0);
        REPAIR_RESULT(order_state(3U, &order), LXP_OK);
        REPAIR_CHECK(order.active && order.remaining.hi == 0U && order.remaining.lo == 1U);
        REPAIR_RESULT(repair_cancel(3U, "did:key:alice"), LXP_ERR_UNAUTHORIZED_DEBIT);
        REPAIR_RESULT(repair_cancel(3U, "did:key:bob"), LXP_OK);
        REPAIR_CHECK(ctx_open() == 0);
        REPAIR_RESULT(order_state(3U, &order), LXP_ERR_UNKNOWN_FIELD);
        REPAIR_RESULT(repair_cancel(3U, "did:key:bob"), LXP_ERR_UNKNOWN_FIELD);
        REPAIR_CHECK(repair_reopen() == 0);
        REPAIR_RESULT(order_state(3U, &order), LXP_ERR_UNKNOWN_FIELD);
        REPAIR_CHECK(balance_is(alice_main, 9893U) && balance_is(alice_margin, 107U));
        REPAIR_CHECK(repair_position(alice_margin, LX_PERPS_SIDE_BUY, 1U, 100U, true));
        REPAIR_RESULT(position_increase(alice_margin, 1U, 0U, 7U, "did:key:alice"), exposure_error);
        REPAIR_RESULT(lxp_state_root(&kernel, root_before), LXP_OK);
        fixture_full_effects = true;
        REPAIR_RESULT(position_increase(alice_margin, 0U, 0U, 7U, "did:key:alice"), LXP_ERR_LENGTH_LIMIT);
        fixture_full_effects = false;
        REPAIR_RESULT(lxp_state_root(&kernel, root_after), LXP_OK);
        REPAIR_CHECK(memcmp(root_before, root_after, 32U) == 0 && repair_total() == total);
        REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    }
    (void)puts("halted-margin-policy: registered top-up, authorization, cancellation, exposure refusal and recovery passed");
    return 0;
}

static int test_parallel_oracle_state(void)
{
    uint8_t market_id[32] = { 1U };
    uint8_t before[32];
    uint8_t after[32];
    lx_oracle_committed committed;
    REPAIR_CHECK(repair_setup(1U, 100U, 1000U, 500U) == 0);
    REPAIR_CHECK(repair_pair(100U, 1U, 100U) == 0);
    REPAIR_CHECK(repair_enable_tif() == 0);
    REPAIR_CHECK(repair_health(false, 1U, 100U) == 0);
    batch_timestamp += 100000U;
    REPAIR_CHECK(repair_epoch() == 0);
    REPAIR_CHECK(repair_health(false, 1U, 100U) == 0);
    ++batch_timestamp;
    REPAIR_CHECK(repair_epoch() == 0);
    REPAIR_CHECK(repair_health(true, 1U, 100U) == 0);
    REPAIR_RESULT(position_increase(alice_margin, 1U, 0U, 7U, "did:key:alice"), LXP_ERR_MARKET_HALTED);
    REPAIR_CHECK(repair_reopen() == 0 && repair_health(true, 1U, 100U) == 0);
    REPAIR_RESULT(repair_halt(true), LXP_OK);
    REPAIR_RESULT(repair_halt(false), LXP_OK);
    REPAIR_CHECK(repair_health(true, 1U, 100U) == 0);
    REPAIR_RESULT(lxp_state_root(&kernel, before), LXP_OK);
    REPAIR_RESULT(repair_transport_push(2U, 110U, batch_timestamp, true), LXP_ERR_UNAUTHORIZED_ORACLE);
    REPAIR_RESULT(repair_transport_push(2U, 110U, 1000U, false), LXP_ERR_ORACLE_STALE);
    REPAIR_RESULT(repair_transport_push(1U, 110U, batch_timestamp, false), LXP_ERR_ORACLE_SEQUENCE);
    REPAIR_RESULT(repair_transport_push(2U, 1000001U, batch_timestamp, false), LXP_ERR_ORACLE_BOUNDS);
    REPAIR_RESULT(repair_transport_push(2U, 300U, batch_timestamp, false), LXP_ERR_ORACLE_DEVIATION);
    REPAIR_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    REPAIR_CHECK(memcmp(before, after, 32U) == 0 && repair_health(true, 1U, 100U) == 0);
    fixture_full_effects = true;
    REPAIR_RESULT(repair_transport_push(2U, 110U, batch_timestamp, false), LXP_ERR_LENGTH_LIMIT);
    fixture_full_effects = false;
    REPAIR_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    REPAIR_CHECK(memcmp(before, after, 32U) == 0 && repair_health(true, 1U, 100U) == 0);
    REPAIR_RESULT(repair_transport_push(2U, 110U, batch_timestamp, false), LXP_OK);
    REPAIR_CHECK(repair_health(false, 2U, 110U) == 0);
    REPAIR_RESULT(position_increase(alice_margin, 1U, 0U, 7U, "did:key:alice"), LXP_OK);
    REPAIR_CHECK(repair_reopen() == 0 && repair_health(false, 2U, 110U) == 0);
    REPAIR_RESULT(repair_halt(true), LXP_OK);
    REPAIR_RESULT(repair_transport_push(3U, 120U, batch_timestamp, false), LXP_OK);
    REPAIR_RESULT(position_increase(alice_margin, 1U, 0U, 7U, "did:key:alice"), LXP_ERR_MARKET_HALTED);
    REPAIR_CHECK(repair_reopen() == 0 && ctx_open() == 0);
    REPAIR_RESULT(lx_oracle_committed_read(&ctx, market_id, &committed), LXP_ERR_MARKET_HALTED);
    REPAIR_RESULT(repair_halt(false), LXP_OK);
    REPAIR_CHECK(repair_health(false, 3U, 120U) == 0);
    REPAIR_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    (void)puts("parallel-oracle-state: signed transport, committed halt, atomic fresh recovery and snapshot restoration passed");
    return 0;
}

#endif
