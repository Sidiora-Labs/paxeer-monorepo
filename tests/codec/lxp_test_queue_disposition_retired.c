#include "lxp_kernel_case.h"

#include "layerx/lxp_daemon.h"
#include "layerx/lxp_fee.h"
#include "layerx/lxp_state.h"

#include <stdlib.h>

extern lxp_result lxp_queue_disposition_encode(
    const lxp_queue_disposition *, uint8_t *);
extern lxp_result lxp_queue_disposition_decode(
    const uint8_t *, size_t, lxp_queue_disposition *);
extern bool lxp_queue_disposition_header(const uint8_t *, size_t);

#pragma weak lxp_queue_disposition_encode
#pragma weak lxp_queue_disposition_decode
#pragma weak lxp_queue_disposition_header

static int disposition_file_lacks(const char *path,
    const char *const *names, size_t count)
{
    FILE *file = fopen(path, "rb");
    long length;
    size_t bytes;
    size_t i;
    char *contents;
    int read_ok;
    int close_ok;
    int absent = 1;

    LXP_CASE_CHECK(file != NULL);
    if (fseek(file, 0L, SEEK_END) != 0 || (length = ftell(file)) <= 0L ||
        fseek(file, 0L, SEEK_SET) != 0) {
        (void)fclose(file);
        LXP_CASE_CHECK(0);
    }
    bytes = (size_t)length;
    if (bytes == SIZE_MAX) {
        (void)fclose(file);
        LXP_CASE_CHECK(0);
    }
    contents = malloc(bytes + 1U);
    if (contents == NULL) {
        (void)fclose(file);
        LXP_CASE_CHECK(0);
    }
    read_ok = fread(contents, 1U, bytes, file) == bytes && !ferror(file);
    close_ok = fclose(file) == 0;
    contents[bytes] = '\0';
    if (read_ok) {
        for (i = 0U; i < count; ++i) {
            if (strstr(contents, names[i]) != NULL) {
                fprintf(stderr, "%s still contains %s\n", path, names[i]);
                absent = 0;
            }
        }
    }
    free(contents);
    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_ok);
    LXP_CASE_CHECK(absent);
    return 0;
}

static int queue_disposition_codec_removed(void)
{
    static const char *const codec_names[] = {
        "lxp_queue_disposition_encode", "lxp_queue_disposition_decode",
        "lxp_queue_disposition_header", "LXP_QUEUE_DISPOSITION_"
    };
    static const char *const helper_names[] = {
        "disposition_canonical", "disposition_get32", "disposition_put32",
        "disposition_get64", "disposition_put64"
    };
    static lxp_state_store state;
    lxp_queue_disposition disposition = {
        .admission_order = 1U,
        .ordering_sequence = 2U,
        .activity_id = {3U},
        .idempotency_key = {4U},
        .result_code = LXP_ERR_FEE_LIMIT,
        .parameter_version = LXP_FEE_TWO_CLASS_PARAMETER_VERSION
    };

    LXP_CASE_CHECK(disposition_file_lacks("include/layerx/lxp_fee.h",
        codec_names, sizeof(codec_names) / sizeof(codec_names[0])) == 0);
    LXP_CASE_CHECK(disposition_file_lacks("src/state/lxp_journal.c",
        codec_names, sizeof(codec_names) / sizeof(codec_names[0])) == 0);
    LXP_CASE_CHECK(disposition_file_lacks("src/state/lxp_journal.c",
        helper_names, sizeof(helper_names) / sizeof(helper_names[0])) == 0);

    LXP_CASE_CHECK(sizeof(lxp_queue_disposition) == sizeof(disposition));
    LXP_CASE_CHECK(sizeof(((lxp_daemon_lni_server *)0)->dispositions) ==
        LXP_DAEMON_QUEUE_CAPACITY * sizeof(disposition));
    LXP_CASE_CHECK(disposition.admission_order == 1U &&
        disposition.ordering_sequence == 2U &&
        disposition.activity_id[0] == 3U &&
        disposition.idempotency_key[0] == 4U &&
        disposition.result_code == LXP_ERR_FEE_LIMIT &&
        disposition.parameter_version == LXP_FEE_TWO_CLASS_PARAMETER_VERSION);

    LXP_CASE_CHECK(lxp_state_store_init(&state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_queue_disposition_encode == NULL);
    LXP_CASE_CHECK(lxp_queue_disposition_decode == NULL);
    LXP_CASE_CHECK(lxp_queue_disposition_header == NULL);
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "queue_disposition_codec_removed", queue_disposition_codec_removed }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
