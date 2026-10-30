#include "lxp_kernel_case.h"
#include "layerx/lxp_genesis.h"
#include "../../src/modules/programs/artifact.h"
#include "../../src/modules/programs/occupancy.h"
#include "../../src/modules/programs/storage.h"
#include "../../src/protocol/lxp_kernel.c"

#include <openssl/evp.h>

static const uint8_t receipt_seed[32] = {1U};
static const uint8_t receipt_did[] = "did:lxp:sandbox-tenant";

static int receipt_sign(const uint8_t digest[32], uint8_t signature[64],
                        uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(
        EVP_PKEY_ED25519, NULL, receipt_seed, sizeof(receipt_seed));
    EVP_MD_CTX *context = EVP_MD_CTX_new();
    size_t key_length = 32U, signature_length = 64U;
    int ok = key != NULL && context != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &key_length) == 1 &&
        key_length == 32U &&
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(context, signature, &signature_length, digest, 32U) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(ok);
    return 0;
}

static void write_u64(uint8_t *bytes, uint64_t value)
{
    for (size_t i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> (56U - 8U * i));
}

typedef struct sandbox_fixture {
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_identity_store identities;
    uint8_t public_key[32];
    uint8_t signature[64];
    lxp_module_ctx ctx;
    lxp_arena arena;
    uint8_t arena_bytes[4U * 1024U * 1024U];
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

static int requested_lease(sandbox_fixture *f, uint64_t funding,
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

static int fixture_init_with_resources(sandbox_fixture *f, uint64_t balance,
                                        uint64_t funding, const uint64_t *limits,
                                        const uint8_t *wasm, size_t wasm_length)
{
    static const uint8_t tenant_name[] = "agent:did:lxp:sandbox-tenant:main";
    static const uint8_t seed_domain[] = "sandbox-lease-escrow/v1";
    uint8_t seed[sizeof(seed_domain) + 32U], id[32];
    lx_account_registration registration;
    lxp_identity *identity;
    uint8_t digest[32] = {0};
    bool created;
    size_t offset = 4U;
    uint8_t *out = f->fund;

    LXP_CASE_CHECK(receipt_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_identity_register(&f->identities, receipt_did,
        sizeof(receipt_did) - 1U, f->public_key, &identity) == LXP_OK);
    memcpy(f->authority.actor, identity->did_id, 32U);
    memcpy(f->authority.verified_key, f->public_key, 32U);
    f->authority.kind = LXP_AUTHORITY_OWNER;
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
                    (lxp_u128){0U, balance}, 2U) == LXP_OK);
    (void)memcpy(seed, seed_domain, sizeof(seed_domain));
    (void)memcpy(seed + sizeof(seed_domain), f->lease_id, 32U);
    LXP_CASE_CHECK(lxp_programs_account_derive(f->host, seed, sizeof(seed), id) == LXP_OK);
    if (wasm == NULL) {
        LXP_CASE_CHECK(lx_account_module_value_prepare(&f->accounts,
                        (const uint8_t *)"programs", 8U, id, f->asset.asset_id,
                        1U, &registration, &f->escrow, &created) == LXP_OK);
        LXP_CASE_CHECK(created);
        LXP_CASE_CHECK(lx_account_registration_commit(&f->accounts, &registration,
                                                      &f->escrow) == LXP_OK);
    } else {
        static const uint8_t fee_name[] = "system:fees";
        lx_account *treasury;
        LXP_CASE_CHECK(lx_account_id_from_string(fee_name,
                        sizeof(fee_name) - 1U, id) == LXP_OK);
        LXP_CASE_CHECK(lx_account_open(&f->accounts, fee_name,
                        sizeof(fee_name) - 1U, id, 1U, LX_ACCOUNT_OPEN_GENESIS,
                        NULL, &treasury) == LXP_OK);
        LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(treasury, f->asset.asset_id,
                        (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    }
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, wasm == NULL ? 1U : 2U) == LXP_OK);
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
    (void)memcpy(f->authority.principal, f->authority.actor, 32U);
    f->authority.authority_hash[0] = 7U;
    LXP_CASE_CHECK(fixture_begin(f, 2U) == 0);
    if (wasm != NULL) {
        uint8_t program_key[40] = "program", program[71] = {1U};
        uint8_t account_payload[73U + sizeof(seed)] = {0};
        lx_programs_account_binding binding;
        void *decoded = NULL;
        LXP_CASE_CHECK(lxp_state_journal_open(&f->state,
                        f->ctx.global_sequence, &f->journal) == LXP_OK);
        f->ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
        (void)memcpy(program_key + 8U, f->host, 32U);
        (void)memcpy(program + 1U, f->authority.actor, 32U);
        LXP_CASE_CHECK(lxp_hash_sha256(wasm, wasm_length, program + 33U) == LXP_OK);
        program[66U] = LX_PROGRAMS_GUEST_ABI_V2_VERSION;
        put_u32(program + 67U, 1U);
        LXP_CASE_CHECK(lxp_ctx_kv_put(&f->ctx, program_key, sizeof(program_key),
                                      program, sizeof(program)) == LXP_OK);
        LXP_CASE_CHECK(lxp_programs_artifact_store(&f->ctx, f->host,
                        program + 33U, wasm, wasm_length) == LXP_OK);
        LXP_CASE_CHECK(lxp_programs_account_owner_bind(&f->ctx, f->host,
                                                       f->authority.actor) == LXP_OK);
        (void)memcpy(account_payload, f->host, 32U);
        (void)memcpy(account_payload + 32U, "LXPA2", 5U);
        (void)memcpy(account_payload + 37U, f->asset.asset_id, 32U);
        put_u32(account_payload + 69U, (uint32_t)sizeof(seed));
        (void)memcpy(account_payload + 73U, seed, sizeof(seed));
        LXP_CASE_CHECK(lxp_programs_account_decode(&f->ctx, account_payload,
                        sizeof(account_payload), &decoded) == LXP_OK);
        LXP_CASE_CHECK(lxp_programs_account_validate(&f->ctx, &f->activity,
                        &f->authority, decoded) == LXP_OK);
        LXP_CASE_CHECK(lxp_programs_account_execute(&f->ctx, &f->activity,
                        &f->authority, decoded, &f->effects) == LXP_OK);
        LXP_CASE_CHECK(lxp_module_ctx_prepare_commit(&f->ctx) == LXP_OK);
        LXP_CASE_CHECK(lxp_state_journal_commit(&f->journal) == LXP_OK);
        LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
        LXP_CASE_CHECK(fixture_begin(f, 2U) == 0);
        LXP_CASE_CHECK(lxp_programs_account_lookup(&f->ctx, f->host, seed,
                        sizeof(seed), &binding, &f->escrow) == LXP_OK);
    }
    memcpy(f->authority.principal, f->tenant->id, 32U);
    LXP_CASE_CHECK(requested_lease(f, funding, limits) == 0);
    out[0] = 1U; out[1] = 2U;
    (void)memcpy(out + offset, f->lease_id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->tenant->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->host, 32U); offset += 32U;
    (void)memcpy(out + offset, f->escrow->id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->asset.asset_id, 32U); offset += 32U;
    write_u64(out + offset + 8U, funding); offset += 16U;
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
    write_u64(out + offset + 8U, funding); offset += 16U;
    f->fund_length = offset;
    return 0;
}

static int execute_payload(sandbox_fixture *f, const uint8_t *payload,
                           size_t length, lxp_result expected)
{
    void *decoded = NULL;
    LXP_CASE_CHECK(lxp_programs_sandbox_decode(&f->ctx, payload, length,
                                              &decoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_sandbox_execute(&f->ctx, &f->activity,
        &f->authority, decoded, &f->effects) == expected);
    return 0;
}

static int activate_payload(sandbox_fixture *f, uint8_t payload[76])
{
    const uint8_t *current;
    size_t length;
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, f->lease_id, 0U,
                                             &current, &length) == LXP_OK);
    (void)memset(payload, 0, 76U);
    payload[0] = 1U; payload[1] = 3U;
    (void)memcpy(payload + 4U, f->lease_id, 32U);
    LXP_CASE_CHECK(lxp_hash_sha256(current, length, payload + 36U) == LXP_OK);
    write_u64(payload + 68U, 3U);
    return 0;
}

static int sandbox_execute_receipt_binds_escrow_charge(void)
{
    static const uint8_t wasm[] = {
        0,97,115,109,1,0,0,0,1,12,2,96,2,127,127,1,127,96,1,127,1,127,
        3,3,2,0,1,5,4,1,1,1,1,7,41,3,
        11,'l','a','y','e','r','x','_','c','a','l','l',0,0,
        14,'l','a','y','e','r','x','_','r','e','s','e','r','v','e',0,1,
        6,'m','e','m','o','r','y',2,0,10,11,2,4,0,65,0,11,4,0,65,0,11
    };
    static const uint8_t entrypoint[] = "layerx_call";
    static const uint8_t access[] = "LayerX/programs/access-declaration/v1\0";
    static const uint64_t budget[LX_PROGRAMS_CALL_BUDGET_FIELDS] = {
        100000U, 65536U, 0U, 0U, 1U, 0U, 0U
    };
    static const uint64_t limits[8] = {
        100000U, 65536U, 0U, 0U, 1U, 0U, 0U, 0U
    };
    sandbox_fixture *f = calloc(1U, sizeof(*f));
    const size_t arena_capacity = sizeof(lxp_programs_occupancy_bridge) +
        sizeof(lxp_receipt) + 2U * sizeof(f->arena_bytes);
    uint8_t *arena_bytes = malloc(arena_capacity);
    uint8_t activate[76], payload[512] = {1U, 1U};
    uint8_t key[40] = "program", record[71] = {1U}, storage_namespace[65];
    uint8_t digest[32];
    const uint8_t *lease;
    size_t lease_length, offset = 4U, call_offset, staged_count;
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    lx_programs_fee_genesis_parameters fee_genesis = {0};
    lx_programs_metering_schedule metering = {0};
    lxp_kernel_execution execution = {0};
    lxp_fee_params fees = {0};
    lxp_receipt *receipt = calloc(1U, sizeof(*receipt));
    size_t monetary_transfers = 0U;
    lxp_result status;

    LXP_CASE_CHECK(f != NULL && arena_bytes != NULL);
    LXP_CASE_CHECK(fixture_init_with_resources(f, 2000000U, 1000000U,
                                                limits, wasm, sizeof(wasm)) == 0);
    LXP_CASE_CHECK(execute_payload(f, f->fund, f->fund_length, LXP_OK) == 0);
    LXP_CASE_CHECK(f->tenant->balance.hi == 0U &&
        f->tenant->balance.lo == 1000000U);
    LXP_CASE_CHECK(f->escrow->balance.hi == 0U &&
        f->escrow->balance.lo == 1000000U);
    LXP_CASE_CHECK(activate_payload(f, activate) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture_begin(f, 3U) == 0);
    LXP_CASE_CHECK(execute_payload(f, activate, sizeof(activate), LXP_OK) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(fixture_begin(f, 4U) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, arena_bytes, arena_capacity) == LXP_OK);
    f->ctx.global_sequence = f->state.next_sequence;
    memcpy(f->authority.principal, f->authority.actor, 32U);
    f->ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    (void)memcpy(key + 8U, f->host, 32U);
    (void)memcpy(record + 1U, f->authority.actor, 32U);
    LXP_CASE_CHECK(lxp_hash_sha256(wasm, sizeof(wasm), record + 33U) == LXP_OK);
    record[66U] = LX_PROGRAMS_GUEST_ABI_V3_VERSION;
    put_u32(record + 67U, 1U);
    LXP_CASE_CHECK(lxp_ctx_kv_put(&f->ctx, key, sizeof(key),
                                  record, sizeof(record)) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_artifact_store(&f->ctx, f->host,
                    record + 33U, wasm, sizeof(wasm)) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_account_owner_bind(&f->ctx, f->host,
                                                   f->authority.actor) == LXP_OK);
    (void)memcpy(storage_namespace, f->host, 32U);
    storage_namespace[32U] = 0U;
    (void)memcpy(storage_namespace + 33U, f->authority.principal, 32U);
    staged_count = f->ctx.staged_count;
    LXP_CASE_CHECK(lxp_programs_storage_stage_final(&f->ctx, storage_namespace,
                                                    65U, NULL, 0U) == LXP_OK);
    storage_namespace[32U] = 1U;
    LXP_CASE_CHECK(lxp_programs_storage_stage_final(&f->ctx, storage_namespace,
                                                    33U, NULL, 0U) == LXP_OK);
    LXP_CASE_CHECK(f->ctx.staged_count == staged_count + 2U);
    (void)memcpy(payload + offset, f->lease_id, 32U); offset += 32U;
    write_u64(payload + offset, 1U); offset += 8U;
    LXP_CASE_CHECK(lxp_programs_sandbox_read(&f->ctx, f->lease_id, 0U,
                                             &lease, &lease_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_sha256(lease, lease_length, payload + offset) == LXP_OK);
    offset += 32U;
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
    payload[offset + 1U] = LX_PROGRAMS_GUEST_ABI_V3_VERSION; offset += 2U;
    payload[offset + 1U] = (uint8_t)(sizeof(entrypoint) - 1U); offset += 2U;
    offset += 4U;
    payload[offset + 1U] = 2U; offset += 2U;
    put_u32(payload + offset, (uint32_t)sizeof(access)); offset += 4U;
    offset += 4U;
    for (size_t i = 0U; i < LX_PROGRAMS_CALL_BUDGET_FIELDS; ++i) {
        write_u64(payload + offset, budget[i]); offset += 8U;
    }
    (void)memcpy(payload + offset, entrypoint, sizeof(entrypoint) - 1U);
    offset += sizeof(entrypoint) - 1U;
    offset += 2U;
    (void)memcpy(payload + offset, access, sizeof(access)); offset += sizeof(access);
    put_u32(payload + call_offset - 4U, (uint32_t)(offset - call_offset));
    LXP_CASE_CHECK(manifest != NULL && receipt != NULL);
    LXP_CASE_CHECK(lxp_module_ctx_prepare_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    memcpy(manifest->signer_public_key, f->public_key, 32U);
    fee_genesis.schedule = (lx_programs_fee_schedule){
        1U, 1U, 1U, 1U, 1U, 1U, 1U, 1U};
    memcpy(fee_genesis.occupancy_asset_id, f->asset.asset_id, 32U);
    fee_genesis.target_occupancy_byte_batches = 3U;
    fee_genesis.response_denominator = 1U;
    fee_genesis.maximum_change_numerator = 1U;
    fee_genesis.maximum_change_denominator = 1U;
    fee_genesis.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fee_genesis.maximum_fee_units_per_occupancy_byte_batch = 10U;
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(manifest,
        &fee_genesis) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_materialize(manifest,
        &f->kernel) == LXP_OK);
    metering.version = 1U;
    metering.activation_batch = 1U;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    for (size_t i = 0U; i < 9U; ++i)
        metering.coefficients[i] = i < 5U ? 1U : (i == 7U ? 64U : 8U);
    LXP_CASE_CHECK(lxp_hash_payload(f->public_key, 32U,
        metering.authority_digest) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_append(manifest,
        &metering) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_materialize(manifest,
        &f->kernel) == LXP_OK);
    f->runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;
    f->runtime.metering_schedule_context = &f->kernel;
    f->runtime.resolve_occupancy_parameters =
        lxp_programs_fee_governance_resolve_runtime;
    f->runtime.occupancy_parameter_context = &f->kernel;
    fees.version = 1U;
    fees.multiplier_basis_points = 10000U;
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->activity.network_id = 7U;
    f->activity.actor_did = (lxp_byte_span){receipt_did, sizeof(receipt_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.account_sequence = f->identities.identities[0].next_sequence;
    f->activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    f->activity.fee_limit = (lxp_u128){0U, 165537U};
    f->activity.idempotency_key[0] = 1U;
    f->activity.payload = (lxp_byte_span){payload, offset};
    LXP_CASE_CHECK(lxp_hash_payload(payload, offset,
        f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    LXP_CASE_CHECK(receipt_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    execution.network_id = 7U;
    execution.batch_number = 4U;
    execution.batch_timestamp_ms = 10U;
    execution.maximum_timestamp_window = 100U;
    execution.global_sequence = f->state.next_sequence;
    execution.recorded_module_version =
        programs_module_registration_v3()->abi_version;
    execution.recorded_fee_schedule_version = 1U;
    execution.recorded_metering_schedule_version = 1U;
    execution.parameter_version = 1U;
    execution.signature_valid = true;
    execution.identities = &f->identities;
    execution.authority = &f->authority;
    execution.fee_parameters = &fees;
    execution.fee_balance = f->tenant->balance;
    execution.gas_limit = 1000000U;
    execution.arena = &f->arena;
    execution.sequencer_private_key = receipt_seed;
    execution.batch_id[0] = 5U;
    LXP_CASE_CHECK(lxp_state_root(&f->kernel,
        f->kernel.current_state_root) == LXP_OK);
    LXP_CASE_CHECK(f->tenant->next_sequence == execution.global_sequence);
    status = lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &execution, receipt);
    if (status != LXP_OK || receipt->result_code != LXP_OK)
        fprintf(stderr, "%s: kernel result=%d receipt result=%d terminal=%u "
            "outcome result=%d\n", __func__, status, receipt->result_code,
            (unsigned int)receipt->program_outcome.terminal_kind,
            receipt->program_outcome.result_code);
    LXP_CASE_CHECK(status == LXP_OK);
    LXP_CASE_CHECK(receipt->result_code == LXP_OK);
    LXP_CASE_CHECK(receipt->program_outcome.present);
    LXP_CASE_CHECK(receipt->program_outcome.terminal_kind ==
        LXP_PROGRAM_TERMINAL_SUCCESS);
    LXP_CASE_CHECK(!lxp_ct_is_zero(
        receipt->program_outcome.occupancy_transfer_root, 32U));
    for (size_t i = 0U; i < receipt->effects.count; ++i) {
        const lxp_effect *effect = &receipt->effects.effects[i];
        if (!effect->monetary || effect->kind != LXP_EFFECT_TRANSFER) continue;
        ++monetary_transfers;
        LXP_CASE_CHECK(memcmp(effect->transfer_set_root,
            receipt->program_outcome.occupancy_transfer_root, 32U) == 0);
    }
    LXP_CASE_CHECK(monetary_transfers == 1U);
    LXP_CASE_CHECK(receipt->program_outcome.fee_units.hi == 0U &&
        receipt->program_outcome.fee_units.lo > 0U);
    LXP_CASE_CHECK(memcmp(receipt->transfer_set_root,
        receipt->program_outcome.transfer_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(receipt->transfer_set_root,
        receipt->program_outcome.occupancy_transfer_root, 32U) == 0);
    free(manifest);
    free(receipt);
    while (f->kernel.blob_count != 0U)
        free(f->kernel.blobs[--f->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(arena_bytes);
    free(f);
    return 0;
}

static int sandbox_receipt_refuses_foreign_monetary_transfer(void)
{
    lxp_module_ctx *ctx = calloc(1U, sizeof(*ctx));
    lxp_receipt *receipt = calloc(1U, sizeof(*receipt));
    lxp_effect_buffer *effects = calloc(1U, sizeof(*effects));
    lxp_effect effect = {0};

    LXP_CASE_CHECK(ctx != NULL && receipt != NULL && effects != NULL);
    LXP_CASE_CHECK(lxp_effect_buffer_init(effects) == LXP_OK);
    ctx->effects = effects;
    ctx->module_id = LXP_MODULE_PROGRAMS;
    ctx->program_outcome.present = true;
    ctx->program_outcome.occupancy_transfer_root[0] = 1U;
    ctx->program_outcome.transfer_root[0] = 2U;
    effect.module_id = LXP_MODULE_PROGRAMS;
    effect.kind = LXP_EFFECT_TRANSFER;
    effect.monetary = true;
    effect.transfer_set_root[0] = 3U;
    LXP_CASE_CHECK(lxp_effect_buffer_add(effects, &effect) == LXP_OK);
    LXP_CASE_CHECK(receipt_bind_ledger_projection(receipt, ctx) ==
        LXP_FATAL_INVARIANT);
    memcpy(effects->effects[0].transfer_set_root,
        ctx->program_outcome.transfer_root, 32U);
    LXP_CASE_CHECK(receipt_bind_ledger_projection(receipt, ctx) ==
        LXP_FATAL_INVARIANT);
    memcpy(effects->effects[0].transfer_set_root,
        ctx->program_outcome.occupancy_transfer_root, 32U);
    LXP_CASE_CHECK(receipt_bind_ledger_projection(receipt, ctx) == LXP_OK);
    LXP_CASE_CHECK(memcmp(receipt->transfer_set_root,
        ctx->program_outcome.occupancy_transfer_root, 32U) == 0);
    ctx->program_outcome.present = false;
    LXP_CASE_CHECK(receipt_bind_ledger_projection(receipt, ctx) ==
        LXP_FATAL_INVARIANT);
    ctx->program_outcome.present = true;
    ctx->module_id = LXP_MODULE_ASSET;
    LXP_CASE_CHECK(receipt_bind_ledger_projection(receipt, ctx) ==
        LXP_FATAL_INVARIANT);
    free(effects);
    free(receipt);
    free(ctx);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"sandbox_execute_receipt_binds_escrow_charge",
         sandbox_execute_receipt_binds_escrow_charge},
        {"sandbox_receipt_refuses_foreign_monetary_transfer",
         sandbox_receipt_refuses_foreign_monetary_transfer}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
