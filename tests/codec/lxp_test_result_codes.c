#include "layerx/lxp_result.h"
#include "lxp_kernel_case.h"

#include <string.h>

static int result_arbiter_codes_named(void)
{
    LXP_CASE_CHECK(LXP_ERR_ARBITER_REQUIRED == -737);
    LXP_CASE_CHECK(LXP_ERR_UNAUTHORIZED_ARBITER == -738);
    LXP_CASE_CHECK(LXP_ERR_ESCROW_LINK_MISMATCH == -739);
    LXP_CASE_CHECK(strcmp(lxp_result_name(LXP_ERR_ARBITER_REQUIRED),
                          "LXP_ERR_ARBITER_REQUIRED") == 0);
    LXP_CASE_CHECK(strcmp(lxp_result_name(LXP_ERR_UNAUTHORIZED_ARBITER),
                          "LXP_ERR_UNAUTHORIZED_ARBITER") == 0);
    LXP_CASE_CHECK(strcmp(lxp_result_name(LXP_ERR_ESCROW_LINK_MISMATCH),
                          "LXP_ERR_ESCROW_LINK_MISMATCH") == 0);
    return 0;
}

static int result_arbiter_codes_module_domain(void)
{
    const lxp_result codes[] = {
        LXP_ERR_ARBITER_REQUIRED,
        LXP_ERR_UNAUTHORIZED_ARBITER,
        LXP_ERR_ESCROW_LINK_MISMATCH
    };
    size_t i;

    for (i = 0U; i < sizeof(codes) / sizeof(codes[0]); ++i) {
        LXP_CASE_CHECK(lxp_result_domain(codes[i]) == LXP_RESULT_DOMAIN_MODULE);
        LXP_CASE_CHECK(!lxp_result_is_fatal(codes[i]));
    }
    return 0;
}

static int result_replay_and_engine_codes_named(void)
{
    LXP_CASE_CHECK(LXP_ERR_REPLAY_BOUND == -740);
    LXP_CASE_CHECK(LXP_ERR_ENGINE_FAULT == -741);
    LXP_CASE_CHECK(strcmp(lxp_result_name(LXP_ERR_REPLAY_BOUND),
                          "LXP_ERR_REPLAY_BOUND") == 0);
    LXP_CASE_CHECK(strcmp(lxp_result_name(LXP_ERR_ENGINE_FAULT),
                          "LXP_ERR_ENGINE_FAULT") == 0);
    LXP_CASE_CHECK(lxp_result_domain(LXP_ERR_REPLAY_BOUND) == LXP_RESULT_DOMAIN_MODULE);
    LXP_CASE_CHECK(lxp_result_domain(LXP_ERR_ENGINE_FAULT) == LXP_RESULT_DOMAIN_MODULE);
    LXP_CASE_CHECK(!lxp_result_is_fatal(LXP_ERR_REPLAY_BOUND));
    LXP_CASE_CHECK(!lxp_result_is_fatal(LXP_ERR_ENGINE_FAULT));
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "result_replay_and_engine_codes_named", result_replay_and_engine_codes_named },
        { "result_arbiter_codes_named", result_arbiter_codes_named },
        { "result_arbiter_codes_module_domain",
          result_arbiter_codes_module_domain }
    };

    if (argc == 2 && strcmp(argv[1], "--replay-status-names") == 0) {
        if (result_replay_and_engine_codes_named() != 0) return 1;
        return printf("%s\n%s\n", lxp_result_name(LXP_ERR_REPLAY_BOUND),
                      lxp_result_name(LXP_ERR_ENGINE_FAULT)) < 0 ? 1 : 0;
    }
    if (argc == 1) {
        size_t i;
        for (i = 0U; i < sizeof(cases) / sizeof(cases[0]); ++i) {
            if (cases[i].run() != 0) return 1;
        }
        return 0;
    }
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
