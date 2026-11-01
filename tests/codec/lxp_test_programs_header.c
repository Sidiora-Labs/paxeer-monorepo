#include "layerx/programs.h"

#ifdef LAYERX_LXP_REPLICA_H
enum { programs_header_includes_replica = 1 };
#else
enum { programs_header_includes_replica = 0 };
#endif

#include "lxp_kernel_case.h"

static int programs_header_without_replica(void)
{
    LXP_CASE_CHECK(programs_header_includes_replica == 0);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "programs_header_without_replica", programs_header_without_replica }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
