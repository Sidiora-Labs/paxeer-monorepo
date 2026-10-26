#include "lxp_kernel_case.h"

static unsigned int dispatch_sequence;

static int dispatch_first(void)
{
    dispatch_sequence = dispatch_sequence * 10U + 1U;
    return 0;
}

static int dispatch_second(void)
{
    dispatch_sequence = dispatch_sequence * 10U + 2U;
    return 0;
}

static int dispatch_failure(void)
{
    dispatch_sequence = dispatch_sequence * 10U + 3U;
    return 7;
}

static int case_dispatch_runs_named_case(void)
{
    const lxp_kernel_case cases[] = {
        { "first", dispatch_first },
        { "second", dispatch_second },
        { "failure", dispatch_failure }
    };
    const size_t count = sizeof(cases) / sizeof(cases[0]);
    char program[] = "case_dispatch";
    char first[] = "first";
    char second[] = "second";
    char failure[] = "failure";
    char *single[] = { program, second, NULL };
    char *ordered[] = { program, second, first, second, NULL };
    char *failing[] = { program, first, failure, second, NULL };

    dispatch_sequence = 0U;
    LXP_CASE_CHECK(lxp_kernel_case_main(cases, count, 2, single) == 0);
    LXP_CASE_CHECK(dispatch_sequence == 2U);

    dispatch_sequence = 0U;
    LXP_CASE_CHECK(lxp_kernel_case_main(cases, count, 4, ordered) == 0);
    LXP_CASE_CHECK(dispatch_sequence == 212U);

    dispatch_sequence = 0U;
    LXP_CASE_CHECK(lxp_kernel_case_main(cases, count, 4, failing) == 1);
    LXP_CASE_CHECK(dispatch_sequence == 13U);
    return 0;
}

static int case_dispatch_rejects_unknown_case(void)
{
    const lxp_kernel_case cases[] = {
        { "first", dispatch_first },
        { "second", dispatch_second }
    };
    const size_t count = sizeof(cases) / sizeof(cases[0]);
    char program[] = "case_dispatch";
    char first[] = "first";
    char second[] = "second";
    char unknown[] = "no_such_case";
    char *unknown_only[] = { program, unknown, NULL };
    char *known_then_unknown[] = { program, first, second, unknown, NULL };
    char *no_names[] = { program, NULL };
    char *empty_table[] = { program, first, NULL };

    dispatch_sequence = 0U;
    LXP_CASE_CHECK(lxp_kernel_case_main(cases, count, 2, unknown_only) == 2);
    LXP_CASE_CHECK(dispatch_sequence == 0U);
    LXP_CASE_CHECK(lxp_kernel_case_main(cases, count, 4,
                                      known_then_unknown) == 2);
    LXP_CASE_CHECK(dispatch_sequence == 0U);
    LXP_CASE_CHECK(lxp_kernel_case_main(cases, count, 1, no_names) == 2);
    LXP_CASE_CHECK(dispatch_sequence == 0U);
    LXP_CASE_CHECK(lxp_kernel_case_main(cases, 0U, 2, empty_table) == 2);
    LXP_CASE_CHECK(dispatch_sequence == 0U);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "case_dispatch_runs_named_case", case_dispatch_runs_named_case },
        { "case_dispatch_rejects_unknown_case",
          case_dispatch_rejects_unknown_case }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
