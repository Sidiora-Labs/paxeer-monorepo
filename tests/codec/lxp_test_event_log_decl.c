#include "layerx/programs.h"
#include "lxp_kernel_case.h"

/* Keep the header declaration required while allowing a kernel-only link. */
#pragma weak layerx_programs_event_log_encode

typedef int32_t (*event_log_encoder)(uint64_t, uint32_t, uint8_t *, uint32_t);

static int event_log_encode_declared(void)
{
    event_log_encoder encode = &layerx_programs_event_log_encode;

    LXP_CASE_CHECK(_Generic(&layerx_programs_event_log_encode,
                           event_log_encoder: 1, default: 0));
    LXP_CASE_CHECK(encode == &layerx_programs_event_log_encode);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "event_log_encode_declared", event_log_encode_declared }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
