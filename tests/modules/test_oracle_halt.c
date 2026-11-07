#include "layerx/lx_oracle.h"
#include "layerx/lx_service.h"
#include "layerx/lxp_kernel.h"

#include <string.h>

static int encode_signed(lx_oracle_observation *compat_observation,
                         const uint8_t compat_seed[32], uint8_t compat_payload[72],
                         size_t *compat_payload_length)
{
    if (lx_oracle_observation_sign(compat_observation, compat_seed) != LXP_OK ||
        lx_oracle_observation_encode(compat_observation, compat_payload, 72U,
                                     compat_payload_length) != LXP_OK)
        return 1;
    return 0;
}

static int test_compatibility_oracle_halt(void)
{
    static const uint8_t compat_seed[32] = { 15U };
    lx_oracle_market_store compat_markets;
    lx_oracle_market *compat_market;
    lx_oracle_store compat_store;
    lx_oracle_observation compat_observation;
    lx_oracle_push_request compat_request;
    lx_oracle_accepted compat_accepted;
    lxp_state_store compat_state;
    lxp_state_journal compat_journal;
    lxp_kernel compat_kernel;
    lxp_module_ctx compat_ctx;
    lxp_arena compat_arena;
    uint8_t compat_arena_bytes[4096];
    uint8_t compat_payload[72];
    size_t compat_payload_length;
    uint64_t compat_parameters = 1U;

    (void)memset(&compat_markets, 0, sizeof(compat_markets));
    (void)memset(&compat_store, 0, sizeof(compat_store));
    (void)memset(&compat_observation, 0, sizeof(compat_observation));
    compat_markets.count = 1U;
    compat_market = &compat_markets.markets[0];
    compat_market->market_id[0] = 1U;
    compat_market->maximum_staleness = 100U;
    compat_market->minimum_price = (lxp_u128){ 0U, 50U };
    compat_market->maximum_price = (lxp_u128){ 0U, 200U };
    compat_market->maximum_deviation_basis_points = 1000U;
    compat_observation.market_id[0] = 1U;
    compat_observation.observation_sequence = 1U;
    compat_observation.price = (lxp_u128){ 0U, 100U };
    compat_observation.observed_at = 900U;
    compat_observation.source_identifier = 2U;
    if (encode_signed(&compat_observation, compat_seed, compat_payload, &compat_payload_length) != 0)
        return 1;
    compat_market->permitted_key_count = 1U;
    (void)memcpy(compat_market->permitted_keys[0],
                 compat_observation.oracle_public_key, 32U);
    if (lx_oracle_store_put(&compat_store, &compat_observation, compat_payload, compat_payload_length,
                            1U) != LXP_OK ||
        lx_oracle_fail_closed_eval(compat_market, &compat_store, 1001U) !=
            LXP_ERR_MARKET_HALTED || !lx_oracle_market_halted(compat_market))
        return 1;
    if (lx_oracle_market_action_check(compat_market, LX_ORACLE_ACTION_ORDER_PLACE) !=
            LXP_ERR_MARKET_HALTED ||
        lx_oracle_market_action_check(compat_market,
            LX_ORACLE_ACTION_POSITION_INCREASE) != LXP_ERR_MARKET_HALTED ||
        lx_oracle_market_action_check(compat_market, LX_ORACLE_ACTION_LIQUIDATE) !=
            LXP_ERR_MARKET_HALTED ||
        lx_oracle_market_action_check(compat_market, LX_ORACLE_ACTION_FUNDING_TICK) !=
            LXP_ERR_MARKET_HALTED ||
        lx_oracle_market_action_check(compat_market, LX_ORACLE_ACTION_ORDER_CANCEL) !=
            LXP_OK ||
        lx_oracle_market_action_check(compat_market, LX_ORACLE_ACTION_MARGIN_ADD) !=
            LXP_OK)
        return 1;
    if (lxp_state_store_init(&compat_state, 0U) != LXP_OK ||
        lxp_kernel_create(&compat_kernel, &compat_state, &compat_journal, &compat_parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(&compat_kernel, lx_service_module_iface()) != LXP_OK ||
        lxp_arena_init(&compat_arena, compat_arena_bytes, sizeof(compat_arena_bytes)) != LXP_OK ||
        lxp_module_ctx_init(&compat_ctx, &compat_kernel, LXP_MODULE_SERVICE, 1001U, 0U, 2U,
                            1000U, &compat_arena, true) != LXP_OK)
        return 1;
    compat_observation.observation_sequence = 2U;
    compat_observation.observed_at = 1001U;
    compat_observation.price = (lxp_u128){ 0U, 105U };
    if (encode_signed(&compat_observation, compat_seed, compat_payload, &compat_payload_length) != 0)
        return 1;
    (void)memset(&compat_request, 0, sizeof(compat_request));
    compat_request.store = &compat_store;
    compat_request.markets = &compat_markets;
    compat_request.payload = compat_payload;
    compat_request.payload_length = compat_payload_length;
    compat_request.oracle_public_key = compat_observation.oracle_public_key;
    compat_request.signature = compat_observation.signature;
    if (lx_oracle_push_execute(&compat_ctx, &compat_request, &compat_accepted) != LXP_OK ||
        lx_oracle_market_halted(compat_market) ||
        lx_oracle_market_action_check(compat_market,
            LX_ORACLE_ACTION_ORDER_PLACE) != LXP_OK ||
        compat_accepted.observation.observation_sequence != 2U)
        return 1;
    compat_observation.observation_sequence = 3U;
    compat_observation.observed_at = 800U;
    if (encode_signed(&compat_observation, compat_seed, compat_payload, &compat_payload_length) != 0)
        return 1;
    compat_request.payload = compat_payload;
    compat_request.signature = compat_observation.signature;
    if (lx_oracle_push_execute(&compat_ctx, &compat_request, &compat_accepted) !=
            LXP_ERR_ORACLE_STALE || !lx_oracle_market_halted(compat_market) ||
        compat_store.count != 2U ||
        lxp_state_store_destroy(&compat_state) != LXP_OK)
        return 1;
    return 0;
}

/* Reuse the registered perps harness, including its snapshot loader and real
 * signed oracle transport, without running the unrelated position suite. */
#define main perps_position_harness_main
#include "test_perps_position.c"
#undef main

#define HALT_CHECK(expression) do { \
    if (!(expression)) { \
        (void)fprintf(stderr, "oracle halt: line %d: %s\n", __LINE__, #expression); \
        return 1; \
    } \
} while (0)
#define HALT_RESULT(expression, expected) do { \
    lxp_result halt_result = (expression); \
    if (halt_result != (expected)) { \
        (void)fprintf(stderr, "oracle halt: line %d: %s = %d; expected %d\n", \
            __LINE__, #expression, (int)halt_result, (int)(expected)); \
        return 1; \
    } \
} while (0)

static int test_oracle_halt_committed_recovery(void)
{
    uint8_t before[32], after[32];
    uint8_t position_before[LX_PERPS_POSITION_BYTES];
    uint8_t position_after[LX_PERPS_POSITION_BYTES];
    uint8_t funding_before[LX_PERPS_FUNDING_BYTES];
    uint8_t funding_after[LX_PERPS_FUNDING_BYTES];
    lx_perps_order pending;
    uint64_t total;
    HALT_CHECK(repair_setup(1U, 100U, 1000U, 500U) == 0);
    HALT_CHECK(repair_pair(100U, 1U, 100U) == 0);
    HALT_RESULT(order_place(3U, bob_margin, LX_PERPS_SIDE_SELL, 110U,
                              1U, "did:key:bob"), LXP_OK);
    HALT_CHECK(repair_enable_tif() == 0);
    batch_timestamp += 100001U;
    HALT_CHECK(repair_epoch() == 0 && repair_health(true, 1U, 100U) == 0);
    HALT_RESULT(lxp_state_root(&kernel, before), LXP_OK);
    HALT_RESULT(repair_transport_push(2U, 110U, batch_timestamp, true), LXP_ERR_UNAUTHORIZED_ORACLE);
    HALT_RESULT(repair_transport_push(2U, 110U, 1000U, false), LXP_ERR_ORACLE_STALE);
    HALT_RESULT(repair_transport_push(1U, 110U, batch_timestamp, false), LXP_ERR_ORACLE_SEQUENCE);
    HALT_RESULT(repair_transport_push(2U, 1000001U, batch_timestamp, false), LXP_ERR_ORACLE_BOUNDS);
    HALT_RESULT(repair_transport_push(2U, 300U, batch_timestamp, false), LXP_ERR_ORACLE_DEVIATION);
    HALT_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    HALT_CHECK(memcmp(before, after, 32U) == 0);
    HALT_CHECK(repair_reopen() == 0 && repair_health(true, 1U, 100U) == 0);
    HALT_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    HALT_CHECK(memcmp(before, after, 32U) == 0);
    HALT_RESULT(position_increase(alice_margin, 1U, 0U, 7U,
                                   "did:key:alice"), LXP_ERR_MARKET_HALTED);
    HALT_RESULT(order_place(4U, alice_margin, LX_PERPS_SIDE_BUY, 100U,
                              1U, "did:key:alice"), LXP_ERR_MARKET_HALTED);
    HALT_RESULT(position_open(carol_margin->id, carol_margin,
        LX_PERPS_SIDE_BUY, 1U, 100U, "did:key:carol"), LXP_ERR_MARKET_HALTED);
    HALT_RESULT(lxp_state_root(&kernel, after), LXP_OK);
    HALT_CHECK(memcmp(before, after, 32U) == 0);
    HALT_CHECK(repair_exposure_records(position_before, funding_before) == 0);
    total = repair_total();
    uint64_t sequence = alice_main->next_sequence;
    HALT_RESULT(position_increase(alice_margin, 0U, 0U, 7U,
                                   "did:key:alice"), LXP_OK);
    HALT_CHECK(balance_is(alice_main, 9893U) && balance_is(alice_margin, 107U) &&
        alice_main->next_sequence == sequence + 1U && repair_total() == total);
    HALT_CHECK(repair_exposure_records(position_after, funding_after) == 0);
    HALT_CHECK(memcmp(position_before, position_after, sizeof(position_before)) == 0 &&
        memcmp(funding_before, funding_after, sizeof(funding_before)) == 0);
    HALT_RESULT(order_state(3U, &pending), LXP_OK);
    HALT_CHECK(pending.active);
    HALT_RESULT(repair_cancel(3U, "did:key:alice"), LXP_ERR_UNAUTHORIZED_DEBIT);
    HALT_RESULT(repair_cancel(3U, "did:key:bob"), LXP_OK);
    HALT_CHECK(ctx_open() == 0);
    HALT_RESULT(order_state(3U, &pending), LXP_ERR_UNKNOWN_FIELD);
    HALT_CHECK(repair_reopen() == 0 && repair_health(true, 1U, 100U) == 0);
    HALT_CHECK(ctx_open() == 0);
    HALT_RESULT(order_state(3U, &pending), LXP_ERR_UNKNOWN_FIELD);
    HALT_CHECK(balance_is(alice_main, 9893U) && balance_is(alice_margin, 107U) &&
        alice_main->next_sequence == sequence + 1U && repair_total() == total);
    HALT_RESULT(repair_transport_push(2U, 110U, batch_timestamp, false), LXP_OK);
    HALT_CHECK(repair_health(false, 2U, 110U) == 0);
    HALT_RESULT(position_increase(alice_margin, 1U, 0U, 7U,
                                   "did:key:alice"), LXP_OK);
    HALT_CHECK(repair_total() == total);
    HALT_CHECK(repair_reopen() == 0 && repair_health(false, 2U, 110U) == 0);
    HALT_CHECK(ctx_open() == 0);
    HALT_RESULT(order_state(3U, &pending), LXP_ERR_UNKNOWN_FIELD);
    HALT_RESULT(lxp_state_store_destroy(&store), LXP_OK);
    (void)puts("oracle-halt-test-seam: committed halt, protective actions and durable recovery passed");
    return 0;
}

int main(void)
{
    return test_compatibility_oracle_halt() || test_oracle_halt_committed_recovery();
}
