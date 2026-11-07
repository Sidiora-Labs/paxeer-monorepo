#include "lxp_kernel_case.h"

#include "layerx/lxp_crypto.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/programs.h"

typedef struct bounds_fixture {
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
} bounds_fixture;

static int bounds_fixture_init(bounds_fixture *fixture)
{
    static const uint64_t parameters = 1U;

    LXP_CASE_CHECK(lxp_state_store_init(&fixture->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&fixture->kernel, &fixture->state,
        &fixture->journal, &parameters, 0U) == LXP_OK);
    return 0;
}

static void bounds_row(lxp_module_kv_entry *entry, size_t index)
{
    (void)memset(entry, 0, sizeof(*entry));
    entry->module_id = 1U;
    entry->key_length = 2U;
    entry->key[0] = (uint8_t)(index >> 8U);
    entry->key[1] = (uint8_t)index;
    entry->value_length = 2U;
    entry->value[0] = (uint8_t)(entry->key[0] ^ 0xa5U);
    entry->value[1] = (uint8_t)(entry->key[1] ^ 0x5aU);
}

static int bounds_digest_matches(const uint8_t digest[32], const char *hex)
{
    static const char digits[] = "0123456789abcdef";
    size_t i;

    for (i = 0U; i < 32U; ++i) {
        LXP_CASE_CHECK(hex[2U * i] == digits[digest[i] >> 4U]);
        LXP_CASE_CHECK(hex[2U * i + 1U] == digits[digest[i] & 15U]);
    }
    return 0;
}

static int module_kv_bound_raised(void)
{
    static const struct {
        size_t count;
        size_t snapshot_length;
        const char *head_root;
        const char *head_snapshot_sha256;
    } golden[] = {
        { 0U, 366U,
          "74d0da76a86db333f8c72bb0d02f18bde7112b30780e530ba1e11e59d4ce0a3f",
          "31c94cd024253dabb3092f1048ee8a817dff3ed7c66e0f1939e8156320e49406" },
        { 64U, 1262U,
          "bbd9dd23506754bc5052a879292252e0e90f97d92dd2bf17784cc527cbdc732f",
          "d2ed60b341e5dd36d83db9484d8763abd7850ddee2cf443a661ece504da7107a" },
        { 512U, 7534U,
          "3900d52ed07015d71cfae1fe7cada9f9a4a1e7f12a845eef3aab08896b0bacea",
          "3c83cb5a1e0c76adcd72b8649cb5f840f8fedcdaa45a3567f1dfec0e0d693423" }
    };
    static bounds_fixture original;
    static bounds_fixture restored;
    static uint8_t snapshot_bytes[131072];
    static uint8_t restored_bytes[131072];
    const size_t counts[] = { 0U, 64U, 512U, 1200U, 4096U };
    lxp_snapshot_manifest_record manifest;
    lxp_byte_span snapshot;
    lxp_byte_span rewritten;
    lxp_arena arena;
    lxp_arena restored_arena;
    uint8_t root[32];
    uint8_t restored_root[32];
    uint8_t digest[32];
    uint8_t receipt_root[32] = { 0x91U };
    uint8_t *encoded;
    size_t c;
    size_t i;

    LXP_CASE_CHECK(LXP_KERNEL_MAX_MODULE_KV == 4096);
    LXP_CASE_CHECK(bounds_fixture_init(&original) == 0);
    LXP_CASE_CHECK(bounds_fixture_init(&restored) == 0);
    for (i = 0U; i < LXP_KERNEL_MAX_MODULE_KV; ++i)
        bounds_row(&original.kernel.module_kv[i], i);
    for (c = 0U; c < sizeof(counts) / sizeof(counts[0]); ++c) {
        original.kernel.module_kv_count = counts[c];
        LXP_CASE_CHECK(lxp_state_root(&original.kernel, root) == LXP_OK);
        LXP_CASE_CHECK(lxp_arena_init(&arena, snapshot_bytes,
                                     sizeof(snapshot_bytes)) == LXP_OK);
        LXP_CASE_CHECK(lxp_snapshot_write(&original.kernel, 0U, &arena,
                                          &snapshot) == LXP_OK);
        if (c < sizeof(golden) / sizeof(golden[0])) {
            LXP_CASE_CHECK(counts[c] == golden[c].count);
            LXP_CASE_CHECK(snapshot.length == golden[c].snapshot_length);
            LXP_CASE_CHECK(bounds_digest_matches(root,
                                                 golden[c].head_root) == 0);
            LXP_CASE_CHECK(lxp_hash_sha256(snapshot.bytes, snapshot.length,
                                           digest) == LXP_OK);
            LXP_CASE_CHECK(bounds_digest_matches(digest,
                golden[c].head_snapshot_sha256) == 0);
        }
        LXP_CASE_CHECK(lxp_snapshot_manifest(snapshot.bytes, snapshot.length,
            0U, root, receipt_root, &manifest) == LXP_OK);
        LXP_CASE_CHECK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
            &manifest, &restored.kernel) == LXP_OK);
        LXP_CASE_CHECK(restored.kernel.module_kv_count == counts[c]);
        LXP_CASE_CHECK(restored.state.next_sequence == 1U);
        for (i = 0U; i < counts[c]; ++i) {
            const lxp_module_kv_entry *left = &original.kernel.module_kv[i];
            const lxp_module_kv_entry *right = &restored.kernel.module_kv[i];
            LXP_CASE_CHECK(right->module_id == left->module_id);
            LXP_CASE_CHECK(right->key_length == left->key_length);
            LXP_CASE_CHECK(right->value_length == left->value_length);
            LXP_CASE_CHECK(memcmp(right->key, left->key, left->key_length) == 0);
            LXP_CASE_CHECK(memcmp(right->value, left->value,
                                   left->value_length) == 0);
        }
        LXP_CASE_CHECK(lxp_state_root(&restored.kernel, restored_root) == LXP_OK);
        LXP_CASE_CHECK(memcmp(root, restored_root, sizeof(root)) == 0);
        LXP_CASE_CHECK(lxp_snapshot_verify_root(&restored.kernel,
                                               &manifest) == LXP_OK);
        LXP_CASE_CHECK(lxp_arena_init(&restored_arena, restored_bytes,
                                     sizeof(restored_bytes)) == LXP_OK);
        LXP_CASE_CHECK(lxp_snapshot_write(&restored.kernel, 0U, &restored_arena,
                                          &rewritten) == LXP_OK);
        LXP_CASE_CHECK(rewritten.length == snapshot.length);
        LXP_CASE_CHECK(memcmp(rewritten.bytes, snapshot.bytes,
                               snapshot.length) == 0);
    }
    original.kernel.module_kv_count = 4097U;
    LXP_CASE_CHECK(lxp_state_root(&original.kernel, root) == LXP_ERR_LENGTH_LIMIT);
    LXP_CASE_CHECK(lxp_arena_reset(&arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_write(&original.kernel, 0U, &arena,
                                      &snapshot) == LXP_ERR_LENGTH_LIMIT);
    encoded = (uint8_t *)rewritten.bytes;
    encoded[24] = 0U;
    encoded[25] = 0U;
    encoded[26] = 0x10U;
    encoded[27] = 0x01U;
    LXP_CASE_CHECK(lxp_snapshot_manifest(rewritten.bytes, rewritten.length,
        0U, restored_root, receipt_root, &manifest) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_load(rewritten.bytes, rewritten.length,
        &manifest, &restored.kernel) == LXP_ERR_LENGTH_LIMIT);
    LXP_CASE_CHECK(restored.kernel.module_kv_count == 4096U);
    LXP_CASE_CHECK(lxp_state_root(&restored.kernel, root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root, restored_root, sizeof(root)) == 0);
    LXP_CASE_CHECK(lxp_state_store_destroy(&original.state) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(&restored.state) == LXP_OK);
    return 0;
}

static int staged_writes_bound_raised(void)
{
    static bounds_fixture fixture;
    static lxp_module_ctx ctx;
    static uint8_t arena_bytes[4096];
    lxp_arena arena;
    uint8_t key[34] = { (uint8_t)'s' };
    const uint8_t value[] = { 0x31U, 0x73U };
    const uint8_t *found;
    size_t found_length;
    size_t lease;
    size_t kind;
    size_t i;

    LXP_CASE_CHECK(LXP_MODULE_MAX_STAGED_WRITES == 512);
    LXP_CASE_CHECK(bounds_fixture_init(&fixture) == 0);
    LXP_CASE_CHECK(lxp_kernel_register_module(&fixture.kernel,
        programs_module_registration()) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes,
                                 sizeof(arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &fixture.kernel,
        LXP_MODULE_PROGRAMS, 0U, 0U, 1U, UINT64_MAX, &arena, true) == LXP_OK);
    for (lease = 0U; lease < 128U; ++lease) {
        key[33] = (uint8_t)(lease + 1U);
        for (kind = 0U; kind < 3U; ++kind) {
            key[1] = (uint8_t)kind;
            LXP_CASE_CHECK(lxp_ctx_kv_put(&ctx, key, sizeof(key), value,
                                         sizeof(value)) == LXP_OK);
        }
    }
    LXP_CASE_CHECK(ctx.staged_count == 384U);
    LXP_CASE_CHECK(fixture.kernel.module_kv_count == 0U);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture.kernel.module_kv_count == 384U);
    for (lease = 0U; lease < 128U; ++lease) {
        key[33] = (uint8_t)(lease + 1U);
        for (kind = 0U; kind < 3U; ++kind) {
            key[1] = (uint8_t)kind;
            LXP_CASE_CHECK(lxp_ctx_kv_del(&ctx, key, sizeof(key)) == LXP_OK);
        }
    }
    LXP_CASE_CHECK(ctx.staged_count == 384U);
    LXP_CASE_CHECK(fixture.kernel.module_kv_count == 384U);
    for (i = 0U; i < 384U; ++i) {
        LXP_CASE_CHECK(ctx.staged[i].deleted);
        LXP_CASE_CHECK(lxp_ctx_kv_get(&ctx, ctx.staged[i].key,
            ctx.staged[i].key_length, &found, &found_length) == LXP_ERR_UNKNOWN_FIELD);
    }
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture.kernel.module_kv_count == 0U);
    LXP_CASE_CHECK(ctx.staged_count == 0U);
    for (i = 0U; i < 512U; ++i) {
        key[0] = (uint8_t)(i >> 8U);
        key[1] = (uint8_t)i;
        LXP_CASE_CHECK(lxp_ctx_kv_put(&ctx, key, 2U, value,
                                     sizeof(value)) == LXP_OK);
    }
    key[0] = 2U;
    key[1] = 0U;
    LXP_CASE_CHECK(lxp_ctx_kv_put(&ctx, key, 2U, value,
                                 sizeof(value)) == LXP_ERR_ARENA_EXHAUSTED);
    LXP_CASE_CHECK(lxp_ctx_kv_del(&ctx, key, 2U) == LXP_ERR_ARENA_EXHAUSTED);
    LXP_CASE_CHECK(ctx.staged_count == 512U);
    LXP_CASE_CHECK(fixture.kernel.module_kv_count == 0U);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&ctx, key, 2U, &found,
                                 &found_length) == LXP_ERR_UNKNOWN_FIELD);
    for (i = 0U; i < 512U; ++i) {
        key[0] = (uint8_t)(i >> 8U);
        key[1] = (uint8_t)i;
        LXP_CASE_CHECK(lxp_ctx_kv_get(&ctx, key, 2U, &found,
                                     &found_length) == LXP_OK);
        LXP_CASE_CHECK(found_length == sizeof(value));
        LXP_CASE_CHECK(memcmp(found, value, sizeof(value)) == 0);
    }
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(ctx.staged_count == 0U);
    LXP_CASE_CHECK(fixture.kernel.module_kv_count == 512U);
    for (i = 0U; i < 512U; ++i) {
        key[0] = (uint8_t)(i >> 8U);
        key[1] = (uint8_t)i;
        LXP_CASE_CHECK(lxp_ctx_kv_get(&ctx, key, 2U, &found,
                                     &found_length) == LXP_OK);
        LXP_CASE_CHECK(found_length == sizeof(value));
        LXP_CASE_CHECK(memcmp(found, value, sizeof(value)) == 0);
    }
    LXP_CASE_CHECK(lxp_state_store_destroy(&fixture.state) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "module_kv_bound_raised", module_kv_bound_raised },
        { "staged_writes_bound_raised", staged_writes_bound_raised }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
