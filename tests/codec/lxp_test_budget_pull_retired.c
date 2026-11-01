#include "lxp_kernel_case.h"
#include "layerx/lx_budget.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_identity.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_protocol.h"
#include "layerx/lxp_state_proof.h"

#include <openssl/evp.h>
#include <stdlib.h>
#include <string.h>

enum {
    CAPABILITY_KEY_BYTES = 80,
    CAPABILITY_BYTES = 195,
    RETIRED_PULL_TYPE = 0x0003000b,
    RETIRED_PULL_PAYLOAD_BYTES = 460
};

static const char owner_did[] = "did:key:owner";
static const char delegate_did[] = "did:key:dele";
static const uint8_t owner_seed[32] = {11U};
static const uint8_t delegate_seed[32] = {22U};
static const uint8_t test_budget_id[32] = {0x0DU, 0xEFU, 0x03U};

typedef struct budget_env {
    lx_account_registry accounts;
    lx_asset_record asset;
    lxp_transfer_asset_state asset_state;
    lx_asset_runtime asset_runtime;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    const lxp_module_registration *registration;
    lx_account *owner;
    lx_account *budget;
    lx_account *recipient;
    lx_account *delegate;
    uint8_t delegate_actor[32];
    uint8_t owner_actor[32];
    uint64_t parameters;
    uint64_t global_sequence;
} budget_env;

typedef struct budget_call {
    uint32_t activity_type;
    const char *did;
    const uint8_t *seed;
    const uint8_t *payload;
    size_t payload_length;
    uint64_t sequence;
    uint64_t timestamp;
} budget_call;

static budget_env env;

static int sign_raw(const uint8_t seed[32], const uint8_t *message,
                    size_t message_length, uint8_t signature[64],
                    uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                  seed, 32U);
    EVP_MD_CTX *context = EVP_MD_CTX_new();
    size_t public_length = 32U;
    size_t signature_length = 64U;
    int ok = key != NULL && context != NULL &&
             EVP_PKEY_get_raw_public_key(key, public_key, &public_length) == 1 &&
             public_length == 32U &&
             EVP_DigestSignInit(context, NULL, NULL, NULL, key) == 1 &&
             EVP_DigestSign(context, signature, &signature_length,
                            message, message_length) == 1 &&
             signature_length == 64U;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static int sign_grant(lxp_payer_grant *grant, const uint8_t seed[32])
{
    uint8_t message[384];
    uint8_t digest[32];
    size_t message_length;
    size_t signature_length = 64U;
    size_t public_length = 32U;
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                  seed, 32U);
    EVP_MD_CTX *context = EVP_MD_CTX_new();
    int failed = key == NULL || context == NULL ||
        EVP_PKEY_get_raw_public_key(key, grant->public_key,
                                    &public_length) != 1 ||
        lxp_grant_authorization_message(grant, message, sizeof(message),
                                        &message_length) != LXP_OK ||
        lxp_hash_authority(message, message_length, grant->grant_id) != LXP_OK ||
        lxp_hash_domain(LXP_DOMAIN_AUTHORITY_HASH, message, message_length,
                        digest) != LXP_OK ||
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) != 1 ||
        EVP_DigestSign(context, grant->signature, &signature_length,
                       digest, sizeof(digest)) != 1 ||
        signature_length != 64U;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    return failed;
}

static void put_u64(uint8_t *bytes, uint64_t value)
{
    size_t i;
    for (i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> (56U - i * 8U));
}

static void put_u128(uint8_t *bytes, uint64_t high, uint64_t low)
{
    put_u64(bytes, high);
    put_u64(bytes + 8U, low);
}

static uint64_t get_u64(const uint8_t *bytes)
{
    uint64_t value = 0U;
    size_t i;
    for (i = 0U; i < 8U; ++i) value = (value << 8U) | bytes[i];
    return value;
}

static int open_account(const char *name, uint64_t balance,
                        const uint8_t *authority_key, lx_account **account)
{
    uint8_t id[32];
    size_t length = strlen(name);
    if (lx_account_id_from_string((const uint8_t *)name, length, id) != LXP_OK ||
        lx_account_open(&env.accounts, (const uint8_t *)name, length, id, 1U,
                        LX_ACCOUNT_OPEN_CREDIT, NULL, account) != LXP_OK ||
        lxp_ledger_bootstrap_balance(*account, env.asset.asset_id,
                                     (lxp_u128){ 0U, balance }, 0U) != LXP_OK)
        return 1;
    if (authority_key != NULL) {
        (void)memcpy((*account)->authority_key, authority_key, 32U);
        (*account)->has_authority_key = true;
    }
    return lx_account_validate_canonical(*account) == LXP_OK ? 0 : 1;
}

static int env_init(void)
{
    uint8_t owner_key[32];
    uint8_t delegate_key[32];
    uint8_t signature[64];
    (void)memset(&env, 0, sizeof(env));
    env.parameters = 1U;
    env.asset.asset_id[0] = 0x5AU;
    env.asset.asset_id[31] = 0xC3U;
    LXP_CASE_CHECK(sign_raw(owner_seed, env.asset.asset_id, 0U, signature,
        owner_key) == 0);
    LXP_CASE_CHECK(sign_raw(delegate_seed, env.asset.asset_id, 0U, signature,
        delegate_key) == 0);
    LXP_CASE_CHECK(lxp_did_id_derive((const uint8_t *)owner_did, strlen(owner_did),
        env.owner_actor) == LXP_OK);
    LXP_CASE_CHECK(lxp_did_id_derive((const uint8_t *)delegate_did,
        strlen(delegate_did),
        env.delegate_actor) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_transfer_state(&env.asset, &env.asset_state) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&env.accounts) == LXP_OK);
    LXP_CASE_CHECK(open_account("agent:did:key:owner:main", 1000U, owner_key,
        &env.owner) == 0);
    LXP_CASE_CHECK(open_account("agent:did:key:owner:budget:b3", 0U, NULL,
        &env.budget) == 0);
    LXP_CASE_CHECK(open_account("agent:did:key:recv:main", 0U, NULL,
        &env.recipient) == 0);
    LXP_CASE_CHECK(open_account("agent:did:key:dele:main", 0U, delegate_key,
        &env.delegate) == 0);
    LXP_CASE_CHECK(lxp_state_store_init(&env.state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&env.state, &env.accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_require_account_root(&env.state) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&env.kernel, &env.state, &env.journal,
        &env.parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&env.kernel,
        lx_budget_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&env.kernel,
        lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(
        &env.kernel, NULL, lxp_kernel_canonical_ledger_apply) == LXP_OK);
    env.asset_runtime.accounts = &env.accounts;
    env.asset_runtime.assets = &env.asset;
    env.asset_runtime.asset_count = 1U;
    env.asset_runtime.transfer_assets = &env.asset_state;
    env.asset_runtime.transfer_asset_count = 1U;
    env.asset_runtime.network_id = 7U;
    env.asset_runtime.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&env.kernel, LXP_MODULE_ASSET,
        &env.asset_runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&env.kernel, LX_BUDGET_DELEGATE_SPEND,
        0U, &env.registration) == LXP_OK);
    return 0;
}

static int submit(const budget_call *call, lxp_effect_buffer *effects,
                  lxp_result *result)
{
    static uint8_t arena_bytes[262144];
    static uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
    uint8_t payload_copy[RETIRED_PULL_PAYLOAD_BYTES];
    uint8_t name[LX_ACCOUNT_NAME_MAX];
    uint8_t principal[32];
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t digest[32];
    lxp_activity activity;
    lxp_activity received;
    const lxp_module_registration *registration = NULL;
    lxp_authority_resolved authority;
    lxp_module_ctx ctx;
    lxp_arena arena;
    lxp_arena wire_arena;
    lxp_byte_span wire;
    size_t did_length = strlen(call->did);
    size_t length = call->payload_length;
    LXP_CASE_CHECK(length <= sizeof(payload_copy));
    (void)memcpy(payload_copy, call->payload, length);
    LXP_CASE_CHECK(11U + did_length <= sizeof(name));
    (void)memcpy(name, "agent:", 6U);
    (void)memcpy(name + 6U, call->did, did_length);
    (void)memcpy(name + 6U + did_length, ":main", 5U);
    LXP_CASE_CHECK(lx_account_id_from_string(name, 11U + did_length,
        principal) == LXP_OK);
    LXP_CASE_CHECK(sign_raw(call->seed, payload_copy, 0U, signature, public_key) == 0);
    (void)memset(&activity, 0, sizeof(activity));
    activity.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    activity.network_id = 7U;
    activity.activity_type = call->activity_type;
    activity.actor_did.bytes = (const uint8_t *)call->did;
    activity.actor_did.length = did_length;
    activity.authority.bytes = public_key;
    activity.authority.length = 32U;
    activity.signature.bytes = signature;
    activity.signature.length = 64U;
    activity.payload.bytes = payload_copy;
    activity.payload.length = length;
    activity.account_sequence = call->sequence;
    activity.idempotency_key[0] = (uint8_t)(env.global_sequence + 1U);
    activity.idempotency_key[1] = (uint8_t)call->activity_type;
    activity.fee_limit.lo = 1000000U;
    activity.timestamp_bound.not_after = call->timestamp + 1000U;
    LXP_CASE_CHECK(lxp_hash_payload(payload_copy, length,
        activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&activity, digest) == LXP_OK);
    LXP_CASE_CHECK(sign_raw(call->seed, digest, 32U, signature, public_key) == 0);
    (void)memset(&authority, 0, sizeof(authority));
    authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(authority.verified_key, public_key, 32U);
    LXP_CASE_CHECK(lxp_did_id_derive(activity.actor_did.bytes, did_length,
        authority.actor) == LXP_OK);
    (void)memcpy(authority.principal, principal, 32U);
    LXP_CASE_CHECK(lxp_arena_init(&wire_arena, wire_bytes,
        sizeof(wire_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_encode(&activity, &wire_arena, &wire) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_decode(wire.bytes, wire.length, &received) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&received) == LXP_OK);
    *result = lxp_kernel_module_for_activity(&env.kernel,
        received.activity_type, 0U, &registration);
    LXP_CASE_CHECK(lxp_effect_buffer_init(effects) == LXP_OK);
    if (*result != LXP_OK) return 0;
    LXP_CASE_CHECK(registration->module_id == LXP_MODULE_BUDGET);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) == LXP_OK);
    ++env.global_sequence;
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &env.kernel, LXP_MODULE_BUDGET,
        call->timestamp, 0U, env.global_sequence,
        1000000U, &arena, true) == LXP_OK);
    ctx.protocol_version = activity.protocol_version;
    ctx.batch_number = env.global_sequence;
    LXP_CASE_CHECK(lxp_activity_id(wire.bytes, wire.length, ctx.activity_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&ctx, effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_dispatch(registration, &ctx, &received, &authority,
        effects, result) == LXP_OK);
    if (*result == LXP_OK) LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    else lxp_module_ctx_rollback(&ctx);
    return 0;
}

static void capability_key(const uint8_t budget_id[32],
                           const uint8_t holder[32],
                           uint8_t key[CAPABILITY_KEY_BYTES])
{
    (void)memcpy(key, "budget-delegate:", 16U);
    (void)memcpy(key + 16U, budget_id, 32U);
    (void)memcpy(key + 48U, holder, 32U);
}

static int seed_capability(const uint8_t budget_id[32], uint64_t per_spend,
                           uint64_t total)
{
    static uint8_t arena_bytes[4096];
    uint8_t key[CAPABILITY_KEY_BYTES];
    uint8_t bytes[CAPABILITY_BYTES];
    lxp_effect_buffer effects;
    lxp_module_ctx ctx;
    lxp_arena arena;
    (void)memset(bytes, 0, sizeof(bytes));
    bytes[1] = 1U;
    (void)memcpy(bytes + 2U, env.delegate_actor, 32U);
    (void)memcpy(bytes + 34U, env.asset.asset_id, 32U);
    (void)memcpy(bytes + 66U, env.recipient->id, 32U);
    bytes[98] = 0x55U;
    put_u128(bytes + 130U, 0U, per_spend);
    put_u128(bytes + 146U, 0U, total);
    put_u64(bytes + 178U, 1000000U);
    put_u64(bytes + 186U, 1U);
    capability_key(budget_id, env.delegate_actor, key);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_effect_buffer_init(&effects) == LXP_OK);
    ++env.global_sequence;
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &env.kernel, LXP_MODULE_BUDGET, 400U, 0U,
        env.global_sequence, 1000000U, &arena,
        true) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&ctx, &effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_kv_put(&ctx, key, sizeof(key), bytes,
        sizeof(bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    return 0;
}

static int read_consumed(const uint8_t budget_id[32], uint64_t *consumed)
{
    uint8_t key[CAPABILITY_KEY_BYTES];
    uint8_t root[32];
    lxp_byte_span span;
    lxp_state_witness *proof = (lxp_state_witness *)malloc(sizeof(*proof));
    int failed;
    LXP_CASE_CHECK(proof != NULL);
    capability_key(budget_id, env.delegate_actor, key);
    span.bytes = key;
    span.length = sizeof(key);
    failed = lxp_state_root(&env.kernel, root) != LXP_OK ||
             lxp_state_proof_build(&env.kernel, LXP_MODULE_BUDGET, span,
                                   proof) != LXP_OK ||
             lxp_state_proof_verify(proof, root) != LXP_OK ||
             proof->key_length != (uint32_t)sizeof(key) ||
             memcmp(proof->key, key, sizeof(key)) != 0 ||
             proof->value_length != (uint32_t)CAPABILITY_BYTES ||
             memcmp(proof->value + 2U, env.delegate_actor, 32U) != 0 ||
             get_u64(proof->value + 162U) != 0U;
    if (!failed) *consumed = get_u64(proof->value + 170U);
    free(proof);
    LXP_CASE_CHECK(failed == 0);
    return 0;
}

static int expect_spent(const lxp_effect_buffer *effects,
                        const uint8_t budget_id[32], uint64_t amount)
{
    uint8_t body[80];
    (void)memcpy(body, budget_id, 32U);
    (void)memcpy(body + 32U, env.recipient->id, 32U);
    put_u128(body + 64U, 0U, amount);
    LXP_CASE_CHECK(effects->count == 1U);
    LXP_CASE_CHECK(effects->effects[0].module_id == LXP_MODULE_BUDGET);
    LXP_CASE_CHECK(effects->effects[0].kind == LXP_EFFECT_EVENT);
    LXP_CASE_CHECK(effects->effects[0].event_type == 6U);
    LXP_CASE_CHECK((size_t)effects->effects[0].body_length == sizeof(body));
    LXP_CASE_CHECK(memcmp(effects->effects[0].body, body, sizeof(body)) == 0);
    return 0;
}

static int unchanged(const uint8_t before[32])
{
    uint8_t after[32];
    if (lxp_state_root(&env.kernel, after) != LXP_OK) return 0;
    return memcmp(before, after, 32U) == 0;
}

static int owner_setup(const uint8_t budget_id[32], lx_account *account,
                       uint64_t amount)
{
    uint8_t create[LX_BUDGET_CREATE_PAYLOAD_BYTES] = {0U, 1U};
    uint8_t add[LX_BUDGET_DELEGATE_PAYLOAD_BYTES] = {0U, 1U};
    lxp_effect_buffer effects;
    budget_call call;
    lxp_result result = LXP_OK;
    (void)memcpy(create + 2U, budget_id, 32U);
    (void)memcpy(create + 34U, account->id, 32U);
    (void)memcpy(create + 66U, env.asset.asset_id, 32U);
    create[98] = 0x55U;
    put_u128(create + 130U, 0U, 200U);
    put_u128(create + 146U, 0U, 50U);
    put_u128(create + 162U, 0U, amount);
    put_u64(create + 178U, 1000U);
    put_u64(create + 186U, 0U);
    put_u64(create + 194U, 1000000U);
    put_u64(create + 202U, 1U);
    create[210] = (uint8_t)LX_BUDGET_ROLLOVER_CAPPED;
    (void)memcpy(add + 2U, budget_id, 32U);
    (void)memcpy(add + 34U, env.delegate_actor, 32U);
    (void)memset(&call, 0, sizeof(call));
    call.did = owner_did;
    call.seed = owner_seed;
    call.timestamp = 400U;
    call.activity_type = LX_BUDGET_CREATE;
    call.payload = create;
    call.payload_length = sizeof(create);
    call.sequence = env.owner->next_sequence;
    LXP_CASE_CHECK(submit(&call, &effects, &result) == 0 && result == LXP_OK);
    LXP_CASE_CHECK(account->balance.lo == amount);
    call.activity_type = LX_BUDGET_DELEGATE_ADD;
    call.payload = add;
    call.payload_length = sizeof(add);
    call.sequence = env.owner->next_sequence;
    LXP_CASE_CHECK(submit(&call, &effects, &result) == 0 && result == LXP_OK);
    return 0;
}

static int delegate_spend(const uint8_t budget_id[32],
                          const uint8_t recipient[32], uint64_t amount,
                          uint64_t timestamp, lxp_effect_buffer *effects,
                          lxp_result *result)
{
    uint8_t payload[LX_BUDGET_DELEGATE_SPEND_PAYLOAD_BYTES] = {0U, 1U};
    budget_call call;
    (void)memcpy(payload + 2U, budget_id, 32U);
    (void)memcpy(payload + 34U, recipient, 32U);
    put_u128(payload + 66U, 0U, amount);
    payload[82] = 0xA5U;
    payload[113] = (uint8_t)env.global_sequence;
    (void)memset(&call, 0, sizeof(call));
    call.did = delegate_did;
    call.seed = delegate_seed;
    call.timestamp = timestamp;
    call.activity_type = LX_BUDGET_DELEGATE_SPEND;
    call.payload = payload;
    call.payload_length = sizeof(payload);
    call.sequence = env.delegate->next_sequence;
    return submit(&call, effects, result);
}

static int env_close(void)
{
    LXP_CASE_CHECK(lxp_state_store_destroy(&env.state) == LXP_OK);
    lx_account_registry_release(&env.accounts);
    return 0;
}

static int budget_pull_type_refused(void)
{
    const lxp_module_iface *iface = lx_budget_module_iface();
    const lxp_module_registration *registration = NULL;
    uint8_t payload[RETIRED_PULL_PAYLOAD_BYTES] = {0U, 1U};
    uint8_t before[32];
    uint8_t arena_bytes[4096];
    lxp_payer_grant grant = {0};
    lxp_effect_buffer effects;
    lxp_module_ctx ctx;
    lxp_arena arena;
    void *decoded = NULL;
    lxp_result result = LXP_OK;
    size_t length = 0U;
    budget_call call = {0};

    LXP_CASE_CHECK(env_init() == 0);
    LXP_CASE_CHECK(iface->activity_type_count == 10U);
    for (size_t i = 0U; i < iface->activity_type_count; ++i)
        LXP_CASE_CHECK(iface->activity_types[i] != RETIRED_PULL_TYPE);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&env.kernel, RETIRED_PULL_TYPE,
        0U, &registration) == LXP_ERR_UNKNOWN_ACTIVITY);
    LXP_CASE_CHECK(registration == NULL);
    LXP_CASE_CHECK(owner_setup(test_budget_id, env.budget, 30U) == 0);
    LXP_CASE_CHECK(seed_capability(test_budget_id, 10U, 10U) == 0);
    (void)memcpy(payload + 2U, test_budget_id, 32U);
    (void)memcpy(payload + 34U, env.recipient->id, 32U);
    put_u128(payload + 66U, 0U, 10U);
    payload[82] = 0xA5U;
    (void)memcpy(grant.from, env.owner->id, 32U);
    (void)memcpy(grant.recipient, env.recipient->id, 32U);
    (void)memcpy(grant.asset, env.asset.asset_id, 32U);
    grant.per_draw_maximum = (lxp_u128){0U, 10U};
    grant.allowance = (lxp_u128){0U, 30U};
    grant.expiration = 1000000U;
    grant.revocation_sequence = 1U;
    grant.purpose_hash[0] = 0x55U;
    LXP_CASE_CHECK(sign_grant(&grant, owner_seed) == 0);
    LXP_CASE_CHECK(lxp_verify_payer_grant(&grant, env.owner) == LXP_OK);
    LXP_CASE_CHECK(lxp_payer_grant_encode(&grant,
        payload + LX_BUDGET_DELEGATE_SPEND_PAYLOAD_BYTES,
        sizeof(payload) - LX_BUDGET_DELEGATE_SPEND_PAYLOAD_BYTES,
        &length) == LXP_OK);
    LXP_CASE_CHECK(length + LX_BUDGET_DELEGATE_SPEND_PAYLOAD_BYTES == sizeof(payload));
    call.activity_type = RETIRED_PULL_TYPE;
    call.did = delegate_did;
    call.seed = delegate_seed;
    call.payload = payload;
    call.payload_length = sizeof(payload);
    call.sequence = env.delegate->next_sequence;
    call.timestamp = 700U;
    LXP_CASE_CHECK(lxp_state_root(&env.kernel, before) == LXP_OK);
    LXP_CASE_CHECK(submit(&call, &effects, &result) == 0);
    LXP_CASE_CHECK(result == LXP_ERR_UNKNOWN_ACTIVITY);
    LXP_CASE_CHECK(unchanged(before));
    LXP_CASE_CHECK(effects.count == 0U && env.delegate->next_sequence == 0U);
    LXP_CASE_CHECK(env.budget->balance.lo == 30U && env.recipient->balance.lo == 0U);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &env.kernel, LXP_MODULE_BUDGET,
        700U, 0U, env.global_sequence + 1U, 1000000U, &arena, true) == LXP_OK);
    LXP_CASE_CHECK(iface->decode(&ctx, 11U, payload, sizeof(payload),
        &decoded) == LXP_ERR_UNKNOWN_ACTIVITY);
    LXP_CASE_CHECK(decoded == NULL && ctx.staged_count == 0U);
    lxp_module_ctx_rollback(&ctx);
    LXP_CASE_CHECK(env_close() == 0);
    (void)puts("budget_pull_type_refused passed");
    return 0;
}

static int budget_delegate_spend_unchanged(void)
{
    lxp_effect_buffer effects;
    lxp_result result = LXP_OK;
    lx_budget_record record;
    uint8_t key[LX_BUDGET_STATE_KEY_BYTES];
    uint8_t before[32];
    lxp_state_witness *proof = malloc(sizeof(*proof));
    uint64_t consumed = 0U;

    LXP_CASE_CHECK(proof != NULL && env_init() == 0);
    LXP_CASE_CHECK(LX_BUDGET_DELEGATE_SPEND == 0x0003000a);
    LXP_CASE_CHECK(env.registration->activity_types[9] == LX_BUDGET_DELEGATE_SPEND);
    LXP_CASE_CHECK(owner_setup(test_budget_id, env.budget, 30U) == 0);
    LXP_CASE_CHECK(seed_capability(test_budget_id, 10U, 10U) == 0);
    LXP_CASE_CHECK(delegate_spend(test_budget_id, env.recipient->id, 10U, 700U,
        &effects, &result) == 0);
    LXP_CASE_CHECK(result == LXP_OK);
    LXP_CASE_CHECK(read_consumed(test_budget_id, &consumed) == 0 && consumed == 10U);
    LXP_CASE_CHECK(env.recipient->balance.hi == 0U && env.recipient->balance.lo == 10U);
    LXP_CASE_CHECK(env.budget->balance.hi == 0U && env.budget->balance.lo == 20U);
    LXP_CASE_CHECK(env.owner->balance.hi == 0U && env.owner->balance.lo == 970U);
    LXP_CASE_CHECK(env.delegate->next_sequence == 1U);
    LXP_CASE_CHECK(expect_spent(&effects, test_budget_id, 10U) == 0);
    LXP_CASE_CHECK(lx_budget_state_key(test_budget_id, key) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&env.kernel, before) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_proof_build(&env.kernel, LXP_MODULE_BUDGET,
        (lxp_byte_span){key, sizeof(key)}, proof) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_proof_verify(proof, before) == LXP_OK);
    LXP_CASE_CHECK(lx_budget_record_decode(proof->value, proof->value_length,
        &record) == LXP_OK);
    LXP_CASE_CHECK(record.spent_this_period.hi == 0U && record.spent_this_period.lo == 10U);
    LXP_CASE_CHECK(record.delegate_count == 1U &&
        memcmp(record.delegates[0], env.delegate_actor, 32U) == 0);
    LXP_CASE_CHECK(delegate_spend(test_budget_id, env.recipient->id, 1U, 710U,
        &effects, &result) == 0);
    LXP_CASE_CHECK(result == LXP_ERR_UNAUTHORIZED_DELEGATE);
    LXP_CASE_CHECK(unchanged(before) && effects.count == 0U);
    LXP_CASE_CHECK(env.delegate->next_sequence == 1U);
    LXP_CASE_CHECK(read_consumed(test_budget_id, &consumed) == 0 && consumed == 10U);
    LXP_CASE_CHECK(env_close() == 0);
    free(proof);
    (void)puts("budget_delegate_spend_unchanged passed");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"budget_pull_type_refused", budget_pull_type_refused},
        {"budget_delegate_spend_unchanged", budget_delegate_spend_unchanged}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
