#include "layerx/lxp_result.h"
#include "lxp_kernel_case.h"

#define LXP_RESULT_TEST_VALUE(name, value) value,
static const lxp_result registered_results[] = {
    LXP_RESULT_CODE_LIST(LXP_RESULT_TEST_VALUE)
};
#undef LXP_RESULT_TEST_VALUE

static int result_domain_unregistered_unknown(void)
{
    const lxp_result unregistered[] = {
        INT32_MIN, -1100, -1004, -1000, -999, -907, -899, -806,
        -799, -740, -699, -603, -599, -504, -499, -419, -399,
        -308, -299, -223, -199, -109, -99, -9, 1, 10, INT32_MAX
    };
    const struct {
        lxp_result result;
        lxp_result_domain_id domain;
    } registered[] = {
        { LXP_OK, LXP_RESULT_DOMAIN_SUCCESS },
        { LXP_ERR_TRUNCATED, LXP_RESULT_DOMAIN_CODEC },
        { LXP_ERR_WRONG_NETWORK, LXP_RESULT_DOMAIN_ENVELOPE },
        { LXP_ERR_BAD_SIGNATURE, LXP_RESULT_DOMAIN_AUTHORITY },
        { LXP_ERR_SEQUENCE_GAP, LXP_RESULT_DOMAIN_SEQUENCING },
        { LXP_ERR_INSUFFICIENT_BALANCE, LXP_RESULT_DOMAIN_LEDGER },
        { LXP_ERR_OVERFLOW, LXP_RESULT_DOMAIN_ARITHMETIC },
        { LXP_ERR_FEE_LIMIT, LXP_RESULT_DOMAIN_METERING },
        { LXP_ERR_ESCROW_LINK_MISMATCH, LXP_RESULT_DOMAIN_MODULE },
        { LXP_ERR_BATCH_GAP, LXP_RESULT_DOMAIN_BATCH },
        { LXP_ERR_LOG_CORRUPT, LXP_RESULT_DOMAIN_STORAGE },
        { LXP_FATAL_INVARIANT, LXP_RESULT_DOMAIN_FATAL }
    };
    size_t i;

    for (i = 0U; i < sizeof(unregistered) / sizeof(unregistered[0]); ++i) {
        LXP_CASE_CHECK(lxp_result_domain(unregistered[i]) ==
                       LXP_RESULT_DOMAIN_UNKNOWN);
    }
    for (i = 0U; i < sizeof(registered) / sizeof(registered[0]); ++i) {
        LXP_CASE_CHECK(lxp_result_domain(registered[i].result) ==
                       registered[i].domain);
    }
    return 0;
}

static int result_is_registered_matches_list(void)
{
    lxp_result result;
    size_t i;

    for (i = 0U; i < sizeof(registered_results) / sizeof(registered_results[0]);
         ++i) {
        LXP_CASE_CHECK(lxp_result_is_registered(registered_results[i]));
    }
    for (result = -1100; result <= 10; ++result) {
        bool expected = false;

        for (i = 0U;
             i < sizeof(registered_results) / sizeof(registered_results[0]);
             ++i) {
            if (registered_results[i] == result) {
                expected = true;
                break;
            }
        }
        LXP_CASE_CHECK(lxp_result_is_registered(result) == expected);
        if (!expected) {
            LXP_CASE_CHECK(lxp_result_domain(result) ==
                           LXP_RESULT_DOMAIN_UNKNOWN);
        }
    }
    LXP_CASE_CHECK(!lxp_result_is_registered(INT32_MIN));
    LXP_CASE_CHECK(!lxp_result_is_registered(INT32_MAX));
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "result_domain_unregistered_unknown",
          result_domain_unregistered_unknown },
        { "result_is_registered_matches_list",
          result_is_registered_matches_list }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
