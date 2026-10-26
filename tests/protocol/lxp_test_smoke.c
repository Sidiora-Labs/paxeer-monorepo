#include "lxp_test_harness.h"

#include <string.h>

static int no_op_suite(void)
{
    static const unsigned char expected[] = { 0x4cU, 0x58U, 0x50U };
    unsigned char produced[sizeof(expected)];
    (void)memcpy(produced, expected, sizeof(expected));
    return LXP_ASSERT_BYTES(expected, produced, sizeof(expected)) |
           LXP_ASSERT_U64(UINT64_C(1), UINT64_C(1)) |
           LXP_ASSERT_RESULT(LXP_OK, LXP_OK);
}

static int run_one_runs_named_case(void)
{
    return LXP_ASSERT_U64(UINT64_C(0),
                          (uint64_t)lxp_test_run_one("protocol.no-op"));
}

static int run_one_reports_unknown_case(void)
{
    char program[] = "lxp_test_harness";
    char option[] = "--run";
    char name[] = "harness.no-such-case";
    char *argv[] = { program, option, name, NULL };

    return LXP_ASSERT_U64(UINT64_C(2), (uint64_t)lxp_test_run_one(name)) |
           LXP_ASSERT_U64(UINT64_C(2), (uint64_t)lxp_test_main(3, argv));
}

int main(int argc, char **argv)
{
    if (lxp_test_register("protocol.no-op", no_op_suite) != 0) return 1;
    if (lxp_test_register("harness.run-one-runs-named-case",
                          run_one_runs_named_case) != 0) return 1;
    if (lxp_test_register("harness.run-one-reports-unknown-case",
                          run_one_reports_unknown_case) != 0) return 1;
    return lxp_test_main(argc, argv);
}
