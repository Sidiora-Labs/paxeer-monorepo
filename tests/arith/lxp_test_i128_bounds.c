#include "layerx/lxp_u128.h"
#include "lxp_kernel_case.h"

#include <stdint.h>
#include <string.h>

lxp_result lxp_i128_sub(lxp_i128 left, lxp_i128 right, lxp_i128 *out);

static int i128_sub_overflow_refused(void)
{
    unsigned int sign;

    for (sign = 0U; sign < 2U; ++sign) {
        const lxp_i128 left = {
            sign != 0U, { UINT64_MAX, UINT64_MAX }
        };
        const lxp_i128 right = { sign == 0U, { 0U, 1U } };
        lxp_i128 out;
        unsigned char before[sizeof(out)];

        memset(&out, 0xa5, sizeof(out));
        out.negative = sign == 0U;
        out.magnitude = (lxp_u128){ UINT64_C(0x123456789abcdef0),
                                    UINT64_C(0xfedcba9876543210) };
        memcpy(before, &out, sizeof(out));

        LXP_CASE_CHECK(lxp_i128_sub(left, right, &out) == LXP_ERR_OVERFLOW);
        LXP_CASE_CHECK(memcmp(before, &out, sizeof(out)) == 0);
    }
    return 0;
}

static uint64_t next_seeded_word(uint64_t *state)
{
    *state ^= *state << 13U;
    *state ^= *state >> 7U;
    *state ^= *state << 17U;
    return *state;
}

static int i128_sub_zero_canonical(void)
{
    uint64_t state = UINT64_C(0x6a09e667f3bcc909);
    unsigned int i;
    const lxp_i128 negative_zero = { true, { 0U, 0U } };
    const lxp_i128 zero = { false, { 0U, 0U } };
    lxp_i128 out;

    for (i = 0U; i < 1000U; ++i) {
        lxp_i128 x;

        x.negative = (i % 2U) != 0U;
        x.magnitude.hi = next_seeded_word(&state);
        x.magnitude.lo = next_seeded_word(&state);
        if (i < 2U) x.magnitude = (lxp_u128){ 0U, 0U };
        else if (i < 4U)
            x.magnitude = (lxp_u128){ UINT64_MAX, UINT64_MAX };

        out = (lxp_i128){ true, { 7U, 9U } };
        LXP_CASE_CHECK(lxp_i128_sub(x, x, &out) == LXP_OK);
        LXP_CASE_CHECK(out.negative == false);
        LXP_CASE_CHECK(out.magnitude.hi == 0U);
        LXP_CASE_CHECK(out.magnitude.lo == 0U);
    }

    out = (lxp_i128){ true, { 7U, 9U } };
    LXP_CASE_CHECK(lxp_i128_sub(negative_zero, zero, &out) == LXP_OK);
    LXP_CASE_CHECK(out.negative == false);
    LXP_CASE_CHECK(out.magnitude.hi == 0U);
    LXP_CASE_CHECK(out.magnitude.lo == 0U);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "i128_sub_overflow_refused", i128_sub_overflow_refused },
        { "i128_sub_zero_canonical", i128_sub_zero_canonical }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
