#include "layerx/lxp_crypto.h"
#include "../codec/lxp_kernel_case.h"

#include <stdint.h>
#include <string.h>

static uint64_t reference_rotate(uint64_t value, unsigned shift)
{
    if (shift == 0U) return value;
    return (value << shift) | (value >> (64U - shift));
}

static void reference_keccak_f(uint64_t state[25])
{
    static const uint64_t round_constants[24] = {
        UINT64_C(0x0000000000000001), UINT64_C(0x0000000000008082),
        UINT64_C(0x800000000000808a), UINT64_C(0x8000000080008000),
        UINT64_C(0x000000000000808b), UINT64_C(0x0000000080000001),
        UINT64_C(0x8000000080008081), UINT64_C(0x8000000000008009),
        UINT64_C(0x000000000000008a), UINT64_C(0x0000000000000088),
        UINT64_C(0x0000000080008009), UINT64_C(0x000000008000000a),
        UINT64_C(0x000000008000808b), UINT64_C(0x800000000000008b),
        UINT64_C(0x8000000000008089), UINT64_C(0x8000000000008003),
        UINT64_C(0x8000000000008002), UINT64_C(0x8000000000000080),
        UINT64_C(0x000000000000800a), UINT64_C(0x800000008000000a),
        UINT64_C(0x8000000080008081), UINT64_C(0x8000000000008080),
        UINT64_C(0x0000000080000001), UINT64_C(0x8000000080008008)
    };
    static const unsigned rotations[25] = {
         0U,  1U, 62U, 28U, 27U,
        36U, 44U,  6U, 55U, 20U,
         3U, 10U, 43U, 25U, 39U,
        41U, 45U, 15U, 21U,  8U,
        18U,  2U, 61U, 56U, 14U
    };
    size_t round;
    size_t x;
    size_t y;

    for (round = 0U; round < 24U; ++round) {
        uint64_t columns[5];
        uint64_t rotated[25];

        for (x = 0U; x < 5U; ++x)
            columns[x] = state[x] ^ state[x + 5U] ^ state[x + 10U] ^
                         state[x + 15U] ^ state[x + 20U];
        for (x = 0U; x < 5U; ++x) {
            uint64_t delta = columns[(x + 4U) % 5U] ^
                             reference_rotate(columns[(x + 1U) % 5U], 1U);
            for (y = 0U; y < 5U; ++y) state[x + 5U * y] ^= delta;
        }
        for (x = 0U; x < 5U; ++x)
            for (y = 0U; y < 5U; ++y)
                rotated[y + 5U * ((2U * x + 3U * y) % 5U)] =
                    reference_rotate(state[x + 5U * y], rotations[x + 5U * y]);
        for (x = 0U; x < 5U; ++x)
            for (y = 0U; y < 5U; ++y)
                state[x + 5U * y] = rotated[x + 5U * y] ^
                    ((~rotated[(x + 1U) % 5U + 5U * y]) &
                     rotated[(x + 2U) % 5U + 5U * y]);
        state[0] ^= round_constants[round];
    }
}

static void reference_keccak256(const uint8_t *data, size_t length,
                                uint8_t out[32])
{
    const size_t rate = 136U;
    uint64_t state[25] = {0U};
    size_t offset = 0U;
    size_t i;

    for (i = 0U; i < length; ++i) {
        state[offset / 8U] ^= (uint64_t)data[i] << (8U * (offset % 8U));
        if (++offset == rate) {
            reference_keccak_f(state);
            offset = 0U;
        }
    }
    /* Ethereum Keccak uses 0x01, rather than SHA3's 0x06 suffix. */
    state[offset / 8U] ^= UINT64_C(0x01) << (8U * (offset % 8U));
    state[(rate - 1U) / 8U] ^= UINT64_C(0x80) << 56U;
    reference_keccak_f(state);
    for (i = 0U; i < 32U; ++i)
        out[i] = (uint8_t)(state[i / 8U] >> (8U * (i % 8U)));
}

static int digest_matches_hex(const uint8_t digest[32], const char *expected)
{
    static const char digits[] = "0123456789abcdef";
    char hex[65];
    size_t i;

    for (i = 0U; i < 32U; ++i) {
        hex[2U * i] = digits[digest[i] >> 4U];
        hex[2U * i + 1U] = digits[digest[i] & 0x0fU];
    }
    hex[64] = '\0';
    return strcmp(hex, expected) == 0;
}

static int keccak_known_answers(void)
{
    static const uint8_t abc[] = {0x61U, 0x62U, 0x63U};
    static const char empty_digest[] =
        "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470";
    static const char abc_digest[] =
        "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45";
    uint8_t produced[32];
    uint8_t reference[32];

    LXP_CASE_CHECK(lxp_keccak256(NULL, 0U, produced) == LXP_OK);
    LXP_CASE_CHECK(digest_matches_hex(produced, empty_digest));
    reference_keccak256(NULL, 0U, reference);
    LXP_CASE_CHECK(digest_matches_hex(reference, empty_digest));
    LXP_CASE_CHECK(lxp_keccak256(abc, sizeof(abc), produced) == LXP_OK);
    LXP_CASE_CHECK(digest_matches_hex(produced, abc_digest));
    reference_keccak256(abc, sizeof(abc), reference);
    LXP_CASE_CHECK(digest_matches_hex(reference, abc_digest));
    return 0;
}

static int keccak_block_boundaries(void)
{
    static const size_t lengths[] = {135U, 136U, 137U};
    uint8_t input[137];
    uint8_t produced[32];
    uint8_t expected[32];
    size_t i;

    (void)memset(input, 0x61, sizeof(input));
    for (i = 0U; i < sizeof(lengths) / sizeof(lengths[0]); ++i) {
        reference_keccak256(input, lengths[i], expected);
        LXP_CASE_CHECK(lxp_keccak256(input, lengths[i], produced) == LXP_OK);
        LXP_CASE_CHECK(memcmp(produced, expected, sizeof(expected)) == 0);
    }
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"keccak_known_answers", keccak_known_answers},
        {"keccak_block_boundaries", keccak_block_boundaries}
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
