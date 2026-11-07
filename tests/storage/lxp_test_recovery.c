#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_storage.h"
#include "layerx/lxp_fault.h"
#include "layerx/lxp_snapshot.h"

#include <dirent.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

typedef struct replay_state {
    uint8_t balance;
    uint8_t expected;
    int have_expected;
    int force_divergence;
} replay_state;

static lxp_result replay(void *opaque, const lxp_log_record_header *header,
                         const uint8_t *body)
{
    replay_state *state = (replay_state *)opaque;
    if (header->body_length != 1U) return LXP_FATAL_REPLAY_DIVERGENCE;
    if (header->record_kind == (uint8_t)LXP_LOG_CHECKPOINT)
        state->balance = body[0];
    else if (header->record_kind == (uint8_t)LXP_LOG_RECEIPT) {
        state->expected = body[0];
        state->have_expected = 1;
    } else if (header->record_kind == (uint8_t)LXP_LOG_STATE_DIFF) {
        state->balance = (uint8_t)(state->balance + body[0]);
        if (state->force_divergence != 0 || state->have_expected == 0 ||
            state->balance != state->expected)
            return LXP_FATAL_REPLAY_DIVERGENCE;
        state->have_expected = 0;
    }
    return LXP_OK;
}

static lxp_result recover_mode(lxp_log *log, bool complete_records)
{
    return complete_records ?
        lxp_log_recover_complete_records(log, NULL, NULL) :
        lxp_log_recover(log, NULL, NULL);
}

static int recover_existing_log(const char *prefix,
                                lxp_log_record_kind kind,
                                uint64_t sequence, uint8_t value,
                                bool complete_records)
{
    char directory[64];
    char path[128];
    lxp_log log;
    lxp_log_record_header header;
    uint8_t recovered = 0U;
    uint8_t appended = (uint8_t)(value + 1U);
    uint64_t durable_end;
    uint64_t appended_offset;
    uint64_t appended_end;
    lxp_result recovery_status;
    int length;
    length = snprintf(directory, sizeof(directory), "/tmp/%s-XXXXXX", prefix);
    if (length < 0 || (size_t)length >= sizeof(directory) ||
        mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 4096U) != LXP_OK)
        return 1;
    if (lxp_log_append(&log, kind, sequence, &value, 1U, NULL) != LXP_OK ||
        lxp_log_write_boundary(&log) != LXP_OK) return 1;
    durable_end = log.write_offset;
    if (lxp_log_close(&log) != LXP_OK ||
        snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U) < 0 ||
        lxp_log_open(&log, path) != LXP_OK || log.write_offset != 0U)
        return 1;
    recovery_status = recover_mode(&log, complete_records);
    if (recovery_status != LXP_OK ||
        log.write_offset != durable_end ||
        lxp_log_resume_sequence(&log) != sequence + 1U ||
        lxp_log_read(&log, 0U, &header, &recovered, sizeof(recovered)) !=
            LXP_OK ||
        header.record_kind != (uint8_t)kind ||
        header.global_sequence != sequence || recovered != value)
        return 1;
    if (lxp_log_append(&log, kind, lxp_log_resume_sequence(&log), &appended,
                       1U, &appended_offset) != LXP_OK ||
        appended_offset != durable_end ||
        lxp_log_write_boundary(&log) != LXP_OK)
        return 1;
    appended_end = log.write_offset;
    if (appended_end <= durable_end || lxp_log_close(&log) != LXP_OK ||
        lxp_log_open(&log, path) != LXP_OK || log.write_offset != 0U)
        return 1;
    recovery_status = recover_mode(&log, complete_records);
    if (recovery_status != LXP_OK ||
        log.write_offset != appended_end ||
        log.previous_record_offset != appended_offset ||
        lxp_log_resume_sequence(&log) != sequence + 2U ||
        lxp_log_read(&log, 0U, &header, &recovered, sizeof(recovered)) !=
            LXP_OK ||
        header.record_kind != (uint8_t)kind ||
        header.global_sequence != sequence || recovered != value ||
        lxp_log_read(&log, appended_offset, &header, &recovered,
                     sizeof(recovered)) != LXP_OK ||
        header.record_kind != (uint8_t)kind ||
        header.global_sequence != sequence + 1U || recovered != appended ||
        lxp_log_close(&log) != LXP_OK || unlink(path) != 0 ||
        rmdir(directory) != 0) return 1;
    return 0;
}

static int classify_read_failure(void)
{
    char directory[] = "/tmp/lxp-read-io-XXXXXX";
    char path[128];
    lxp_log log;
    uint64_t valid_end;
    uint64_t last;
    uint64_t next;
    lxp_result status;
    if (mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 4096U) != LXP_OK ||
        snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U) < 0)
        return 1;
    if (close(log.descriptor) != 0)
        return 1;
    status = lxp_log_scan_tail(&log, &valid_end, &last, &next);
    if (status != LXP_ERR_IO) {
        (void)fprintf(stderr, "closed-log scan returned %d\n", (int)status);
        return 1;
    }
    log.descriptor = -1;
    return unlink(path) == 0 && rmdir(directory) == 0 ? 0 : 1;
}

static int propagate_recovery_io_failure(const char *prefix,
                                         lxp_log_record_kind kind,
                                         bool complete_records)
{
    char directory[64];
    char path[128];
    uint8_t body = 0x93U;
    uint8_t recovered = 0U;
    uint64_t durable_end;
    lxp_log log;
    lxp_log before;
    lxp_log_record_header header;
    struct stat on_disk_before;
    struct stat on_disk_after;
    lxp_result status;
    int length = snprintf(directory, sizeof(directory),
                          "/tmp/%s-XXXXXX", prefix);
    if (length < 0 || (size_t)length >= sizeof(directory) ||
        mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 4096U) != LXP_OK ||
        lxp_log_append(&log, kind, 31U, &body, 1U, NULL) != LXP_OK ||
        lxp_log_write_boundary(&log) != LXP_OK)
        return 1;
    durable_end = log.write_offset;
    if (lxp_log_close(&log) != LXP_OK ||
        snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U) < 0 ||
        lxp_log_open(&log, path) != LXP_OK ||
        !log.has_durable_marker || log.durable_offset != durable_end ||
        stat(path, &on_disk_before) != 0)
        return 1;
    before = log;
    if (close(log.descriptor) != 0) return 1;
    status = recover_mode(&log, complete_records);
    if (status != LXP_ERR_IO) {
        (void)fprintf(stderr, "%s recovery returned %d\n", prefix,
                      (int)status);
        return 1;
    }
    if (log.write_offset != before.write_offset ||
        log.previous_record_offset != before.previous_record_offset ||
        log.next_sequence != before.next_sequence ||
        log.durable_offset != before.durable_offset ||
        log.durable_previous_record_offset !=
            before.durable_previous_record_offset ||
        log.durable_next_sequence != before.durable_next_sequence ||
        log.durable_generation != before.durable_generation ||
        log.has_durable_marker != before.has_durable_marker ||
        stat(path, &on_disk_after) != 0 ||
        on_disk_after.st_size != on_disk_before.st_size)
        return 1;
    log.descriptor = -1;
    if (lxp_log_open(&log, path) != LXP_OK ||
        recover_mode(&log, complete_records) != LXP_OK ||
        log.write_offset != durable_end ||
        lxp_log_resume_sequence(&log) != 32U ||
        lxp_log_read(&log, 0U, &header, &recovered, sizeof(recovered)) !=
            LXP_OK ||
        header.global_sequence != 31U || recovered != body ||
        lxp_log_close(&log) != LXP_OK || unlink(path) != 0 ||
        rmdir(directory) != 0)
        return 1;
    return 0;
}

static int refuse_receipt_only_canonical(void)
{
    char directory[] = "/tmp/lxp-receipt-only-XXXXXX";
    char path[128];
    uint8_t body = 0x63U;
    lxp_log log;
    if (mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 4096U) != LXP_OK ||
        lxp_log_append(&log, LXP_LOG_RECEIPT, 9U, &body, 1U, NULL) != LXP_OK ||
        lxp_log_write_boundary(&log) != LXP_OK ||
        lxp_log_close(&log) != LXP_OK ||
        snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U) < 0 ||
        lxp_log_open(&log, path) != LXP_OK ||
        lxp_log_recover(&log, NULL, NULL) != LXP_ERR_LOG_CORRUPT ||
        log.write_offset != 0U || lxp_log_close(&log) != LXP_OK ||
        unlink(path) != 0 || rmdir(directory) != 0) return 1;
    return 0;
}

static int recover_torn_tail(const char *prefix,
                             lxp_log_record_kind kind,
                             bool complete_records)
{
    char directory[64];
    char path[128];
    uint8_t body = 0x71U;
    const uint8_t partial[] = {'L', 'X', 'P', 'L', 1U, 0U, 0U};
    uint8_t after = 0xffU;
    uint64_t durable_end;
    lxp_log log;
    lxp_result status;
    int length = snprintf(directory, sizeof(directory),
                          "/tmp/%s-XXXXXX", prefix);
    if (length < 0 || (size_t)length >= sizeof(directory) ||
        mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 4096U) != LXP_OK ||
        lxp_log_append(&log, kind, 13U, &body, 1U, NULL) != LXP_OK ||
        lxp_log_write_boundary(&log) != LXP_OK)
        return 1;
    durable_end = log.write_offset;
    if (pwrite(log.descriptor, partial, sizeof(partial),
               (off_t)durable_end) != (ssize_t)sizeof(partial) ||
        ftruncate(log.descriptor,
                  (off_t)(durable_end + sizeof(partial))) != 0 ||
        lxp_log_close(&log) != LXP_OK ||
        snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U) < 0 ||
        lxp_log_open(&log, path) != LXP_OK)
        return 1;
    status = complete_records ?
        lxp_log_recover_complete_records(&log, NULL, NULL) :
        lxp_log_recover(&log, NULL, NULL);
    if (status != LXP_OK || log.write_offset != durable_end ||
        lxp_log_resume_sequence(&log) != 14U ||
        pread(log.descriptor, &after, 1U, (off_t)durable_end) != 1 ||
        after != 0U || lxp_log_close(&log) != LXP_OK ||
        unlink(path) != 0 || rmdir(directory) != 0)
        return 1;
    return 0;
}

static int refuse_corrupt_chain(const char *prefix,
                                lxp_log_record_kind kind,
                                bool complete_records)
{
    char directory[64];
    char path[128];
    uint8_t body = 0x82U;
    uint8_t corrupt;
    uint64_t second_offset;
    struct stat before;
    struct stat after;
    lxp_log log;
    lxp_result status;
    int length = snprintf(directory, sizeof(directory),
                          "/tmp/%s-XXXXXX", prefix);
    if (length < 0 || (size_t)length >= sizeof(directory) ||
        mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 4096U) != LXP_OK ||
        lxp_log_append(&log, kind, 21U, &body, 1U, NULL) != LXP_OK ||
        lxp_log_append(&log, kind, 22U, &body, 1U, &second_offset) != LXP_OK ||
        lxp_log_write_boundary(&log) != LXP_OK ||
        pread(log.descriptor, &corrupt, 1U,
              (off_t)(second_offset + 31U)) != 1)
        return 1;
    corrupt ^= 1U;
    if (pwrite(log.descriptor, &corrupt, 1U,
               (off_t)(second_offset + 31U)) != 1 ||
        fstat(log.descriptor, &before) != 0 ||
        lxp_log_close(&log) != LXP_OK ||
        snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U) < 0 ||
        lxp_log_open(&log, path) != LXP_OK)
        return 1;
    status = complete_records ?
        lxp_log_recover_complete_records(&log, NULL, NULL) :
        lxp_log_recover(&log, NULL, NULL);
    if (status != LXP_ERR_LOG_CORRUPT || log.write_offset != 0U ||
        fstat(log.descriptor, &after) != 0 ||
        before.st_size != after.st_size ||
        lxp_log_close(&log) != LXP_OK || unlink(path) != 0 ||
        rmdir(directory) != 0)
        return 1;
    return 0;
}

static int recover_from_checkpoint(bool complete_records)
{
    char directory[] = "/tmp/lxp-checkpoint-restart-XXXXXX";
    char path[128];
    uint8_t checkpoint = 9U;
    uint8_t activity = 0x31U;
    uint8_t expected = 10U;
    uint8_t increment = 1U;
    replay_state state = {0U, 0U, 0, 0};
    lxp_log log;
    if (mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 4096U) != LXP_OK ||
        lxp_log_append(&log, LXP_LOG_CHECKPOINT, 5U, &checkpoint, 1U,
                       NULL) != LXP_OK ||
        lxp_log_append(&log, LXP_LOG_ACTIVITY, 6U, &activity, 1U,
                       NULL) != LXP_OK ||
        lxp_log_append(&log, LXP_LOG_RECEIPT, 6U, &expected, 1U,
                       NULL) != LXP_OK ||
        lxp_log_append(&log, LXP_LOG_STATE_DIFF, 6U, &increment, 1U,
                       NULL) != LXP_OK ||
        lxp_log_write_boundary(&log) != LXP_OK ||
        lxp_log_close(&log) != LXP_OK ||
        snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U) < 0 ||
        lxp_log_open(&log, path) != LXP_OK ||
        (complete_records ?
             lxp_log_recover_complete_records(&log, replay, &state) :
             lxp_log_recover(&log, replay, &state)) != LXP_OK ||
        state.balance != 10U || state.have_expected != 0 ||
        lxp_log_resume_sequence(&log) != 7U ||
        lxp_log_close(&log) != LXP_OK || unlink(path) != 0 ||
        rmdir(directory) != 0)
        return 1;
    return 0;
}

typedef struct snapshot_restart_fixture {
    const char *directory;
    const lxp_snapshot_manifest_record *manifest;
    lxp_byte_span snapshot;
} snapshot_restart_fixture;

static lxp_result snapshot_restart_workload(void *context)
{
    const snapshot_restart_fixture *fixture = context;
    return lxp_snapshot_store_write(fixture->directory, fixture->manifest,
                                     fixture->snapshot.bytes,
                                     fixture->snapshot.length);
}

static int snapshot_temporary_files(const char *directory, int remove_files)
{
    DIR *entries = opendir(directory);
    struct dirent *entry;
    int count = 0;
    if (entries == NULL) return -1;
    while ((entry = readdir(entries)) != NULL) {
        char path[256];
        if (strstr(entry->d_name, ".lxs.tmp.") == NULL) continue;
        ++count;
        if (remove_files) {
            int length = snprintf(path, sizeof(path), "%s/%s", directory,
                                    entry->d_name);
            if (length < 0 || (size_t)length >= sizeof(path) ||
                unlink(path) != 0) {
                (void)closedir(entries);
                return -1;
            }
        }
    }
    return closedir(entries) == 0 ? count : -1;
}

static int test_snapshot_crash_same_sequence_retry(void)
{
    static const lxp_fault_boundary boundaries[] = {
        LXP_FAULT_CHECKPOINT_HEADER_WRITTEN,
        LXP_FAULT_CHECKPOINT_BODY_WRITTEN,
        LXP_FAULT_CHECKPOINT_FILE_SYNCED
    };
    uint8_t snapshot_storage[8192];
    uint8_t read_storage[8192];
    uint8_t root[32];
    uint8_t key[32] = { 0x53U };
    uint8_t recovered_root[32];
    uint64_t parameters = 1U;
    lxp_u128 value = { 0U, 0U };
    lxp_state_store original_state;
    lxp_state_store recovered_state;
    lxp_state_journal original_journal;
    lxp_state_journal recovered_journal;
    lxp_kernel *original = calloc(1U, sizeof(*original));
    lxp_kernel *recovered = calloc(1U, sizeof(*recovered));
    lxp_arena arena;
    lxp_byte_span snapshot;
    lxp_snapshot_manifest_record manifest;
    size_t i;
    value.lo = 79U;
    if (original == NULL || recovered == NULL) return 1;
    original->state = &original_state;
    original->journal = &original_journal;
    original->parameter_set = &parameters;
    recovered->state = &recovered_state;
    recovered->journal = &recovered_journal;
    recovered->parameter_set = &parameters;
    if (lxp_state_store_init(&original_state, 0U) != LXP_OK ||
        lxp_state_store_init(&recovered_state, 0U) != LXP_OK ||
        lxp_state_journal_open(&original_state, 0U, &original_journal) != LXP_OK ||
        lxp_state_journal_set(&original_journal, key, value) != LXP_OK ||
        lxp_state_journal_commit(&original_journal) != LXP_OK ||
        lxp_state_root(original, root) != LXP_OK ||
        lxp_arena_init(&arena, snapshot_storage, sizeof(snapshot_storage)) !=
            LXP_OK ||
        lxp_snapshot_write(original, 0U, &arena, &snapshot) != LXP_OK ||
        lxp_snapshot_manifest(snapshot.bytes, snapshot.length, 0U, root,
                               root, &manifest) != LXP_OK)
        return 1;
    for (i = 0U; i < sizeof(boundaries) / sizeof(boundaries[0]); ++i) {
        char directory[] = "/tmp/lxp-snapshot-retry-XXXXXX";
        char path[128];
        lxp_snapshot_manifest_record stored_manifest;
        lxp_snapshot_manifest_record conflicting_manifest;
        lxp_byte_span stored_snapshot;
        snapshot_restart_fixture fixture = { directory, &manifest, snapshot };
        lxp_arena read_arena;
        lxp_u128 recovered_value;
        uint8_t conflicting_root[32];
        bool found;
        int child_status;
        int descriptor;
        uint8_t byte;
        if (mkdtemp(directory) == NULL ||
            snprintf(path, sizeof(path), "%s/%020u.lxs", directory, 0U) < 0 ||
            lxp_fault_crash_at_boundary(boundaries[i], 1U,
                snapshot_restart_workload, &fixture, &child_status) != LXP_OK ||
            access(path, F_OK) == 0 ||
            snapshot_temporary_files(directory, 0) != 1 ||
            lxp_snapshot_store_write(directory, &manifest, snapshot.bytes,
                                      snapshot.length) != LXP_OK ||
            snapshot_temporary_files(directory, 0) != 1 ||
            lxp_arena_init(&read_arena, read_storage, sizeof(read_storage)) !=
                LXP_OK ||
            lxp_snapshot_store_read(path, &read_arena, &stored_manifest,
                                     &stored_snapshot) != LXP_OK ||
            stored_manifest.global_sequence != 0U ||
            memcmp(stored_manifest.snapshot_digest, manifest.snapshot_digest,
                   32U) != 0 || stored_snapshot.length != snapshot.length ||
            memcmp(stored_snapshot.bytes, snapshot.bytes, snapshot.length) != 0 ||
            lxp_snapshot_load(stored_snapshot.bytes, stored_snapshot.length,
                               &stored_manifest, recovered) != LXP_OK ||
            lxp_snapshot_verify_root(recovered, &stored_manifest) != LXP_OK ||
            lxp_state_root(recovered, recovered_root) != LXP_OK ||
            memcmp(root, recovered_root, 32U) != 0 ||
            recovered_state.next_sequence != 1U || recovered_state.count != 1U ||
            lxp_state_store_get(&recovered_state, key, &recovered_value,
                                 &found) != LXP_OK || !found ||
            recovered_value.lo != value.lo || recovered_value.hi != value.hi ||
            lxp_snapshot_store_write(directory, &manifest, snapshot.bytes,
                                      snapshot.length) != LXP_ERR_NON_CANONICAL)
            return 1;
        (void)memcpy(conflicting_root, root, sizeof(conflicting_root));
        conflicting_root[0] ^= 1U;
        if (lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length, 0U,
                conflicting_root, root, &conflicting_manifest) != LXP_OK ||
            lxp_snapshot_store_write(directory, &conflicting_manifest,
                snapshot.bytes, snapshot.length) != LXP_ERR_NON_CANONICAL)
            return 1;
        conflicting_manifest = manifest;
        conflicting_manifest.snapshot_digest[0] ^= 1U;
        if (lxp_snapshot_store_write(directory, &conflicting_manifest,
                snapshot.bytes, snapshot.length) != LXP_ERR_SNAPSHOT_MISMATCH ||
            lxp_arena_init(&read_arena, read_storage, sizeof(read_storage)) !=
                LXP_OK ||
            lxp_snapshot_store_read(path, &read_arena, &stored_manifest,
                                     &stored_snapshot) != LXP_OK ||
            memcmp(stored_manifest.canonical_state_root, root, 32U) != 0)
            return 1;
        descriptor = open(path, O_RDWR | O_CLOEXEC);
        if (descriptor < 0 || pread(descriptor, &byte, 1U, 116) != 1)
            return 1;
        byte ^= 1U;
        if (pwrite(descriptor, &byte, 1U, 116) != 1 ||
            fdatasync(descriptor) != 0 || close(descriptor) != 0 ||
            lxp_arena_init(&read_arena, read_storage, sizeof(read_storage)) !=
                LXP_OK ||
            lxp_snapshot_store_read(path, &read_arena, &stored_manifest,
                                     &stored_snapshot) != LXP_ERR_SNAPSHOT_MISMATCH ||
            snapshot_temporary_files(directory, 1) != 1 ||
            unlink(path) != 0 || rmdir(directory) != 0)
            return 1;
    }
    if (lxp_state_store_destroy(&original_state) != LXP_OK ||
        lxp_state_store_destroy(&recovered_state) != LXP_OK)
        return 1;
    free(original);
    free(recovered);
    return 0;
}

int main(void)
{
    char directory[] = "/tmp/lxp-recovery-XXXXXX";
    char path[128];
    uint8_t one = 1U;
    uint8_t expected = 1U;
    uint8_t partial[7] = { 'L', 'X', 'P', 'L', 1U, 0U, 0U };
    lxp_log log;
    replay_state state = { 0U, 0U, 0, 0 };
    replay_state divergent = { 0U, 0U, 0, 1 };
    uint64_t incomplete_offset;
    if (mkdtemp(directory) == NULL ||
        lxp_log_segment_create(&log, directory, 0U, 4096U) != LXP_OK) {
        (void)fprintf(stderr, "recovery setup failed\n");
        return 1;
    }
    if (lxp_log_append(&log, LXP_LOG_ACTIVITY, 0U, &one, 1U, NULL) != LXP_OK ||
        lxp_log_sync(&log) != LXP_OK ||
        lxp_log_append(&log, LXP_LOG_RECEIPT, 0U, &expected, 1U, NULL) != LXP_OK ||
        lxp_log_append(&log, LXP_LOG_STATE_DIFF, 0U, &one, 1U, NULL) != LXP_OK ||
        lxp_log_sync(&log) != LXP_OK ||
        lxp_log_append(&log, LXP_LOG_ACTIVITY, 1U, &one, 1U,
                       &incomplete_offset) != LXP_OK) {
        (void)fprintf(stderr, "recovery append failed\n");
        return 1;
    }
    if (pwrite(log.descriptor, partial, sizeof(partial),
               (off_t)log.write_offset) != (ssize_t)sizeof(partial) ||
        ftruncate(log.descriptor,
                  (off_t)(log.write_offset + sizeof(partial))) != 0 ||
        lxp_log_close(&log) != LXP_OK) {
        (void)fprintf(stderr, "partial tail write failed\n");
        return 1;
    }
    if (snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U) < 0 ||
        lxp_log_open(&log, path) != LXP_OK) {
        (void)fprintf(stderr, "recovery open failed\n");
        return 1;
    }
    {
        lxp_result recovery_status = lxp_log_recover(&log, replay, &state);
        if (recovery_status != LXP_OK) {
            (void)fprintf(stderr, "recovery operation failed: %d\n",
                          (int)recovery_status);
            return 1;
        }
    }
    if (state.balance != 1U || state.have_expected != 0 ||
        lxp_log_resume_sequence(&log) != 1U ||
        log.write_offset != incomplete_offset) {
        (void)fprintf(stderr, "recovery state failed: %u %d %llu %llu %llu\n",
                      state.balance, state.have_expected,
                      (unsigned long long)lxp_log_resume_sequence(&log),
                      (unsigned long long)log.write_offset,
                      (unsigned long long)incomplete_offset);
        return 1;
    }
    if (lxp_log_recover(&log, replay, &divergent) !=
        LXP_FATAL_REPLAY_DIVERGENCE) {
        (void)fprintf(stderr, "divergence was not fatal\n");
        return 1;
    }
    if (lxp_log_close(&log) != LXP_OK || unlink(path) != 0 ||
        rmdir(directory) != 0) return 1;
    if (recover_existing_log("lxp-canonical-restart", LXP_LOG_CHECKPOINT,
                             7U, 0x41U, false) != 0 ||
        recover_existing_log("lxp-canonical-occ-restart", LXP_LOG_CHECKPOINT,
                             7U, 0x43U, true) != 0 ||
        recover_existing_log("lxp-activity-restart", LXP_LOG_ACTIVITY,
                             11U, 0x52U, true) != 0) {
        (void)fprintf(stderr, "durable restart recovery failed\n");
        return 1;
    }
    if (classify_read_failure() != 0) {
        (void)fprintf(stderr, "recovery IO classification failed\n");
        return 1;
    }
    if (propagate_recovery_io_failure("lxp-canonical-io",
                                      LXP_LOG_CHECKPOINT, false) != 0 ||
        propagate_recovery_io_failure("lxp-canonical-occ-io",
                                      LXP_LOG_CHECKPOINT, true) != 0 ||
        propagate_recovery_io_failure("lxp-activity-io",
                                      LXP_LOG_ACTIVITY, true) != 0) {
        (void)fprintf(stderr, "recovery IO failure was not propagated\n");
        return 1;
    }
    if (refuse_receipt_only_canonical() != 0) {
        (void)fprintf(stderr, "receipt-only canonical log was accepted\n");
        return 1;
    }
    if (recover_torn_tail("lxp-canonical-torn", LXP_LOG_CHECKPOINT,
                          false) != 0 ||
        recover_torn_tail("lxp-canonical-occ-torn", LXP_LOG_CHECKPOINT,
                          true) != 0 ||
        recover_torn_tail("lxp-activity-torn", LXP_LOG_ACTIVITY,
                          true) != 0) {
        (void)fprintf(stderr, "torn-tail recovery failed\n");
        return 1;
    }
    if (refuse_corrupt_chain("lxp-canonical-corrupt", LXP_LOG_CHECKPOINT,
                             false) != 0 ||
        refuse_corrupt_chain("lxp-canonical-occ-corrupt", LXP_LOG_CHECKPOINT,
                             true) != 0 ||
        refuse_corrupt_chain("lxp-activity-corrupt", LXP_LOG_ACTIVITY,
                             true) != 0) {
        (void)fprintf(stderr, "corrupt-chain recovery was not refused\n");
        return 1;
    }
    if (recover_from_checkpoint(false) != 0 ||
        recover_from_checkpoint(true) != 0) {
        (void)fprintf(stderr, "checkpoint restart replay failed\n");
        return 1;
    }
    if (test_snapshot_crash_same_sequence_retry() != 0) {
        (void)fprintf(stderr, "snapshot crash/same-sequence retry failed\n");
        return 1;
    }
    (void)printf("PASS test_snapshot_crash_same_sequence_retry\n");
    return 0;
}
