#include "lxp_kernel_case.h"
#include "layerx/lx_asset.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_identity.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_protocol.h"

#include <openssl/evp.h>
#include <stdlib.h>
#include <string.h>

static const uint8_t binding_seed[32] = {17U};
static const uint8_t binding_did[] = "did:key:binding";

typedef struct binding_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lx_account *from;
    lx_account *to;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lx_asset_runtime runtime;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_arena arena;
    lxp_activity activity;
    lxp_authority_resolved authority;
    lxp_send send;
    uint64_t parameters;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[512];
    uint8_t arena_bytes[1048576];
    uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
} binding_fixture;

static int binding_sign(const uint8_t *message, size_t length,
                        uint8_t signature[64], uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                 binding_seed, 32U);
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    size_t key_length = 32U;
    size_t signature_length = 64U;
    int ok = key != NULL && ctx != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &key_length) == 1 &&
        key_length == 32U && EVP_DigestSignInit(ctx, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(ctx, signature, &signature_length, message, length) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(ok);
    return 0;
}

static int binding_open(binding_fixture *f)
{
    static const char *names[] = {
        "agent:did:key:binding:main", "agent:did:key:recipient:main",
        "system:paxeer-withdrawals"
    };
    uint8_t scratch[64];
    LXP_CASE_CHECK(binding_sign(binding_seed, sizeof(binding_seed), scratch,
                                f->public_key) == 0);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    LXP_CASE_CHECK(lxp_did_id_derive(binding_did, sizeof(binding_did) - 1U,
                                    f->authority.actor) == LXP_OK);
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    f->asset.asset_id[0] = 1U;
    f->asset.symbol[0] = 'A';
    f->asset.symbol_length = 1U;
    f->asset.name[0] = 'A';
    f->asset.name_length = 1U;
    f->asset.issuer_kind = 2U;
    f->asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    f->asset.custody_reference[0] = 1U;
    f->asset.custody_reference_length = 1U;
    (void)memcpy(f->asset.issuer_did32, f->authority.actor, 32U);
    LXP_CASE_CHECK(lx_asset_transfer_state(&f->asset, &f->transfer_asset) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    for (size_t i = 0U; i < sizeof(names) / sizeof(names[0]); ++i) {
        uint8_t id[32];
        lx_account *account;
        const uint8_t *name = (const uint8_t *)names[i];
        LXP_CASE_CHECK(lx_account_id_from_string(name, strlen(names[i]), id) == LXP_OK);
        LXP_CASE_CHECK(lx_account_open(&f->accounts, name, strlen(names[i]), id,
            1U, i == 2U ? LX_ACCOUNT_OPEN_GENESIS : LX_ACCOUNT_OPEN_CREDIT,
            NULL, &account) == LXP_OK);
        LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(account, f->asset.asset_id,
            (lxp_u128){0U, i == 0U ? 100U : 0U}, 0U) == LXP_OK);
        if (i == 0U) f->from = account;
        if (i == 1U) f->to = account;
    }
    f->from->has_authority_key = true;
    (void)memcpy(f->from->authority_key, f->public_key, 32U);
    (void)memcpy(f->authority.principal, f->from->id, 32U);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                    &f->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
                                    lxp_kernel_canonical_ledger_apply) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    f->runtime.assets = &f->asset;
    f->runtime.asset_count = 1U;
    f->runtime.transfer_assets = &f->transfer_asset;
    f->runtime.transfer_asset_count = 1U;
    f->runtime.network_id = 7U;
    f->runtime.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    LXP_CASE_CHECK(!f->runtime.binding_authority);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET,
                                                 &f->runtime) == LXP_OK);
    return 0;
}

static int binding_payload(binding_fixture *f, uint32_t type, size_t length)
{
    (void)memset(&f->activity, 0, sizeof(f->activity));
    (void)memset(f->signature, 0, sizeof(f->signature));
    f->activity.protocol_version = f->runtime.protocol_version;
    f->activity.network_id = f->runtime.network_id;
    f->activity.activity_type = type;
    f->activity.actor_did = (lxp_byte_span){binding_did, sizeof(binding_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.payload = (lxp_byte_span){f->payload, length};
    f->activity.timestamp_bound.not_after = 100U;
    f->activity.idempotency_key[0] = 1U;
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, length,
                                   f->activity.payload_hash) == LXP_OK);
    return 0;
}

static int binding_send_payload(binding_fixture *f)
{
    size_t length;
    LXP_CASE_CHECK(lxp_send_encode(&f->send, f->payload, sizeof(f->payload),
                                  &length) == LXP_OK);
    return binding_payload(f, LX_ASSET_SEND, length);
}

static int binding_send(binding_fixture *f)
{
    uint8_t material[144];
    (void)memset(&f->send, 0, sizeof(f->send));
    (void)memcpy(f->send.from, f->from->id, 32U);
    (void)memcpy(f->send.to, f->to->id, 32U);
    (void)memcpy(f->send.asset, f->asset.asset_id, 32U);
    f->send.amount.lo = 25U;
    f->send.expires_at = 100U;
    f->send.idempotency_key[0] = 1U;
    f->send.authorization.kind = LXP_AUTH_OWNER;
    f->send.authorization.network_id = f->runtime.network_id;
    f->send.authorization.protocol_version = f->runtime.protocol_version;
    (void)memcpy(f->send.authorization.controller, f->send.from, 32U);
    (void)memcpy(f->send.authorization.public_key, f->public_key, 32U);
    (void)memcpy(material, f->send.from, 32U);
    (void)memcpy(material + 32U, f->send.to, 32U);
    (void)memcpy(material + 64U, f->send.asset, 32U);
    LXP_CASE_CHECK(lxp_u128_to_be(f->send.amount, material + 96U) == LXP_OK);
    (void)memcpy(material + 112U, f->send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_context_value(material, sizeof(material),
                                         f->send.context_hash) == LXP_OK);
    (void)memcpy(f->send.authorization.signed_context_hash, f->send.context_hash, 32U);
    return binding_send_payload(f);
}

static int binding_sign_envelope(binding_fixture *f)
{
    uint8_t digest[32];
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    return binding_sign(digest, sizeof(digest), f->signature, f->public_key);
}

static int binding_invoke(binding_fixture *f, bool execute, lxp_result expected)
{
    const lxp_module_iface *iface = lx_asset_module_iface();
    lxp_arena wire_arena;
    lxp_byte_span wire;
    void *decoded = NULL;
    lxp_result result;
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
                                 sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_effect_buffer_init(&f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_ASSET,
        10U, 0U, 1U, 100000U, &f->arena, true) == LXP_OK);
    f->ctx.protocol_version = f->runtime.protocol_version;
    f->ctx.batch_number = 1U;
    f->ctx.activity = &f->activity;
    LXP_CASE_CHECK(lxp_arena_init(&wire_arena, f->wire_bytes,
                                 sizeof(f->wire_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_encode(&f->activity, &wire_arena, &wire) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_id(wire.bytes, wire.length, f->ctx.activity_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&f->ctx, &f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_ledger_admission(&f->ctx, &f->authority,
                                   f->activity.activity_type) == LXP_OK);
    LXP_CASE_CHECK(iface->decode(&f->ctx,
        lxp_activity_type_ordinal(f->activity.activity_type), f->payload,
        f->activity.payload.length, &decoded) == LXP_OK);
    result = iface->validate(&f->ctx, &f->activity, &f->authority, decoded);
    if (execute && result == LXP_OK)
        result = iface->execute(&f->ctx, &f->activity, &f->authority,
                                 decoded, &f->effects);
    if (iface->release != NULL) iface->release(&f->ctx, decoded);
    if (result != expected)
        (void)fprintf(stderr, "activity %u: result %d, expected %d\n",
                       f->activity.activity_type, (int)result, (int)expected);
    LXP_CASE_CHECK(result == expected);
    return 0;
}

static int binding_close(binding_fixture *f)
{
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int binding_authority_substitutes_signature(void)
{
    binding_fixture *f = calloc(1U, sizeof(*f));
    uint8_t authorization[512];
    uint8_t expected_hash[32];
    size_t length;
    const uint8_t *record_bytes;
    lx_asset_record record;
    uint8_t key[38];
    LXP_CASE_CHECK(f != NULL && binding_open(f) == 0);
    f->runtime.binding_authority = true;
    LXP_CASE_CHECK(binding_send(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_OK) == 0);
    LXP_CASE_CHECK(f->from->balance.lo == 75U && f->to->balance.lo == 25U &&
                    f->from->next_sequence == 1U);
    LXP_CASE_CHECK(f->ctx.ledger_receipt_present && f->effects.count == 1U);
    LXP_CASE_CHECK(lxp_send_authorization_message(&f->send, authorization,
        sizeof(authorization), &length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, authorization,
                                   length, expected_hash) == LXP_OK);
    LXP_CASE_CHECK(memcmp(f->ctx.ledger_receipt.authorization_hash, expected_hash, 32U) == 0);
    LXP_CASE_CHECK(memcmp(f->ctx.ledger_receipt.context_hash, f->send.context_hash, 32U) == 0);

    (void)memset(f->payload, 0, sizeof(f->payload));
    f->payload[1] = 1U;
    (void)memcpy(f->payload + 2U, f->asset.asset_id, 32U);
    LXP_CASE_CHECK(binding_payload(f, LX_ASSET_PAUSE, 34U) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_OK) == 0);
    (void)memcpy(key, "asset:", 6U);
    (void)memcpy(key + 6U, f->asset.asset_id, 32U);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&f->ctx, key, sizeof(key), &record_bytes,
                                  &length) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_record_decode(record_bytes, length, &record) == LXP_OK);
    LXP_CASE_CHECK(record.paused && f->effects.count == 1U);
    LXP_CASE_CHECK(f->ctx.ledger_receipt.supply_binding_version == 1U);
    LXP_CASE_CHECK(binding_close(f) == 0);
    (void)puts("binding_authority_substitutes_signature passed");
    return 0;
}

static int native_runtime_verifies_signature(void)
{
    binding_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL && binding_open(f) == 0);
    LXP_CASE_CHECK(!f->runtime.binding_authority);
    LXP_CASE_CHECK(binding_send(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_BAD_SIGNATURE) == 0);
    f->runtime.binding_authority = true;
    f->activity.payload_hash[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_PAYLOAD_HASH_MISMATCH) == 0);
    LXP_CASE_CHECK(f->from->balance.lo == 100U && f->to->balance.lo == 0U);

    (void)memset(f->payload, 0, sizeof(f->payload));
    f->payload[1] = 1U;
    (void)memcpy(f->payload + 2U, f->asset.asset_id, 32U);
    LXP_CASE_CHECK(binding_payload(f, LX_ASSET_PAUSE, 34U) == 0);
    f->runtime.binding_authority = false;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_BAD_SIGNATURE) == 0);
    f->kernel.module_runtime[LXP_MODULE_ASSET] = NULL;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_ASSET_MISMATCH) == 0);
    f->kernel.module_runtime[LXP_MODULE_ASSET] = &f->runtime;
    LXP_CASE_CHECK(binding_sign_envelope(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_OK) == 0);
    f->runtime.binding_authority = true;
    f->activity.payload_hash[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_PAYLOAD_HASH_MISMATCH) == 0);
    f->activity.payload_hash[0] ^= 1U;
    f->authority.actor[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.actor[0] ^= 1U;
    f->authority.verified_key[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.verified_key[0] ^= 1U;
    f->authority.kind = LXP_AUTHORITY_SESSION_KEY;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.kind = LXP_AUTHORITY_OWNER;

    (void)memset(f->payload, 0, sizeof(f->payload));
    (void)memcpy(f->payload, f->asset.asset_id, 32U);
    f->payload[47] = 25U;
    f->payload[48] = 9U;
    f->payload[68] = 3U;
    LXP_CASE_CHECK(binding_payload(f, LX_ASSET_WITHDRAW, 108U) == 0);
    f->runtime.binding_authority = false;
    LXP_CASE_CHECK(binding_invoke(f, false, LXP_ERR_BAD_SIGNATURE) == 0);
    f->runtime.binding_authority = true;
    LXP_CASE_CHECK(binding_invoke(f, false, LXP_OK) == 0);
    f->activity.payload_hash[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, false, LXP_ERR_PAYLOAD_HASH_MISMATCH) == 0);
    f->activity.payload_hash[0] ^= 1U;
    f->authority.verified_key[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, false, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    LXP_CASE_CHECK(binding_close(f) == 0);
    (void)puts("native_runtime_verifies_signature passed");
    return 0;
}

static int send_environment_copies_binding(void)
{
    binding_fixture *f = calloc(1U, sizeof(*f));
    lxp_send_environment environment = {0};
    uint8_t message[512];
    uint8_t digest[32];
    size_t length;
    LXP_CASE_CHECK(f != NULL && binding_open(f) == 0 && binding_send(f) == 0);
    LXP_CASE_CHECK(binding_sign_envelope(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, false, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->runtime.binding_authority = true;
    LXP_CASE_CHECK(binding_invoke(f, false, LXP_OK) == 0);
    environment.accounts = &f->accounts;
    environment.network_id = f->runtime.network_id;
    environment.protocol_version = f->runtime.protocol_version;
    environment.batch_timestamp = 10U;
    LXP_CASE_CHECK(!environment.binding_authority);
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_UNAUTHORIZED_DEBIT);
    environment.binding_authority = true;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_OK);
    f->send.authorization.controller[0] ^= 1U;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_UNAUTHORIZED_DEBIT);
    f->send.authorization.controller[0] ^= 1U;
    f->send.authorization.public_key[0] ^= 1U;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_UNAUTHORIZED_DEBIT);
    f->send.authorization.public_key[0] ^= 1U;
    ++environment.network_id;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_UNAUTHORIZED_DEBIT);
    --environment.network_id;
    ++environment.protocol_version;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_UNAUTHORIZED_DEBIT);
    --environment.protocol_version;
    ++f->send.sequence;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_SEQUENCE_MISMATCH);
    --f->send.sequence;
    environment.batch_timestamp = 101U;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_EXPIRED);
    environment.batch_timestamp = 10U;
    f->send.condition_count = 1U;
    f->send.conditions[0] = (lxp_send_condition){LXP_CONDITION_NOT_BEFORE, 11U};
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_CONDITION_UNMET);
    f->send.condition_count = 0U;
    f->send.authorization.signed_context_hash[0] ^= 1U;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_CONTEXT_MISMATCH);
    f->send.authorization.signed_context_hash[0] ^= 1U;
    f->send.authorization.kind = 0U;
    LXP_CASE_CHECK(lxp_send_validate(&f->send, &environment) == LXP_ERR_UNKNOWN_AUTHORITY_KIND);
    f->send.authorization.kind = LXP_AUTH_OWNER;
    f->send.context_hash[0] ^= 1U;
    f->send.authorization.signed_context_hash[0] ^= 1U;
    LXP_CASE_CHECK(binding_send_payload(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, false, LXP_ERR_CONTEXT_MISMATCH) == 0);
    f->send.context_hash[0] ^= 1U;
    f->send.authorization.signed_context_hash[0] ^= 1U;

    LXP_CASE_CHECK(lxp_send_authorization_message(&f->send, message,
        sizeof(message), &length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message,
                                   length, digest) == LXP_OK);
    LXP_CASE_CHECK(binding_sign(digest, sizeof(digest),
        f->send.authorization.signature, f->public_key) == 0);
    LXP_CASE_CHECK(binding_send_payload(f) == 0 && binding_sign_envelope(f) == 0);
    f->runtime.binding_authority = false;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_OK) == 0);
    LXP_CASE_CHECK(binding_close(f) == 0);
    (void)puts("send_environment_copies_binding passed");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"binding_authority_substitutes_signature", binding_authority_substitutes_signature},
        {"native_runtime_verifies_signature", native_runtime_verifies_signature},
        {"send_environment_copies_binding", send_environment_copies_binding}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
