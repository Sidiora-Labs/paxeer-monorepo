#define _POSIX_C_SOURCE 200809L

#include "lxp_kernel_case.h"

#include "layerx/lxp_storage.h"

#include <dlfcn.h>
#include <stdlib.h>
#include <sys/types.h>
#include <unistd.h>

static int log_append_refuses_retired_kinds(void)
{
    static const struct {
        lxp_log_record_kind kind;
        lxp_result expected;
    } attempts[] = {
        { LXP_LOG_ACTIVITY, LXP_OK },
        { LXP_LOG_RECEIPT, LXP_OK },
        { LXP_LOG_BATCH_HEADER, LXP_ERR_NON_CANONICAL },
        { LXP_LOG_CHECKPOINT, LXP_OK },
        { LXP_LOG_STATE_DIFF, LXP_OK },
        { LXP_LOG_ORACLE, LXP_ERR_NON_CANONICAL },
        { LXP_LOG_GENESIS, LXP_OK },
        { LXP_LOG_REPLICA_ACK, LXP_ERR_NON_CANONICAL },
        { LXP_LOG_BATCH_BODY, LXP_ERR_NON_CANONICAL },
        { (lxp_log_record_kind)0, LXP_ERR_NON_CANONICAL },
        { (lxp_log_record_kind)10, LXP_ERR_NON_CANONICAL },
        { (lxp_log_record_kind)257, LXP_ERR_NON_CANONICAL },
        { (lxp_log_record_kind)-1, LXP_ERR_NON_CANONICAL }
    };
    static const uint8_t body[] = { 0x01U, 0x02U, 0x80U, 0xffU };
    char directory[] = "/tmp/lxp-log-kinds-XXXXXX";
    char path[128];
    lxp_log log;
    size_t i;
    int length;

    LXP_CASE_CHECK(mkdtemp(directory) != NULL);
    length = snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U);
    LXP_CASE_CHECK(length > 0 && (size_t)length < sizeof(path));
    LXP_CASE_CHECK(lxp_log_segment_create(&log, directory, 0U, 4096U) ==
                   LXP_OK);
    for (i = 0U; i < sizeof(attempts) / sizeof(attempts[0]); ++i) {
        uint64_t before = log.write_offset;
        uint64_t previous = log.previous_record_offset;
        uint64_t next = log.next_sequence;
        uint64_t offset = UINT64_MAX;
        uint8_t before_bytes[64];
        uint8_t after_bytes[sizeof(before_bytes)];

        LXP_CASE_CHECK(pread(log.descriptor, before_bytes, sizeof(before_bytes),
                              (off_t)before) == (ssize_t)sizeof(before_bytes));
        LXP_CASE_CHECK(lxp_log_append(&log, attempts[i].kind, (uint64_t)i,
                           body, (uint32_t)sizeof(body), &offset) ==
                       attempts[i].expected);
        if (attempts[i].expected == LXP_OK) {
            lxp_log_record_header header;
            uint8_t decoded[sizeof(body)];

            LXP_CASE_CHECK(offset == before);
            LXP_CASE_CHECK(log.write_offset == before +
                           LXP_LOG_HEADER_BYTES + sizeof(body));
            LXP_CASE_CHECK(log.previous_record_offset == before);
            LXP_CASE_CHECK(log.next_sequence == (uint64_t)i + 1U);
            LXP_CASE_CHECK(lxp_log_read(&log, offset, &header, decoded,
                                       sizeof(decoded)) == LXP_OK);
            LXP_CASE_CHECK(header.record_kind == (uint8_t)attempts[i].kind);
            LXP_CASE_CHECK(header.global_sequence == (uint64_t)i);
            LXP_CASE_CHECK(header.previous_record_offset == previous);
            LXP_CASE_CHECK(header.body_length == sizeof(body));
            LXP_CASE_CHECK(memcmp(decoded, body, sizeof(body)) == 0);
        } else {
            LXP_CASE_CHECK(log.write_offset == before);
            LXP_CASE_CHECK(log.previous_record_offset == previous);
            LXP_CASE_CHECK(log.next_sequence == next);
            LXP_CASE_CHECK(offset == UINT64_MAX);
            LXP_CASE_CHECK(pread(log.descriptor, after_bytes,
                                  sizeof(after_bytes), (off_t)before) ==
                           (ssize_t)sizeof(after_bytes));
            LXP_CASE_CHECK(memcmp(before_bytes, after_bytes,
                                  sizeof(before_bytes)) == 0);
        }
    }
    LXP_CASE_CHECK(lxp_log_close(&log) == LXP_OK);
    LXP_CASE_CHECK(unlink(path) == 0);
    LXP_CASE_CHECK(rmdir(directory) == 0);
    return 0;
}

static const lxp_log_record_kind historic_kinds[] = {
    LXP_LOG_ACTIVITY, LXP_LOG_BATCH_BODY, LXP_LOG_BATCH_HEADER,
    LXP_LOG_ORACLE, LXP_LOG_REPLICA_ACK, LXP_LOG_RECEIPT
};
static const uint8_t historic_body[] = { 0x12U, 0x00U, 0x80U, 0xffU };

static int write_historic_segment(const char *directory)
{
    typedef lxp_result (*create_fn)(lxp_log *, const char *, uint64_t,
                                     uint64_t);
    typedef lxp_result (*append_fn)(lxp_log *, lxp_log_record_kind, uint64_t,
                                     const void *, uint32_t, uint64_t *);
    typedef lxp_result (*log_fn)(lxp_log *);
    char command[512];
    char library[128];
    void *handle;
    create_fn create;
    append_fn append;
    log_fn boundary;
    log_fn close_log;
    lxp_log log;
    size_t i;
    int length;

    length = snprintf(library, sizeof(library), "%s/legacy-log.so", directory);
    LXP_CASE_CHECK(length > 0 && (size_t)length < sizeof(library));
    length = snprintf(command, sizeof(command),
        "git show 6cdf7860f29324a94f3482a633aa2ad8c5ec4376:"
        "src/storage/lxp_log.c | "
        "cc -std=c17 -fPIC -shared -Wl,-Bsymbolic -Iinclude -x c - "
        "src/storage/lxp_fault_hooks.c -o '%s'", library);
    LXP_CASE_CHECK(length > 0 && (size_t)length < sizeof(command));
    LXP_CASE_CHECK(system(command) == 0);
    handle = dlopen(library, RTLD_NOW | RTLD_LOCAL);
    if (handle == NULL) fprintf(stderr, "historic producer: %s\n", dlerror());
    LXP_CASE_CHECK(handle != NULL);
    create = (create_fn)dlsym(handle, "lxp_log_segment_create");
    append = (append_fn)dlsym(handle, "lxp_log_append");
    boundary = (log_fn)dlsym(handle, "lxp_log_write_boundary");
    close_log = (log_fn)dlsym(handle, "lxp_log_close");
    LXP_CASE_CHECK(create != NULL && append != NULL && boundary != NULL &&
                   close_log != NULL);
    LXP_CASE_CHECK(create(&log, directory, 0U, 4096U) == LXP_OK);
    for (i = 0U; i < sizeof(historic_kinds) / sizeof(historic_kinds[0]); ++i) {
        uint64_t offset = UINT64_MAX;

        LXP_CASE_CHECK(append(&log, historic_kinds[i], 41U, historic_body,
                              (uint32_t)sizeof(historic_body), &offset) ==
                       LXP_OK);
        LXP_CASE_CHECK(offset == i *
                       (LXP_LOG_HEADER_BYTES + sizeof(historic_body)));
    }
    LXP_CASE_CHECK(boundary(&log) == LXP_OK);
    LXP_CASE_CHECK(close_log(&log) == LXP_OK);
    LXP_CASE_CHECK(dlclose(handle) == 0);
    LXP_CASE_CHECK(unlink(library) == 0);
    return 0;
}

static lxp_result replay_historic_record(void *context,
                                        const lxp_log_record_header *header,
                                        const uint8_t *body)
{
    size_t *count = context;
    uint64_t record_bytes = LXP_LOG_HEADER_BYTES + sizeof(historic_body);

    if (*count >= sizeof(historic_kinds) / sizeof(historic_kinds[0]) ||
        header->record_kind != (uint8_t)historic_kinds[*count] ||
        header->global_sequence != 41U ||
        header->body_length != sizeof(historic_body) ||
        header->body_crc32c != lxp_log_crc32c(historic_body,
                                            sizeof(historic_body)) ||
        header->previous_record_offset !=
            (*count == 0U ? 0U : (*count - 1U) * record_bytes) ||
        memcmp(body, historic_body, sizeof(historic_body)) != 0)
        return LXP_FATAL_REPLAY_DIVERGENCE;
    ++*count;
    return LXP_OK;
}

static int log_read_accepts_historic_kind_record(void)
{
    char directory[] = "/tmp/lxp-historic-kinds-XXXXXX";
    char path[128];
    uint64_t record_bytes = LXP_LOG_HEADER_BYTES + sizeof(historic_body);
    size_t record_count = sizeof(historic_kinds) / sizeof(historic_kinds[0]);
    size_t mode;
    int length;

    LXP_CASE_CHECK(mkdtemp(directory) != NULL);
    length = snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U);
    LXP_CASE_CHECK(length > 0 && (size_t)length < sizeof(path));
    LXP_CASE_CHECK(write_historic_segment(directory) == 0);
    for (mode = 0U; mode < 2U; ++mode) {
        lxp_log log;
        size_t replayed = 0U;
        size_t i;
        uint64_t valid_end;
        uint64_t last;
        uint64_t next;

        LXP_CASE_CHECK(lxp_log_open(&log, path) == LXP_OK);
        LXP_CASE_CHECK(log.has_durable_marker);
        LXP_CASE_CHECK(lxp_log_scan_tail(&log, &valid_end, &last, &next) ==
                       LXP_OK);
        LXP_CASE_CHECK(valid_end == record_count * record_bytes);
        LXP_CASE_CHECK(last == (record_count - 1U) * record_bytes);
        LXP_CASE_CHECK(next == 42U);
        LXP_CASE_CHECK((mode == 0U ?
            lxp_log_recover(&log, replay_historic_record, &replayed) :
            lxp_log_recover_complete_records(&log, replay_historic_record,
                                             &replayed)) == LXP_OK);
        LXP_CASE_CHECK(replayed == record_count);
        LXP_CASE_CHECK(log.write_offset == valid_end);
        LXP_CASE_CHECK(log.previous_record_offset == last);
        LXP_CASE_CHECK(lxp_log_resume_sequence(&log) == 42U);
        for (i = 0U; i < record_count; ++i) {
            lxp_log_record_header header;
            uint8_t decoded[sizeof(historic_body)];

            LXP_CASE_CHECK(lxp_log_read(&log, i * record_bytes, &header,
                                       decoded, sizeof(decoded)) == LXP_OK);
            LXP_CASE_CHECK(header.record_kind == (uint8_t)historic_kinds[i]);
            LXP_CASE_CHECK(header.global_sequence == 41U);
            LXP_CASE_CHECK(header.body_length == sizeof(historic_body));
            LXP_CASE_CHECK(memcmp(decoded, historic_body,
                                  sizeof(historic_body)) == 0);
        }
        LXP_CASE_CHECK(lxp_log_close(&log) == LXP_OK);
    }
    LXP_CASE_CHECK(unlink(path) == 0);
    LXP_CASE_CHECK(rmdir(directory) == 0);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "log_append_refuses_retired_kinds", log_append_refuses_retired_kinds },
        { "log_read_accepts_historic_kind_record",
          log_read_accepts_historic_kind_record }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
