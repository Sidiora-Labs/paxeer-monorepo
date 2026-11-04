#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_hash.h"
#include "layerx/lxp_fault.h"
#include "layerx/lxp_projection.h"

#include <sqlite3.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static int database_digest(lxp_projection *projection, uint8_t digest[32])
{
    static const char query[] =
        "SELECT hex(account_id)||hex(asset_id)||hex(amount) FROM balances "
        "UNION ALL SELECT hex(activity_id)||hex(idempotency_key)||"
        "printf('%lld:%d:',global_sequence,result_code)||hex(canonical_receipt) "
        "FROM receipts ORDER BY 1";
    sqlite3_stmt *statement = NULL;
    lxp_hash_context hash;
    if (sqlite3_prepare_v2((sqlite3 *)projection->database, query, -1,
                           &statement, NULL) != SQLITE_OK) return 0;
    lxp_hash_init(&hash);
    while (sqlite3_step(statement) == SQLITE_ROW) {
        const void *text = sqlite3_column_text(statement, 0);
        int length = sqlite3_column_bytes(statement, 0);
        if (length < 0) return 0;
        if (lxp_hash_update(&hash, text, (size_t)length) != LXP_OK) return 0;
    }
    (void)sqlite3_finalize(statement);
    return lxp_hash_final(&hash, digest) == LXP_OK;
}

static void fill_record(lxp_projection_record *record, uint8_t id,
                        const uint8_t *receipt, size_t receipt_length)
{
    (void)memset(record, 0, sizeof(*record));
    record->activity_id[0] = id;
    record->idempotency_key[0] = (uint8_t)(id + 10U);
    record->account_id[0] = (uint8_t)(id + 20U);
    record->asset_id[0] = 1U;
    record->amount[15] = (uint8_t)(id + 30U);
    record->receipt = receipt;
    record->receipt_length = receipt_length;
}

typedef struct projection_restart_paths {
    const char *database_path;
    const char *log_path;
} projection_restart_paths;

static lxp_result interrupted_projection_rebuild(void *context)
{
    const projection_restart_paths *paths = context;
    lxp_projection projection;
    lxp_log log;
    lxp_result status = lxp_projection_open(&projection, paths->database_path,
                                            "migrations/0001_projection.sql");
    if (status == LXP_OK) status = lxp_log_open(&log, paths->log_path);
    if (status == LXP_OK)
        status = lxp_projection_rebuild(&projection, &log,
                                         "migrations/0001_projection.sql");
    return status;
}

static int projection_recovered_records(lxp_projection *projection)
{
    sqlite3_stmt *statement = NULL;
    uint64_t sequence;
    if (sqlite3_prepare_v2((sqlite3 *)projection->database,
            "SELECT r.global_sequence, r.activity_id, r.idempotency_key, "
            "r.canonical_receipt, b.amount FROM receipts r JOIN balances b "
            "ON b.account_id=? AND b.asset_id=? WHERE r.global_sequence=?",
            -1, &statement, NULL) != SQLITE_OK) return 0;
    for (sequence = 0U; sequence < 3U; ++sequence) {
        lxp_projection_record expected;
        const uint8_t receipt[] = { 4U, 5U };
        fill_record(&expected, (uint8_t)(sequence + 1U), receipt,
                     sizeof(receipt));
        if (sqlite3_bind_blob(statement, 1, expected.account_id, 32,
                               SQLITE_TRANSIENT) != SQLITE_OK ||
            sqlite3_bind_blob(statement, 2, expected.asset_id, 32,
                               SQLITE_TRANSIENT) != SQLITE_OK ||
            sqlite3_bind_int64(statement, 3, (sqlite3_int64)sequence) !=
                SQLITE_OK || sqlite3_step(statement) != SQLITE_ROW ||
            sqlite3_column_int64(statement, 0) != (sqlite3_int64)sequence ||
            sqlite3_column_bytes(statement, 1) != 32 ||
            memcmp(sqlite3_column_blob(statement, 1), expected.activity_id,
                   32U) != 0 ||
            sqlite3_column_bytes(statement, 2) != 32 ||
            memcmp(sqlite3_column_blob(statement, 2), expected.idempotency_key,
                   32U) != 0 ||
            sqlite3_column_bytes(statement, 3) != (int)sizeof(receipt) ||
            memcmp(sqlite3_column_blob(statement, 3), receipt,
                   sizeof(receipt)) != 0 ||
            sqlite3_column_bytes(statement, 4) != 16 ||
            memcmp(sqlite3_column_blob(statement, 4), expected.amount, 16U) != 0) {
            (void)sqlite3_finalize(statement);
            return 0;
        }
        if (sqlite3_step(statement) != SQLITE_DONE ||
            sqlite3_reset(statement) != SQLITE_OK) {
            (void)sqlite3_finalize(statement);
            return 0;
        }
    }
    if (sqlite3_finalize(statement) != SQLITE_OK) return 0;
    if (sqlite3_prepare_v2((sqlite3 *)projection->database,
            "SELECT (SELECT count(*) FROM receipts), count(*) FROM balances",
            -1, &statement, NULL) != SQLITE_OK) return 0;
    if (sqlite3_step(statement) != SQLITE_ROW ||
        sqlite3_column_int(statement, 0) != 3 ||
        sqlite3_column_int(statement, 1) != 3) {
        (void)sqlite3_finalize(statement);
        return 0;
    }
    return sqlite3_finalize(statement) == SQLITE_OK;
}

static int test_projection_stale_restart_rebuild(void)
{
    char directory[] = "/tmp/lxp-stale-restart-XXXXXX";
    char log_path[128];
    char database_path[128];
    const uint8_t activity[] = { 9U };
    const uint8_t receipt[] = { 4U, 5U };
    uint8_t encoded[256];
    size_t encoded_length;
    lxp_projection_record records[3];
    lxp_projection projection;
    lxp_log log;
    uint64_t sequence;
    uint64_t watermark;
    bool has_watermark;
    uint8_t before[32];
    uint8_t after[32];
    int child_status;
    projection_restart_paths paths = { database_path, log_path };
    if (mkdtemp(directory) == NULL ||
        snprintf(log_path, sizeof(log_path), "%s/%020u.lxp", directory, 0U) < 0 ||
        snprintf(database_path, sizeof(database_path), "%s/projection.db",
                 directory) < 0 ||
        lxp_log_segment_create(&log, directory, 0U, 8192U) != LXP_OK ||
        lxp_projection_open(&projection, database_path,
                             "migrations/0001_projection.sql") != LXP_OK)
        return 1;
    for (sequence = 0U; sequence < 3U; ++sequence) {
        fill_record(&records[sequence], (uint8_t)(sequence + 1U), receipt,
                     sizeof(receipt));
        if (lxp_projection_record_encode(&records[sequence], encoded,
                sizeof(encoded), &encoded_length) != LXP_OK ||
            lxp_log_append(&log, LXP_LOG_ACTIVITY, sequence, activity,
                            sizeof(activity), NULL) != LXP_OK ||
            lxp_log_append(&log, LXP_LOG_RECEIPT, sequence, encoded,
                            (uint32_t)encoded_length, NULL) != LXP_OK ||
            lxp_log_sync(&log) != LXP_OK)
            return 1;
    }
    if (lxp_projection_apply(&projection, 0U, &records[0]) != LXP_OK ||
        !database_digest(&projection, before) ||
        lxp_projection_apply(&projection, 1U, &records[0]) !=
            LXP_ERR_PROJECTION_STALE || !projection.stale ||
        lxp_projection_close(&projection) != LXP_OK ||
        lxp_log_close(&log) != LXP_OK ||
        lxp_projection_open(&projection, database_path,
                             "migrations/0001_projection.sql") != LXP_OK ||
        !projection.stale ||
        lxp_projection_watermark(&projection, &watermark, &has_watermark) !=
            LXP_OK || !has_watermark || watermark != 0U ||
        lxp_projection_apply(&projection, 2U, &records[2]) !=
            LXP_ERR_PROJECTION_STALE || !projection.stale ||
        lxp_projection_apply(&projection, 1U, &records[1]) !=
            LXP_ERR_PROJECTION_STALE ||
        !database_digest(&projection, after) ||
        memcmp(before, after, sizeof(before)) != 0 ||
        lxp_projection_close(&projection) != LXP_OK ||
        lxp_fault_crash_at_boundary(LXP_FAULT_INDEX_WATERMARK_WRITTEN, 2U,
            interrupted_projection_rebuild, &paths, &child_status) != LXP_OK ||
        lxp_projection_open(&projection, database_path,
                             "migrations/0001_projection.sql") != LXP_OK ||
        !projection.stale ||
        lxp_projection_watermark(&projection, &watermark, &has_watermark) !=
            LXP_OK || !has_watermark || watermark != 0U ||
        !database_digest(&projection, after) ||
        memcmp(before, after, sizeof(before)) != 0 ||
        lxp_log_open(&log, log_path) != LXP_OK ||
        lxp_log_recover(&log, NULL, NULL) != LXP_OK ||
        lxp_log_resume_sequence(&log) != 3U ||
        lxp_projection_rebuild(&projection, &log,
                                "migrations/0001_projection.sql") != LXP_OK ||
        projection.stale || !projection_recovered_records(&projection) ||
        lxp_projection_watermark(&projection, &watermark, &has_watermark) !=
            LXP_OK || !has_watermark || watermark != 2U ||
        lxp_projection_close(&projection) != LXP_OK ||
        lxp_projection_open(&projection, database_path,
                             "migrations/0001_projection.sql") != LXP_OK ||
        projection.stale || !projection_recovered_records(&projection) ||
        lxp_projection_watermark(&projection, &watermark, &has_watermark) !=
            LXP_OK || !has_watermark || watermark != 2U ||
        lxp_projection_close(&projection) != LXP_OK ||
        lxp_log_close(&log) != LXP_OK || unlink(log_path) != 0 ||
        unlink(database_path) != 0 || rmdir(directory) != 0)
        return 1;
    return 0;
}

int main(void)
{
    char directory[] = "/tmp/lxp-rebuild-log-XXXXXX";
    char log_path[128];
    char first_db[] = "/tmp/lxp-rebuild-a-XXXXXX";
    char second_db[] = "/tmp/lxp-rebuild-b-XXXXXX";
    const uint8_t activity[] = { 9U };
    const uint8_t receipt_bytes[] = { 4U, 5U };
    uint8_t encoded[256];
    size_t encoded_length;
    lxp_projection_record record;
    lxp_projection projection;
    lxp_log log;
    uint8_t first_digest[32];
    uint8_t second_digest[32];
    uint8_t preserved_digest[32];
    uint64_t receipt_offsets[2];
    uint8_t corrupt;
    int first_fd = mkstemp(first_db);
    int second_fd = mkstemp(second_db);
    uint64_t sequence;
    if (first_fd < 0 || second_fd < 0 || close(first_fd) != 0 ||
        close(second_fd) != 0 || unlink(first_db) != 0 || unlink(second_db) != 0 ||
        mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 8192U) != LXP_OK)
        return 1;
    for (sequence = 0U; sequence < 2U; ++sequence) {
        fill_record(&record, (uint8_t)(sequence + 1U), receipt_bytes,
                    sizeof(receipt_bytes));
        if (lxp_projection_record_encode(&record, encoded, sizeof(encoded),
                                         &encoded_length) != LXP_OK ||
            lxp_log_append(&log, LXP_LOG_ACTIVITY, sequence, activity, 1U,
                           NULL) != LXP_OK || lxp_log_sync(&log) != LXP_OK ||
            lxp_log_append(&log, LXP_LOG_RECEIPT, sequence, encoded,
                           (uint32_t)encoded_length,
                           &receipt_offsets[sequence]) != LXP_OK ||
            lxp_log_append(&log, LXP_LOG_STATE_DIFF, sequence, activity, 1U,
                           NULL) != LXP_OK || lxp_log_sync(&log) != LXP_OK)
            return 1;
    }
    if (lxp_log_close(&log) != LXP_OK ||
        snprintf(log_path, sizeof(log_path), "%s/%020u.lxp", directory,
                 0U) < 0 ||
        lxp_log_open(&log, log_path) != LXP_OK ||
        lxp_log_recover(&log, NULL, NULL) != LXP_OK ||
        lxp_log_resume_sequence(&log) != 2U)
        return 1;
    if (lxp_projection_open(&projection, first_db,
                            "migrations/0001_projection.sql") != LXP_OK ||
        lxp_projection_rebuild(&projection, &log,
                               "migrations/0001_projection.sql") != LXP_OK ||
        !database_digest(&projection, first_digest) ||
        lxp_projection_close(&projection) != LXP_OK) return 1;
    if (lxp_projection_open(&projection, second_db,
                            "migrations/0001_projection.sql") != LXP_OK ||
        lxp_projection_rebuild(&projection, &log,
                               "migrations/0001_projection.sql") != LXP_OK ||
        !database_digest(&projection, second_digest) ||
        memcmp(first_digest, second_digest, sizeof(first_digest)) != 0 ||
        pread(log.descriptor, &corrupt, 1U,
              (off_t)(receipt_offsets[1] + LXP_LOG_HEADER_BYTES)) != 1)
        return 1;
    corrupt ^= 1U;
    if (pwrite(log.descriptor, &corrupt, 1U,
               (off_t)(receipt_offsets[1] + LXP_LOG_HEADER_BYTES)) != 1 ||
        lxp_projection_rebuild(&projection, &log,
                               "migrations/0001_projection.sql") !=
            LXP_ERR_LOG_CORRUPT ||
        !database_digest(&projection, preserved_digest) ||
        memcmp(second_digest, preserved_digest, sizeof(second_digest)) != 0 ||
        lxp_projection_close(&projection) != LXP_OK)
        return 1;
    if (lxp_log_close(&log) != LXP_OK || unlink(log_path) != 0 ||
        rmdir(directory) != 0 || unlink(first_db) != 0 || unlink(second_db) != 0)
        return 1;
    if (test_projection_stale_restart_rebuild() != 0) {
        (void)fprintf(stderr, "stale projection restart/rebuild failed\n");
        return 1;
    }
    (void)printf("PASS test_projection_stale_restart_rebuild\n");
    return 0;
}
