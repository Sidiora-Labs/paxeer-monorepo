#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_state_proof.h"
#include "lxp_kernel_case.h"

#include <stdlib.h>

typedef struct snapshot_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    uint64_t parameters;
} snapshot_fixture;

static int snapshot_fixture_init(snapshot_fixture *fixture)
{
    uint8_t key[32] = { 0x31U };

    fixture->parameters = 1U;
    LXP_CASE_CHECK(lxp_state_store_init(&fixture->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&fixture->kernel, &fixture->state,
                                    &fixture->journal, &fixture->parameters,
                                    0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_open(&fixture->state, 1U,
                                         &fixture->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_set(&fixture->journal, key,
                                        (lxp_u128){ 0U, 42U }) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&fixture->journal) == LXP_OK);
    return 0;
}

static int add_idempotency(snapshot_fixture *fixture)
{
    lxp_idempotency_key_state entry = { 0 };

    entry.key_hash[0] = 0x52U;
    entry.receipt[0] = 0x71U;
    entry.receipt_length = 1U;
    entry.recorded_sequence = 1U;
    LXP_CASE_CHECK(lxp_idempotency_append(&fixture->state, &entry, 1U) ==
                   LXP_OK);
    LXP_CASE_CHECK(fixture->state.idempotency_count == 1U);
    return 0;
}

static int root_ignores_idempotency(void)
{
    snapshot_fixture *fixture = calloc(1U, sizeof(*fixture));
    lxp_state_witness *proof = calloc(1U, sizeof(*proof));
    uint8_t expected[32], actual[32];
    uint8_t key[33] = { 2U };
    lxp_idempotency_key_state *entries;

    LXP_CASE_CHECK(fixture != NULL && proof != NULL);
    LXP_CASE_CHECK(snapshot_fixture_init(fixture) == 0);
    LXP_CASE_CHECK(lxp_state_root(&fixture->kernel, expected) == LXP_OK);
    LXP_CASE_CHECK(add_idempotency(fixture) == 0);
    LXP_CASE_CHECK(lxp_state_root(&fixture->kernel, actual) == LXP_OK);
    LXP_CASE_CHECK(memcmp(actual, expected, sizeof(actual)) == 0);
    (void)memcpy(key + 1U, fixture->state.idempotency[0].key_hash, 32U);
    LXP_CASE_CHECK(lxp_state_proof_build(&fixture->kernel, 0U,
        (lxp_byte_span){ key, sizeof(key) }, proof) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(lxp_state_proof_build(&fixture->kernel, 0U,
        (lxp_byte_span){ (const uint8_t *)"sequence", 8U }, proof) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_proof_verify(proof, expected) == LXP_OK);

    fixture->state.idempotency[0].key_hash[0] ^= 1U;
    fixture->state.idempotency[0].receipt[0] ^= 1U;
    fixture->state.idempotency[0].receipt_length =
        LXP_STATE_MAX_RECEIPT_BYTES + 1U;
    LXP_CASE_CHECK(lxp_state_root(&fixture->kernel, actual) == LXP_OK);
    LXP_CASE_CHECK(memcmp(actual, expected, sizeof(actual)) == 0);
    entries = fixture->state.idempotency;
    fixture->state.idempotency = NULL;
    fixture->state.idempotency_count = SIZE_MAX;
    LXP_CASE_CHECK(lxp_state_root(&fixture->kernel, actual) == LXP_OK);
    LXP_CASE_CHECK(memcmp(actual, expected, sizeof(actual)) == 0);
    fixture->state.idempotency = entries;
    fixture->state.idempotency_count = 1U;

    fixture->state.cells[0].value.lo += 1U;
    LXP_CASE_CHECK(lxp_state_root(&fixture->kernel, actual) == LXP_OK);
    LXP_CASE_CHECK(memcmp(actual, expected, sizeof(actual)) != 0);
    LXP_CASE_CHECK(lxp_state_store_destroy(&fixture->state) == LXP_OK);
    free(proof);
    free(fixture);
    return 0;
}

static int snapshot_count_word_zero(void)
{
    snapshot_fixture *source = calloc(1U, sizeof(*source));
    snapshot_fixture *target = calloc(1U, sizeof(*target));
    uint8_t empty_storage[4096], populated_storage[4096];
    lxp_arena empty_arena, populated_arena;
    lxp_byte_span empty_snapshot, populated_snapshot, key;
    lxp_snapshot_manifest_record manifest;
    lxp_codec_reader reader;
    uint8_t root[32], loaded_root[32];
    lxp_u128 value;
    uint64_t sequence;
    uint32_t count;
    uint16_t version, root_count;
    size_t count_offset;
    size_t i;
    const uint32_t rejected_counts[] = { 1U, LXP_STATE_MAX_IDEMPOTENCY,
                                         UINT32_MAX };

    LXP_CASE_CHECK(source != NULL && target != NULL);
    LXP_CASE_CHECK(snapshot_fixture_init(source) == 0);
    LXP_CASE_CHECK(snapshot_fixture_init(target) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&empty_arena, empty_storage,
                                 sizeof(empty_storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&populated_arena, populated_storage,
                                 sizeof(populated_storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_write(&source->kernel, 1U, &empty_arena,
                                     &empty_snapshot) == LXP_OK);
    LXP_CASE_CHECK(add_idempotency(source) == 0);
    source->state.idempotency[0].receipt_length =
        LXP_STATE_MAX_RECEIPT_BYTES + 1U;
    source->state.idempotency[0].canonical_length =
        LXP_STATE_MAX_RECEIPT_BYTES + 1U;
    LXP_CASE_CHECK(lxp_snapshot_write(&source->kernel, 1U, &populated_arena,
                                     &populated_snapshot) == LXP_OK);
    LXP_CASE_CHECK(populated_snapshot.length == empty_snapshot.length);
    LXP_CASE_CHECK(memcmp(populated_snapshot.bytes, empty_snapshot.bytes,
                          empty_snapshot.length) == 0);

    LXP_CASE_CHECK(lxp_codec_reader_init(&reader, populated_snapshot.bytes,
                                         populated_snapshot.length) == LXP_OK);
    LXP_CASE_CHECK(lxp_codec_read_struct_header_version(&reader,
        LXP_SNAPSHOT_FORMAT_VERSION, &version) == LXP_OK);
    LXP_CASE_CHECK(version == LXP_PROTOCOL_VERSION_LEGACY);
    LXP_CASE_CHECK(lxp_codec_read_u64(&reader, &sequence) == LXP_OK);
    LXP_CASE_CHECK(sequence == 1U);
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == 1U);
    LXP_CASE_CHECK(lxp_codec_read_bytes(&reader, &key, 32U) == LXP_OK);
    LXP_CASE_CHECK(key.length == 32U && key.bytes[0] == 0x31U);
    LXP_CASE_CHECK(lxp_codec_read_u128(&reader, &value) == LXP_OK);
    LXP_CASE_CHECK(value.hi == 0U && value.lo == 42U);
    count_offset = reader.offset;
    LXP_CASE_CHECK(count_offset == 16U + 52U);
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == 0U);
    LXP_CASE_CHECK(memcmp(populated_snapshot.bytes + count_offset,
                          "\0\0\0\0", 4U) == 0);
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == source->kernel.module_count);
    LXP_CASE_CHECK(count == 0U);
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == 0U);
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == 0U);
    LXP_CASE_CHECK(lxp_codec_read_u64(&reader, &sequence) == LXP_OK);
    LXP_CASE_CHECK(sequence == 0U);
    LXP_CASE_CHECK(lxp_codec_read_u16(&reader, &root_count) == LXP_OK);
    LXP_CASE_CHECK(root_count == 9U);
    for (i = 0U; i < root_count; ++i) {
        LXP_CASE_CHECK(lxp_codec_read_bytes(&reader, &key, 32U) == LXP_OK);
        LXP_CASE_CHECK(key.length == 32U);
    }
    LXP_CASE_CHECK(lxp_codec_finish(&reader) == LXP_OK);

    LXP_CASE_CHECK(lxp_state_root(&source->kernel, root) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_manifest_build(populated_snapshot.bytes,
        populated_snapshot.length, 1U, root, root, &manifest) == LXP_OK);
    target->state.cells[0].value.lo = 99U;
    target->state.next_sequence = 7U;
    LXP_CASE_CHECK(lxp_snapshot_load(populated_snapshot.bytes,
        populated_snapshot.length, &manifest, &target->kernel) == LXP_OK);
    LXP_CASE_CHECK(target->state.next_sequence == 2U);
    LXP_CASE_CHECK(target->state.cells[0].value.lo == 42U);
    LXP_CASE_CHECK(target->state.idempotency_count == 0U);
    LXP_CASE_CHECK(target->state.idempotency == NULL);
    LXP_CASE_CHECK(lxp_state_root(&target->kernel, loaded_root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(loaded_root, root, sizeof(root)) == 0);

    for (i = 0U; i < sizeof(rejected_counts) / sizeof(rejected_counts[0]);
         ++i) {
        uint32_t rejected = rejected_counts[i];
        uint8_t *count_word = populated_storage +
            (size_t)(populated_snapshot.bytes - populated_storage) +
            count_offset;
        count_word[0] = (uint8_t)(rejected >> 24U);
        count_word[1] = (uint8_t)(rejected >> 16U);
        count_word[2] = (uint8_t)(rejected >> 8U);
        count_word[3] = (uint8_t)rejected;
        LXP_CASE_CHECK(lxp_snapshot_manifest_build(populated_snapshot.bytes,
            populated_snapshot.length, 1U, root, root, &manifest) == LXP_OK);
        target->state.cells[0].value.lo = 99U;
        target->state.next_sequence = 7U;
        LXP_CASE_CHECK(lxp_snapshot_load(populated_snapshot.bytes,
            populated_snapshot.length, &manifest, &target->kernel) ==
            LXP_ERR_SNAPSHOT_MISMATCH);
        LXP_CASE_CHECK(target->state.cells[0].value.lo == 99U);
        LXP_CASE_CHECK(target->state.next_sequence == 7U);
        LXP_CASE_CHECK(target->state.idempotency_count == 0U);
        LXP_CASE_CHECK(target->state.idempotency == NULL);
        LXP_CASE_CHECK(memcmp(target->kernel.current_state_root, root,
                              sizeof(root)) == 0);
    }
    LXP_CASE_CHECK(lxp_state_store_destroy(&source->state) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(&target->state) == LXP_OK);
    free(source);
    free(target);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "root_ignores_idempotency", root_ignores_idempotency },
        { "snapshot_count_word_zero", snapshot_count_word_zero }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
