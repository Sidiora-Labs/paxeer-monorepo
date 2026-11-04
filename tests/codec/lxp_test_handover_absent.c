#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_batch.h"
#include "layerx/lxp_genesis.h"
#include "lxp_kernel_case.h"

struct lxp_kernel;
struct lxp_module_ctx;
struct lxp_handover_evidence;

#ifndef LXP_TEST_LIBRARY_PATH
#error "Makefile must supply LXP_TEST_LIBRARY_PATH from $(abspath $(LIBRARY))"
#endif

static FILE *retired_archive_symbols(void)
{
    static const char prefix[] = "nm -P --defined-only --extern-only -- '";
    static const char archive[] = LXP_TEST_LIBRARY_PATH;
    char command[sizeof(prefix) + 4U * sizeof(archive) + 1U];
    size_t used = sizeof(prefix) - 1U;

    if (archive[0] == '\0') {
        (void)fputs("LXP_TEST_LIBRARY_PATH must name the linked archive\n", stderr);
        return NULL;
    }
    (void)memcpy(command, prefix, used);
    for (size_t i = 0U; archive[i] != '\0'; ++i) {
        if (archive[i] == '\'') {
            (void)memcpy(command + used, "'\\''", 4U);
            used += 4U;
        } else {
            command[used++] = archive[i];
        }
    }
    command[used++] = '\'';
    command[used] = '\0';
    return popen(command, "r");
}

extern lxp_result lxp_handover_prepare(struct lxp_kernel *kernel,
    const lxp_activity *activity, uint64_t batch_number, lxp_arena *arena);
extern lxp_result lxp_handover_stage(struct lxp_module_ctx *ctx,
    const lxp_activity *activity, const lxp_authority_resolved *authority);
extern lxp_result lxp_handover_evidence_decode(lxp_byte_span encoded,
    struct lxp_handover_evidence *evidence);
extern lxp_result lxp_handover_history_resolve(const struct lxp_kernel *kernel,
    uint64_t batch_number, lxp_sequencer_authorization *authorization,
    uint64_t *epoch, lxp_arena *arena);
extern lxp_result lxp_handover_genesis_authority(
    const lxp_genesis_manifest *manifest, uint8_t public_key[32], bool *present);

#pragma weak lxp_handover_prepare
#pragma weak lxp_handover_stage
#pragma weak lxp_handover_evidence_decode
#pragma weak lxp_handover_history_resolve
#pragma weak lxp_handover_genesis_authority

static int handover_symbols_absent(void)
{
    static const char *const retired[] = {
        "lxp_handover_prepare",
        "lxp_handover_stage",
        "lxp_handover_evidence_decode",
        "lxp_handover_history_resolve",
        "lxp_handover_genesis_authority"
    };
    char line[1024];
    char symbol[256];
    char kind;
    size_t defined = 0U;
    int absent = 1;
    int read_error;
    int status;
    FILE *symbols;

    LXP_CASE_CHECK(lxp_handover_prepare == NULL);
    LXP_CASE_CHECK(lxp_handover_stage == NULL);
    LXP_CASE_CHECK(lxp_handover_evidence_decode == NULL);
    LXP_CASE_CHECK(lxp_handover_history_resolve == NULL);
    LXP_CASE_CHECK(lxp_handover_genesis_authority == NULL);

    symbols = retired_archive_symbols();
    LXP_CASE_CHECK(symbols != NULL);
    while (fgets(line, sizeof(line), symbols) != NULL) {
        if (sscanf(line, "%255s %c", symbol, &kind) != 2) continue;
        ++defined;
        for (size_t i = 0U; i < sizeof(retired) / sizeof(retired[0]); ++i) {
            if (strcmp(symbol, retired[i]) == 0) {
                fprintf(stderr, "Retired symbol remains in archive: %s\n", symbol);
                absent = 0;
            }
        }
    }
    read_error = ferror(symbols);
    status = pclose(symbols);
    LXP_CASE_CHECK(read_error == 0);
    LXP_CASE_CHECK(status == 0);
    LXP_CASE_CHECK(defined > 0U);
    LXP_CASE_CHECK(absent == 1);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "handover_symbols_absent", handover_symbols_absent }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
