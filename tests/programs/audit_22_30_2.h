/* Regression paths for the Programs findings in audit 22.30.2. */
#include "../../src/modules/programs/storage.h"
#include "../../src/modules/programs/sandbox.h"

#define AUDIT_CHECK(condition) do { if (!(condition)) { \
    (void)fprintf(stderr, "%s:%d: %s\n", __func__, __LINE__, #condition); \
    return 1; } } while (0)

#define AUDIT_RESULT(expression, expected) do { \
    lxp_result audit_status = (expression); \
    if (audit_status != (expected)) { \
        (void)fprintf(stderr, "%s:%d: %s returned %d, expected %d\n", \
            __func__, __LINE__, #expression, audit_status, (expected)); \
        return 1; \
    } \
} while (0)

typedef struct audit_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lxp_arena arena;
    uint8_t arena_bytes[4U * LXP_MAX_ACTIVITY_BYTES];
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
} audit_fixture;

static const uint8_t audit_owner_seed[32] = {0x51U};
static const uint8_t audit_governor_seed[32] = {0x52U};
static const uint64_t audit_parameters = 1U;

static void audit_u64(uint8_t *out, uint64_t value)
{
    for (size_t i = 0U; i < 8U; ++i)
        out[i] = (uint8_t)(value >> (56U - 8U * i));
}

static int audit_begin(audit_fixture *f, uint64_t batch)
{
    AUDIT_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    AUDIT_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_PROGRAMS,
        10U, f->kernel.epoch, f->state.next_sequence, UINT64_MAX,
        &f->arena, true) == LXP_OK);
    f->ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->ctx.batch_number = batch;
    f->ctx.activity_id[0] = 0xa1U;
    audit_u64(f->ctx.activity_id + 24U, batch);
    AUDIT_CHECK(lxp_effect_buffer_init(&f->effects) == LXP_OK);
    AUDIT_CHECK(lxp_module_ctx_bind_effects(&f->ctx, &f->effects) == LXP_OK);
    return 0;
}

static int audit_register(audit_fixture *f)
{
    lxp_genesis_module_plan plan;
    AUDIT_CHECK(lxp_genesis_module_plan_resolve(&migration_genesis, &plan) == LXP_OK);
    AUDIT_CHECK(lxp_genesis_module_plan_register(&plan, &f->kernel) == LXP_OK);
    return 0;
}

static int audit_init(audit_fixture *f)
{
    lxp_byte_span preimage;
    lx_account_registry *accounts = calloc(1U, sizeof(*accounts));
    AUDIT_CHECK(accounts != NULL && lx_account_registry_init(accounts) == LXP_OK);
    AUDIT_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
        sizeof(f->arena_bytes)) == LXP_OK);
    AUDIT_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    AUDIT_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
        &audit_parameters, 0U) == LXP_OK);
    AUDIT_CHECK(project_metering_genesis_with(&f->kernel, audit_owner_seed, 1) == 0);
    /* Build the authoritative governance configuration through genesis. */
    migration_genesis.parameters[migration_genesis.parameter_count].module_id = LXP_MODULE_GOVERNANCE;
    (void)memcpy(migration_genesis.parameters[migration_genesis.parameter_count].key,
        "handover-authority", 18U);
    AUDIT_CHECK(public_key_for(audit_governor_seed,
        migration_genesis.parameters[migration_genesis.parameter_count].value) == 0);
    ++migration_genesis.parameter_count;
    for (size_t i = 1U; i < migration_genesis.parameter_count; ++i) {
        lxp_genesis_parameter value = migration_genesis.parameters[i];
        size_t position = i;
        while (position != 0U) {
            const lxp_genesis_parameter *prior = &migration_genesis.parameters[position - 1U];
            if (prior->module_id < value.module_id ||
                (prior->module_id == value.module_id && memcmp(prior->key, value.key, 32U) < 0)) break;
            migration_genesis.parameters[position] = *prior;
            --position;
        }
        migration_genesis.parameters[position] = value;
    }
    AUDIT_CHECK(lxp_genesis_state_root(&migration_genesis, &f->arena,
        migration_genesis.genesis_state_root) == LXP_OK);
    AUDIT_CHECK(lxp_genesis_receipt_state_root(migration_genesis.network_id,
        migration_genesis.genesis_state_root,
        migration_genesis.genesis_receipt_state_root) == LXP_OK);
    AUDIT_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    AUDIT_CHECK(lxp_genesis_encode(&migration_genesis, false, &f->arena,
        &preimage) == LXP_OK);
    AUDIT_CHECK(sign_raw(audit_owner_seed, preimage.bytes, preimage.length,
        migration_genesis.signature) == 0);
    AUDIT_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    AUDIT_CHECK(lxp_genesis_verify_signature(&migration_genesis, &f->arena) == LXP_OK);
    AUDIT_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    AUDIT_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    AUDIT_CHECK(lxp_state_store_bind_accounts(&f->state, accounts) == LXP_OK);
    AUDIT_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
        &audit_parameters, 0U) == LXP_OK);
    AUDIT_CHECK(audit_register(f) == 0);
    AUDIT_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    AUDIT_CHECK(lxp_genesis_materialize(&migration_genesis, &f->arena,
        &f->kernel) == LXP_OK);
    AUDIT_CHECK(migration_runtime(&f->kernel) == 0);
    return audit_begin(f, 1U);
}

/* Close all live kernel and account storage before restoring the snapshot. */
static int audit_reopen(audit_fixture *f)
{
    uint8_t *storage = malloc(ADMISSION_BUFFER_BYTES);
    lxp_arena arena;
    lxp_byte_span snapshot;
    lxp_snapshot_manifest_record manifest;
    uint8_t root[32];
    uint64_t sequence = f->state.next_sequence - 1U;
    AUDIT_CHECK(storage != NULL);
    AUDIT_CHECK(lxp_arena_init(&arena, storage, ADMISSION_BUFFER_BYTES) == LXP_OK);
    AUDIT_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
    (void)memcpy(f->kernel.current_state_root, root, 32U);
    AUDIT_CHECK(lxp_snapshot_write(&f->kernel, sequence, &arena, &snapshot) == LXP_OK);
    AUDIT_CHECK(lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length,
        sequence, root, root, &manifest) == LXP_OK);
    migration_destroy(&f->kernel, &f->state);
    (void)memset(&f->kernel, 0, sizeof(f->kernel));
    (void)memset(&f->journal, 0, sizeof(f->journal));
    AUDIT_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    AUDIT_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
        &audit_parameters, 0U) == LXP_OK);
    AUDIT_CHECK(audit_register(f) == 0);
    AUDIT_CHECK(migration_runtime(&f->kernel) == 0);
    AUDIT_CHECK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
        &manifest, &f->kernel) == LXP_OK);
    AUDIT_CHECK(lxp_snapshot_verify_root(&f->kernel, &manifest) == LXP_OK);
    free(storage);
    return audit_begin(f, 1U);
}

static size_t audit_migration_guest(uint8_t *out, bool upgraded)
{
    static const uint8_t header[] = {0,97,115,109,1,0,0,0};
    static const uint8_t types[] = {
        3,0x60,2,0x7f,0x7f,1,0x7f,0x60,1,0x7f,1,0x7f,0x60,0,0};
    static const uint8_t functions[] = {3,0,1,2};
    static const uint8_t memory[] = {1,1,1,1};
    uint8_t section_bytes[256];
    admission_bytes module = {out, sizeof(header)};
    admission_bytes section = {section_bytes, 0U};
    (void)memcpy(out, header, sizeof(header));
    (void)memcpy(section.bytes, types, sizeof(types)); section.length = sizeof(types);
    admission_section(&module, 1U, &section);
    (void)memcpy(section.bytes, functions, sizeof(functions)); section.length = sizeof(functions);
    admission_section(&module, 3U, &section);
    (void)memcpy(section.bytes, memory, sizeof(memory)); section.length = sizeof(memory);
    admission_section(&module, 5U, &section);
    section.length = 0U; admission_byte(&section, 4U);
    migration_name(&section, "layerx_call"); admission_byte(&section, 0U); admission_byte(&section, 0U);
    migration_name(&section, "layerx_reserve"); admission_byte(&section, 0U); admission_byte(&section, 1U);
    migration_name(&section, "migrate"); admission_byte(&section, 0U); admission_byte(&section, 2U);
    migration_name(&section, "memory"); admission_byte(&section, 2U); admission_byte(&section, 0U);
    admission_section(&module, 7U, &section);
    section.length = 0U; admission_byte(&section, 3U);
    for (size_t i = 0U; i < 2U; ++i) {
        admission_byte(&section, 4U); admission_byte(&section, 0U);
        admission_byte(&section, 0x41U); admission_byte(&section, 0U); admission_byte(&section, 0x0bU);
    }
    admission_byte(&section, upgraded ? 3U : 2U); admission_byte(&section, 0U);
    if (upgraded) admission_byte(&section, 0x01U);
    admission_byte(&section, 0x0bU);
    admission_section(&module, 10U, &section);
    return module.length;
}

static int audit_activate_migration(audit_fixture *f, lxp_identity *governor)
{
    const uint64_t limits[7] = {
        1000000U,16777216U,1048576U,1048576U,64U,1048576U,4096U};
    lxp_migration_profile profile = {0};
    lxp_activity activity;
    lxp_authority_grant grant;
    lxp_authority_resolved authority;
    lxp_kernel_execution execution;
    lxp_fee_params fees = {0};
    lxp_receipt receipt;
    lxp_verified_receipt_index index;
    uint8_t proposal[81], signature[64], digest[32], sequencer_key[32];
    profile.version = 1U;
    profile.module_version = LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION;
    profile.parameter_version = 1U; profile.activation_epoch = 1U;
    (void)memcpy(profile.limits, limits, sizeof(limits));
    fees.version = 1U; fees.multiplier_basis_points = 10000U;
    AUDIT_CHECK(lxp_programs_migration_proposal_encode(&profile, proposal) == LXP_OK);
    AUDIT_CHECK(migration_actor(&f->kernel, &activity, governor, audit_governor_seed,
        signature, LXP_GOVERNANCE_MIGRATION_BUDGET, proposal, sizeof(proposal),
        100000000U, 0xa0U, &grant, &authority) == 0);
    /* The profile staging API consumes its historical protocol-three receipt. */
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    AUDIT_CHECK(lxp_activity_signing_preimage(&activity, digest) == LXP_OK);
    AUDIT_CHECK(sign_raw(audit_governor_seed, digest, sizeof(digest), signature) == 0);
    migration_execution(&f->kernel, &execution, &f->identities, &authority,
        &fees, &f->arena, audit_owner_seed, 1U);
    AUDIT_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity,
        &execution, &receipt) == LXP_OK);
    AUDIT_CHECK(receipt.result_code == LXP_OK && receipt.effects.count == 1U);
    AUDIT_CHECK(public_key_for(audit_owner_seed, sequencer_key) == 0);
    AUDIT_CHECK(lxp_verified_receipt_index_init(&index) == LXP_OK);
    AUDIT_CHECK(lxp_verified_receipt_index_add(&index, &receipt,
        sequencer_key, &f->arena) == LXP_OK);
    AUDIT_CHECK(audit_begin(f, 1U) == 0);
    f->ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->ctx.verified_receipts = &index;
    AUDIT_CHECK(lxp_programs_migration_profile_stage(&f->ctx, &profile, &receipt) == LXP_OK);
    AUDIT_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    AUDIT_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    AUDIT_CHECK(lxp_kernel_epoch_transition(&f->kernel, 1U, 10U, &f->arena) == LXP_OK);
    return 0;
}

static int audit_migration_abi5_bound_authority_recovery(void)
{
    audit_fixture *f = calloc(1U, sizeof(*f));
    uint8_t public_key[32], signature[64], old_wasm[512], wasm[512];
    uint8_t deploy[1024] = {0xa5U}, upgrade[1024] = {0};
    uint8_t before[32], after[32], record[613], recovered[613], activity_id[32];
    lxp_identity *owner, *governor;
    lxp_authority_grant grant;
    lxp_authority_resolved authority, forged;
    lxp_activity activity;
    lxp_kernel_execution execution;
    lxp_receipt receipt;
    lxp_fee_params fees = {0};
    size_t old_length = audit_migration_guest(old_wasm, false);
    size_t wasm_length = audit_migration_guest(wasm, true);
    AUDIT_CHECK(f != NULL && audit_init(f) == 0);
    AUDIT_CHECK(public_key_for(audit_owner_seed, public_key) == 0);
    AUDIT_CHECK(lxp_identity_register(&f->identities,
        (const uint8_t *)"did:lxp:native-migration-owner", 30U, public_key, &owner) == LXP_OK);
    AUDIT_CHECK(public_key_for(audit_governor_seed, public_key) == 0);
    AUDIT_CHECK(lxp_identity_register(&f->identities,
        (const uint8_t *)"did:lxp:native-migration-governor", 33U, public_key, &governor) == LXP_OK);
    AUDIT_CHECK(audit_activate_migration(f, governor) == 0);
    write_u16(deploy + 32U, 5U); deploy[34U] = 1U;
    (void)memcpy(deploy + 36U, owner->did_id, 32U);
    AUDIT_CHECK(lxp_hash_sha256(old_wasm, old_length, deploy + 68U) == LXP_OK);
    write_u32(deploy + 100U, (uint32_t)old_length);
    (void)memcpy(deploy + 104U, old_wasm, old_length);
    fees.version = 1U; fees.multiplier_basis_points = 10000U;
    AUDIT_CHECK(migration_actor(&f->kernel, &activity, owner, audit_owner_seed,
        signature, LX_PROGRAMS_DEPLOY, deploy, 104U + old_length,
        100000000U, 0xa1U, &grant, &authority) == 0);
    migration_execution(&f->kernel, &execution, &f->identities, &authority,
        &fees, &f->arena, audit_owner_seed, LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION);
    AUDIT_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity, &execution, &receipt) == LXP_OK);
    AUDIT_CHECK(receipt.result_code == LXP_OK);
    AUDIT_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    (void)memcpy(upgrade, deploy, 32U); write_u16(upgrade + 32U, 5U); upgrade[34U] = 1U;
    (void)memcpy(upgrade + 36U, deploy + 68U, 32U);
    AUDIT_CHECK(lxp_hash_sha256(wasm, wasm_length, upgrade + 68U) == LXP_OK);
    write_u16(upgrade + 100U, 7U); write_u32(upgrade + 102U, (uint32_t)wasm_length);
    (void)memcpy(upgrade + 106U, "migrate", 7U);
    (void)memcpy(upgrade + 113U, wasm, wasm_length);
    AUDIT_CHECK(migration_actor(&f->kernel, &activity, owner, audit_owner_seed,
        signature, LX_PROGRAMS_UPGRADE, upgrade, 113U + wasm_length,
        100000000U, 0xa2U, &grant, &authority) == 0);
    migration_execution(&f->kernel, &execution, &f->identities, &authority,
        &fees, &f->arena, audit_owner_seed, LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION);
    execution.binding_authority = true;
    AUDIT_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    signature[0] ^= 1U;
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity, &execution, &receipt) == LXP_ERR_BAD_SIGNATURE);
    signature[0] ^= 1U;
    activity.signature = (lxp_byte_span){NULL, 0U};
    execution.binding_authority = false;
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity, &execution, &receipt) == LXP_ERR_BAD_SIGNATURE);
    execution.binding_authority = true;
    forged = authority; forged.principal[0] ^= 1U; execution.authority = &forged;
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity, &execution, &receipt) == LXP_ERR_AUTH_SCOPE);
    execution.authority = &authority;
    upgrade[68U] ^= 1U;
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity, &execution, &receipt) == LXP_ERR_PAYLOAD_HASH_MISMATCH);
    upgrade[68U] ^= 1U;
    AUDIT_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK && memcmp(before, after, 32U) == 0);
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity, &execution, &receipt) == LXP_OK);
    AUDIT_CHECK(receipt.result_code == LXP_OK);
    (void)memcpy(activity_id, receipt.activity_id, 32U);
    AUDIT_CHECK(migration_accounting_record(&f->kernel, activity_id, record) == LXP_OK);
    AUDIT_CHECK(record[11U] == 0U && record[12U] == 5U);
    AUDIT_CHECK(memcmp(record + 93U, deploy, 32U) == 0);
    AUDIT_CHECK(memcmp(record + 157U, upgrade + 68U, 32U) == 0);
    AUDIT_CHECK(audit_reopen(f) == 0);
    AUDIT_CHECK(migration_accounting_record(&f->kernel, activity_id, recovered) == LXP_OK);
    AUDIT_CHECK(memcmp(record, recovered, sizeof(record)) == 0);
    AUDIT_CHECK(lxp_programs_abi_transition_validate(1U, 6U) == LXP_ERR_VERSION_UNSUPPORTED);
    migration_destroy(&f->kernel, &f->state); free(f);
    (void)puts("audit_migration_abi5_bound_authority_recovery: passed");
    return 0;
}

typedef struct audit_lease_request {
    uint8_t lease_id[32], host[32], asset_id[32];
    lx_account *tenant, *escrow;
    uint8_t requested[1024];
    size_t requested_length, state_offset;
} audit_lease_request;

static int audit_requested_lease(audit_lease_request *f, uint64_t funding,
                            const uint64_t *limits, uint64_t expiry)
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
    AUDIT_CHECK(lxp_hash_sha256(namespace_preimage, sizeof(namespace_preimage),
                                   out + offset) == LXP_OK);
    offset += 32U;
    (void)memcpy(out + offset, f->asset_id, 32U); offset += 32U;
    (void)memcpy(out + offset, f->escrow->id, 32U); offset += 32U;
    AUDIT_CHECK(lx_account_id_from_string((const uint8_t *)"system:fees",
                                             11U, fees) == LXP_OK);
    (void)memcpy(out + offset, fees, 32U); offset += 32U;
    audit_u64(out + offset + 8U, funding); offset += 16U;
    for (size_t i = 0U; i < 8U; ++i) {
        audit_u64(out + offset, limits[i]); offset += 8U;
    }
    audit_u64(out + offset, 1U); offset += 8U;
    audit_u64(out + offset, expiry); offset += 8U;
    request_length = offset - fields;
    offset += 64U;
    schedule = offset;
    write_u32(out + offset, 1U); offset += 4U;
    for (size_t i = 0U; i < 7U; ++i) {
        audit_u64(out + offset, 1U); offset += 8U;
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
    AUDIT_CHECK(lxp_hash_sha256(request_preimage,
                    sizeof(request_domain) + request_length + 60U,
                    out + offset) == LXP_OK);
    offset += 32U;
    AUDIT_CHECK(lxp_hash_sha256(out, offset, out + offset) == LXP_OK);
    offset += 32U;
    audit_u64(out + offset, 1U); offset += 8U;
    offset += 3U;
    f->requested_length = offset;
    return 0;
}

static int audit_dispatch_commit(audit_fixture *f, lxp_activity *activity,
    const lxp_authority_resolved *authority)
{
    const lxp_module_registration *registration;
    lxp_result result = LXP_FATAL_INVARIANT;
    AUDIT_CHECK(lxp_kernel_module_for_activity(&f->kernel, activity->activity_type,
        f->kernel.epoch, &registration) == LXP_OK);
    AUDIT_CHECK(lxp_state_journal_open(&f->state, f->ctx.global_sequence,
        &f->journal) == LXP_OK);
    AUDIT_RESULT(lxp_kernel_dispatch(registration, &f->ctx, activity, authority,
        &f->effects, &result), LXP_OK);
    if (result != LXP_OK) (void)fprintf(stderr, "audit dispatch result=%d type=%u\n",
        result, activity->activity_type);
    if (result != LXP_OK) (void)fprintf(stderr, "audit batch=%llu sequence=%llu blobs=%zu\n",
        (unsigned long long)f->ctx.batch_number,
        (unsigned long long)f->ctx.global_sequence, f->ctx.staged_blob_count);
    AUDIT_CHECK(result == LXP_OK);
    AUDIT_CHECK(f->ctx.staged_blob_count <= LXP_KERNEL_MAX_STAGED_BLOBS);
    AUDIT_CHECK(lxp_module_ctx_prepare_commit(&f->ctx) == LXP_OK);
    AUDIT_CHECK(lxp_state_journal_commit(&f->journal) == LXP_OK);
    AUDIT_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    return 0;
}

static int audit_fund_lease(audit_fixture *f, audit_lease_request *r, uint64_t expiry)
{
    static const uint64_t limits[8] = {1U, 1U, 1U, 1U, 1U, 1U, 1U, 1024U};
    static const uint8_t domain[] = "sandbox-lease-escrow/v1";
    uint8_t seed[sizeof(domain) + 32U], id[32], fund[2048] = {1U, 2U};
    lx_account_registration registration;
    bool created;
    lxp_authority_resolved authority = {0};
    lxp_activity activity = {0};
    size_t offset = 4U;
    r->lease_id[0] = 0x61U; r->host[0] = 0x62U; r->asset_id[0] = 1U;
    r->tenant = migration_account(&f->kernel, "agent:did:lxp:native-migration-owner:main");
    AUDIT_CHECK(r->tenant != NULL);
    (void)memcpy(seed, domain, sizeof(domain));
    (void)memcpy(seed + sizeof(domain), r->lease_id, 32U);
    AUDIT_CHECK(lxp_programs_account_derive(r->host, seed, sizeof(seed), id) == LXP_OK);
    AUDIT_CHECK(lx_account_module_value_prepare(f->state.accounts,
        (const uint8_t *)"programs", 8U, id, r->asset_id, 1U,
        &registration, &r->escrow, &created) == LXP_OK && created);
    AUDIT_CHECK(lx_account_registration_commit(f->state.accounts, &registration,
        &r->escrow) == LXP_OK);
    AUDIT_CHECK(audit_begin(f, 1U) == 0);
    AUDIT_CHECK(audit_requested_lease(r, 100U, limits, expiry) == 0);
    (void)memcpy(fund + offset, r->lease_id, 32U); offset += 32U;
    (void)memcpy(fund + offset, r->tenant->id, 32U); offset += 32U;
    (void)memcpy(fund + offset, r->host, 32U); offset += 32U;
    (void)memcpy(fund + offset, r->escrow->id, 32U); offset += 32U;
    (void)memcpy(fund + offset, r->asset_id, 32U); offset += 32U;
    audit_u64(fund + offset + 8U, 100U); offset += 16U;
    audit_u64(fund + offset, expiry); offset += 8U;
    write_u32(fund + offset, 1U); offset += 4U;
    for (size_t i = 0U; i < 7U; ++i) { audit_u64(fund + offset, 1U); offset += 8U; }
    write_u32(fund + offset, (uint32_t)r->requested_length); offset += 4U;
    (void)memcpy(fund + offset, r->requested, r->requested_length); offset += r->requested_length;
    write_u32(fund + offset, 146U); offset += 4U;
    (void)memcpy(fund + offset, r->host, 32U); offset += 32U;
    write_u16(fund + offset, 1U); offset += 2U;
    (void)memcpy(fund + offset, r->tenant->id, 32U); offset += 32U;
    (void)memcpy(fund + offset, r->asset_id, 32U); offset += 32U;
    (void)memcpy(fund + offset, r->escrow->id, 32U); offset += 32U;
    audit_u64(fund + offset + 8U, 100U); offset += 16U;
    authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(authority.principal, r->tenant->id, 32U);
    authority.authority_hash[0] = 1U;
    activity.activity_type = LX_PROGRAMS_SANDBOX;
    activity.payload = (lxp_byte_span){fund, offset};
    activity.account_sequence = r->tenant->next_sequence;
    AUDIT_CHECK(audit_dispatch_commit(f, &activity, &authority) == 0);
    AUDIT_CHECK(r->tenant->balance.lo == UINT64_C(1000000000) - 100U);
    AUDIT_CHECK(r->escrow->balance.lo == 100U);
    return 0;
}

static int audit_storage_write(audit_fixture *f, const uint8_t *ns,
    uint16_t ns_length, const lxp_programs_storage_cell *cells, uint32_t count)
{
    AUDIT_CHECK(audit_begin(f, 4U) == 0);
    AUDIT_CHECK(lxp_programs_storage_stage_final(&f->ctx, ns, ns_length,
        cells, count) == LXP_OK);
    AUDIT_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    return 0;
}

static lxp_result audit_check_cell(void *user, const uint8_t *key,
    uint16_t key_length, const uint8_t *value, uint32_t value_length)
{
    uint32_t *count = user;
    const uint8_t expected_key = (uint8_t)('a' + *count);
    const uint8_t expected_value = (uint8_t)('A' + *count);
    if (key_length != 1U || key[0] != expected_key ||
        value_length != 1U || value[0] != expected_value)
        return LXP_FATAL_REPLAY_DIVERGENCE;
    ++*count;
    return LXP_OK;
}

static int audit_destroy_archives(audit_fixture *f, const uint8_t lease_id[32],
    const uint8_t lease_root[32], const uint8_t ledger_root[32])
{
    uint8_t key[34] = {'t','s'}, digest[32];
    const uint8_t *terminal, *descriptor, *blob;
    size_t terminal_length, descriptor_length, blob_length;
    (void)memcpy(key + 2U, lease_id, 32U);
    AUDIT_RESULT(lxp_ctx_kv_get(&f->ctx, key, sizeof(key), &terminal,
        &terminal_length), LXP_OK);
    AUDIT_CHECK(terminal_length == 352U && memcmp(terminal, "LXSD1", 5U) == 0);
    for (uint8_t kind = 0U; kind < 4U; ++kind) {
        const uint8_t *expected = kind == 0U ? terminal + 197U :
            kind == 1U ? terminal + 229U : kind == 2U ? ledger_root : lease_root;
        key[0] = (uint8_t)'d'; key[1] = kind;
        AUDIT_RESULT(lxp_ctx_kv_get(&f->ctx, key, sizeof(key), &descriptor,
            &descriptor_length), LXP_OK);
        AUDIT_CHECK(descriptor_length == 41U &&
            memcmp(descriptor, "LXDA1", 5U) == 0);
        AUDIT_CHECK(memcmp(descriptor + 9U, expected, 32U) == 0);
        AUDIT_RESULT(lxp_ctx_blob_get(&f->ctx, descriptor + 9U, &blob,
            &blob_length), LXP_OK);
        AUDIT_CHECK(blob_length == migration_u32(descriptor + 5U));
        AUDIT_RESULT(lxp_hash_sha256(blob, blob_length, digest), LXP_OK);
        AUDIT_CHECK(memcmp(digest, expected, sizeof(digest)) == 0);
    }
    return 0;
}

static int audit_storage_environment_recovery(unsigned int root_mode)
{
    audit_fixture *f = calloc(1U, sizeof(*f));
    audit_lease_request request = {0};
    const bool shared = root_mode != 0U;
    uint8_t ns[65], shared_ns[33] = {0x71U}, key[73], snapshot_key[73];
    uint8_t cleanup_key[34] = {'x','c'}, destroy[84] = {1U,1U};
    uint8_t escrow_id[32], expected_manifest[38], lease_root[32], ledger_root[32];
    const uint8_t keys[3] = {'a','b','c'}, values[3] = {'A','B','C'};
    const uint8_t private_key = 'z', private_value = 'Z';
    const uint8_t *lease, *head, *cleanup;
    size_t lease_length, head_length, cleanup_length;
    lxp_programs_storage_cell cells[3];
    lxp_programs_storage_cell private_cell = {&private_key,1U,&private_value,1U};
    lxp_activity activity = {0};
    lxp_authority_resolved authority = {0};
    uint64_t tenant_before;
    AUDIT_CHECK(f != NULL && audit_init(f) == 0);
    AUDIT_CHECK(audit_fund_lease(f, &request, 100U) == 0);
    (void)memcpy(escrow_id, request.escrow->id, 32U);
    tenant_before = request.tenant->balance.lo;
    AUDIT_CHECK(audit_begin(f, 4U) == 0);
    AUDIT_CHECK(lxp_programs_sandbox_read(&f->ctx, request.lease_id,
        0U, &lease, &lease_length) == LXP_OK);
    AUDIT_RESULT(lxp_hash_sha256(lease, lease_length, lease_root), LXP_OK);
    (void)memcpy(ns, request.host, 32U); ns[32U] = 0U;
    (void)memcpy(ns + 33U, lease + 167U, 32U);
    AUDIT_RESULT(lxp_programs_sandbox_read(&f->ctx, request.lease_id,
        2U, &lease, &lease_length), LXP_OK);
    AUDIT_RESULT(lxp_hash_sha256(lease, lease_length, ledger_root), LXP_OK);
    (void)memcpy(key, "progstor", 8U); (void)memcpy(key + 8U, ns, 65U);
    (void)memcpy(snapshot_key, key, sizeof(key)); snapshot_key[40U] = 2U;
    for (size_t i = 0U; i < 3U; ++i)
        cells[i] = (lxp_programs_storage_cell){keys+i,1U,values+i,1U};
    /* Transport the storage producer's canonical head into the snapshot root. */
    AUDIT_CHECK(audit_storage_write(f, ns, 65U, cells, 3U) == 0);
    AUDIT_CHECK(audit_begin(f, 4U) == 0);
    AUDIT_CHECK(lxp_ctx_kv_get(&f->ctx, key, sizeof(key), &head, &head_length) == LXP_OK);
    AUDIT_CHECK(head_length == sizeof(expected_manifest));
    (void)memcpy(expected_manifest, head, sizeof(expected_manifest));
    AUDIT_CHECK(lxp_ctx_kv_put(&f->ctx, snapshot_key, sizeof(snapshot_key),
        head, head_length) == LXP_OK);
    AUDIT_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    AUDIT_CHECK(audit_storage_write(f, ns, 65U, &private_cell, 1U) == 0);
    shared_ns[32U] = 1U;
    if (root_mode == 1U) {
        AUDIT_CHECK(audit_storage_write(f, shared_ns, 33U, cells, 3U) == 0);
        AUDIT_CHECK(audit_begin(f, 4U) == 0);
        uint8_t shared_key[41];
        (void)memcpy(shared_key, "progstor", 8U); (void)memcpy(shared_key + 8U, shared_ns, 33U);
        AUDIT_CHECK(lxp_ctx_kv_get(&f->ctx, shared_key, sizeof(shared_key),
            &head, &head_length) == LXP_OK);
        AUDIT_CHECK(head_length == sizeof(expected_manifest) &&
            memcmp(head, expected_manifest, sizeof(expected_manifest)) == 0);
    }
    (void)memcpy(cleanup_key + 2U, request.lease_id, 32U);
    (void)memcpy(destroy + 4U, request.lease_id, 32U);
    activity.activity_type = LX_PROGRAMS_SANDBOX_DESTROY;
    activity.payload = (lxp_byte_span){destroy, sizeof(destroy)};
    authority.kind = LXP_AUTHORITY_PROTOCOL_MODULE;
    for (uint64_t step = 0U; step < 4U; ++step) {
        AUDIT_CHECK(audit_begin(f, 100U + step) == 0);
        AUDIT_CHECK(lxp_programs_sandbox_read(&f->ctx, request.lease_id,
            0U, &lease, &lease_length) == LXP_OK);
        AUDIT_CHECK(lxp_hash_sha256(lease, lease_length, destroy + 36U) == LXP_OK);
        audit_u64(destroy + 68U, f->ctx.global_sequence);
        audit_u64(destroy + 76U, f->ctx.batch_number);
        if (root_mode == 2U && step == 1U) {
            /* Maintenance consumes the same journal as earlier block work;
             * activity dispatch starts with an unstaged validation context. */
            AUDIT_RESULT(lxp_state_journal_open(&f->state, f->ctx.global_sequence,
                &f->journal), LXP_OK);
            AUDIT_CHECK(lxp_programs_storage_stage_final(&f->ctx, shared_ns,
                33U, cells, 3U) == LXP_OK);
            AUDIT_RESULT(lxp_programs_sandbox_finalize_expiry_batch(&f->ctx,
                f->ctx.batch_number), LXP_OK);
            AUDIT_CHECK(f->ctx.staged_blob_count <= LXP_KERNEL_MAX_STAGED_BLOBS);
            AUDIT_RESULT(lxp_module_ctx_prepare_commit(&f->ctx), LXP_OK);
            AUDIT_RESULT(lxp_state_journal_commit(&f->journal), LXP_OK);
            AUDIT_RESULT(lxp_module_ctx_commit(&f->ctx), LXP_OK);
        } else AUDIT_CHECK(audit_dispatch_commit(f, &activity, &authority) == 0);
        if (step < 3U) {
            AUDIT_CHECK(lxp_ctx_kv_get(&f->ctx, cleanup_key, sizeof(cleanup_key),
                &cleanup, &cleanup_length) == LXP_OK);
            AUDIT_CHECK(cleanup_length == 96U && cleanup[5U] == step + 1U);
            AUDIT_CHECK(migration_account(&f->kernel,
                "agent:did:lxp:native-migration-owner:main")->balance.lo == tenant_before);
            if (!shared && step == 1U)
                AUDIT_CHECK(f->kernel.blob_count == 2U);
            if (step == 2U) {
                AUDIT_RESULT(lxp_ctx_kv_get(&f->ctx, key, sizeof(key),
                    &head, &head_length), LXP_ERR_UNKNOWN_FIELD);
                AUDIT_RESULT(lxp_ctx_kv_get(&f->ctx, snapshot_key,
                    sizeof(snapshot_key), &head, &head_length), LXP_ERR_UNKNOWN_FIELD);
                AUDIT_CHECK(f->kernel.blob_count == (shared ? 4U : 0U));
            }
        } else {
            AUDIT_CHECK(lxp_programs_sandbox_lease_terminal(&f->ctx,
                request.lease_id) == LXP_ERR_IDEMPOTENT_REPLAY);
            AUDIT_CHECK(lxp_programs_sandbox_read(&f->ctx, request.lease_id,
                0U, &lease, &lease_length) == LXP_ERR_UNKNOWN_FIELD);
            AUDIT_CHECK(lxp_ctx_kv_get(&f->ctx, cleanup_key, sizeof(cleanup_key),
                &cleanup, &cleanup_length) == LXP_ERR_UNKNOWN_FIELD);
            AUDIT_CHECK(migration_account(&f->kernel,
                "agent:did:lxp:native-migration-owner:main")->balance.lo == tenant_before + 100U);
        }
        AUDIT_CHECK(audit_reopen(f) == 0);
    }
    AUDIT_CHECK(audit_destroy_archives(f, request.lease_id, lease_root,
        ledger_root) == 0);
    AUDIT_CHECK(lxp_programs_sandbox_lease_terminal(&f->ctx,
        request.lease_id) == LXP_ERR_IDEMPOTENT_REPLAY);
    if (shared) {
        uint32_t count = 0U;
        AUDIT_CHECK(lxp_programs_storage_import(&f->ctx, shared_ns, 33U,
            audit_check_cell, &count) == LXP_OK && count == 3U);
        uint8_t shared_key[41];
        (void)memcpy(shared_key, "progstor", 8U); (void)memcpy(shared_key + 8U, shared_ns, 33U);
        AUDIT_CHECK(lxp_ctx_kv_get(&f->ctx, shared_key, sizeof(shared_key),
            &head, &head_length) == LXP_OK);
        AUDIT_CHECK(head_length == sizeof(expected_manifest) &&
            memcmp(head, expected_manifest, sizeof(expected_manifest)) == 0);
    }
    AUDIT_CHECK(f->kernel.blob_count == (shared ? 8U : 4U));
    lx_account *escrow = NULL;
    AUDIT_CHECK(lxp_ctx_account_find(&f->ctx, escrow_id, &escrow) == LXP_OK);
    AUDIT_CHECK(lxp_u128_is_zero(escrow->balance));
    migration_destroy(&f->kernel, &f->state); free(f);
    return 0;
}

static int audit_storage_environment_live_blob_references(void)
{
    AUDIT_CHECK(audit_storage_environment_recovery(1U) == 0);
    (void)puts("audit_storage_environment_live_blob_references: passed");
    return 0;
}

static int audit_storage_environment_cleanup_delete_budget(void)
{
    AUDIT_CHECK(audit_storage_environment_recovery(0U) == 0);
    (void)puts("audit_storage_environment_cleanup_delete_budget: passed");
    return 0;
}

static int audit_storage_environment_staged_blob_references(void)
{
    AUDIT_CHECK(audit_storage_environment_recovery(2U) == 0);
    (void)puts("audit_storage_environment_staged_blob_references: passed");
    return 0;
}

static int audit_create_occupancy(audit_fixture *f)
{
    static const uint8_t wasm[] = {
        0,97,115,109,1,0,0,0,
        1,20,3,0x60,4,0x7f,0x7f,0x7f,0x7f,1,0x7f,
        0x60,1,0x7f,1,0x7f,0x60,2,0x7f,0x7f,1,0x7f,
        2,27,1,9,'l','a','y','e','r','x','_','v','1',
        13,'s','t','o','r','a','g','e','_','w','r','i','t','e',0,0,
        3,3,2,1,2,5,4,1,1,1,1,
        7,41,3,14,'l','a','y','e','r','x','_','r','e','s','e','r','v','e',0,1,
        11,'l','a','y','e','r','x','_','c','a','l','l',0,2,
        6,'m','e','m','o','r','y',2,0,
        10,19,2,4,0,0x41,0,0x0b,
        12,0,0x41,0,0x41,1,0x41,1,0x41,1,0x10,0,0x0b,
        11,8,1,0,0x41,0,0x0b,2,'m','M'
    };
    static const uint8_t access[] = "LayerX/programs/access-declaration/v1\0";
    static const uint64_t budget[7] = {100000U,65536U,1024U,1024U,1U,1024U,0U};
    uint8_t public_key[32], signature[64], deploy[1024] = {0x81U}, call[512] = {0};
    lxp_identity *owner;
    lxp_activity activity;
    lxp_authority_grant grant;
    lxp_authority_resolved authority;
    lxp_kernel_execution execution;
    lxp_fee_params fees = {0};
    lxp_receipt receipt;
    size_t offset = 106U;
    AUDIT_CHECK(public_key_for(audit_owner_seed, public_key) == 0);
    AUDIT_CHECK(lxp_identity_register(&f->identities,
        (const uint8_t *)"did:lxp:native-migration-owner", 30U,
        public_key, &owner) == LXP_OK);
    write_u16(deploy + 32U, 1U); deploy[34U] = 1U;
    (void)memcpy(deploy + 36U, owner->did_id, 32U);
    AUDIT_CHECK(lxp_hash_sha256(wasm, sizeof(wasm), deploy + 68U) == LXP_OK);
    write_u32(deploy + 100U, (uint32_t)sizeof(wasm));
    (void)memcpy(deploy + 104U, wasm, sizeof(wasm));
    fees.version = 1U; fees.multiplier_basis_points = 10000U;
    AUDIT_CHECK(migration_actor(&f->kernel, &activity, owner, audit_owner_seed,
        signature, LX_PROGRAMS_DEPLOY, deploy, 104U + sizeof(wasm),
        100000000U, 0xb1U, &grant, &authority) == 0);
    migration_execution(&f->kernel, &execution, &f->identities, &authority,
        &fees, &f->arena, audit_owner_seed, LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION);
    AUDIT_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity,
        &execution, &receipt) == LXP_OK && receipt.result_code == LXP_OK);
    AUDIT_CHECK(audit_begin(f, 1U) == 0);
    (void)memcpy(call, deploy, 32U);
    write_u16(call + 32U, 1U); write_u16(call + 34U, 11U);
    write_u16(call + 40U, 3U); write_u32(call + 42U, (uint32_t)sizeof(access));
    write_u32(call + 46U, 16U);
    for (size_t j = 0U; j < 7U; ++j) audit_u64(call + 50U + j * 8U, budget[j]);
    (void)memcpy(call + offset, "layerx_call", 11U); offset += 11U;
    call[offset++] = 0U; call[offset++] = 1U; call[offset++] = 2U;
    (void)memcpy(call + offset, access, sizeof(access)); offset += sizeof(access);
    AUDIT_CHECK(migration_actor(&f->kernel, &activity, owner, audit_owner_seed,
        signature, LX_PROGRAMS_CALL, call, offset,
        100000000U, 0xb2U, &grant, &authority) == 0);
    migration_execution(&f->kernel, &execution, &f->identities, &authority,
        &fees, &f->arena, audit_owner_seed, LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION);
    execution.batch_number = 1U;
    AUDIT_CHECK(lxp_kernel_execute_activity(&f->kernel, &activity,
        &execution, &receipt) == LXP_OK && receipt.result_code == LXP_OK);
    AUDIT_CHECK(receipt.program_outcome.present &&
        receipt.program_outcome.storage_write_bytes == 2U);
    lxp_programs_occupancy_receipt occupancy;
    lxp_byte_span encoded;
    AUDIT_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    AUDIT_RESULT(lxp_programs_finalize_occupancy_batch_selected(&f->kernel,
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT, 1U, 1U, 10U,
        f->state.next_sequence, 1U, &f->arena, &occupancy, &encoded), LXP_OK);
    return 0;
}

static int audit_maintenance_boundary_recovery(void)
{
    audit_fixture *f = calloc(1U, sizeof(*f));
    audit_lease_request request = {0};
    uint8_t root[32], after[32], asset[32];
    lxp_programs_occupancy_receipt receipt, decoded;
    lxp_byte_span encoded;
    lx_programs_fee_schedule schedule;
    uint64_t paid = 0U, treasury_before, tenant_before;
    AUDIT_CHECK(f != NULL && audit_init(f) == 0);
    AUDIT_CHECK(audit_fund_lease(f, &request, 2U) == 0);
    AUDIT_CHECK(audit_create_occupancy(f) == 0);
    treasury_before = migration_account(&f->kernel, "system:fees")->balance.lo;
    tenant_before = migration_account(&f->kernel,
        "agent:did:lxp:native-migration-owner:main")->balance.lo;
    AUDIT_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    for (uint64_t step = 0U; step < 3U; ++step) {
        AUDIT_CHECK(audit_begin(f, 2U + step) == 0);
        AUDIT_CHECK(lxp_programs_fee_schedule_current(&f->ctx, &schedule, asset) == LXP_OK);
        AUDIT_RESULT(lxp_programs_finalize_occupancy_batch_selected(&f->kernel,
            LXP_PROTOCOL_VERSION_STATE_COMMITMENT, schedule.version,
            2U + step, 10U, f->state.next_sequence, 1U,
            &f->arena, &receipt, &encoded), LXP_OK);
        AUDIT_CHECK(lxp_programs_occupancy_receipt_decode(encoded.bytes,
            encoded.length, &decoded) == LXP_OK);
        AUDIT_CHECK(decoded.batch_number == 2U + step);
        AUDIT_CHECK(decoded.byte_batches.hi == 0U && decoded.byte_batches.lo == 2U);
        AUDIT_CHECK(receipt.paid_fee_units.hi == 0U);
        paid += receipt.paid_fee_units.lo;
        AUDIT_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
        AUDIT_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
        AUDIT_RESULT(lxp_programs_finalize_occupancy_batch_selected(&f->kernel,
            LXP_PROTOCOL_VERSION_STATE_COMMITMENT, schedule.version,
            2U + step, 10U, f->state.next_sequence, 1U,
            &f->arena, &receipt, &encoded), LXP_ERR_IDEMPOTENT_REPLAY);
        AUDIT_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
        AUDIT_CHECK(memcmp(root, after, 32U) == 0);
        AUDIT_CHECK(audit_reopen(f) == 0);
        AUDIT_CHECK(lxp_programs_fee_schedule_current(&f->ctx, &schedule, asset) == LXP_OK);
        AUDIT_CHECK(schedule.version == (uint32_t)(step + 3U));
    }
    AUDIT_CHECK(paid > 0U);
    AUDIT_CHECK(migration_account(&f->kernel, "system:fees")->balance.lo == treasury_before + paid);
    AUDIT_CHECK(migration_account(&f->kernel,
        "agent:did:lxp:native-migration-owner:main")->balance.lo == tenant_before + 100U - paid);
    AUDIT_CHECK(lxp_programs_sandbox_lease_terminal(&f->ctx,
        request.lease_id) == LXP_ERR_IDEMPOTENT_REPLAY);
    migration_destroy(&f->kernel, &f->state); free(f);
    (void)puts("audit_maintenance_boundary_recovery: passed (C boundary; native wiring required)");
    return 0;
}
