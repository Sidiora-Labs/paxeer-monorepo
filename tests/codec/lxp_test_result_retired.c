#include "layerx/lxp_result.h"
#include "lxp_kernel_case.h"

#include <string.h>

static int result_da_missing_unregistered(void)
{
    const char *name = lxp_result_name(-804);
    size_t i;

    LXP_CASE_CHECK(!lxp_result_is_registered(-804));
    LXP_CASE_CHECK(name != NULL);
    LXP_CASE_CHECK(strcmp(name, "LXP_ERR_UNKNOWN") == 0);
    LXP_CASE_CHECK(lxp_result_domain(-804) == LXP_RESULT_DOMAIN_UNKNOWN);
    for (i = 0U; i < lxp_result_registry_count(); ++i) {
        lxp_result value = 0;

        LXP_CASE_CHECK(lxp_result_registry_at(i, &value, &name) == LXP_OK);
        LXP_CASE_CHECK(value != -804);
        LXP_CASE_CHECK(name != NULL);
        LXP_CASE_CHECK(strcmp(name, "LXP_ERR_DA_MISSING") != 0);
    }
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "result_da_missing_unregistered", result_da_missing_unregistered }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
