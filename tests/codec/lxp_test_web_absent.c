#define _POSIX_C_SOURCE 200809L

#include "lxp_kernel_case.h"

#include "layerx/lxp_codec.h"
#include "layerx/lxp_module.h"

struct lx_web_answer;

extern const lxp_module_iface *lx_web_module_iface(void);
extern lxp_result lx_web_committed_read(
    lxp_module_ctx *ctx, const uint8_t program_id[32], uint64_t request_id,
    struct lx_web_answer *answer);
extern lxp_result lx_web_request_record_decode(
    const uint8_t *data, size_t length, uint64_t *request_id, uint8_t *kind,
    lxp_byte_span *payload);

#pragma weak lx_web_module_iface
#pragma weak lx_web_committed_read
#pragma weak lx_web_request_record_decode

static int web_symbols_absent(void)
{
    static const char *const retired[] = {
        "lx_web_module_iface",
        "lx_web_committed_read",
        "lx_web_request_record_decode"
    };
    FILE *symbols;
    char line[4096];
    int absent = 1;
    int read_ok;
    int close_status;
    size_t i;

    LXP_CASE_CHECK(lx_web_module_iface == NULL);
    LXP_CASE_CHECK(lx_web_committed_read == NULL);
    LXP_CASE_CHECK(lx_web_request_record_decode == NULL);

    symbols = popen("nm -g --defined-only build/liblayerx.a", "r");
    LXP_CASE_CHECK(symbols != NULL);
    while (fgets(line, (int)sizeof(line), symbols) != NULL) {
        size_t length = strlen(line);
        char *name;
        if (length == 0U || line[length - 1U] != '\n') {
            absent = 0;
            continue;
        }
        line[length - 1U] = '\0';
        name = strrchr(line, ' ');
        if (name == NULL) continue;
        ++name;
        for (i = 0U; i < sizeof(retired) / sizeof(retired[0]); ++i) {
            if (strcmp(name, retired[i]) == 0) {
                fprintf(stderr, "archive still defines %s\n", name);
                absent = 0;
            }
        }
    }
    read_ok = !ferror(symbols);
    close_status = pclose(symbols);
    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_status == 0);
    LXP_CASE_CHECK(absent);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "web_symbols_absent", web_symbols_absent }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
