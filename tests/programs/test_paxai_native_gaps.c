#include "layerx/programs.h"

#include "layerx/lxp_activity.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_fee.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_identity.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_module_ctx.h"
#include "layerx/lxp_protocol.h"
#include "layerx/lxp_receipt.h"
#include "layerx/lxp_state_proof.h"

#include <openssl/evp.h>

#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

enum {
    ACTORS = 4,
    OWNER = 0,
    CONTROLLER = 3,
    NETWORK_ID = 7,
    PROOF_CAPACITY = 6144,
    REPLAY_DOMAIN_BYTES = 29,
    REPLAY_BODY_BYTES = 365,
    WITNESS_BYTES = 600,
    ORIGIN_SEQUENCE = 1,
    CUTOFF_SEQUENCE = 2,
    V4_ACTIVITY_TYPES = 10,
    CONTROL_LAST_CUTOFF = 179,
    CONTROL_RETIREMENTS = 187,
    CONTROL_BLOBS = 195,
    CONTROL_BYTES = 203,
    CONTROL_REPLAYS = 211
};

_Static_assert((int)REPLAY_DOMAIN_BYTES + (int)REPLAY_BODY_BYTES == 394,
               "replay record size changed");

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

static const char *const dids[ACTORS] = {
    "did:lxp:paxai-owner", "did:lxp:paxai-treasury", "did:lxp:paxai-outsider",
    "did:lxp:paxai-controller"
};
static const uint8_t seeds[ACTORS][32] = {{0x63U}, {0x64U}, {0x65U}, {0x67U}};
static const uint8_t sequencer_seed[32] = {0x66U};
static const uint8_t archive_seed_a[32] = {0x68U};
static const uint8_t archive_seed_b[32] = {0x69U};
static const uint8_t asset_id[32] = {9U};
static const uint8_t replay_domain[REPLAY_DOMAIN_BYTES] =
    "LXP/program-replay-native/v1";
static node *primary, *scratch_node;
static lxp_state_witness witness;
static uint8_t retirement_payload[LXP_PROGRAMS_RETIREMENT_HEADER_BYTES +
                                  LXP_PROGRAMS_RETIREMENT_CANDIDATE_BYTES +
                                  PROOF_CAPACITY];
static uint8_t sequencer_public[32];
static uint8_t program_id[32];
static uint8_t profile_digest[32];
static size_t passed, failed;

#define CHECK(condition, name) \
    do { \
        if (!(condition)) { \
            (void)printf("NATIVE_GAPS_FAIL %s line %d: %s\n", name, __LINE__, \
                         #condition); \
            (void)fflush(stdout); \
            return 1; \
        } \
    } while (0)

static int pass(const char *name)
{
    ++passed;
    (void)printf("NATIVE_GAPS_CASE %s\n", name);
    (void)fflush(stdout);
    return 0;
}

static int tally(int result)
{
    if (result != 0) ++failed;
    return result;
}

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

static uint32_t get_u32(const uint8_t *p)
{
    return ((uint32_t)p[0] << 24U) | ((uint32_t)p[1] << 16U) |
           ((uint32_t)p[2] << 8U) | p[3];
}

static uint64_t get_u64(const uint8_t *p)
{
    return ((uint64_t)get_u32(p) << 32U) | get_u32(p + 4U);
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
    static const uint8_t active_key[] = "progmet/active/v1";
    static const uint8_t history_key[] = {
        'p', 'r', 'o', 'g', 'm', 'e', 't', '/', 'h', 'i', 's', 't', 'o',
        'r', 'y', '/', 'v', '1', '/', 0U, 0U, 0U, 1U
    };
    static const uint64_t coefficients[9] = {1U, 1U, 1U, 1U, 1U,
                                              8U, 8U, 64U, 8U};
    uint8_t record[LX_PROGRAMS_METERING_RECORD_BYTES] = {0U};
    size_t offset = 0U;
    size_t index;
    if (kernel == NULL || kernel->module_kv_count != 0U)
        return LXP_ERR_NON_CANONICAL;
    (void)memcpy(record + offset, "LXMR1", 5U);
    offset += 5U;
    put_u32(record + offset, 1U);
    offset += 4U;
    for (index = 0U; index < 9U; ++index) {
        put_u64(record + offset, coefficients[index]);
        offset += 8U;
    }
    put_u64(record + offset, 1U);
    offset += 8U;
    record[offset++] = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    (void)memset(record + offset, 0xa5, 32U);
    if (offset + 32U != sizeof(record)) return LXP_FATAL_INVARIANT;
    kernel->module_kv[0].module_id = LXP_MODULE_PROGRAMS;
    kernel->module_kv[0].key_length = sizeof(active_key) - 1U;
    kernel->module_kv[0].value_length = sizeof(record);
    (void)memcpy(kernel->module_kv[0].key, active_key,
                 sizeof(active_key) - 1U);
    (void)memcpy(kernel->module_kv[0].value, record, sizeof(record));
    kernel->module_kv[1].module_id = LXP_MODULE_PROGRAMS;
    kernel->module_kv[1].key_length = sizeof(history_key);
    kernel->module_kv[1].value_length = sizeof(record);
    (void)memcpy(kernel->module_kv[1].key, history_key, sizeof(history_key));
    (void)memcpy(kernel->module_kv[1].value, record, sizeof(record));
    kernel->module_kv_count = 2U;
    return LXP_OK;
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
    if (lxp_arena_reset(&n->arena, 0U) != LXP_OK) return LXP_FATAL_INVARIANT;
    (void)memset(&n->receipt, 0, sizeof(n->receipt));
    result = lxp_kernel_execute_activity(&n->kernel, &activity, &execution,
                                         &n->receipt);
    if (result == LXP_OK) {
        uint8_t root[32];
        if (lxp_receipt_verify(&n->receipt, sequencer_public, &n->arena) !=
            LXP_OK)
            return LXP_ERR_BAD_SIGNATURE;
        if (lxp_state_root(&n->kernel, root) != LXP_OK ||
            memcmp(root, n->receipt.resulting_state_root, 32U) != 0 ||
            memcmp(root, n->kernel.current_state_root, 32U) != 0)
            return LXP_FATAL_INVARIANT;
    }
    return result;
}

static int run(node *n, unsigned actor, uint32_t type, const uint8_t *payload,
               size_t length, lxp_result *status)
{
    const lxp_identity *identity = &n->identities.identities[actor];
    const uint64_t sequence = n->state.next_sequence;
    const uint64_t account_sequence = identity->next_sequence;
    *status = execute(n, actor, type, payload, length);
    if (*status != LXP_OK)
        return n->state.next_sequence == sequence &&
            identity->next_sequence == account_sequence ? 0 : 1;
    return n->state.next_sequence == sequence + 1U ? 0 : 1;
}

static const lxp_module_kv_entry *find_kv(const lxp_kernel *kernel,
                                          const uint8_t *key, size_t length)
{
    size_t i;
    for (i = 0U; i < kernel->module_kv_count; ++i) {
        const lxp_module_kv_entry *entry = &kernel->module_kv[i];
        if (entry->module_id == LXP_MODULE_PROGRAMS &&
            entry->key_length == length && memcmp(entry->key, key, length) == 0)
            return entry;
    }
    return NULL;
}

static const lxp_module_blob *find_blob(const lxp_kernel *kernel,
                                        const uint8_t key[32])
{
    size_t i;
    for (i = 0U; i < kernel->blob_count; ++i) {
        const lxp_module_blob *blob = &kernel->blobs[i];
        if (!blob->deleted && blob->module_id == LXP_MODULE_PROGRAMS &&
            memcmp(blob->key, key, 32U) == 0)
            return blob;
    }
    return NULL;
}

static int seed_open(node *n, lxp_module_ctx *ctx, lxp_arena *arena,
                     uint8_t *bytes, size_t length)
{
    return lxp_arena_init(arena, bytes, length) != LXP_OK ||
        lxp_module_ctx_init(ctx, &n->kernel, LXP_MODULE_PROGRAMS, 10U,
                            n->kernel.epoch, n->state.next_sequence,
                            UINT64_C(100000000), arena, true) != LXP_OK;
}

static size_t retirement_encode(uint8_t count, uint64_t cutoff,
                                const uint8_t digest[32], uint32_t size,
                                const uint8_t origin_root[32],
                                const uint8_t replay_id[32],
                                const uint8_t *proof, uint16_t proof_length,
                                const uint8_t controller[32])
{
    uint8_t *out = retirement_payload;
    uint8_t certificate[32];
    size_t cursor = LXP_PROGRAMS_RETIREMENT_HEADER_BYTES;
    (void)memset(out, 0, sizeof(retirement_payload));
    out[0] = LXP_PROGRAMS_RETIREMENT_REQUEST_VERSION;
    out[1] = count;
    put_u32(out + 4U, NETWORK_ID);
    put_u16(out + 8U, LXP_PROGRAMS_RETIREMENT_PROFILE_VERSION);
    (void)memcpy(out + 10U, profile_digest, 32U);
    (void)memcpy(out + 42U, controller, 32U);
    put_u64(out + 74U, primary->state.next_sequence);
    (void)memcpy(out + 82U, primary->kernel.current_state_root, 32U);
    put_u64(out + 114U, cutoff);
    (void)memset(out + 122U, 0x5a, 32U);
    (void)memset(out + 154U, 0x5b, 32U);
    put_u64(out + 186U, 4096U);
    put_u64(out + 194U, ORIGIN_SEQUENCE);
    put_u64(out + 202U, cutoff);
    (void)memset(out + 210U, 0x5c, 32U);
    if (proof != NULL) {
        uint8_t *fixed = out + cursor;
        fixed[0] = LXP_PROGRAMS_RETIREMENT_CLASS_REPLAY;
        (void)memcpy(fixed + 37U, digest, 32U);
        put_u32(fixed + 69U, size);
        put_u64(fixed + 73U, ORIGIN_SEQUENCE);
        (void)memcpy(fixed + 81U, origin_root, 32U);
        (void)memcpy(fixed + 113U, replay_id, 32U);
        put_u16(fixed + 145U, proof_length);
        put_u16(fixed + 147U, 0U);
        cursor += LXP_PROGRAMS_RETIREMENT_CANDIDATE_BYTES;
        (void)memcpy(out + cursor, proof, proof_length);
        cursor += proof_length;
        if (lxp_programs_retirement_certificate_digest(out, cursor,
                                                       certificate) != LXP_OK ||
            sign_raw(archive_seed_a, certificate, 32U, out + 242U) != 0 ||
            sign_raw(archive_seed_b, certificate, 32U, out + 306U) != 0)
            return 0U;
    }
    return cursor;
}

static void replay_record(uint8_t record[REPLAY_DOMAIN_BYTES + REPLAY_BODY_BYTES],
                          const uint8_t replay_id[32],
                          const uint8_t blob_key[32])
{
    uint8_t *body = record + REPLAY_DOMAIN_BYTES;
    (void)memset(record, 0, REPLAY_DOMAIN_BYTES + REPLAY_BODY_BYTES);
    (void)memcpy(record, replay_domain, REPLAY_DOMAIN_BYTES);
    put_u16(body, 1U);
    put_u32(body + 2U, NETWORK_ID);
    (void)memcpy(body + 6U, replay_id, 32U);
    put_u64(body + 38U, ORIGIN_SEQUENCE);
    (void)memset(body + 46U, 0x74, 32U);
    (void)memcpy(body + 78U, program_id, 32U);
    (void)memset(body + 110U, 0x75, 32U);
    (void)memset(body + 142U, 0x76, 32U);
    put_u16(body + 174U, 1U);
    put_u16(body + 176U, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
    put_u32(body + 178U, 1U);
    put_u32(body + 182U, 1U);
    put_u16(body + 186U, 1U);
    put_u32(body + 188U, 2U);
    put_u32(body + 192U, LXP_PROGRAMS_REPLAY_MAX_BYTES);
    body[196] = 0U;
    put_u32(body + 197U, 1U);
    (void)memset(body + 201U, 0x77, 32U);
    (void)memset(body + 233U, 0x78, 32U);
    (void)memset(body + 265U, 0x79, 32U);
    (void)memset(body + 297U, 0x7a, 32U);
    put_u32(body + 329U, 0U);
    (void)memcpy(body + 333U, blob_key, 32U);
}

static int case_module5_retirement(void)
{
    static const char name[] = "module5_retirement_resolves_and_applies";
    static const uint8_t control_key[] = "progretire/v1";
    static uint8_t bytes[65536];
    static uint8_t witness_bytes[WITNESS_BYTES];
    static uint8_t proof[PROOF_CAPACITY];
    const lxp_module_iface *registration = programs_module_registration_v4();
    lxp_programs_retirement_profile profile;
    lxp_module_ctx ctx;
    lxp_arena arena;
    uint8_t record[REPLAY_DOMAIN_BYTES + REPLAY_BODY_BYTES];
    uint8_t replay_id[32], blob_key[32], origin_root[32], recovered[32];
    uint8_t key[LXP_PROGRAMS_REPLAY_KEY_BYTES];
    const lxp_module_kv_entry *control;
    const lxp_effect *header;
    lxp_result status = LXP_OK;
    size_t length, proof_length = 0U;
    uint64_t sequence;
    (void)memset(&profile, 0, sizeof(profile));
    profile.version = LXP_PROGRAMS_RETIREMENT_PROFILE_VERSION;
    profile.network_id = NETWORK_ID;
    (void)memcpy(profile.controller, primary->principals[CONTROLLER], 32U);
    (void)memcpy(profile.paxai_program_id, program_id, 32U);
    profile.hot_horizon = 1U;
    CHECK(public_key_for(archive_seed_a, profile.archive_keys[0]) == 0 &&
          public_key_for(archive_seed_b, profile.archive_keys[1]) == 0 &&
          lxp_programs_retirement_profile_digest(&profile, profile_digest) ==
              LXP_OK, name);
    (void)memset(replay_id, 0x73, 32U);
    (void)memset(witness_bytes, 0x5e, sizeof(witness_bytes));
    CHECK(lxp_hash_sha256(witness_bytes, sizeof(witness_bytes), blob_key) ==
          LXP_OK, name);
    replay_record(record, replay_id, blob_key);
    CHECK(lxp_programs_replay_record_blob_key(
              (lxp_byte_span){record, sizeof(record)}, recovered) == LXP_OK &&
          memcmp(recovered, blob_key, 32U) == 0, name);
    lxp_programs_replay_record_key(replay_id, key);
    CHECK(seed_open(primary, &ctx, &arena, bytes, sizeof(bytes)) == 0, name);
    status = lxp_programs_retirement_profile_stage(&ctx, &profile);
    if (status == LXP_OK)
        status = lxp_ctx_blob_put(&ctx, blob_key, witness_bytes,
                                  sizeof(witness_bytes));
    if (status == LXP_OK)
        status = lxp_ctx_kv_put(&ctx, key, sizeof(key), record, sizeof(record));
    if (status != LXP_OK) lxp_module_ctx_rollback(&ctx);
    CHECK(status == LXP_OK, name);
    CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK &&
          lxp_state_root(&primary->kernel,
                         primary->kernel.current_state_root) == LXP_OK, name);
    (void)memcpy(origin_root, primary->kernel.current_state_root, 32U);
    CHECK(lxp_state_proof_build(&primary->kernel, LXP_MODULE_PROGRAMS,
                                (lxp_byte_span){key, sizeof(key)},
                                &witness) == LXP_OK &&
          lxp_state_proof_verify(&witness, origin_root) == LXP_OK &&
          lxp_state_proof_encode(&witness, proof, sizeof(proof),
                                 &proof_length) == LXP_OK &&
          proof_length != 0U && proof_length <= UINT16_MAX, name);

    length = retirement_encode(1U, CUTOFF_SEQUENCE, NULL, 0U, NULL, NULL, NULL,
                               0U, primary->principals[CONTROLLER]);
    CHECK(registration->abi_version == LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION,
          name);
    node_destroy(scratch_node);
    CHECK(node_genesis(scratch_node, &registration, 1U) == 0 &&
          lxp_programs_module_version(&scratch_node->kernel) ==
              LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION, name);
    CHECK(run(scratch_node, CONTROLLER, LX_PROGRAMS_STORAGE_RETIREMENT,
              retirement_payload, length, &status) == 0, name);
    (void)printf("NATIVE_GAPS_NOTE %s module4_status=%d\n", name, (int)status);
    CHECK(status == LXP_ERR_AUTH_SCOPE, name);
    node_destroy(scratch_node);

    sequence = primary->state.next_sequence;
    CHECK(sequence == ORIGIN_SEQUENCE, name);
    CHECK(run(primary, CONTROLLER, LX_PROGRAMS_STORAGE_RETIREMENT,
              retirement_payload, length, &status) == 0, name);
    (void)printf("NATIVE_GAPS_NOTE %s header_only status=%d result=%d\n", name,
                 (int)status,
                 status == LXP_OK ? (int)primary->receipt.result_code : 0);
    CHECK(status == LXP_OK &&
          primary->receipt.result_code == LXP_ERR_TRUNCATED &&
          primary->receipt.module_id == LXP_MODULE_PROGRAMS &&
          primary->receipt.module_version == LX_PROGRAMS_GUEST_ABI_V5_VERSION &&
          primary->state.next_sequence == sequence + 1U, name);

    length = retirement_encode(1U, CUTOFF_SEQUENCE, blob_key, WITNESS_BYTES,
                               origin_root, replay_id, proof,
                               (uint16_t)proof_length,
                               primary->principals[CONTROLLER]);
    CHECK(length == LXP_PROGRAMS_RETIREMENT_HEADER_BYTES +
                        LXP_PROGRAMS_RETIREMENT_CANDIDATE_BYTES + proof_length,
          name);
    CHECK(run(primary, CONTROLLER, LX_PROGRAMS_STORAGE_RETIREMENT,
              retirement_payload, length, &status) == 0, name);
    (void)printf("NATIVE_GAPS_NOTE %s early status=%d result=%d\n", name,
                 (int)status,
                 status == LXP_OK ? (int)primary->receipt.result_code : 0);
    CHECK(status == LXP_OK &&
          primary->receipt.result_code == LXP_ERR_NOT_YET_VALID &&
          primary->receipt.module_version == LX_PROGRAMS_GUEST_ABI_V5_VERSION &&
          find_kv(&primary->kernel, key, sizeof(key)) != NULL &&
          find_blob(&primary->kernel, blob_key) != NULL, name);

    sequence = primary->state.next_sequence;
    CHECK(sequence == CUTOFF_SEQUENCE + 1U, name);
    length = retirement_encode(1U, CUTOFF_SEQUENCE, blob_key, WITNESS_BYTES,
                               origin_root, replay_id, proof,
                               (uint16_t)proof_length,
                               primary->principals[CONTROLLER]);
    CHECK(length != 0U, name);
    CHECK(run(primary, CONTROLLER, LX_PROGRAMS_STORAGE_RETIREMENT,
              retirement_payload, length, &status) == 0, name);
    (void)printf("NATIVE_GAPS_NOTE %s apply status=%d result=%d\n", name,
                 (int)status,
                 status == LXP_OK ? (int)primary->receipt.result_code : 0);
    CHECK(status == LXP_OK && primary->receipt.result_code == LXP_OK &&
          primary->receipt.module_version == LX_PROGRAMS_GUEST_ABI_V5_VERSION &&
          primary->state.next_sequence == sequence + 1U, name);
    CHECK(find_kv(&primary->kernel, key, sizeof(key)) == NULL &&
          find_blob(&primary->kernel, blob_key) == NULL, name);
    control = find_kv(&primary->kernel, control_key, sizeof(control_key) - 1U);
    CHECK(control != NULL &&
          control->value_length == LXP_PROGRAMS_RETIREMENT_CONTROL_BYTES &&
          get_u64(control->value + CONTROL_LAST_CUTOFF) == CUTOFF_SEQUENCE &&
          get_u64(control->value + CONTROL_RETIREMENTS) == 1U &&
          get_u64(control->value + CONTROL_BLOBS) == 1U &&
          get_u64(control->value + CONTROL_BYTES) == WITNESS_BYTES &&
          get_u64(control->value + CONTROL_REPLAYS) == 1U, name);
    CHECK(primary->receipt.effects.count == 2U &&
          primary->receipt.effects.effects[0].kind == LXP_EFFECT_EVENT &&
          primary->receipt.effects.effects[0].event_type ==
              LXP_PROGRAMS_RETIREMENT_EVENT &&
          primary->receipt.effects.effects[1].kind == LXP_EFFECT_EVENT &&
          primary->receipt.effects.effects[1].event_type ==
              LXP_PROGRAMS_RETIREMENT_EVENT, name);
    header = &primary->receipt.effects.effects[0];
    CHECK(header->body_length == 226U && header->body[199] == 1U &&
          header->body[200] == 1U && header->body[201] == 1U, name);
    return pass(name);
}

static bool lists_v4(const lxp_module_iface *iface)
{
    size_t i;
    if (iface->activity_type_count < V4_ACTIVITY_TYPES) return false;
    for (i = 0U; i < V4_ACTIVITY_TYPES; ++i)
        if (iface->activity_types[i] !=
            programs_module_registration_v4()->activity_types[i])
            return false;
    return true;
}

static bool lists(const lxp_module_iface *iface, uint32_t type)
{
    size_t i;
    for (i = 0U; i < iface->activity_type_count; ++i)
        if (iface->activity_types[i] == type) return true;
    return false;
}

static int pin_for(const lxp_module_iface *const *registrations, size_t count,
                   uint32_t *pin)
{
    node_destroy(scratch_node);
    if (node_genesis(scratch_node, registrations, count) != 0) return 1;
    *pin = lxp_programs_module_version(&scratch_node->kernel);
    node_destroy(scratch_node);
    return 0;
}

static int case_module_pin(void)
{
    static const char name[] = "module_pin_follows_highest_registration";
    const lxp_module_iface *v4 = programs_module_registration_v4();
    const lxp_module_iface *v4r =
        programs_module_registration_v4_storage_retirement();
    const lxp_module_iface *v5 = programs_module_registration_v5();
    const lxp_module_iface *only_v4[] = {v4};
    const lxp_module_iface *only_v4r[] = {v4r};
    const lxp_module_iface *only_v5[] = {v5};
    const lxp_module_iface *v4_then_v5[] = {v4, v5};
    const lxp_module_iface *v4r_then_v5[] = {v4r, v5};
    uint32_t pin = 0U;
    CHECK(v4->abi_version == LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION &&
          v4->activity_type_count == V4_ACTIVITY_TYPES &&
          !lists(v4, LX_PROGRAMS_STORAGE_RETIREMENT), name);
    CHECK(v4r->abi_version == LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION &&
          v4r->activity_type_count == V4_ACTIVITY_TYPES + 1U && lists_v4(v4r) &&
          v4r->activity_types[V4_ACTIVITY_TYPES] ==
              LX_PROGRAMS_STORAGE_RETIREMENT, name);
    CHECK(v5->module_id == LXP_MODULE_PROGRAMS &&
          v5->abi_version == LX_PROGRAMS_GUEST_ABI_V5_VERSION &&
          v5->activity_type_count == V4_ACTIVITY_TYPES + 1U && lists_v4(v5) &&
          v5->activity_types[V4_ACTIVITY_TYPES] ==
              LX_PROGRAMS_STORAGE_RETIREMENT &&
          memcmp(v5->activity_types, v4r->activity_types,
                 v5->activity_type_count * sizeof(v5->activity_types[0])) == 0,
          name);
    CHECK(lxp_programs_module_version(NULL) ==
          LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION, name);
    CHECK(pin_for(NULL, 0U, &pin) == 0 &&
          pin == LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION, name);
    CHECK(pin_for(only_v4, 1U, &pin) == 0 &&
          pin == LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION, name);
    CHECK(pin_for(only_v4r, 1U, &pin) == 0 &&
          pin == LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION, name);
    CHECK(pin_for(only_v5, 1U, &pin) == 0 &&
          pin == LX_PROGRAMS_GUEST_ABI_V5_VERSION, name);
    CHECK(pin_for(v4_then_v5, 2U, &pin) == 0 &&
          pin == LX_PROGRAMS_GUEST_ABI_V5_VERSION, name);
    CHECK(pin_for(v4r_then_v5, 2U, &pin) == 0 &&
          pin == LX_PROGRAMS_GUEST_ABI_V5_VERSION, name);
    CHECK(lxp_programs_module_version(&primary->kernel) ==
          LX_PROGRAMS_GUEST_ABI_V5_VERSION, name);
    return pass(name);
}

static void cleanup(void)
{
    if (primary != NULL) node_destroy(primary);
    if (scratch_node != NULL) node_destroy(scratch_node);
    free(primary);
    free(scratch_node);
}

static int setup(void)
{
    const lxp_module_iface *registration = programs_module_registration_v5();
    primary = (node *)calloc(1U, sizeof(node));
    scratch_node = (node *)calloc(1U, sizeof(node));
    if (primary == NULL || scratch_node == NULL ||
        public_key_for(sequencer_seed, sequencer_public) != 0)
        return 1;
    (void)memset(program_id, 0x71, 32U);
    return node_genesis(primary, &registration, 1U) != 0;
}

int main(void)
{
    if (setup() != 0) {
        (void)printf("NATIVE_GAPS_FAIL setup\n");
        cleanup();
        return 1;
    }
    (void)tally(case_module_pin());
    (void)tally(case_module5_retirement());
    cleanup();
    (void)printf("PASSED %zu FAILED %zu\n", passed, failed);
    return failed == 0U ? 0 : 1;
}
