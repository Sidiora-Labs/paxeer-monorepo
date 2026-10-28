#include "lxp_kernel_case.h"

#include "layerx/lxp_hash.h"
#include "../../src/modules/programs/artifact.h"
#include "../../src/modules/programs/occupancy.h"
#include "../../src/modules/programs/sandbox.c"

typedef struct sandbox_fixture {
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
    uint64_t parameters;
    uint8_t lease_id[32];
    uint8_t host[32];
    uint8_t requested[1024];
    size_t requested_length;
    size_t state_offset;
    uint8_t fund[2048];
    size_t fund_length;
} sandbox_fixture;

static void put_u32(uint8_t *bytes, uint32_t value)
{
    bytes[0] = (uint8_t)(value >> 24U);
    bytes[1] = (uint8_t)(value >> 16U);
    bytes[2] = (uint8_t)(value >> 8U);
    bytes[3] = (uint8_t)value;
}

static int requested_lease(sandbox_fixture *f)
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
    write_u64(out + offset + 8U, 100U); offset += 16U;
    for (size_t i = 0U; i < 8U; ++i) {
        write_u64(out + offset, 1U); offset += 8U;
    }
    write_u64(out + offset, 1U); offset += 8U;
    write_u64(out + offset, 100U); offset += 8U;
    request_length = offset - fields;
    offset += 64U;
    schedule = offset;
    put_u32(out + offset, 1U); offset += 4U;
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

static int fixture_begin(sandbox_fixture *f, uint8_t marker)
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

static int fixture_init(sandbox_fixture *f)
{
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
                    (lxp_u128){0U, 1000U}, 0U) == LXP_OK);
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
    f->authority.authority_hash[0] = 7U;
    LXP_CASE_CHECK(fixture_begin(f, 2U) == 0);
    LXP_CASE_CHECK(requested_lease(f) == 0);
    out[0] = 1U; out[1] = SANDBOX_FUND;
    (void)memcpy(out + offset, f->lease_id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->tenant->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->host, 32U); offset += 32U;
    (void)memcpy(out + offset, f->escrow->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->asset.asset_id, 32U); offset += 32U;
    write_u64(out + offset + 8U, 100U); offset += 16U;
    write_u64(out + offset, 100U); offset += 8U;
    put_u32(out + offset, 1U); offset += 4U;
    for (size_t i = 0U; i < 7U; ++i) {
        write_u64(out + offset, 1U); offset += 8U;
    }
    put_u32(out + offset, (uint32_t)f->requested_length); offset += 4U;
    (void)memcpy(out + offset, f->requested, f->requested_length);
    offset += f->requested_length;
    put_u32(out + offset, 146U); offset += 4U;
    (void)memcpy(out + offset, f->host, 32U); offset += 32U;
    out[offset++] = 0U; out[offset++] = 1U;
    (void)memcpy(out + offset, f->tenant->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->asset.asset_id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->escrow->id, 32U); offset += 32U;
    write_u64(out + offset + 8U, 100U); offset += 16U;
    f->fund_length = offset;
    return 0;
}

static int buffers_released(const programs_sandbox_activity *value)
{
    LXP_CASE_CHECK(active_sandbox == NULL);
    LXP_CASE_CHECK(value->lifecycle_reserved == 0U);
    LXP_CASE_CHECK(value->lifecycle_reserve == 0U);
    LXP_CASE_CHECK(value->host_reserved == 0U && value->reservation_token == 0U);
    LXP_CASE_CHECK(value->usage_result.receipt == NULL);
    for (size_t kind = 0U; kind < 3U; ++kind) {
        LXP_CASE_CHECK(value->staged[kind].bytes == NULL);
        LXP_CASE_CHECK(value->staged[kind].capacity == 0U);
        LXP_CASE_CHECK(value->staged[kind].length == 0U);
        LXP_CASE_CHECK(value->staged[kind].written == 0U);
        LXP_CASE_CHECK(value->staged[kind].begun == 0U);
        LXP_CASE_CHECK(value->staged[kind].applied == 0U);
    }
    return 0;
}

static int execute_payload(sandbox_fixture *f, const uint8_t *payload,
                           size_t length, lxp_result expected)
{
    void *decoded = NULL;
    LXP_CASE_CHECK(lxp_programs_sandbox_decode(&f->ctx, payload, length,
                                                &decoded) == LXP_OK);
    if (((programs_sandbox_activity *)decoded)->operation == SANDBOX_EXECUTE)
        LXP_CASE_CHECK(lxp_programs_sandbox_validate(&f->ctx, &f->activity,
                        &f->authority, decoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_sandbox_execute(&f->ctx, &f->activity,
                    &f->authority, decoded, &f->effects) == expected);
    LXP_CASE_CHECK(!((programs_sandbox_activity *)decoded)->guest_savepoint.active);
    LXP_CASE_CHECK(buffers_released(decoded) == 0);
    return 0;
}

static int activate_payload(sandbox_fixture *f, uint8_t payload[76])
{
    const uint8_t *current;
    size_t length;
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, f->lease_id, 0U,
                                             &current, &length) == LXP_OK);
    (void)memset(payload, 0, 76U);
    payload[0] = 1U; payload[1] = SANDBOX_ACTIVATE;
    (void)memcpy(payload + 4U, f->lease_id, 32U);
    LXP_CASE_CHECK(lxp_hash_sha256(current, length, payload + 36U) == LXP_OK);
    write_u64(payload + 68U, 3U);
    return 0;
}

static int fixture_destroy(sandbox_fixture *f)
{
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int sandbox_fund_activate_native(void)
{
    sandbox_fixture *f = calloc(1U, sizeof(*f));
    const uint8_t *bytes;
    size_t length, funded_length;
    uint8_t activate[76];

    LXP_CASE_CHECK(f != NULL && fixture_init(f) == 0);
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length, LXP_OK) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 3U && f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(f->tenant->balance.lo == 900U && f->escrow->balance.lo == 100U);
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length,
                                    LXP_ERR_IDEMPOTENT_REPLAY) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 3U && f->ctx.staged_reserve == 0U);
    for (uint16_t kind = 0U; kind < 3U; ++kind) {
        LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, f->lease_id, kind,
                                                 &bytes, &length) == LXP_OK);
        LXP_CASE_CHECK(length != 0U);
    }
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, f->lease_id, 0U,
                                             &bytes, &funded_length) == LXP_OK);
    LXP_CASE_CHECK(bytes[f->state_offset] == 1U);
    LXP_CASE_CHECK(bytes[f->state_offset + 1U] == 2U);
    LXP_CASE_CHECK(activate_payload(f, activate) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture_begin(f, 3U) == 0);
    activate[36U] ^= 1U;
    LXP_CASE_CHECK(execute_payload(f, activate, sizeof(activate),
                                    LXP_ERR_NON_CANONICAL) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 0U && f->ctx.staged_reserve == 0U);
    activate[36U] ^= 1U;
    LXP_CASE_CHECK(execute_payload(f, activate, sizeof(activate), LXP_OK) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 1U && f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, f->lease_id, 0U,
                                             &bytes, &length) == LXP_OK);
    LXP_CASE_CHECK(bytes[f->state_offset] == 2U);
    LXP_CASE_CHECK(bytes[f->state_offset + 1U] == 3U);
    LXP_CASE_CHECK(length == funded_length + 107U);
    LXP_CASE_CHECK(execute_payload(f, activate, sizeof(activate),
                                    LXP_ERR_NON_CANONICAL) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 1U && f->ctx.staged_reserve == 0U);
    return fixture_destroy(f);
}

static int fill_staging(sandbox_fixture *f, size_t count)
{
    const size_t before = f->ctx.staged_count;
    for (size_t i = 0U; i < count; ++i) {
        uint8_t key[3] = {0xf1U, (uint8_t)(i >> 8U), (uint8_t)i};
        uint8_t byte = (uint8_t)i;
        LXP_CASE_CHECK(lxp_ctx_kv_put(&f->ctx, key, sizeof(key),
                                      &byte, 1U) == LXP_OK);
    }
    LXP_CASE_CHECK(f->ctx.staged_count == before + count);
    return 0;
}

static int sandbox_stage_begin_accepts_lifecycle(void)
{
    sandbox_fixture *f = calloc(1U, sizeof(*f));
    uint8_t activate[76];
    size_t before;

    LXP_CASE_CHECK(f != NULL && fixture_init(f) == 0);
    LXP_CASE_CHECK(fill_staging(f, LXP_MODULE_MAX_STAGED_WRITES - 2U) == 0);
    before = f->ctx.staged_count;
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length,
                                    LXP_ERR_ARENA_EXHAUSTED) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == before && f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(f->tenant->balance.lo == 1000U && f->escrow->balance.lo == 0U);
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(fixture_begin(f, 2U) == 0);
    LXP_CASE_CHECK(lxp_module_staged_reserve(&f->ctx, 1U) == LXP_OK);
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length,
                                    LXP_ERR_ARENA_EXHAUSTED) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 0U && f->ctx.staged_reserve == 1U);
    LXP_CASE_CHECK(lxp_module_staged_release(&f->ctx, 1U) == LXP_OK);
    f->fund[36U] ^= 1U;
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length,
                                    LXP_ERR_NON_CANONICAL) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 0U && f->ctx.staged_reserve == 0U);
    f->fund[36U] ^= 1U;
    f->fund[f->fund_length - 112U] ^= 1U;
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length,
                                    LXP_ERR_UNKNOWN_ACCOUNT_NAMESPACE) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == 0U && f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(f->tenant->balance.lo == 1000U && f->escrow->balance.lo == 0U);
    f->fund[f->fund_length - 112U] ^= 1U;
    LXP_CASE_CHECK(fill_staging(f, LXP_MODULE_MAX_STAGED_WRITES - 3U) == 0);
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length, LXP_OK) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == LXP_MODULE_MAX_STAGED_WRITES);
    LXP_CASE_CHECK(f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(activate_payload(f, activate) == 0);
    before = f->ctx.staged_count;
    LXP_CASE_CHECK(execute_payload(f, activate, sizeof(activate),
                                    LXP_ERR_ARENA_EXHAUSTED) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == before && f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture_begin(f, 3U) == 0);
    LXP_CASE_CHECK(fill_staging(f, LXP_MODULE_MAX_STAGED_WRITES - 1U) == 0);
    LXP_CASE_CHECK(execute_payload(f, activate, sizeof(activate), LXP_OK) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == LXP_MODULE_MAX_STAGED_WRITES);
    LXP_CASE_CHECK(f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(layerx_programs_call_sandbox_state_stage_begin(0U, 0U, 1U) ==
                    LXP_ERR_NON_CANONICAL);
    return fixture_destroy(f);
}

static int sandbox_execute_refusal_propagates(void)
{
    static const uint8_t wasm[] = {
        0,97,115,109,1,0,0,0,1,12,2,96,2,127,127,1,127,96,1,127,1,127,
        3,3,2,0,1,5,3,1,0,1,7,41,3,
        11,'l','a','y','e','r','x','_','c','a','l','l',0,0,
        14,'l','a','y','e','r','x','_','r','e','s','e','r','v','e',0,1,
        6,'m','e','m','o','r','y',2,0,10,11,2,4,0,65,0,11,4,0,65,0,11
    };
    static const uint8_t entrypoint[] = "layerx_call";
    static const uint64_t budget[LX_PROGRAMS_CALL_BUDGET_FIELDS] = {
        39U, 65536U, 0U, 0U, 1U, 0U, 0U
    };
    sandbox_fixture *f = calloc(1U, sizeof(*f));
    const size_t arena_capacity = sizeof(lxp_programs_occupancy_bridge) +
                                  sizeof(lxp_receipt) + 2U * sizeof(f->arena_bytes);
    uint8_t *arena_bytes = malloc(arena_capacity);
    uint8_t activate[76], payload[512] = {1U, SANDBOX_EXECUTE};
    uint8_t key[40] = "program", record[71] = {1U};
    const uint8_t *lease;
    size_t lease_length, offset = 4U, call_offset, staged_count, effect_count;
    lxp_call_admission_facts *admission;

    LXP_CASE_CHECK(f != NULL && arena_bytes != NULL && fixture_init(f) == 0);
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length, LXP_OK) == 0);
    LXP_CASE_CHECK(activate_payload(f, activate) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture_begin(f, 3U) == 0);
    LXP_CASE_CHECK(execute_payload(f, activate, sizeof(activate), LXP_OK) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture_begin(f, 4U) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, arena_bytes, arena_capacity) == LXP_OK);
    f->ctx.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    (void)memcpy(key + 8U, f->host, 32U);
    (void)memcpy(record + 1U, f->tenant->id, 32U);
    LXP_CASE_CHECK(lxp_hash_sha256(wasm, sizeof(wasm), record + 33U) == LXP_OK);
    record[66U] = LX_PROGRAMS_GUEST_ABI_V2_VERSION;
    put_u32(record + 67U, 1U);
    LXP_CASE_CHECK(lxp_ctx_kv_put(&f->ctx, key, sizeof(key),
                                  record, sizeof(record)) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_artifact_store(&f->ctx, f->host,
                    record + 33U, wasm, sizeof(wasm)) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_account_owner_bind(&f->ctx, f->host,
                                                   f->tenant->id) == LXP_OK);
    admission = &f->ctx.call_admission;
    admission->present = true;
    (void)memcpy(admission->activity_binding, f->ctx.activity_id, 32U);
    (void)memcpy(admission->payer, f->tenant->id, 32U);
    admission->available_fee_units = f->tenant->balance;
    admission->signed_fee_limit = (lxp_u128){0U, 65576U};
    admission->fee_schedule_version = 1U;
    admission->metering_schedule_version = LXP_PROGRAM_METERING_SCHEDULE_VERSION_V1;
    admission->parameter_version = 1U;
    for (size_t i = 0U; i < 9U; ++i)
        admission->metering_schedule_coefficients[i] = 1U;
    admission->metering_schedule_coefficients[5U] = 8U;
    admission->metering_schedule_coefficients[6U] = 8U;
    admission->metering_schedule_coefficients[7U] = 64U;
    admission->metering_schedule_coefficients[8U] = 8U;
    for (size_t i = 0U; i < 7U; ++i)
        admission->fee_schedule_prices[i] = 1U;
    (void)memcpy(payload + offset, f->lease_id, 32U); offset += 32U;
    write_u64(payload + offset, 4U); offset += 8U;
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, f->lease_id, 0U,
                                             &lease, &lease_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_sha256(lease, lease_length, payload + offset) == LXP_OK);
    payload[offset] ^= 1U; offset += 32U;
    (void)memcpy(payload + offset, f->escrow->id, 32U); offset += 32U;
    (void)memcpy(payload + offset, f->asset.asset_id, 32U); offset += 32U;
    LXP_CASE_CHECK(lx_account_id_from_string((const uint8_t *)"system:fees",
                                             11U, payload + offset) == LXP_OK);
    offset += 32U;
    put_u32(payload + offset, 1U); offset += 4U;
    for (size_t i = 0U; i < 7U; ++i) {
        write_u64(payload + offset, 1U); offset += 8U;
    }
    call_offset = offset + 4U;
    offset = call_offset;
    (void)memcpy(payload + offset, f->host, 32U); offset += 32U;
    payload[offset + 1U] = LX_PROGRAMS_GUEST_ABI_V2_VERSION; offset += 2U;
    payload[offset + 1U] = (uint8_t)(sizeof(entrypoint) - 1U); offset += 2U;
    offset += 14U;
    for (size_t i = 0U; i < LX_PROGRAMS_CALL_BUDGET_FIELDS; ++i) {
        write_u64(payload + offset, budget[i]); offset += 8U;
    }
    (void)memcpy(payload + offset, entrypoint, sizeof(entrypoint) - 1U);
    offset += sizeof(entrypoint) - 1U;
    put_u32(payload + call_offset - 4U, (uint32_t)(offset - call_offset));
    staged_count = f->ctx.staged_count;
    effect_count = f->effects.count;
    LXP_CASE_CHECK(execute_payload(f, payload, offset, LXP_ERR_NON_CANONICAL) == 0);
    LXP_CASE_CHECK(f->ctx.staged_count == staged_count && f->ctx.staged_reserve == 0U);
    LXP_CASE_CHECK(f->effects.count == effect_count);
    LXP_CASE_CHECK(f->tenant->balance.lo == 900U && f->escrow->balance.lo == 100U);
    LXP_CASE_CHECK(fixture_destroy(f) == 0);
    free(arena_bytes);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "sandbox_fund_activate_native", sandbox_fund_activate_native },
        { "sandbox_stage_begin_accepts_lifecycle",
          sandbox_stage_begin_accepts_lifecycle },
        { "sandbox_execute_refusal_propagates", sandbox_execute_refusal_propagates }
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
