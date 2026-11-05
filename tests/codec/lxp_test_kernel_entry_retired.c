#define _POSIX_C_SOURCE 200809L
#include "lxp_kernel_case.h"
#include "layerx/programs.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include <openssl/evp.h>
#include <stdlib.h>

#ifndef LXP_TEST_LIBRARY_PATH
#error "Makefile must supply LXP_TEST_LIBRARY_PATH from $(abspath $(LIBRARY))"
#endif

static FILE *retired_archive_symbols(void)
{
    static const char prefix[] = "nm -g --defined-only -- '";
    static const char archive[] = LXP_TEST_LIBRARY_PATH;
    char command[sizeof(prefix) + 4U * sizeof(archive) + 1U];
    size_t used = sizeof(prefix) - 1U;

    if (archive[0] == '\0') {
        (void)fputs("LXP_TEST_LIBRARY_PATH must name the linked archive\n", stderr);
        return NULL;
    }
    (void)memcpy(command, prefix, used);
    for (size_t i = 0U; archive[i] != '\0'; ++i) {
        if (archive[i] == '\'') {
            (void)memcpy(command + used, "'\\''", 4U);
            used += 4U;
        } else {
            command[used++] = archive[i];
        }
    }
    command[used++] = '\'';
    command[used] = '\0';
    return popen(command, "r");
}

extern void lxp_kernel_capacity_observe(void);
extern void lxp_kernel_capacity_reserve(void);
extern void lxp_kernel_capacity_reconcile(void);
extern void lxp_kernel_capacity_cancel(void);
extern void lxp_capacity_ledger_install(void);
extern void lxp_kernel_finalize_batch_maintenance(void);
extern void lxp_kernel_idempotency_receipt_validate(void);
extern void lxp_kernel_idempotency_state_value(void);

#pragma weak lxp_kernel_capacity_observe
#pragma weak lxp_kernel_capacity_reserve
#pragma weak lxp_kernel_capacity_reconcile
#pragma weak lxp_kernel_capacity_cancel
#pragma weak lxp_capacity_ledger_install
#pragma weak lxp_kernel_finalize_batch_maintenance
#pragma weak lxp_kernel_idempotency_receipt_validate
#pragma weak lxp_kernel_idempotency_state_value

static int kernel_entry_symbols_absent(void)
{
    LXP_CASE_CHECK(lxp_kernel_capacity_observe == NULL);
    LXP_CASE_CHECK(lxp_kernel_capacity_reserve == NULL);
    LXP_CASE_CHECK(lxp_kernel_capacity_reconcile == NULL);
    LXP_CASE_CHECK(lxp_kernel_capacity_cancel == NULL);
    LXP_CASE_CHECK(lxp_capacity_ledger_install == NULL);
    LXP_CASE_CHECK(lxp_kernel_finalize_batch_maintenance == NULL);
    LXP_CASE_CHECK(lxp_kernel_idempotency_receipt_validate == NULL);
    LXP_CASE_CHECK(lxp_kernel_idempotency_state_value == NULL);
    static const char *const retired[] = {
        "lxp_kernel_capacity_observe",
        "lxp_kernel_capacity_reserve",
        "lxp_kernel_capacity_reconcile",
        "lxp_kernel_capacity_cancel",
        "lxp_capacity_ledger_install",
        "lxp_kernel_finalize_batch_maintenance",
        "lxp_kernel_idempotency_receipt_validate",
        "lxp_kernel_idempotency_state_value",
    };
    FILE *symbols = retired_archive_symbols();
    char line[4096];
    int absent = 1;
    int read_ok;
    LXP_CASE_CHECK(symbols != NULL);
    while (fgets(line, (int)sizeof(line), symbols) != NULL) {
        size_t length = strlen(line);
        char *name;
        if (length == 0U || line[length - 1U] != '\n') {
            absent = 0;
            continue;
        }
        line[length - 1U] = '\0';
        name = strrchr(line, ' ');
        if (name == NULL) continue;
        for (size_t i = 0U; i < sizeof(retired) / sizeof(retired[0]); ++i)
            if (strcmp(name + 1U, retired[i]) == 0) {
                fprintf(stderr, "archive still defines %s\n", retired[i]);
                absent = 0;
            }
    }
    read_ok = !ferror(symbols);
    LXP_CASE_CHECK(pclose(symbols) == 0);
    LXP_CASE_CHECK(read_ok && absent);
    return 0;
}

enum { ACTORS = 1, OWNER = 0, NETWORK_ID = 7 };
typedef struct node {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lx_account_registry accounts;
    lxp_transfer_asset_state asset;
    lx_programs_transfer_runtime runtime;
    lxp_fee_params fees;
    uint64_t parameters;
    uint64_t height;
    uint8_t keys[ACTORS][32];
    uint8_t principals[ACTORS][32];
    uint8_t account_ids[ACTORS][32];
    uint8_t arena_bytes[4U * LXP_MAX_ACTIVITY_BYTES];
    lxp_arena arena;
    lxp_receipt receipt;
    bool live;
} node;

static const char *const dids[ACTORS] = {"did:lxp:blob-owner"};
static const uint8_t seeds[ACTORS][32] = {{0x63U}};
static const uint8_t sequencer_seed[32] = {0x66U};
static const uint8_t asset_id[32] = {9U};
static uint8_t sequencer_public[32];

static void put_u16(uint8_t *p, uint16_t v)
{
    p[0] = (uint8_t)(v >> 8U);
    p[1] = (uint8_t)v;
}

static void put_u32(uint8_t *p, uint32_t v)
{
    p[0] = (uint8_t)(v >> 24U);
    p[1] = (uint8_t)(v >> 16U);
    p[2] = (uint8_t)(v >> 8U);
    p[3] = (uint8_t)v;
}

static void put_u64(uint8_t *p, uint64_t v)
{
    put_u32(p, (uint32_t)(v >> 32U));
    put_u32(p + 4U, (uint32_t)v);
}

static int public_key_for(const uint8_t private_key[32], uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(
        EVP_PKEY_ED25519, NULL, private_key, 32U);
    size_t length = 32U;
    int ok = key != NULL && EVP_PKEY_get_raw_public_key(
        key, public_key, &length) == 1 && length == 32U;
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static int sign_raw(const uint8_t private_key[32], const uint8_t *message,
                    size_t message_length, uint8_t signature[64])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(
        EVP_PKEY_ED25519, NULL, private_key, 32U);
    EVP_MD_CTX *context = EVP_MD_CTX_new();
    size_t signature_length = 64U;
    int ok = key != NULL && context != NULL &&
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(context, signature, &signature_length,
                       message, message_length) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static lxp_result install_metering_v1(lxp_kernel *kernel)
{
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    lx_programs_metering_schedule schedule = {0};
    lxp_result status;
    if (manifest == NULL) return LXP_ERR_ARENA_EXHAUSTED;
    schedule.version = LXP_PROGRAM_METERING_SCHEDULE_VERSION_V1;
    for (size_t i = 0U; i < 5U; ++i) schedule.coefficients[i] = 1U;
    schedule.coefficients[5] = 8U;
    schedule.coefficients[6] = 8U;
    schedule.coefficients[7] = 64U;
    schedule.coefficients[8] = 8U;
    schedule.activation_batch = 1U;
    schedule.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    manifest->signer_public_key[0] = 1U;
    status = lxp_hash_payload(manifest->signer_public_key, 32U,
                              schedule.authority_digest);
    if (status == LXP_OK)
        status = lxp_programs_metering_genesis_append(manifest, &schedule);
    if (status == LXP_OK)
        status = lxp_programs_metering_genesis_materialize(manifest, kernel);
    free(manifest);
    return status;
}

static void node_destroy(node *n)
{
    if (!n->live) return;
    while (n->kernel.blob_count != 0U)
        free(n->kernel.blobs[--n->kernel.blob_count].bytes);
    lx_account_registry_release(&n->accounts);
    (void)lxp_state_store_destroy(&n->state);
    (void)memset(n, 0, sizeof(*n));
}

static int node_genesis(node *n, const lxp_module_iface *const *registrations,
                        size_t registration_count)
{
    static const uint8_t treasury_name[] = "system:fees";
    uint8_t treasury_id[32];
    lx_account *account;
    lxp_genesis_manifest manifest = {0};
    lx_programs_fee_genesis_parameters genesis = {0};
    size_t i;
    (void)memset(n, 0, sizeof(*n));
    n->live = true;
    n->parameters = 1U;
    n->height = 10U;
    if (lx_account_registry_init(&n->accounts) != LXP_OK) return 1;
    for (i = 0U; i < ACTORS; ++i) {
        char account_name[128];
        lxp_identity *identity;
        int length = snprintf(account_name, sizeof(account_name),
                              "agent:%s:main", dids[i]);
        if (length <= 0 || (size_t)length >= sizeof(account_name) ||
            public_key_for(seeds[i], n->keys[i]) != 0 ||
            lxp_identity_register(&n->identities, (const uint8_t *)dids[i],
                                  strlen(dids[i]), n->keys[i],
                                  &identity) != LXP_OK ||
            lxp_did_id_derive((const uint8_t *)dids[i], strlen(dids[i]),
                              n->principals[i]) != LXP_OK ||
            lx_account_id_from_string((const uint8_t *)account_name,
                                      (size_t)length,
                                      n->account_ids[i]) != LXP_OK ||
            lx_account_open(&n->accounts, (const uint8_t *)account_name,
                            (size_t)length, n->account_ids[i], 1U,
                            LX_ACCOUNT_OPEN_GENESIS, NULL, &account) != LXP_OK ||
            lxp_ledger_bootstrap_balance(account, asset_id,
                (lxp_u128){0U, UINT64_C(100000000000000)}, 1U) != LXP_OK)
            return 1;
    }
    if (lx_account_id_from_string(treasury_name, sizeof(treasury_name) - 1U,
                                  treasury_id) != LXP_OK ||
        lx_account_open(&n->accounts, treasury_name,
                        sizeof(treasury_name) - 1U, treasury_id, 2U,
                        LX_ACCOUNT_OPEN_GENESIS, NULL, &account) != LXP_OK ||
        lxp_ledger_bootstrap_balance(account, asset_id, (lxp_u128){0U, 0U},
                                     0U) != LXP_OK ||
        lxp_state_store_init(&n->state, 1U) != LXP_OK ||
        lxp_state_store_bind_accounts(&n->state, &n->accounts) != LXP_OK ||
        lxp_kernel_create(&n->kernel, &n->state, &n->journal, &n->parameters,
                          0U) != LXP_OK ||
        install_metering_v1(&n->kernel) != LXP_OK)
        return 1;
    for (i = 0U; i < registration_count; ++i)
        if (lxp_kernel_register_module(&n->kernel, registrations[i]) != LXP_OK)
            return 1;
    if (registration_count == 0U)
        return lxp_state_store_require_account_root(&n->state) != LXP_OK;
    if (lxp_state_store_require_account_root(&n->state) != LXP_OK) return 1;
    (void)memcpy(n->asset.asset_id, asset_id, 32U);
    n->asset.registered = true;
    n->runtime.accounts = &n->accounts;
    n->runtime.assets = &n->asset;
    n->runtime.asset_count = 1U;
    n->runtime.fee_schedule =
        (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U};
    n->runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;
    n->runtime.metering_schedule_context = &n->kernel;
    (void)memcpy(n->runtime.occupancy_asset_id, asset_id, 32U);
    n->runtime.resolve_occupancy_parameters =
        lxp_programs_fee_governance_resolve_runtime;
    n->runtime.occupancy_parameter_context = &n->kernel;
    (void)memcpy(manifest.signer_public_key, n->keys[OWNER], 32U);
    genesis.schedule = n->runtime.fee_schedule;
    (void)memcpy(genesis.occupancy_asset_id, asset_id, 32U);
    genesis.target_occupancy_byte_batches = 3U;
    genesis.response_denominator = 1U;
    genesis.maximum_change_numerator = 1U;
    genesis.maximum_change_denominator = 1U;
    genesis.minimum_fee_units_per_occupancy_byte_batch = 1U;
    genesis.maximum_fee_units_per_occupancy_byte_batch = 10U;
    if (lxp_programs_fee_genesis_append(&manifest, &genesis) != LXP_OK ||
        lxp_programs_fee_genesis_materialize(&manifest, &n->kernel) != LXP_OK)
        return 1;
    n->fees.version = 1U;
    n->fees.multiplier_basis_points = 10000U;
    return lxp_kernel_bind_module_runtime(&n->kernel, LXP_MODULE_PROGRAMS,
                                          &n->runtime) != LXP_OK ||
        lxp_programs_bind_fee_transaction(&n->kernel) != LXP_OK ||
        lxp_kernel_set_capabilities(&n->kernel, NULL,
                                    lxp_kernel_canonical_ledger_apply) != LXP_OK ||
        lxp_state_root(&n->kernel, n->kernel.current_state_root) != LXP_OK ||
        lxp_arena_init(&n->arena, n->arena_bytes, sizeof(n->arena_bytes)) !=
            LXP_OK;
}

static lxp_result execute(node *n, unsigned actor, uint32_t type,
                          const uint8_t *payload, size_t length)
{
    lxp_activity activity = {0};
    lxp_activity received;
    lxp_authority_resolved authority = {0};
    lxp_authority_grant grant = {0};
    lxp_transfer_allowance allowance = {0};
    lxp_kernel_execution execution = {0};
    lxp_byte_span encoded;
    lxp_batch_roots roots;
    uint8_t signature[64], preimage[88], digest[32];
    lxp_result result;
    size_t index;
    if (lxp_arena_reset(&n->arena, 0U) != LXP_OK) return LXP_FATAL_INVARIANT;
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.network_id = NETWORK_ID;
    activity.activity_type = type;
    activity.actor_did =
        (lxp_byte_span){(const uint8_t *)dids[actor], strlen(dids[actor])};
    activity.authority = (lxp_byte_span){n->keys[actor], 32U};
    activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    activity.account_sequence = n->identities.identities[actor].next_sequence;
    put_u64(activity.idempotency_key + 24U, n->state.next_sequence);
    activity.payload = (lxp_byte_span){payload, length};
    activity.fee_limit = (lxp_u128){0U, UINT64_C(10000000000)};
    if (lxp_hash_payload(payload, length, activity.payload_hash) != LXP_OK ||
        lxp_activity_signing_preimage(&activity, digest) != LXP_OK ||
        sign_raw(seeds[actor], digest, 32U, signature) != 0)
        return LXP_ERR_BAD_SIGNATURE;
    activity.signature = (lxp_byte_span){signature, 64U};
    if (lxp_activity_verify_signature(&activity) != LXP_OK)
        return LXP_ERR_BAD_SIGNATURE;
    result = lxp_authority_resolve_activity(
        &n->kernel, &n->identities.identities[actor], &activity, true, true,
        10U, 100U, n->state.next_sequence, &grant, &authority);
    if (result != LXP_OK) return result;
    lxp_authority_allowance_bind(&grant, &authority, &allowance);
    execution.network_id = NETWORK_ID;
    execution.epoch = n->kernel.epoch;
    execution.batch_number = n->height;
    execution.batch_timestamp_ms = 10U;
    execution.maximum_timestamp_window = 100U;
    execution.global_sequence = n->state.next_sequence;
    execution.recorded_module_version = lxp_programs_module_version(&n->kernel);
    execution.recorded_metering_schedule_version = 1U;
    execution.recorded_fee_schedule_version = 1U;
    execution.parameter_version = 1U;
    execution.signature_valid = true;
    execution.identities = &n->identities;
    execution.authority = &authority;
    execution.allowance = &allowance;
    execution.fee_parameters = &n->fees;
    for (index = 0U; index < n->accounts.count; ++index)
        if (memcmp(n->accounts.accounts[index].id, n->account_ids[actor], 32U) == 0)
            execution.fee_balance = n->accounts.accounts[index].balance;
    execution.gas_limit = UINT64_C(1000000000);
    execution.arena = &n->arena;
    execution.sequencer_private_key = sequencer_seed;
    if (lxp_activity_encode(&activity, &n->arena, &encoded) != LXP_OK ||
        lxp_batch_roots_compute(
            &(lxp_batch_root_inputs){&encoded, 1U, NULL, 0U, NULL, 0U, NULL, 0U,
                                     NULL, 0U},
            &n->arena, &roots) != LXP_OK)
        return LXP_FATAL_INVARIANT;
    (void)memcpy(preimage, n->kernel.current_state_root, 32U);
    (void)memcpy(preimage + 32U, roots.activity_merkle_root, 32U);
    put_u64(preimage + 64U, execution.global_sequence);
    put_u64(preimage + 72U, execution.global_sequence);
    put_u64(preimage + 80U, execution.batch_number);
    if (lxp_hash_context_value(preimage, sizeof(preimage), execution.batch_id) !=
        LXP_OK)
        return LXP_FATAL_INVARIANT;
    (void)memcpy(execution.activity_root, roots.activity_merkle_root, 32U);
    if (lxp_activity_decode(encoded.bytes, encoded.length, &received) != LXP_OK)
        return LXP_ERR_NON_CANONICAL;
    (void)memset(&n->receipt, 0, sizeof(n->receipt));
    result = lxp_kernel_execute_activity(&n->kernel, &received, &execution,
                                         &n->receipt);
    if (result == LXP_OK) {
        uint8_t root[32];
        uint8_t activity_id[32];
        if (lxp_receipt_verify(&n->receipt, sequencer_public, &n->arena) !=
            LXP_OK)
            return LXP_ERR_BAD_SIGNATURE;
        if (lxp_activity_id(encoded.bytes, encoded.length, activity_id) != LXP_OK ||
            memcmp(activity_id, n->receipt.activity_id, 32U) != 0 ||
            memcmp(execution.batch_id, n->receipt.batch_id, 32U) != 0 ||
            memcmp(execution.activity_root, n->receipt.activity_root, 32U) != 0 ||
            n->receipt.global_sequence != execution.global_sequence ||
            lxp_state_root(&n->kernel, root) != LXP_OK ||
            memcmp(root, n->receipt.resulting_state_root, 32U) != 0 ||
            memcmp(root, n->kernel.current_state_root, 32U) != 0)
            return LXP_FATAL_INVARIANT;
    }
    return result;
}

static int blob_admission_without_capacity(void)
{
    static const uint8_t wasm[] = {
        0,97,115,109,1,0,0,0,1,12,2,96,2,127,127,1,127,96,1,127,1,127,
        3,3,2,0,1,5,3,1,0,1,7,41,3,
        11,'l','a','y','e','r','x','_','c','a','l','l',0,0,
        14,'l','a','y','e','r','x','_','r','e','s','e','r','v','e',0,1,
        6,'m','e','m','o','r','y',2,0,10,11,2,4,0,65,0,11,4,0,65,0,11
    };
    static const uint8_t domain[] = "LayerX/program-interface/v1";
    const lxp_module_iface *registration = programs_module_registration_v5();
    node *n = calloc(1U, sizeof(*n));
    lxp_module_ctx *reader = calloc(1U, sizeof(*reader));
    lxp_receipt *decoded = calloc(1U, sizeof(*decoded));
    lxp_byte_span canonical;
    const uint8_t *stored;
    size_t stored_length;
    uint8_t payload[512] = {1U};
    uint8_t digest[32], decoded_digest[32];
    uint8_t root[32];
    size_t offset = 108U;
    size_t interface_length;

    LXP_CASE_CHECK(n != NULL && reader != NULL && decoded != NULL);
    LXP_CASE_CHECK(public_key_for(sequencer_seed, sequencer_public) == 0);
    LXP_CASE_CHECK(node_genesis(n, &registration, 1U) == 0);
    LXP_CASE_CHECK(n->kernel.blob_count == 0U);
    put_u16(payload + 32U, LX_PROGRAMS_ABI_VERSION);
    payload[34] = 1U;
    (void)memcpy(payload + 36U, n->principals[OWNER], 32U);
    LXP_CASE_CHECK(lxp_hash_sha256(wasm, sizeof(wasm), payload + 68U) == LXP_OK);
    put_u32(payload + 100U, (uint32_t)sizeof(wasm));
    (void)memcpy(payload + offset, domain, sizeof(domain));
    offset += sizeof(domain);
    (void)memcpy(payload + offset, payload + 68U, 32U); offset += 32U;
    put_u16(payload + offset, LX_PROGRAMS_ABI_VERSION); offset += 2U;
    put_u16(payload + offset, 1U); offset += 2U;
    put_u16(payload + offset, 11U); offset += 2U;
    (void)memcpy(payload + offset, "layerx_call", 11U); offset += 11U;
    offset += 4U;
    payload[offset++] = 1U; payload[offset++] = 0x20U;
    put_u32(payload + offset, 64U); offset += 4U;
    payload[offset++] = 1U; payload[offset++] = 0x20U;
    put_u32(payload + offset, 64U); offset += 4U;
    offset += 6U;
    interface_length = offset - 108U;
    put_u32(payload + 104U, (uint32_t)interface_length);
    (void)memcpy(payload + offset, wasm, sizeof(wasm));
    offset += sizeof(wasm);

    LXP_CASE_CHECK(execute(n, OWNER, LX_PROGRAMS_DEPLOY, payload, offset) == LXP_OK);
    LXP_CASE_CHECK(n->receipt.protocol_version == LXP_PROTOCOL_VERSION_STATE_COMMITMENT);
    LXP_CASE_CHECK(n->receipt.result_code == LXP_OK);
    LXP_CASE_CHECK(n->receipt.module_id == LXP_MODULE_PROGRAMS);
    LXP_CASE_CHECK(n->receipt.module_version == registration->abi_version);
    LXP_CASE_CHECK(n->receipt.global_sequence == 1U);
    LXP_CASE_CHECK(n->receipt.effects.count == 1U);
    LXP_CASE_CHECK(n->receipt.effects.effects[0].event_type == LX_PROGRAMS_EVENT_DEPLOYED);
    LXP_CASE_CHECK(n->receipt.effects.effects[0].body_length == 32U);
    LXP_CASE_CHECK(memcmp(n->receipt.effects.effects[0].body, payload + 68U, 32U) == 0);
    LXP_CASE_CHECK(n->kernel.blob_count == 1U);
    LXP_CASE_CHECK(n->kernel.blob_total_bytes == sizeof(wasm));
    LXP_CASE_CHECK(n->state.next_sequence == 2U);
    LXP_CASE_CHECK(n->identities.identities[OWNER].next_sequence == 1U);
    LXP_CASE_CHECK(!n->journal.open);
    LXP_CASE_CHECK(lxp_state_root(&n->kernel, root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root, n->receipt.resulting_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(root, n->receipt.previous_state_root, 32U) != 0);
    LXP_CASE_CHECK(lxp_receipt_encode(&n->receipt, true, &n->arena, &canonical) == LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_decode(canonical.bytes, canonical.length, true, decoded) == LXP_OK);
    LXP_CASE_CHECK(decoded->protocol_version == LXP_PROTOCOL_VERSION_STATE_COMMITMENT);
    LXP_CASE_CHECK(lxp_receipt_verify(decoded, sequencer_public, &n->arena) == LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_digest(&n->receipt, &n->arena, digest) == LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_digest(decoded, &n->arena, decoded_digest) == LXP_OK);
    LXP_CASE_CHECK(memcmp(digest, decoded_digest, 32U) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_init(reader, &n->kernel, LXP_MODULE_PROGRAMS,
        10U, n->kernel.epoch, n->state.next_sequence, 1000000U,
        &n->arena, false) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_blob_get(reader, payload + 68U, &stored, &stored_length) == LXP_OK);
    LXP_CASE_CHECK(stored_length == sizeof(wasm));
    LXP_CASE_CHECK(memcmp(stored, wasm, sizeof(wasm)) == 0);
    lxp_module_ctx_rollback(reader);
    node_destroy(n);
    free(decoded);
    free(reader);
    free(n);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"kernel_entry_symbols_absent", kernel_entry_symbols_absent},
        {"blob_admission_without_capacity", blob_admission_without_capacity}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
