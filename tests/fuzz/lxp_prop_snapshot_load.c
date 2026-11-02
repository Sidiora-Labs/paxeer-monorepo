#include "layerx/lxp_snapshot.h"
#include "layerx/lx_asset.h"
#include "layerx/lx_escrow.h"
#include "../codec/lxp_kernel_case.h"

#include <stdlib.h>

enum { SNAPSHOT_STORAGE_BYTES = 16384, SNAPSHOT_ROWS = 6,
       SNAPSHOT_SEEDS = 3, SNAPSHOT_MUTATIONS_PER_BYTE = 3 };

typedef struct snapshot_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    uint64_t parameters;
} snapshot_fixture;

typedef struct snapshot_corpus {
    snapshot_fixture source;
    snapshot_fixture target;
    uint8_t storage[SNAPSHOT_STORAGE_BYTES];
    uint8_t mutated[SNAPSHOT_STORAGE_BYTES];
    uint8_t restored[SNAPSHOT_STORAGE_BYTES];
    lxp_byte_span base;
    lxp_snapshot_manifest_record manifest;
    size_t row_offsets[SNAPSHOT_ROWS + 1];
    size_t accepted;
    size_t refused;
    size_t mutations;
    size_t truncations;
    size_t swaps;
} snapshot_corpus;

static uint64_t next_random(uint64_t *seed)
{
    *seed ^= *seed << 13U;
    *seed ^= *seed >> 7U;
    *seed ^= *seed << 17U;
    return *seed;
}

static int fixture_init(snapshot_fixture *fixture)
{
    fixture->parameters = 1U;
    LXP_CASE_CHECK(lxp_state_store_init(&fixture->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&fixture->kernel, &fixture->state,
        &fixture->journal, &fixture->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&fixture->kernel,
        lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&fixture->kernel,
        lx_escrow_module_iface()) == LXP_OK);
    return 0;
}

static int locate_rows(snapshot_corpus *corpus)
{
    lxp_codec_reader reader;
    lxp_byte_span bytes;
    lxp_u128 value;
    uint64_t sequence;
    uint32_t count, abi, types, type;
    uint16_t version, module;
    uint8_t enabled;
    size_t i, j;

    LXP_CASE_CHECK(lxp_codec_reader_init(&reader, corpus->base.bytes,
        corpus->base.length) == LXP_OK);
    LXP_CASE_CHECK(lxp_codec_read_struct_header_version(&reader,
        LXP_SNAPSHOT_FORMAT_VERSION, &version) == LXP_OK);
    LXP_CASE_CHECK(version == LXP_PROTOCOL_VERSION_LEGACY);
    LXP_CASE_CHECK(lxp_codec_read_u64(&reader, &sequence) == LXP_OK);
    LXP_CASE_CHECK(sequence == corpus->manifest.global_sequence);
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == corpus->source.state.count);
    for (i = 0U; i < count; ++i) {
        LXP_CASE_CHECK(lxp_codec_read_bytes(&reader, &bytes, 32U) == LXP_OK);
        LXP_CASE_CHECK(bytes.length == 32U);
        LXP_CASE_CHECK(lxp_codec_read_u128(&reader, &value) == LXP_OK);
    }
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == 0U);
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == corpus->source.kernel.module_count);
    for (i = 0U; i < count; ++i) {
        LXP_CASE_CHECK(lxp_codec_read_u16(&reader, &module) == LXP_OK);
        LXP_CASE_CHECK(module == corpus->source.kernel.modules[i].module_id);
        LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &abi) == LXP_OK);
        LXP_CASE_CHECK(abi == corpus->source.kernel.modules[i].abi_version);
        LXP_CASE_CHECK(lxp_codec_read_u64(&reader, &sequence) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_read_u64(&reader, &sequence) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_read_u8(&reader, &enabled) == LXP_OK);
        LXP_CASE_CHECK(enabled == 1U);
        LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &types) == LXP_OK);
        LXP_CASE_CHECK(types ==
            corpus->source.kernel.modules[i].activity_type_count);
        for (j = 0U; j < types; ++j)
            LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &type) == LXP_OK);
    }
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == SNAPSHOT_ROWS);
    for (i = 0U; i < count; ++i) {
        corpus->row_offsets[i] = reader.offset;
        LXP_CASE_CHECK(lxp_codec_read_u16(&reader, &module) == LXP_OK);
        LXP_CASE_CHECK(lxp_codec_read_bytes(&reader, &bytes,
            LXP_MODULE_MAX_KEY_BYTES) == LXP_OK);
        LXP_CASE_CHECK(bytes.length != 0U);
        LXP_CASE_CHECK(lxp_codec_read_bytes(&reader, &bytes,
            LXP_MODULE_MAX_VALUE_BYTES) == LXP_OK);
    }
    corpus->row_offsets[count] = reader.offset;
    return 0;
}

static int corpus_init(snapshot_corpus *corpus, uint64_t *seed)
{
    static const uint8_t keys[SNAPSHOT_ROWS][3] = {
        {0x10U, 0U, 0U}, {0x10U, 0U, 0U}, {0x10U, 0U, 0xffU},
        {0xffU, 0U, 0U}, {0x10U, 0U, 0U}, {0x80U, 0U, 0U}
    };
    static const uint16_t lengths[SNAPSHOT_ROWS] = {1U, 2U, 3U, 1U, 1U, 1U};
    lxp_arena arena;
    uint8_t root[32];
    size_t i, j;

    LXP_CASE_CHECK(fixture_init(&corpus->source) == 0);
    LXP_CASE_CHECK(fixture_init(&corpus->target) == 0);
    LXP_CASE_CHECK(lxp_state_journal_open(&corpus->source.state, 1U,
        &corpus->source.journal) == LXP_OK);
    for (i = 0U; i < 2U; ++i) {
        uint8_t key[32] = {0U};
        lxp_u128 value = {next_random(seed), next_random(seed)};
        key[0] = (uint8_t)(i + 1U);
        LXP_CASE_CHECK(lxp_state_journal_set(&corpus->source.journal,
            key, value) == LXP_OK);
    }
    LXP_CASE_CHECK(lxp_state_journal_commit(&corpus->source.journal) == LXP_OK);
    for (i = 0U; i < SNAPSHOT_ROWS; ++i) {
        lxp_module_kv_entry *entry = &corpus->source.kernel.module_kv[i];
        entry->module_id =
            corpus->source.kernel.modules[i < 4U ? 0U : 1U].module_id;
        entry->key_length = lengths[i];
        (void)memcpy(entry->key, keys[i], lengths[i]);
        entry->value_length = i == 0U ? 0U :
            (uint32_t)(1U + next_random(seed) % 23U);
        for (j = 0U; j < entry->value_length; ++j)
            entry->value[j] = (uint8_t)next_random(seed);
    }
    corpus->source.kernel.module_kv_count = SNAPSHOT_ROWS;
    LXP_CASE_CHECK(lxp_state_root(&corpus->source.kernel, root) == LXP_OK);
    (void)memcpy(corpus->source.kernel.current_state_root, root, sizeof(root));
    LXP_CASE_CHECK(lxp_arena_init(&arena, corpus->storage,
        sizeof(corpus->storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_write(&corpus->source.kernel, 1U, &arena,
        &corpus->base) == LXP_OK);
    LXP_CASE_CHECK(corpus->base.length < sizeof(corpus->mutated));
    LXP_CASE_CHECK(lxp_snapshot_manifest_build(corpus->base.bytes,
        corpus->base.length, 1U, root, root, &corpus->manifest) == LXP_OK);
    LXP_CASE_CHECK(locate_rows(corpus) == 0);
    return 0;
}

static int check_load(snapshot_corpus *corpus, const uint8_t *bytes,
                      size_t length, bool rebuild_manifest, bool canonical,
                      bool must_refuse, lxp_result *result)
{
    lxp_snapshot_manifest_record manifest = corpus->manifest;
    snapshot_fixture *target = &corpus->target;
    lxp_result status;

    target->state.count = 0U;
    target->state.next_sequence = 1U;
    target->kernel.module_kv_count = 0U;
    (void)memcpy(target->kernel.modules, corpus->source.kernel.modules,
        sizeof(target->kernel.modules));
    (void)memset(target->kernel.current_state_root, 0xa5,
        sizeof(target->kernel.current_state_root));
    if (rebuild_manifest)
        LXP_CASE_CHECK(lxp_snapshot_manifest_build(bytes, length,
            corpus->manifest.global_sequence,
            corpus->manifest.canonical_state_root,
            corpus->manifest.receipt_state_root, &manifest) == LXP_OK);
    status = lxp_snapshot_load(bytes, length, &manifest, &target->kernel);
    *result = status;
    LXP_CASE_CHECK(lxp_result_is_registered(status));
    if (must_refuse) LXP_CASE_CHECK(status != LXP_OK);
    if (status != LXP_OK) {
        uint8_t untouched[32];
        (void)memset(untouched, 0xa5, sizeof(untouched));
        LXP_CASE_CHECK(target->kernel.module_kv_count == 0U);
        LXP_CASE_CHECK(target->state.count == 0U);
        LXP_CASE_CHECK(target->state.next_sequence == 1U);
        LXP_CASE_CHECK(memcmp(target->kernel.modules,
            corpus->source.kernel.modules, sizeof(target->kernel.modules)) == 0);
        LXP_CASE_CHECK(memcmp(target->kernel.current_state_root, untouched,
            sizeof(untouched)) == 0);
    } else if (canonical) {
        lxp_arena arena;
        lxp_byte_span restored;
        LXP_CASE_CHECK(lxp_arena_init(&arena, corpus->restored,
            sizeof(corpus->restored)) == LXP_OK);
        LXP_CASE_CHECK(lxp_snapshot_write(&target->kernel,
            manifest.global_sequence, &arena, &restored) == LXP_OK);
        LXP_CASE_CHECK(restored.length == length);
        LXP_CASE_CHECK(memcmp(bytes, restored.bytes, length) == 0);
    }
    return 0;
}

static int check_mutation(snapshot_corpus *corpus, size_t length,
                          bool canonical, bool must_refuse)
{
    lxp_result status;
    LXP_CASE_CHECK(check_load(corpus, corpus->mutated, length, false,
        canonical, true, &status) == 0);
    LXP_CASE_CHECK(status == LXP_ERR_SNAPSHOT_MISMATCH);
    LXP_CASE_CHECK(check_load(corpus, corpus->mutated, length, true,
        canonical, must_refuse, &status) == 0);
    if (status == LXP_OK) ++corpus->accepted;
    else ++corpus->refused;
    if (must_refuse && length == corpus->base.length)
        LXP_CASE_CHECK(status == LXP_ERR_NON_CANONICAL);
    return 0;
}

static int run_mutations(bool canonical)
{
    uint64_t seed = UINT64_C(0x736e617073686f74);
    size_t sample;

    for (sample = 0U; sample < SNAPSHOT_SEEDS; ++sample) {
        snapshot_corpus *corpus = calloc(1U, sizeof(*corpus));
        lxp_result status;
        size_t offset, mutation, row;
        LXP_CASE_CHECK(corpus != NULL);
        LXP_CASE_CHECK(corpus_init(corpus, &seed) == 0);
        LXP_CASE_CHECK(check_load(corpus, corpus->base.bytes,
            corpus->base.length, false, true, false, &status) == 0);
        LXP_CASE_CHECK(status == LXP_OK);
        (void)memcpy(corpus->mutated, corpus->base.bytes, corpus->base.length);
        for (offset = 0U; offset < corpus->base.length; ++offset) {
            for (mutation = 0U; mutation < SNAPSHOT_MUTATIONS_PER_BYTE;
                 ++mutation) {
                uint8_t mask = mutation == 0U ? 1U : mutation == 1U ? 0x80U :
                    (uint8_t)(1U + next_random(&seed) % UINT8_MAX);
                corpus->mutated[offset] = corpus->base.bytes[offset] ^ mask;
                if (check_mutation(corpus, corpus->base.length, canonical,
                                   false) != 0) {
                    (void)fprintf(stderr, "seed %zu, byte %zu, xor %u\n",
                        sample, offset, (unsigned)mask);
                    return 1;
                }
                ++corpus->mutations;
            }
            corpus->mutated[offset] = corpus->base.bytes[offset];
        }
        for (offset = 0U; offset < corpus->base.length; ++offset) {
            LXP_CASE_CHECK(check_mutation(corpus, offset, canonical, true) == 0);
            ++corpus->truncations;
        }
        for (row = 0U; row + 1U < SNAPSHOT_ROWS; ++row) {
            size_t start = corpus->row_offsets[row];
            size_t middle = corpus->row_offsets[row + 1U];
            size_t end = corpus->row_offsets[row + 2U];
            (void)memcpy(corpus->mutated, corpus->base.bytes,
                corpus->base.length);
            (void)memcpy(corpus->mutated + start, corpus->base.bytes + middle,
                end - middle);
            (void)memcpy(corpus->mutated + start + end - middle,
                corpus->base.bytes + start, middle - start);
            LXP_CASE_CHECK(memcmp(corpus->mutated, corpus->base.bytes,
                corpus->base.length) != 0);
            LXP_CASE_CHECK(check_mutation(corpus, corpus->base.length,
                canonical, true) == 0);
            ++corpus->swaps;
        }
        LXP_CASE_CHECK(corpus->mutations ==
            corpus->base.length * SNAPSHOT_MUTATIONS_PER_BYTE);
        LXP_CASE_CHECK(corpus->truncations == corpus->base.length);
        LXP_CASE_CHECK(corpus->swaps == SNAPSHOT_ROWS - 1U);
        LXP_CASE_CHECK(corpus->accepted > 0U && corpus->refused > 0U);
        LXP_CASE_CHECK(corpus->accepted + corpus->refused ==
            corpus->mutations + corpus->truncations + corpus->swaps);
        (void)printf("%s: seed %zu, %zu byte mutations, %zu truncations, "
            "%zu row swaps, %zu accepted, %zu refused\n",
            canonical ? "snapshot_load_accepts_only_canonical" :
                "snapshot_load_mutations_total", sample, corpus->mutations,
            corpus->truncations, corpus->swaps, corpus->accepted,
            corpus->refused);
        LXP_CASE_CHECK(lxp_state_store_destroy(&corpus->target.state) == LXP_OK);
        LXP_CASE_CHECK(lxp_state_store_destroy(&corpus->source.state) == LXP_OK);
        free(corpus);
    }
    return 0;
}

static int snapshot_load_mutations_total(void)
{
    return run_mutations(false);
}

static int snapshot_load_accepts_only_canonical(void)
{
    return run_mutations(true);
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"snapshot_load_mutations_total", snapshot_load_mutations_total},
        {"snapshot_load_accepts_only_canonical", snapshot_load_accepts_only_canonical}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
