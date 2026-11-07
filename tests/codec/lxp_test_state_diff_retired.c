#define _POSIX_C_SOURCE 200809L

#include "lxp_kernel_case.h"

#include <stdlib.h>

#include "layerx/lxp_codec.h"
#include "layerx/lxp_ledger.h"

#include <errno.h>
#include <sys/stat.h>

typedef struct lxp_state_diff_entry lxp_state_diff_entry;

extern lxp_result lxp_state_diff_encode(const lx_account_registry *,
    const lx_account_registry *, lxp_arena *, lxp_byte_span *);
extern lxp_result lxp_state_diff_decode(lxp_byte_span, lxp_arena *,
    lxp_state_diff_entry **, size_t *);

#pragma weak lxp_state_diff_encode
#pragma weak lxp_state_diff_decode

static const char *retirement_library_path;

static FILE *retirement_archive_symbols(void)
{
    static const char prefix[] = "nm -g --defined-only -- ";
    size_t length = strlen(retirement_library_path);
    char *command;
    char *cursor;
    FILE *symbols;

    if (length == 0U || length > (SIZE_MAX - sizeof(prefix) - 3U) / 4U)
        return NULL;
    command = malloc(sizeof(prefix) + 4U * length + 3U);
    if (command == NULL) return NULL;
    memcpy(command, prefix, sizeof(prefix) - 1U);
    cursor = command + sizeof(prefix) - 1U;
    *cursor++ = '\'';
    for (size_t i = 0U; i < length; ++i) {
        if (retirement_library_path[i] == '\'') {
            memcpy(cursor, "'\\''", 4U);
            cursor += 4U;
        } else {
            *cursor++ = retirement_library_path[i];
        }
    }
    *cursor++ = '\'';
    *cursor = '\0';
    symbols = popen(command, "r");
    free(command);
    return symbols;
}

static int state_diff_codec_removed(void)
{
    static const char *const paths[] = {
        "include/layerx/lxp_state_diff.h", "src/state/lxp_state_diff.c"
    };
    struct stat info;
    FILE *symbols;
    char line[4096];
    size_t i;
    int absent = 1;
    int codec_present = 0;
    int read_ok;
    int close_status;

    for (i = 0U; i < sizeof(paths) / sizeof(paths[0]); ++i) {
        errno = 0;
        LXP_CASE_CHECK(lstat(paths[i], &info) == -1 && errno == ENOENT);
    }
    LXP_CASE_CHECK(lxp_state_diff_encode == NULL);
    LXP_CASE_CHECK(lxp_state_diff_decode == NULL);

    symbols = retirement_archive_symbols();
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
        if (strcmp(name, "lxp_codec_writer_init") == 0) codec_present = 1;
        if (strcmp(name, "lxp_state_diff_encode") == 0 ||
            strcmp(name, "lxp_state_diff_decode") == 0) {
            fprintf(stderr, "archive still defines %s\n", name);
            absent = 0;
        }
    }
    read_ok = !ferror(symbols);
    close_status = pclose(symbols);
    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_status == 0);
    LXP_CASE_CHECK(codec_present);
    LXP_CASE_CHECK(absent);
    return 0;
}

int main(int argc, char **argv)
{
    if (argc < 4 || strcmp(argv[1], "--library") != 0 || argv[2][0] == '\0') {
        fprintf(stderr, "usage: %s --library <linked archive> <case>...\n", argv[0]);
        return 2;
    }
    retirement_library_path = argv[2];
    argc -= 2;
    argv += 2;
    const lxp_kernel_case cases[] = {
        { "state_diff_codec_removed", state_diff_codec_removed }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
