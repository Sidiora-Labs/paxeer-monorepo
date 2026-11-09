//go:build ignore

#include "lxp_test_harness.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_receipt.h"

#include <string.h>

#include "repair_22_33_18_receipts.h"

int market_receipt_codec(const uint8_t input[97], lxp_arena *arena,
                         lxp_byte_span *encoded);

static uint8_t arena_bytes[8U * LXP_MAX_ACTIVITY_BYTES];
static lxp_receipt receipt;

static int market_producer_fixture_codec(void)
{
    uint8_t input[97] = {0};
    lxp_arena arena;
    lxp_byte_span encoded;
    input[0] = 1U;
    input[32] = 2U;
    input[52] = 3U;
    input[79] = 7U;
    input[88] = 9U;
    input[96] = 1U;
    if (LXP_ASSERT_RESULT(LXP_OK, lxp_arena_init(&arena, arena_bytes,
                                               sizeof(arena_bytes))) != 0)
        return 1;
    if (LXP_ASSERT_U64(0U, (uint64_t)market_receipt_codec(input, &arena,
                                                       &encoded)) != 0)
        return 1;
    if (LXP_ASSERT_RESULT(LXP_OK, lxp_receipt_decode(encoded.bytes,
                              encoded.length, true, &receipt)) != 0)
        return 1;
    if (LXP_ASSERT_U64(7U, receipt.from_sequence) != 0 ||
        LXP_ASSERT_U64(9U, receipt.batch_id[31]) != 0 ||
        LXP_ASSERT_U64(12U, receipt.module_id) != 0 ||
        LXP_ASSERT_U64(2U, receipt.operation) != 0 ||
        LXP_ASSERT_U64(0U, receipt.effects.count) != 0)
        return 1;
    input[80] = 2U;
    return LXP_ASSERT_U64(1U, (uint64_t)market_receipt_codec(input, &arena,
                                                          &encoded));
}

static int market_node_summary_event_return_binding(void)
{
    lxp_arena arena;
    uint8_t digest[32];
    if (lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK ||
        lxp_receipt_decode(market_node_receipt, market_node_receipt_length,
                           true, &receipt) != LXP_OK ||
        lxp_receipt_verify(&receipt, market_node_key, &arena) != LXP_OK ||
        lxp_hash_sha256(market_node_return_data, market_node_return_data_length,
                        digest) != LXP_OK)
        return 1;
    if (LXP_ASSERT_BYTES(digest, receipt.activity_root, 32U) != 0 ||
        LXP_ASSERT_U64(2U, receipt.effects.count) != 0 ||
        LXP_ASSERT_U64(LXP_EFFECT_STATE, receipt.effects.effects[0].kind) != 0 ||
        LXP_ASSERT_U64(LXP_EFFECT_EVENT, receipt.effects.effects[1].kind) != 0)
        return 1;
    receipt.effects.effects[1].body[0] ^= 1U;
    return LXP_ASSERT_RESULT(LXP_ERR_BAD_SIGNATURE,
                             lxp_receipt_verify(&receipt, market_node_key,
                                                 &arena));
}

static int payment_node_summary_and_ledger_event(void)
{
    lxp_arena arena;
    uint8_t digest[32];
    if (lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK ||
        lxp_receipt_decode(payment_node_receipt, payment_node_receipt_length,
                           true, &receipt) != LXP_OK ||
        lxp_receipt_verify(&receipt, payment_node_key, &arena) != LXP_OK ||
        lxp_hash_sha256(payment_node_return_data, payment_node_return_data_length,
                        digest) != LXP_OK)
        return 1;
    if (LXP_ASSERT_BYTES(digest, receipt.activity_root, 32U) != 0 ||
        LXP_ASSERT_U64(2U, receipt.effects.count) != 0 ||
        LXP_ASSERT_U64(LXP_EFFECT_EVENT, receipt.effects.effects[1].kind) != 0 ||
        LXP_ASSERT_U64(0x10U, receipt.effects.effects[1].body[18]) != 0 ||
        LXP_ASSERT_U64(0x1aU, receipt.effects.effects[1].body[19]) != 0)
        return 1;
    receipt.context_hash[0] ^= 1U;
    return LXP_ASSERT_RESULT(LXP_ERR_BAD_SIGNATURE,
                             lxp_receipt_verify(&receipt, payment_node_key,
                                                 &arena));
}

int main(int argc, char **argv)
{
    if (lxp_test_register("market-producer-fixture.codec",
                          market_producer_fixture_codec) != 0 ||
        lxp_test_register("market-producer-fixture.node-output",
                          market_node_summary_event_return_binding) != 0 ||
        lxp_test_register("web-payment-test-fixture.ledger-event",
                          payment_node_summary_and_ledger_event) != 0)
        return 1;
    return lxp_test_main(argc, argv);
}
