#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_qualification.h"
#include "layerx/lxp_protocol.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

enum {
    CORPUS_HEADER_BYTES = 252,
    ROOT_HEADER_BYTES = 48,
    ROOT_ENTRY_BYTES = 72,
    RECORD_PREFIX_BYTES = 13,
    BATCH_SIZE = 10
};

#define REQUIRE(condition) do { \
    if (!(condition)) { \
        (void)fprintf(stderr, "%s:%d: %s\n", __FILE__, __LINE__, #condition); \
        return 1; \
    } \
} while (0)

static uint32_t load_u32(const uint8_t bytes[4])
{
    return ((uint32_t)bytes[0] << 24U) | ((uint32_t)bytes[1] << 16U) |
           ((uint32_t)bytes[2] << 8U) | (uint32_t)bytes[3];
}

static uint64_t load_u64(const uint8_t bytes[8])
{
    uint64_t value = 0U;
    size_t i;
    for (i = 0U; i < 8U; ++i) value = (value << 8U) | bytes[i];
    return value;
}

static int check_boundaries(FILE *corpus, FILE *ledger,
                            uint64_t activity_count,
                            const lxp_qual_replay_result *result)
{
    uint8_t corpus_header[CORPUS_HEADER_BYTES];
    uint8_t root_header[ROOT_HEADER_BYTES];
    uint8_t record_prefix[RECORD_PREFIX_BYTES];
    uint8_t outputs[32 + LXP_QUAL_RECEIPT_BYTES + LXP_QUAL_EVENT_BYTES];
    uint8_t batch[LXP_QUAL_BATCH_HEADER_BYTES + 32];
    uint8_t entry[ROOT_ENTRY_BYTES];
    uint64_t expected_batches = activity_count / BATCH_SIZE +
                                (activity_count % BATCH_SIZE != 0U ? 1U : 0U);
    uint64_t batches = 0U;
    uint64_t sequence;

    REQUIRE(fread(corpus_header, 1U, sizeof(corpus_header), corpus) ==
            sizeof(corpus_header));
    REQUIRE(fread(root_header, 1U, sizeof(root_header), ledger) ==
            sizeof(root_header));
    REQUIRE(memcmp(corpus_header, "LXPQRP01", 8U) == 0);
    REQUIRE(load_u32(corpus_header + 8U) == 1U);
    REQUIRE(load_u64(corpus_header + 12U) == activity_count);
    REQUIRE(load_u32(corpus_header + 20U) == BATCH_SIZE);
    REQUIRE(load_u32(corpus_header + 24U) == LXP_QUAL_ACTIVITY_TYPE_COUNT);
    REQUIRE(memcmp(root_header, "LXPQRL01", 8U) == 0);
    REQUIRE(load_u32(root_header + 8U) == 1U);
    REQUIRE(load_u32(root_header + 12U) == expected_batches);
    REQUIRE(memcmp(root_header + 16U, result->corpus_digest, 32U) == 0);
    REQUIRE(memcmp(corpus_header + 156U, result->root_ledger_digest, 32U) == 0);
    REQUIRE(memcmp(corpus_header + 188U, result->terminal_root, 32U) == 0);
    REQUIRE(memcmp(corpus_header + 220U, result->corpus_digest, 32U) == 0);
    REQUIRE(fseek(corpus, 4L * LXP_QUAL_ACTIVITY_TYPE_COUNT, SEEK_CUR) == 0);

    for (sequence = 1U; sequence <= activity_count; ++sequence) {
        uint32_t activity_length;
        uint8_t boundary =
            (sequence % BATCH_SIZE == 0U || sequence == activity_count) ? 1U : 0U;
        REQUIRE(fread(record_prefix, 1U, sizeof(record_prefix), corpus) ==
                sizeof(record_prefix));
        REQUIRE(load_u64(record_prefix) == sequence);
        REQUIRE(record_prefix[8] == boundary);
        activity_length = load_u32(record_prefix + 9U);
        REQUIRE(activity_length > 0U && activity_length <= LXP_MAX_ACTIVITY_BYTES);
        REQUIRE(fseek(corpus, (long)activity_length, SEEK_CUR) == 0);
        REQUIRE(fread(outputs, 1U, sizeof(outputs), corpus) == sizeof(outputs));
        if (boundary != 0U) {
            REQUIRE(fread(batch, 1U, sizeof(batch), corpus) == sizeof(batch));
            REQUIRE(fread(entry, 1U, sizeof(entry), ledger) == sizeof(entry));
            REQUIRE(load_u64(batch + 64U) == sequence);
            REQUIRE(load_u64(entry) == sequence);
            REQUIRE(memcmp(batch + 32U, outputs, 32U) == 0);
            REQUIRE(memcmp(entry + 8U, outputs, 32U) == 0);
            REQUIRE(memcmp(entry + 40U, batch + LXP_QUAL_BATCH_HEADER_BYTES,
                           32U) == 0);
            ++batches;
        }
        if (sequence == activity_count) {
            REQUIRE(boundary == 1U);
            REQUIRE(memcmp(entry + 8U, result->terminal_root, 32U) == 0);
        }
    }
    REQUIRE(batches == expected_batches);
    REQUIRE(batches == result->batch_count);
    REQUIRE(fgetc(corpus) == EOF && !ferror(corpus));
    REQUIRE(fgetc(ledger) == EOF && !ferror(ledger));
    return 0;
}

static int finding_22_30_5_14_round_trip(const char *corpus_path,
                                       const char *ledger_path,
                                       uint64_t activity_count)
{
    lxp_qual_replay_result result;
    lxp_result status;
    FILE *corpus;
    FILE *ledger;
    int failed;

    REQUIRE(lxp_qual_corpus_generate(corpus_path, ledger_path, activity_count,
                                    BATCH_SIZE) == LXP_OK);
    status = lxp_qual_replay_matrix(corpus_path, ledger_path, &result);
    if (status != LXP_OK) {
        (void)fprintf(stderr,
            "finding 22.30.5.14: %llu/10 replay status=%d sequence=%llu\n",
            (unsigned long long)activity_count, (int)status,
            (unsigned long long)result.first_divergent_sequence);
    }
    REQUIRE(status == LXP_OK);
    REQUIRE(result.activity_count == activity_count);
    REQUIRE(result.first_divergent_sequence == 0U);
    REQUIRE(lxp_qual_root_ledger(ledger_path, result.batch_count,
                                 result.corpus_digest,
                                 result.root_ledger_digest) == LXP_OK);
    corpus = fopen(corpus_path, "rb");
    ledger = fopen(ledger_path, "rb");
    if (corpus == NULL || ledger == NULL) {
        if (corpus != NULL) (void)fclose(corpus);
        if (ledger != NULL) (void)fclose(ledger);
        (void)fprintf(stderr, "cannot reopen generated corpus and root ledger\n");
        return 1;
    }
    failed = check_boundaries(corpus, ledger, activity_count, &result);
    if (fclose(corpus) != 0) failed = 1;
    if (fclose(ledger) != 0) failed = 1;
    REQUIRE(failed == 0);
    (void)printf("finding 22.30.5.14: %llu/10 round-trip and %llu ledger entries agree\n",
                 (unsigned long long)activity_count,
                 (unsigned long long)result.batch_count);
    return 0;
}

int main(void)
{
    char directory[] = "/tmp/lxp-qual-replay-tail-XXXXXX";
    char corpus_path[128];
    char ledger_path[128];
    int length;
    int failed;

    REQUIRE(mkdtemp(directory) != NULL);
    length = snprintf(corpus_path, sizeof(corpus_path), "%s/corpus.lxq", directory);
    REQUIRE(length >= 0 && (size_t)length < sizeof(corpus_path));
    length = snprintf(ledger_path, sizeof(ledger_path), "%s/corpus.roots", directory);
    REQUIRE(length >= 0 && (size_t)length < sizeof(ledger_path));
    failed = finding_22_30_5_14_round_trip(corpus_path, ledger_path, 53U);
    failed |= finding_22_30_5_14_round_trip(corpus_path, ledger_path, 60U);
    if (unlink(corpus_path) != 0) failed = 1;
    if (unlink(ledger_path) != 0) failed = 1;
    if (rmdir(directory) != 0) failed = 1;
    return failed;
}
