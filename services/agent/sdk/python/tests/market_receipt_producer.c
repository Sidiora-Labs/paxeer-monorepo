#include "layerx/lxp_receipt.h"

#include <stdio.h>
#include <string.h>

static uint64_t read_u64(const uint8_t *bytes)
{
    uint64_t value = 0U;
    size_t index;
    for (index = 0U; index < 8U; ++index)
        value = (value << 8U) | bytes[index];
    return value;
}

int market_receipt_codec(const uint8_t input[97], lxp_arena *arena,
                         lxp_byte_span *encoded);

int market_receipt_codec(const uint8_t input[97], lxp_arena *arena,
                         lxp_byte_span *encoded)
{
    uint8_t previous[32] = {0};
    uint8_t resulting[32];
    uint8_t activity_root[32];
    uint8_t batch[32] = {0};
    uint8_t seed[32];
    lxp_receipt receipt = {0};
    lxp_effect_buffer effects = {0};
    if (input == NULL || arena == NULL || encoded == NULL || input[80] > 1U)
        return 1;
    (void)memset(resulting, 0x44, sizeof(resulting));
    (void)memset(activity_root, 0x55, sizeof(activity_root));
    (void)memset(seed, 0x33, sizeof(seed));
    (void)memcpy(batch + 24U, input + 81U, 8U);
    receipt.protocol_version = 2U;
    if (lxp_receipt_build(&receipt, input, read_u64(input + 89U), previous,
                          resulting, activity_root, (lxp_result)input[80],
                          &effects, (lxp_u128){0U, 0U}, batch, 12U, 1U, 0U) != LXP_OK)
        return 2;
    receipt.operation = 2U;
    (void)memcpy(receipt.from + 12U, input + 32U, 20U);
    (void)memcpy(receipt.to + 12U, input + 52U, 20U);
    receipt.from_sequence = read_u64(input + 72U);
    receipt.timestamp = 1700000000000ULL;
    (void)memset(receipt.context_hash, 0x66, sizeof(receipt.context_hash));
    if (lxp_receipt_sign(&receipt, seed, arena) != LXP_OK ||
        lxp_receipt_encode(&receipt, true, arena, encoded) != LXP_OK)
        return 3;
    return 0;
}

#ifndef MARKET_RECEIPT_CODEC_LIBRARY
int main(void)
{
    static uint8_t arena_bytes[4U * LXP_MAX_ACTIVITY_BYTES];
    uint8_t input[97];
    lxp_arena arena;
    lxp_byte_span encoded;
    int result;
    if (fread(input, 1U, sizeof(input), stdin) != sizeof(input) ||
        getchar() != EOF || input[80] > 1U)
        return 1;
    if (lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK)
        return 2;
    result = market_receipt_codec(input, &arena, &encoded);
    if (result != 0)
        return result;
    if (fwrite(encoded.bytes, 1U, encoded.length, stdout) != encoded.length)
        return 4;
    return 0;
}
#endif
