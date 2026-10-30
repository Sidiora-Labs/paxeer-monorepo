#include "lxp_kernel_case.h"

#include "layerx/lx_stream_read.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/programs.h"

#include <stdlib.h>

lxp_result lx_stream_accrued_read(lxp_module_ctx *ctx,
                                  const uint8_t stream_id[32],
                                  uint64_t timestamp_ms,
                                  lxp_u128 *accrued_total,
                                  lxp_u128 *unsettled);
lxp_result lxp_programs_interface_read(lxp_module_ctx *ctx,
    const uint8_t program_id[32], const uint8_t **bytes, size_t *length);
lxp_result lxp_programs_sandbox_read(lxp_module_ctx *ctx,
    const uint8_t lease_id[32], uint16_t kind,
    const uint8_t **bytes, size_t *length);

typedef struct client_reads_fixture {
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx ctx;
    lxp_arena arena;
    uint8_t arena_bytes[32768];
    lx_account_registry accounts;
    lxp_transfer_asset_state asset;
    lx_programs_transfer_runtime runtime;
    lxp_effect_buffer effects;
    lxp_authority_resolved authority;
    lxp_activity activity;
    lx_account *tenant;
    lx_account *escrow;
    lx_stream_runtime stream_runtime;
    uint8_t stream_id[32];
    uint8_t expected_interface[256];
    size_t expected_interface_length;
    uint64_t parameters;
    uint8_t lease_id[32];
    uint8_t host[32];
    uint8_t requested[1024];
    size_t requested_length;
    size_t state_offset;
    uint8_t fund[2048];
    size_t fund_length;
} client_reads_fixture;

typedef struct client_reads_snapshot {
    lxp_module_kv_entry module_kv[LXP_KERNEL_MAX_MODULE_KV];
    size_t module_kv_count;
    uint8_t root[32];
    uint8_t current_root[32];
} client_reads_snapshot;

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

static void write_u64(uint8_t *bytes, uint64_t value)
{
    for (size_t i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> (56U - 8U * i));
}

static int requested_lease(client_reads_fixture *f, uint64_t funding,
                            const uint64_t *limits)
{
    static const uint8_t domain[] = "LayerX/programs/sandbox/lease-state/v3";
    static const uint8_t namespace_domain[] = "LayerX/programs/sandbox/namespace/v1";
    static const uint8_t request_domain[] = "LayerX/programs/sandbox/request/v3";
    uint8_t namespace_preimage[sizeof(namespace_domain) + 64U];
    uint8_t request_preimage[1024] = {0};
    uint8_t fees[32];
    uint8_t *out = f->requested;
    size_t offset = sizeof(domain), fields = offset, schedule, request_length;

    (void)memcpy(out, domain, sizeof(domain));
    (void)memcpy(out + offset, f->lease_id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->tenant->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->host, 32U); offset += 32U;
    out[offset] = 4U; offset += 32U;
    (void)memcpy(namespace_preimage, namespace_domain, sizeof(namespace_domain));
    (void)memcpy(namespace_preimage + sizeof(namespace_domain), f->host, 32U);
    (void)memcpy(namespace_preimage + sizeof(namespace_domain) + 32U,
                 f->lease_id, 32U);
    LXP_CASE_CHECK(lxp_hash_sha256(namespace_preimage, sizeof(namespace_preimage),
                                   out + offset) == LXP_OK);
    offset += 32U;
    (void)memcpy(out + offset, f->asset.asset_id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->escrow->id, 32U); offset += 32U;
    LXP_CASE_CHECK(lx_account_id_from_string((const uint8_t *)"system:fees",
                                             11U, fees) == LXP_OK);
    (void)memcpy(out + offset, fees, 32U); offset += 32U;
    write_u64(out + offset + 8U, funding); offset += 16U;
    for (size_t i = 0U; i < 8U; ++i) {
        write_u64(out + offset, limits[i]); offset += 8U;
    }
    write_u64(out + offset, 1U); offset += 8U;
    write_u64(out + offset, 100U); offset += 8U;
    request_length = offset - fields;
    offset += 64U;
    schedule = offset;
    write_u32(out + offset, 1U); offset += 4U;
    for (size_t i = 0U; i < 7U; ++i) {
        write_u64(out + offset, 1U); offset += 8U;
    }
    (void)memcpy(request_preimage, request_domain, sizeof(request_domain));
    (void)memcpy(request_preimage + sizeof(request_domain), out + fields,
                 request_length);
    (void)memcpy(request_preimage + sizeof(request_domain) + request_length,
                 out + schedule, 60U);
    offset += 16U;
    f->state_offset = offset;
    out[offset++] = 0U;
    out[offset++] = 1U;
    offset += 3U;
    out[offset] = 8U; offset += 32U;
    LXP_CASE_CHECK(lxp_hash_sha256(request_preimage,
                    sizeof(request_domain) + request_length + 60U,
                    out + offset) == LXP_OK);
    offset += 32U;
    LXP_CASE_CHECK(lxp_hash_sha256(out, offset, out + offset) == LXP_OK);
    offset += 32U;
    write_u64(out + offset, 1U); offset += 8U;
    offset += 3U;
    f->requested_length = offset;
    return 0;
}

static int fixture_begin(client_reads_fixture *f, uint8_t marker)
{
    LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel,
                    LXP_MODULE_PROGRAMS, 1U, 0U, marker, 1000000U,
                    &f->arena, true) == LXP_OK);
    f->ctx.batch_number = marker;
    f->ctx.activity_id[0] = marker;
    LXP_CASE_CHECK(lxp_effect_buffer_init(&f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&f->ctx, &f->effects) == LXP_OK);
    f->activity.activity_type = LX_PROGRAMS_SANDBOX;
    f->activity.account_sequence = f->tenant->next_sequence;
    return 0;
}

static int fixture_init(client_reads_fixture *f)
{
    static const uint64_t limits[8] = {1U, 1U, 1U, 1U, 1U, 1U, 1U, 1U};
    const uint64_t balance = 1000U, funding = 100U;
    static const uint8_t tenant_name[] = "agent:did:lxp:sandbox-tenant:main";
    static const uint8_t seed_domain[] = "sandbox-lease-escrow/v1";
    uint8_t seed[sizeof(seed_domain) + 32U], id[32];
    lx_account_registration registration;
    bool created;
    size_t offset = 4U;
    uint8_t *out = f->fund;

    f->parameters = 1U;
    f->lease_id[0] = 1U;
    f->host[0] = 3U;
    f->asset.asset_id[0] = 9U;
    f->asset.registered = true;
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(tenant_name,
                    sizeof(tenant_name) - 1U, id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, tenant_name,
                    sizeof(tenant_name) - 1U, id, 1U, LX_ACCOUNT_OPEN_GENESIS,
                    NULL, &f->tenant) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(f->tenant, f->asset.asset_id,
                    (lxp_u128){0U, balance}, 0U) == LXP_OK);
    (void)memcpy(seed, seed_domain, sizeof(seed_domain));
    (void)memcpy(seed + sizeof(seed_domain), f->lease_id, 32U);
    LXP_CASE_CHECK(lxp_programs_account_derive(f->host, seed, sizeof(seed), id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_module_value_prepare(&f->accounts,
                        (const uint8_t *)"programs", 8U, id, f->asset.asset_id,
                        1U, &registration, &f->escrow, &created) == LXP_OK);
    LXP_CASE_CHECK(created);
    LXP_CASE_CHECK(lx_account_registration_commit(&f->accounts, &registration,
                                                      &f->escrow) == LXP_OK);

    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                    &f->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                    programs_module_registration_v3()) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    f->runtime.assets = &f->asset;
    f->runtime.asset_count = 1U;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
                    LXP_MODULE_PROGRAMS, &f->runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
                    lxp_kernel_canonical_ledger_apply) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
                                  sizeof(f->arena_bytes)) == LXP_OK);
    (void)memcpy(f->authority.principal, f->tenant->id, 32U);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    f->authority.authority_hash[0] = 7U;
    LXP_CASE_CHECK(fixture_begin(f, 2U) == 0);

    LXP_CASE_CHECK(requested_lease(f, funding, limits) == 0);
    out[0] = 1U; out[1] = 2U;
    (void)memcpy(out + offset, f->lease_id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->tenant->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->host, 32U); offset += 32U;
    (void)memcpy(out + offset, f->escrow->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->asset.asset_id, 32U); offset += 32U;
    write_u64(out + offset + 8U, funding); offset += 16U;
    write_u64(out + offset, 100U); offset += 8U;
    write_u32(out + offset, 1U); offset += 4U;
    for (size_t i = 0U; i < 7U; ++i) {
        write_u64(out + offset, 1U); offset += 8U;
    }
    write_u32(out + offset, (uint32_t)f->requested_length); offset += 4U;
    (void)memcpy(out + offset, f->requested, f->requested_length);
    offset += f->requested_length;
    write_u32(out + offset, 146U); offset += 4U;
    (void)memcpy(out + offset, f->host, 32U); offset += 32U;
    out[offset++] = 0U; out[offset++] = 1U;
    (void)memcpy(out + offset, f->tenant->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->asset.asset_id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->escrow->id, 32U); offset += 32U;
    write_u64(out + offset + 8U, funding); offset += 16U;
    f->fund_length = offset;
    return 0;
}

static int seed_program(client_reads_fixture *f)
{
    static const uint8_t wasm[] = {
        0,97,115,109,1,0,0,0,1,12,2,96,2,127,127,1,127,96,1,127,1,127,
        3,3,2,0,1,5,3,1,0,1,7,41,3,
        11,'l','a','y','e','r','x','_','c','a','l','l',0,0,
        14,'l','a','y','e','r','x','_','r','e','s','e','r','v','e',0,1,
        6,'m','e','m','o','r','y',2,0,10,11,2,4,0,65,0,11,4,0,65,0,11
    };
    static const uint8_t domain[] = "LayerX/program-interface/v1";
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    lx_programs_metering_schedule schedule = {0};
    lxp_activity activity = {0};
    lxp_effect_buffer effects;
    uint8_t payload[512] = {1U};
    uint8_t expected[256] = {0};
    size_t offset = 108U, interface_length;
    void *decoded;

    LXP_CASE_CHECK(manifest != NULL);
    LXP_CASE_CHECK(fixture_begin(f, 2U) == 0);
    (void)memcpy(payload, f->host, 32U);
    payload[34U] = 1U;
    (void)memcpy(payload + 36U, f->authority.principal, 32U);
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
                                                   &f->authority, decoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_lifecycle_execute(&f->ctx, &activity,
                         &f->authority, decoded, &effects) == LXP_OK);
    (void)memcpy(expected, payload, 32U);
    write_u32(expected + 32U, 1U);
    LXP_CASE_CHECK(lxp_hash_sha256(payload + 108U, interface_length,
                                   expected + 36U) == LXP_OK);
    write_u32(expected + 68U, (uint32_t)interface_length);
    (void)memcpy(expected + 72U, payload + 108U, interface_length);
    f->expected_interface_length = 72U + interface_length;
    (void)memcpy(f->expected_interface, expected, f->expected_interface_length);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    return 0;
}

static int seed_sandbox(client_reads_fixture *f)
{
    void *decoded = NULL;

    LXP_CASE_CHECK(fixture_begin(f, 3U) == 0);
    LXP_CASE_CHECK(lxp_programs_sandbox_decode(&f->ctx, f->fund,
                    f->fund_length, &decoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_sandbox_validate(&f->ctx, &f->activity,
                    &f->authority, decoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_sandbox_execute(&f->ctx, &f->activity,
                    &f->authority, decoded, &f->effects) == LXP_OK);
    LXP_CASE_CHECK(f->ctx.staged_count == 3U);
    LXP_CASE_CHECK(f->tenant->balance.lo == 900U);
    LXP_CASE_CHECK(f->escrow->balance.lo == 100U);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    return 0;
}

static int seed_stream(client_reads_fixture *f)
{
    static const uint8_t name[] = "agent:did:lxp:sandbox-tenant:stream:reads";
    const lxp_module_iface *iface = lx_stream_module_iface();
    lx_stream_open_payload open = {0};
    lx_account *account;
    uint8_t id[32], payload[2048];
    size_t length;
    void *decoded = NULL;

    LXP_CASE_CHECK(lx_account_id_from_string(name, sizeof(name) - 1U, id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, name, sizeof(name) - 1U,
                    id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL, &account) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(account, f->asset.asset_id,
                    (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel, iface) == LXP_OK);
    f->stream_runtime.assets = &f->asset;
    f->stream_runtime.asset_count = 1U;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
                    LXP_MODULE_STREAM, &f->stream_runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel,
                    LXP_MODULE_STREAM, 1000U, 0U, 4U, 1000000U,
                    &f->arena, true) == LXP_OK);
    LXP_CASE_CHECK(lxp_effect_buffer_init(&f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&f->ctx, &f->effects) == LXP_OK);
    f->stream_id[0] = 5U;
    (void)memcpy(open.record.stream_id, f->stream_id, 32U);
    (void)memcpy(open.record.payer, f->tenant->id, 32U);
    (void)memcpy(open.record.stream_account, account->id, 32U);
    (void)memcpy(open.record.recipient, f->escrow->id, 32U);
    (void)memcpy(open.record.asset_id, f->asset.asset_id, 32U);
    open.record.mode = LX_STREAM_MODE_TIME;
    open.record.rate = (lxp_u128){0U, 3U};
    open.record.rate_unit = 1000U;
    open.record.start_timestamp = 1000U;
    open.record.last_accrual_timestamp = 1000U;
    open.record.total_cap = (lxp_u128){0U, 100U};
    open.initial_funding = (lxp_u128){0U, 100U};
    LXP_CASE_CHECK(lx_stream_open_encode(&open, payload, sizeof(payload),
                                         &length) == LXP_OK);
    f->activity.activity_type = LX_STREAM_OPEN;
    f->activity.account_sequence = f->tenant->next_sequence;
    LXP_CASE_CHECK(iface->decode(&f->ctx, 1U, payload, length, &decoded) == LXP_OK);
    LXP_CASE_CHECK(iface->validate(&f->ctx, &f->activity,
                                  &f->authority, decoded) == LXP_OK);
    LXP_CASE_CHECK(iface->execute(&f->ctx, &f->activity,
                    &f->authority, decoded, &f->effects) == LXP_OK);
    LXP_CASE_CHECK(account->balance.lo == 100U);
    LXP_CASE_CHECK(f->tenant->balance.lo == 800U);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel,
                                   f->kernel.current_state_root) == LXP_OK);
    return 0;
}

static int fixture_populate(client_reads_fixture *f)
{
    LXP_CASE_CHECK(fixture_init(f) == 0);
    LXP_CASE_CHECK(seed_program(f) == 0);
    LXP_CASE_CHECK(seed_sandbox(f) == 0);
    LXP_CASE_CHECK(seed_stream(f) == 0);
    LXP_CASE_CHECK(f->kernel.module_kv_count != 0U);
    return 0;
}

static int begin_read(client_reads_fixture *f, uint16_t module_id, bool mutable)
{
    LXP_CASE_CHECK(f->ctx.staged_count == 0U);
    LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, module_id,
                    11000U, 0U, 5U, 1000000U, &f->arena, mutable) == LXP_OK);
    return 0;
}

static int snapshot_take(client_reads_fixture *f, client_reads_snapshot *snapshot)
{
    LXP_CASE_CHECK(f->ctx.staged_count == 0U);
    LXP_CASE_CHECK(f->ctx.staged_reserve == 0U);
    snapshot->module_kv_count = f->kernel.module_kv_count;
    (void)memcpy(snapshot->module_kv, f->kernel.module_kv,
                 sizeof(snapshot->module_kv));
    (void)memcpy(snapshot->current_root, f->kernel.current_state_root, 32U);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, snapshot->root) == LXP_OK);
    return 0;
}

static int snapshot_identical(client_reads_fixture *f,
                                const client_reads_snapshot *snapshot)
{
    uint8_t root[32];

    LXP_CASE_CHECK(f->kernel.module_kv_count == snapshot->module_kv_count);
    LXP_CASE_CHECK(memcmp(f->kernel.module_kv, snapshot->module_kv,
                           sizeof(snapshot->module_kv)) == 0);
    LXP_CASE_CHECK(memcmp(f->kernel.current_state_root,
                           snapshot->current_root, 32U) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root, snapshot->root, sizeof(root)) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 0U);
    LXP_CASE_CHECK(f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(f->ctx.staged_blob_count == 0U);
    return 0;
}

static int fixture_destroy(client_reads_fixture *f, client_reads_snapshot *snapshot)
{
    lxp_module_ctx_rollback(&f->ctx);
    while (f->kernel.blob_count != 0U)
        free(f->kernel.blobs[--f->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    free(snapshot);
    free(f);
    return 0;
}

static int client_reads_leave_module_kv_identical(void)
{
    client_reads_fixture *f = calloc(1U, sizeof(*f));
    client_reads_snapshot *snapshot = calloc(1U, sizeof(*snapshot));
    lxp_u128 accrued, unsettled;
    const uint8_t *bytes;
    size_t length;

    LXP_CASE_CHECK(f != NULL && snapshot != NULL);
    LXP_CASE_CHECK(fixture_populate(f) == 0);
    for (unsigned int mode = 0U; mode < 2U; ++mode) {
        LXP_CASE_CHECK(begin_read(f, LXP_MODULE_STREAM, mode != 0U) == 0);
        LXP_CASE_CHECK(snapshot_take(f, snapshot) == 0);
        LXP_CASE_CHECK(lx_stream_accrued_read(&f->ctx, f->stream_id, 11000U,
                                              &accrued, &unsettled) == LXP_OK);
        LXP_CASE_CHECK(accrued.hi == 0U && accrued.lo == 30U);
        LXP_CASE_CHECK(unsettled.hi == 0U && unsettled.lo == 30U);
        LXP_CASE_CHECK(snapshot_identical(f, snapshot) == 0);

        LXP_CASE_CHECK(begin_read(f, LXP_MODULE_PROGRAMS, mode != 0U) == 0);
        LXP_CASE_CHECK(snapshot_take(f, snapshot) == 0);
        LXP_CASE_CHECK(lxp_programs_interface_read(&f->ctx, f->host,
                                                    &bytes, &length) == LXP_OK);
        LXP_CASE_CHECK(length == f->expected_interface_length);
        LXP_CASE_CHECK(memcmp(bytes, f->expected_interface, length) == 0);
        LXP_CASE_CHECK(snapshot_identical(f, snapshot) == 0);

        for (uint16_t kind = 0U; kind < 3U; ++kind) {
            LXP_CASE_CHECK(begin_read(f, LXP_MODULE_PROGRAMS, mode != 0U) == 0);
            LXP_CASE_CHECK(snapshot_take(f, snapshot) == 0);
            LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, f->lease_id, kind,
                                                      &bytes, &length) == LXP_OK);
            LXP_CASE_CHECK(bytes != NULL && length != 0U);
            LXP_CASE_CHECK(snapshot_identical(f, snapshot) == 0);
        }
    }
    return fixture_destroy(f, snapshot);
}

static int client_reads_unknown_ids_refused(void)
{
    client_reads_fixture *f = calloc(1U, sizeof(*f));
    client_reads_snapshot *snapshot = calloc(1U, sizeof(*snapshot));
    const uint8_t absent[32] = {0xFFU};
    lxp_u128 accrued = {0U, 99U}, unsettled = {0U, 98U};
    const uint8_t *bytes = NULL;
    size_t length = 0U;

    LXP_CASE_CHECK(f != NULL && snapshot != NULL);
    LXP_CASE_CHECK(fixture_populate(f) == 0);
    for (unsigned int mode = 0U; mode < 2U; ++mode) {
        LXP_CASE_CHECK(begin_read(f, LXP_MODULE_STREAM, mode != 0U) == 0);
        LXP_CASE_CHECK(snapshot_take(f, snapshot) == 0);
        LXP_CASE_CHECK(lx_stream_accrued_read(&f->ctx, absent, 11000U,
                        &accrued, &unsettled) == LXP_ERR_UNKNOWN_FIELD);
        LXP_CASE_CHECK(accrued.hi == 0U && accrued.lo == 99U);
        LXP_CASE_CHECK(unsettled.hi == 0U && unsettled.lo == 98U);
        LXP_CASE_CHECK(snapshot_identical(f, snapshot) == 0);

        LXP_CASE_CHECK(begin_read(f, LXP_MODULE_PROGRAMS, mode != 0U) == 0);
        LXP_CASE_CHECK(snapshot_take(f, snapshot) == 0);
        LXP_CASE_CHECK(lxp_programs_interface_read(&f->ctx, absent,
                               &bytes, &length) == LXP_ERR_UNKNOWN_FIELD);
        LXP_CASE_CHECK(snapshot_identical(f, snapshot) == 0);

        for (uint16_t kind = 0U; kind < 3U; ++kind) {
            LXP_CASE_CHECK(begin_read(f, LXP_MODULE_PROGRAMS, mode != 0U) == 0);
            LXP_CASE_CHECK(snapshot_take(f, snapshot) == 0);
            LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, absent, kind,
                                   &bytes, &length) == LXP_ERR_UNKNOWN_FIELD);
            LXP_CASE_CHECK(snapshot_identical(f, snapshot) == 0);
        }
    }
    return fixture_destroy(f, snapshot);
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"client_reads_leave_module_kv_identical", client_reads_leave_module_kv_identical},
        {"client_reads_unknown_ids_refused", client_reads_unknown_ids_refused}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
