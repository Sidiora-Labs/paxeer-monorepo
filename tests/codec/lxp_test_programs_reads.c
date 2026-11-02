#include "lxp_kernel_case.h"

#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/programs.h"

#include <stdlib.h>

typedef struct programs_reads_fixture {
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx ctx;
    lxp_arena arena;
    uint8_t arena_bytes[32768];
    uint64_t parameters;
} programs_reads_fixture;

static int fixture_init(programs_reads_fixture *f)
{
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                    &f->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                    programs_module_registration()) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
                                  sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel,
                    LXP_MODULE_PROGRAMS, 1U, 0U, 1U, 1000000U,
                    &f->arena, true) == LXP_OK);
    return 0;
}

static int fixture_read_only(programs_reads_fixture *f)
{
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel,
                    LXP_MODULE_PROGRAMS, 1U, 0U, 2U, 1000000U,
                    &f->arena, false) == LXP_OK);
    return 0;
}

static int fixture_destroy(programs_reads_fixture *f)
{
    lxp_module_ctx_rollback(&f->ctx);
    while (f->kernel.blob_count != 0U)
        free(f->kernel.blobs[--f->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    free(f);
    return 0;
}

static void write_u16(uint8_t *bytes, uint16_t value)
{
    bytes[0] = (uint8_t)(value >> 8U);
    bytes[1] = (uint8_t)value;
}

static void write_u32(uint8_t *bytes, uint32_t value)
{
    bytes[0] = (uint8_t)(value >> 24U);
    bytes[1] = (uint8_t)(value >> 16U);
    bytes[2] = (uint8_t)(value >> 8U);
    bytes[3] = (uint8_t)value;
}

static int interface_read_returns_stored_bytes(void)
{
    static const uint8_t wasm[] = {
        0,97,115,109,1,0,0,0,1,12,2,96,2,127,127,1,127,96,1,127,1,127,
        3,3,2,0,1,5,3,1,0,1,7,41,3,
        11,'l','a','y','e','r','x','_','c','a','l','l',0,0,
        14,'l','a','y','e','r','x','_','r','e','s','e','r','v','e',0,1,
        6,'m','e','m','o','r','y',2,0,10,11,2,4,0,65,0,11,4,0,65,0,11
    };
    static const uint8_t domain[] = "LayerX/program-interface/v1";
    programs_reads_fixture *f = calloc(1U, sizeof(*f));
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    lx_programs_metering_schedule schedule = {0};
    lxp_activity activity = {0};
    lxp_authority_resolved authority = {0};
    lxp_effect_buffer effects;
    uint8_t payload[512] = {1U};
    uint8_t expected[256] = {0};
    uint8_t unknown[32] = {2U};
    uint8_t interface_key[42] = "interface";
    uint8_t before[32], after[32];
    const uint8_t *bytes;
    size_t length, offset = 108U, interface_length, stored_count;
    void *decoded;

    LXP_CASE_CHECK(f != NULL && manifest != NULL);
    LXP_CASE_CHECK(fixture_init(f) == 0);
    schedule.version = LXP_PROGRAM_METERING_SCHEDULE_VERSION_V1;
    for (size_t i = 0U; i < 5U; ++i) schedule.coefficients[i] = 1U;
    schedule.coefficients[5] = 8U;
    schedule.coefficients[6] = 8U;
    schedule.coefficients[7] = 64U;
    schedule.coefficients[8] = 8U;
    schedule.activation_batch = 1U;
    schedule.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    manifest->signer_public_key[0] = 1U;
    LXP_CASE_CHECK(lxp_hash_payload(manifest->signer_public_key, 32U,
                                    schedule.authority_digest) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_append(manifest,
                                                       &schedule) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_materialize(manifest,
                                                            &f->kernel) == LXP_OK);
    free(manifest);
    f->ctx.batch_number = 1U;
    write_u16(payload + 32U, 1U);
    LXP_CASE_CHECK(lxp_hash_sha256(wasm, sizeof(wasm), payload + 68U) == LXP_OK);
    write_u32(payload + 100U, (uint32_t)sizeof(wasm));
    (void)memcpy(payload + offset, domain, sizeof(domain));
    offset += sizeof(domain);
    (void)memcpy(payload + offset, payload + 68U, 32U); offset += 32U;
    write_u16(payload + offset, 1U); offset += 2U;
    write_u16(payload + offset, 1U); offset += 2U;
    write_u16(payload + offset, 11U); offset += 2U;
    (void)memcpy(payload + offset, "layerx_call", 11U); offset += 11U;
    offset += 4U;
    payload[offset++] = 1U; payload[offset++] = 0x20U;
    write_u32(payload + offset, 64U); offset += 4U;
    payload[offset++] = 1U; payload[offset++] = 0x20U;
    write_u32(payload + offset, 64U); offset += 4U;
    offset += 6U;
    interface_length = offset - 108U;
    write_u32(payload + 104U, (uint32_t)interface_length);
    (void)memcpy(payload + offset, wasm, sizeof(wasm));
    offset += sizeof(wasm);
    activity.activity_type = LX_PROGRAMS_DEPLOY;
    LXP_CASE_CHECK(lxp_effect_buffer_init(&effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&f->ctx, &effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_lifecycle_decode(&f->ctx, 1U, payload,
                                                 offset, &decoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_lifecycle_validate(&f->ctx, &activity,
                                                   &authority, decoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_lifecycle_execute(&f->ctx, &activity,
                         &authority, decoded, &effects) == LXP_OK);
    (void)memcpy(expected, payload, 32U);
    write_u32(expected + 32U, 1U);
    LXP_CASE_CHECK(lxp_hash_sha256(payload + 108U, interface_length,
                                   expected + 36U) == LXP_OK);
    write_u32(expected + 68U, (uint32_t)interface_length);
    (void)memcpy(expected + 72U, payload + 108U, interface_length);
    (void)memcpy(interface_key + 10U, payload, 32U);
    LXP_CASE_CHECK(fixture_read_only(f) == 0);
    stored_count = f->kernel.module_kv_count;
    LXP_CASE_CHECK(lxp_state_subtree_root(&f->kernel, LXP_MODULE_PROGRAMS,
                                          before) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_interface_read(&f->ctx, payload,
                                               &bytes, &length) == LXP_OK);
    LXP_CASE_CHECK(length == 72U + interface_length);
    LXP_CASE_CHECK(memcmp(bytes, expected, length) == 0);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&f->ctx, interface_key, sizeof(interface_key),
                                  &bytes, &length) == LXP_OK);
    LXP_CASE_CHECK(length == 72U + interface_length);
    LXP_CASE_CHECK(memcmp(bytes, expected, length) == 0);
    LXP_CASE_CHECK(lxp_programs_interface_read(&f->ctx, unknown,
                           &bytes, &length) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(lxp_programs_interface_read(NULL, payload,
                           &bytes, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_programs_interface_read(&f->ctx, NULL,
                           &bytes, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_programs_interface_read(&f->ctx, payload,
                           NULL, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_programs_interface_read(&f->ctx, payload,
                           &bytes, NULL) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(f->ctx.staged_count == 0U);
    LXP_CASE_CHECK(f->kernel.module_kv_count == stored_count);
    LXP_CASE_CHECK(lxp_state_subtree_root(&f->kernel, LXP_MODULE_PROGRAMS,
                                          after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, sizeof(before)) == 0);
    return fixture_destroy(f);
}

static int sandbox_read_selects_kind(void)
{
    static const uint8_t records[3][8] = {
        {1U, 0U, 0x31U, 0U, 0x81U},
        {1U, 0U, 0x42U, 0U, 0x92U, 0xA2U},
        {1U, 0U, 0x53U, 0U, 0xA3U, 0xB3U, 0xC3U}
    };
    static const size_t lengths[3] = {5U, 6U, 7U};
    programs_reads_fixture *f = calloc(1U, sizeof(*f));
    uint8_t lease_id[32] = {0x41U};
    uint8_t unknown[32] = {0x42U};
    uint8_t key[34] = {'s', 0U};
    uint8_t before[32], after[32];
    const uint8_t *bytes;
    size_t length, stored_count;

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(fixture_init(f) == 0);
    (void)memcpy(key + 2U, lease_id, sizeof(lease_id));
    for (uint16_t kind = 0U; kind < 3U; ++kind) {
        key[1] = (uint8_t)kind;
        LXP_CASE_CHECK(lxp_ctx_kv_put(&f->ctx, key, sizeof(key),
                                      records[kind], lengths[kind]) == LXP_OK);
    }
    LXP_CASE_CHECK(fixture_read_only(f) == 0);
    stored_count = f->kernel.module_kv_count;
    LXP_CASE_CHECK(lxp_state_subtree_root(&f->kernel, LXP_MODULE_PROGRAMS,
                                          before) == LXP_OK);
    for (uint16_t kind = 0U; kind < 3U; ++kind) {
        LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, lease_id, kind,
                                                 &bytes, &length) == LXP_OK);
        LXP_CASE_CHECK(length == lengths[kind]);
        LXP_CASE_CHECK(memcmp(bytes, records[kind], length) == 0);
        LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, unknown, kind,
                             &bytes, &length) == LXP_ERR_UNKNOWN_FIELD);
    }
    LXP_CASE_CHECK(f->ctx.staged_count == 0U);
    LXP_CASE_CHECK(f->kernel.module_kv_count == stored_count);
    LXP_CASE_CHECK(lxp_state_subtree_root(&f->kernel, LXP_MODULE_PROGRAMS,
                                          after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, sizeof(before)) == 0);
    key[1] = 3U;
    LXP_CASE_CHECK(lxp_module_ctx_set_mutable(&f->ctx, true) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_kv_put(&f->ctx, key, sizeof(key), records[0],
                                  lengths[0]) == LXP_OK);
    LXP_CASE_CHECK(fixture_read_only(f) == 0);
    stored_count = f->kernel.module_kv_count;
    LXP_CASE_CHECK(lxp_state_subtree_root(&f->kernel, LXP_MODULE_PROGRAMS,
                                          before) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, lease_id, 3U,
                           &bytes, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, lease_id, UINT16_MAX,
                           &bytes, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_programs_sandbox_read(NULL, lease_id, 0U,
                           &bytes, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, NULL, 0U,
                           &bytes, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, lease_id, 0U,
                           NULL, &length) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, lease_id, 0U,
                           &bytes, NULL) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(f->ctx.staged_count == 0U);
    LXP_CASE_CHECK(f->kernel.module_kv_count == stored_count);
    LXP_CASE_CHECK(lxp_state_subtree_root(&f->kernel, LXP_MODULE_PROGRAMS,
                                          after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, sizeof(before)) == 0);
    return fixture_destroy(f);
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "interface_read_returns_stored_bytes",
          interface_read_returns_stored_bytes },
        { "sandbox_read_selects_kind", sandbox_read_selects_kind }
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
