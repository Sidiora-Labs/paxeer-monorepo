#define _POSIX_C_SOURCE 200809L

#include "layerx/programs.h"

#include "lxp_kernel_case.h"

#include <stdlib.h>

typedef struct lx_programs_state_feed lx_programs_state_feed;
typedef struct lx_programs_state_feed_store lx_programs_state_feed_store;

extern lxp_result lxp_programs_state_feed_store_open(
    lx_programs_state_feed_store *, lxp_log *, lxp_log *, lxp_history *,
    lxp_arena *, pthread_mutex_t *);
extern lxp_result lxp_programs_bind_state_feed(
    lxp_kernel *, const lx_programs_state_feed *);
extern lxp_result lxp_programs_state_feed_observe(
    const lx_programs_state_feed *, const lxp_activity *, const lxp_receipt *);
extern lxp_result lxp_programs_replay_finalize(
    void *, const lxp_batch_header *, uint32_t, uint64_t, const uint8_t *,
    lxp_arena *, lxp_replay_activity_output *);
extern lxp_result lxp_programs_replay_engine_bind(
    lxp_replay_engine *, lxp_kernel *);
extern lxp_result lxp_programs_fee_governance_decode(
    lxp_module_ctx *, const uint8_t *, size_t, void **);
extern lxp_result lxp_programs_fee_governance_validate(
    lxp_module_ctx *, const lxp_activity *, const lxp_authority_resolved *,
    const void *);
extern lxp_result lxp_programs_fee_governance_execute(
    lxp_module_ctx *, const lxp_activity *, const lxp_authority_resolved *,
    const void *, lxp_effect_buffer *);

#pragma weak lxp_programs_state_feed_store_open
#pragma weak lxp_programs_bind_state_feed
#pragma weak lxp_programs_state_feed_observe
#pragma weak lxp_programs_replay_finalize
#pragma weak lxp_programs_replay_engine_bind
#pragma weak lxp_programs_fee_governance_decode
#pragma weak lxp_programs_fee_governance_validate
#pragma weak lxp_programs_fee_governance_execute

static int programs_retired_symbols_absent(void)
{
    static const char *const retired[] = {
        "lxp_programs_state_feed_store_open",
        "lxp_programs_bind_state_feed",
        "lxp_programs_state_feed_observe",
        "lxp_programs_replay_finalize",
        "lxp_programs_replay_engine_bind",
        "lxp_programs_fee_governance_decode",
        "lxp_programs_fee_governance_validate",
        "lxp_programs_fee_governance_execute"
    };
    FILE *symbols;
    char line[4096];
    int absent = 1;
    int read_ok;
    int close_status;
    size_t i;

    LXP_CASE_CHECK(lxp_programs_state_feed_store_open == NULL);
    LXP_CASE_CHECK(lxp_programs_bind_state_feed == NULL);
    LXP_CASE_CHECK(lxp_programs_state_feed_observe == NULL);
    LXP_CASE_CHECK(lxp_programs_replay_finalize == NULL);
    LXP_CASE_CHECK(lxp_programs_replay_engine_bind == NULL);
    LXP_CASE_CHECK(lxp_programs_fee_governance_decode == NULL);
    LXP_CASE_CHECK(lxp_programs_fee_governance_validate == NULL);
    LXP_CASE_CHECK(lxp_programs_fee_governance_execute == NULL);

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

static int programs_header_without_relay(void)
{
    static const char *const retired[] = {
        "LX_PROGRAMS_FEE_GOVERNANCE", "state_feed",
        "lxp_programs_replay_finalize", "lxp_programs_replay_engine_bind",
        "lxp_programs_fee_governance_decode",
        "lxp_programs_fee_governance_validate",
        "lxp_programs_fee_governance_execute"
    };
    FILE *header = fopen("include/layerx/programs.h", "rb");
    char *contents;
    long length;
    size_t bytes;
    size_t i;
    int read_ok;
    int close_status;
    int absent = 1;

    LXP_CASE_CHECK(header != NULL);
    if (fseek(header, 0L, SEEK_END) != 0 || (length = ftell(header)) <= 0L ||
        fseek(header, 0L, SEEK_SET) != 0) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    bytes = (size_t)length;
    if (bytes == SIZE_MAX) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    contents = malloc(bytes + 1U);
    if (contents == NULL) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    read_ok = fread(contents, 1U, bytes, header) == bytes && !ferror(header);
    close_status = fclose(header);
    contents[bytes] = '\0';
    if (read_ok) {
        for (i = 0U; i < sizeof(retired) / sizeof(retired[0]); ++i) {
            if (strstr(contents, retired[i]) != NULL) {
                fprintf(stderr, "Programs header still declares %s\n", retired[i]);
                absent = 0;
            }
        }
    }
    free(contents);
    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_status == 0);
    LXP_CASE_CHECK(absent);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "programs_retired_symbols_absent", programs_retired_symbols_absent },
        { "programs_header_without_relay", programs_header_without_relay }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
