#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_qualification.h"
#include "lxp_kernel_case.h"

#include <unistd.h>

#include "../qualification/lxp_qual_replay.c"

typedef int lxp_partition_sim;
typedef int lxp_sequencer_loss_sim;

static int qual_activity_count_matches_registry(void)
{
    char directory[] = "/tmp/lxp-qual-header-XXXXXX";
    char corpus_path[128];
    char ledger_path[128];
    uint8_t prefix[QUAL_HEADER_PREFIX_BYTES];
    qual_header header = {0};
    lxp_qual_replay_result result;
    lxp_result generated;
    lxp_result replayed;
    lxp_result parsed;
    FILE *corpus;
    size_t i;
    int corpus_removed;
    int ledger_removed;
    int directory_removed;
    int closed;

    LXP_CASE_CHECK(LXP_QUAL_ACTIVITY_TYPE_COUNT == 50);
    LXP_CASE_CHECK(sizeof(activity_types) / sizeof(activity_types[0]) ==
                   LXP_QUAL_ACTIVITY_TYPE_COUNT);
    for (i = 0U; i < LXP_QUAL_ACTIVITY_TYPE_COUNT; ++i) {
        LXP_CASE_CHECK(activity_types[i] != UINT32_C(0x00010006));
        LXP_CASE_CHECK(activity_types[i] != UINT32_C(0x00010007));
        LXP_CASE_CHECK(activity_types[i] != UINT32_C(0x00010008));
        LXP_CASE_CHECK(activity_types[i] != UINT32_C(0x0003000b));
        if (i != 0U)
            LXP_CASE_CHECK(activity_types[i - 1U] < activity_types[i]);
    }

    LXP_CASE_CHECK(mkdtemp(directory) != NULL);
    LXP_CASE_CHECK(snprintf(corpus_path, sizeof(corpus_path), "%s/corpus.lxq",
                            directory) == (int)strlen(directory) + 11);
    LXP_CASE_CHECK(snprintf(ledger_path, sizeof(ledger_path), "%s/roots.lxq",
                            directory) == (int)strlen(directory) + 10);
    generated = lxp_qual_corpus_generate(corpus_path, ledger_path, 50U, 10U);
    replayed = lxp_qual_replay_matrix(corpus_path, ledger_path, &result);
    corpus = fopen(corpus_path, "rb");
    LXP_CASE_CHECK(corpus != NULL);
    parsed = parse_header(corpus, &header, prefix);
    closed = fclose(corpus);
    corpus_removed = unlink(corpus_path);
    ledger_removed = unlink(ledger_path);
    directory_removed = rmdir(directory);

    LXP_CASE_CHECK(generated == LXP_OK);
    LXP_CASE_CHECK(replayed == LXP_OK);
    LXP_CASE_CHECK(parsed == LXP_OK);
    LXP_CASE_CHECK(closed == 0);
    LXP_CASE_CHECK(corpus_removed == 0 && ledger_removed == 0 &&
                   directory_removed == 0);
    LXP_CASE_CHECK(header.activity_type_count == 50U);
    LXP_CASE_CHECK(header.activity_count == 50U);
    LXP_CASE_CHECK(result.activity_count == 50U);
    LXP_CASE_CHECK(result.batch_count == 5U);
    LXP_CASE_CHECK(result.first_divergent_sequence == 0U);
    return 0;
}

static int qual_fault_sims_undeclared(void)
{
    FILE *header = fopen("include/layerx/lxp_qualification.h", "rb");
    char *contents;
    long length;
    size_t bytes;
    int read_ok;
    int close_ok;
    int absent;

    LXP_CASE_CHECK(sizeof(lxp_partition_sim) == sizeof(int));
    LXP_CASE_CHECK(sizeof(lxp_sequencer_loss_sim) == sizeof(int));
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
    close_ok = fclose(header) == 0;
    contents[bytes] = '\0';
    absent = strstr(contents, "lxp_partition_sim") == NULL &&
             strstr(contents, "lxp_sequencer_loss_sim") == NULL;
    free(contents);
    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_ok);
    LXP_CASE_CHECK(absent);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "qual_activity_count_matches_registry",
          qual_activity_count_matches_registry },
        { "qual_fault_sims_undeclared", qual_fault_sims_undeclared }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
