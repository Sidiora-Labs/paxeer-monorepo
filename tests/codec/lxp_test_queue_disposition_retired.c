#define _POSIX_C_SOURCE 200809L

#define main disposition_fixture_reference_main
static int disposition_fixture_reference_main(int argc, char **argv) __attribute__((unused));
#include "lxp_test_settled_maintenance.c"
#undef main

#include "lxp_kernel_case.h"

#include "layerx/lxp_fee.h"
#include "layerx/lxp_state.h"

#include <errno.h>
#include <stdlib.h>
#include <sys/stat.h>

typedef struct lxp_queue_disposition lxp_queue_disposition;

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
        "lxp_queue_disposition_header", "LXP_QUEUE_DISPOSITION_",
        "lxp_queue_disposition"
    };
    static const char *const helper_names[] = {
        "disposition_canonical", "disposition_get32", "disposition_put32",
        "disposition_get64", "disposition_put64"
    };
    static lxp_state_store state;
    struct stat info;

    errno = 0;
    LXP_CASE_CHECK(lstat("include/layerx/lxp_daemon.h", &info) == -1 && errno == ENOENT);

    LXP_CASE_CHECK(disposition_file_lacks("include/layerx/lxp_fee.h",
        codec_names, sizeof(codec_names) / sizeof(codec_names[0])) == 0);
    LXP_CASE_CHECK(disposition_file_lacks("src/state/lxp_journal.c",
        codec_names, sizeof(codec_names) / sizeof(codec_names[0])) == 0);
    LXP_CASE_CHECK(disposition_file_lacks("src/state/lxp_journal.c",
        helper_names, sizeof(helper_names) / sizeof(helper_names[0])) == 0);

    LXP_CASE_CHECK(lxp_state_store_init(&state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_queue_disposition_encode == NULL);
    LXP_CASE_CHECK(lxp_queue_disposition_decode == NULL);
    LXP_CASE_CHECK(lxp_queue_disposition_header == NULL);
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

static int pending_receipt_publication_without_disposition(void)
{
    settled_fixture *f = calloc(1U, sizeof(*f));
    lxp_activity activity;
    lxp_kernel_execution execution;
    lxp_kernel_prepared_batch *prepared = NULL;
    const lxp_receipt *receipt;
    uint8_t payload[512], before[32], digest[32];
    size_t payload_length;

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(settled_fixture_open(f) == 0);
    memcpy(before, f->kernel.current_state_root, 32U);
    LXP_CASE_CHECK(settled_build_send(f, &activity, payload, &payload_length) == 0);
    LXP_CASE_CHECK(settled_execution(f, &activity, &execution, 1U, 1U) == 0);
    LXP_CASE_CHECK(lxp_kernel_prepare_terminal_rejection(&f->kernel, &activity,
        &execution, LXP_ERR_FEE_LIMIT, &prepared) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_prepare_batch_maintenance(prepared, &activity,
        &execution) == LXP_OK);
    receipt = lxp_kernel_prepared_batch_receipts(prepared);
    LXP_CASE_CHECK(receipt != NULL && receipt[0].result_code == LXP_ERR_FEE_LIMIT);
    LXP_CASE_CHECK(f->state.next_sequence == 1U && f->published_count == 0U);
    LXP_CASE_CHECK(memcmp(before, f->kernel.current_state_root, 32U) == 0);
    memcpy(digest, lxp_kernel_prepared_batch_publication_digest(prepared), 32U);
    LXP_CASE_CHECK(lxp_kernel_commit_prepared_batch(&f->kernel, &f->identities,
        prepared, digest) == LXP_OK);
    LXP_CASE_CHECK(f->kernel.batch_publication_pending && f->kernel.publication_poisoned);
    LXP_CASE_CHECK(f->kernel.pending_batch_first_sequence == 1U);
    LXP_CASE_CHECK(f->kernel.pending_batch_last_sequence == 2U);
    LXP_CASE_CHECK(lxp_kernel_batch_publication_next_index(&f->kernel) == 0U);
    LXP_CASE_CHECK(f->published_count == 0U && f->state.next_sequence == 3U);
    LXP_CASE_CHECK(memcmp(f->kernel.pending_batch_publication_digest, digest, 32U) == 0);
    LXP_CASE_CHECK(memcmp(f->kernel.pending_batch_id, receipt[0].batch_id, 32U) == 0);
    LXP_CASE_CHECK(memcmp(f->kernel.pending_batch_base_receipt_root, before, 32U) == 0);
    LXP_CASE_CHECK(lxp_kernel_finalize_prepared_batch_publication(&f->kernel,
        &activity, prepared, digest) == LXP_OK);
    LXP_CASE_CHECK(!f->kernel.batch_publication_pending && !f->kernel.publication_poisoned);
    LXP_CASE_CHECK(f->published_count == 2U && f->published_sequence == 2U);
    LXP_CASE_CHECK(memcmp(f->published_root, f->kernel.current_state_root, 32U) == 0);
    lxp_kernel_prepared_batch_destroy(prepared);
    return settled_fixture_close(f);
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "queue_disposition_codec_removed", queue_disposition_codec_removed },
        { "pending_receipt_publication_without_disposition", pending_receipt_publication_without_disposition }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
