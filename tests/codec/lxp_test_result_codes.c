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

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "result_arbiter_codes_named", result_arbiter_codes_named },
        { "result_arbiter_codes_module_domain",
          result_arbiter_codes_module_domain }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
