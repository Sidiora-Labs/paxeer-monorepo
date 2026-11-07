#define _POSIX_C_SOURCE 200809L

#include "lxp_kernel_case.h"

#include <errno.h>
#include <sys/stat.h>

#if __has_include("layerx/lxp_daemon.h")
enum { daemon_header_present = 1 };
#else
enum { daemon_header_present = 0 };
#endif

static int daemon_header_absent(void)
{
    struct stat metadata;

    LXP_CASE_CHECK(daemon_header_present == 0);
    LXP_CASE_CHECK(lstat("include/layerx/lxp_daemon.h", &metadata) == -1);
    LXP_CASE_CHECK(errno == ENOENT);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "daemon_header_absent", daemon_header_absent }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
