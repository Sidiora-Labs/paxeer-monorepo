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
    lxp_receive receive;
    lxp_payer_grant grant;
    uint64_t parameters;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[1024];
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
        "agent:did:key:payer:main", "agent:did:key:binding:main",
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
    f->to->has_authority_key = true;
    (void)memcpy(f->to->authority_key, f->public_key, 32U);
    (void)memcpy(f->from->authority_key, f->public_key, 32U);
    (void)memcpy(f->authority.principal, f->to->id, 32U);
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

static int binding_receive_payload(binding_fixture *f)
{
    size_t length;
    LXP_CASE_CHECK(lxp_receive_encode(&f->receive, f->payload, sizeof(f->payload),
                                     &length) == LXP_OK);
    return binding_payload(f, LX_ASSET_RECEIVE, length);
}

static int binding_receive_open(binding_fixture *f)
{
    static const uint8_t payer_did[] = "did:key:payer";
    uint8_t message[384];
    uint8_t digest[32];
    size_t length;
    LXP_CASE_CHECK(binding_open(f) == 0);
    (void)memcpy(f->grant.from, f->from->id, 32U);
    (void)memcpy(f->grant.recipient, f->to->id, 32U);
    (void)memcpy(f->grant.asset, f->asset.asset_id, 32U);
    (void)memcpy(f->grant.public_key, f->public_key, 32U);
    f->grant.per_draw_maximum.lo = 30U;
    f->grant.allowance.lo = 50U;
    f->grant.expiration = 100U;
    f->grant.purpose_hash[0] = 8U;
    f->grant.revocation_sequence = 5U;
    LXP_CASE_CHECK(lxp_grant_authorization_message(&f->grant, message,
        sizeof(message), &length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_authority(message, length, f->grant.grant_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_AUTHORITY_HASH, message,
                                   length, digest) == LXP_OK);
    LXP_CASE_CHECK(binding_sign(digest, sizeof(digest), f->grant.signature,
                                f->public_key) == 0);
    LXP_CASE_CHECK(lxp_payer_grant_encode(&f->grant, f->payload,
                                         sizeof(f->payload), &length) == LXP_OK);
    LXP_CASE_CHECK(binding_payload(f, LX_ASSET_GRANT_ISSUE, length) == 0);
    f->activity.actor_did = (lxp_byte_span){payer_did, sizeof(payer_did) - 1U};
    LXP_CASE_CHECK(lxp_did_id_derive(payer_did, sizeof(payer_did) - 1U,
                                    f->authority.actor) == LXP_OK);
    (void)memcpy(f->authority.principal, f->from->id, 32U);
    LXP_CASE_CHECK(binding_sign_envelope(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_OK) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_did_id_derive(binding_did, sizeof(binding_did) - 1U,
                                    f->authority.actor) == LXP_OK);
    (void)memcpy(f->authority.principal, f->to->id, 32U);
    (void)memcpy(f->receive.from, f->from->id, 32U);
    (void)memcpy(f->receive.to, f->to->id, 32U);
    (void)memcpy(f->receive.asset, f->asset.asset_id, 32U);
    (void)memcpy(f->receive.grant_id, f->grant.grant_id, 32U);
    f->receive.amount.lo = 25U;
    f->receive.idempotency_key[0] = 1U;
    f->receive.payer_grant = f->grant;
    LXP_CASE_CHECK(lxp_hash_context_value(f->grant.purpose_hash, 32U,
                                         f->receive.context_hash) == LXP_OK);
    f->receive.receiver_authorization.kind = LXP_AUTH_OWNER;
    f->receive.receiver_authorization.network_id = f->runtime.network_id;
    f->receive.receiver_authorization.protocol_version = f->runtime.protocol_version;
    (void)memcpy(f->receive.receiver_authorization.controller, f->to->id, 32U);
    (void)memcpy(f->receive.receiver_authorization.public_key, f->public_key, 32U);
    (void)memcpy(f->receive.receiver_authorization.signed_context_hash,
                 f->receive.context_hash, 32U);
    return binding_receive_payload(f);
}

static int binding_receive_applied(binding_fixture *f)
{
    lxp_grant_state grant;
    uint8_t message[512];
    uint8_t digest[32];
    size_t length;
    LXP_CASE_CHECK(f->from->balance.hi == 0U && f->from->balance.lo == 75U &&
                    f->to->balance.hi == 0U && f->to->balance.lo == 25U &&
                    f->from->next_sequence == 0U && f->to->next_sequence == 1U);
    LXP_CASE_CHECK(f->ctx.ledger_receipt_present && f->effects.count == 1U);
    LXP_CASE_CHECK(lxp_receive_authorization_message(&f->receive, message,
        sizeof(message), &length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message,
                                   length, digest) == LXP_OK);
    LXP_CASE_CHECK(memcmp(f->ctx.ledger_receipt.authorization_hash, digest, 32U) == 0);
    LXP_CASE_CHECK(memcmp(f->ctx.ledger_receipt.context_hash,
                          f->receive.context_hash, 32U) == 0);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_payer_grant_read(&f->kernel, f->grant.grant_id,
                                            &grant) == LXP_OK);
    LXP_CASE_CHECK(grant.drawn_total.hi == 0U && grant.drawn_total.lo == 25U);
    return 0;
}

static int binding_authority_substitutes_receiver_signature(void)
{
    binding_fixture *f = calloc(1U, sizeof(*f));
    uint8_t zero_signature[64] = {0};
    LXP_CASE_CHECK(f != NULL && binding_receive_open(f) == 0);
    f->runtime.binding_authority = true;
    LXP_CASE_CHECK(memcmp(f->signature, zero_signature, 64U) == 0);
    LXP_CASE_CHECK(memcmp(f->receive.receiver_authorization.signature,
                          zero_signature, 64U) == 0);
    f->activity.payload_hash[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_PAYLOAD_HASH_MISMATCH) == 0);
    f->activity.payload_hash[0] ^= 1U;
    f->authority.actor[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.actor[0] ^= 1U;
    f->authority.verified_key[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.verified_key[0] ^= 1U;
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_OK) == 0);
    LXP_CASE_CHECK(binding_receive_applied(f) == 0);
    LXP_CASE_CHECK(binding_close(f) == 0);
    (void)puts("binding_authority_substitutes_receiver_signature passed");
    return 0;
}

static int native_receive_verifies_receiver_signature(void)
{
    binding_fixture *f = calloc(1U, sizeof(*f));
    uint8_t message[512];
    uint8_t digest[32];
    size_t length;
    LXP_CASE_CHECK(f != NULL && binding_receive_open(f) == 0);
    LXP_CASE_CHECK(!f->runtime.binding_authority);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_BAD_SIGNATURE) == 0);
    LXP_CASE_CHECK(binding_sign_envelope(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    LXP_CASE_CHECK(f->from->balance.lo == 100U && f->to->balance.lo == 0U &&
                    f->to->next_sequence == 0U);
    LXP_CASE_CHECK(lxp_receive_authorization_message(&f->receive, message,
        sizeof(message), &length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message,
                                   length, digest) == LXP_OK);
    LXP_CASE_CHECK(binding_sign(digest, sizeof(digest),
        f->receive.receiver_authorization.signature, f->public_key) == 0);
    f->receive.receiver_authorization.signature[0] ^= 1U;
    LXP_CASE_CHECK(binding_receive_payload(f) == 0 && binding_sign_envelope(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->receive.receiver_authorization.signature[0] ^= 1U;
    LXP_CASE_CHECK(binding_receive_payload(f) == 0 && binding_sign_envelope(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_OK) == 0);
    LXP_CASE_CHECK(binding_receive_applied(f) == 0);
    LXP_CASE_CHECK(binding_close(f) == 0);
    (void)puts("native_receive_verifies_receiver_signature passed");
    return 0;
}

static int receive_environment_copies_binding(void)
{
    binding_fixture *f = calloc(1U, sizeof(*f));
    lxp_receive_environment environment = {0};
    lxp_grant_store *grants = calloc(1U, sizeof(*grants));
    lxp_send_store *idempotency = calloc(1U, sizeof(*idempotency));
    lxp_send_receipt_projection receipt;
    LXP_CASE_CHECK(f != NULL && grants != NULL && idempotency != NULL &&
                    binding_receive_open(f) == 0);
    LXP_CASE_CHECK(binding_sign_envelope(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->runtime.binding_authority = true;
    environment.accounts = &f->accounts;
    environment.assets = &f->transfer_asset;
    environment.asset_count = 1U;
    environment.grants = grants;
    environment.idempotency = idempotency;
    environment.batch_timestamp = 10U;
    environment.global_sequence = 1U;
    environment.network_id = f->runtime.network_id;
    environment.protocol_version = f->runtime.protocol_version;
    LXP_CASE_CHECK(lxp_grant_store_put(grants, &f->grant, f->from) == LXP_OK);
    LXP_CASE_CHECK(!environment.binding_authority);
    LXP_CASE_CHECK(lxp_receive_execute(&f->receive, &environment, &receipt) ==
                    LXP_ERR_UNAUTHORIZED_DEBIT);
    environment.binding_authority = true;
    f->receive.receiver_authorization.controller[0] ^= 1U;
    LXP_CASE_CHECK(binding_receive_payload(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->receive.receiver_authorization.controller[0] ^= 1U;
    f->to->authority_key[0] ^= 1U;
    LXP_CASE_CHECK(binding_receive_payload(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->to->authority_key[0] ^= 1U;
    f->receive.receiver_authorization.public_key[0] ^= 1U;
    LXP_CASE_CHECK(lxp_receive_execute(&f->receive, &environment, &receipt) ==
                    LXP_ERR_UNAUTHORIZED_DEBIT);
    f->receive.receiver_authorization.public_key[0] ^= 1U;
    f->to->has_authority_key = false;
    LXP_CASE_CHECK(lxp_receive_execute(&f->receive, &environment, &receipt) ==
                    LXP_ERR_UNAUTHORIZED_DEBIT);
    f->to->has_authority_key = true;
    ++f->receive.receiver_authorization.network_id;
    LXP_CASE_CHECK(binding_receive_payload(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    --f->receive.receiver_authorization.network_id;
    ++f->receive.receiver_authorization.protocol_version;
    LXP_CASE_CHECK(binding_receive_payload(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    --f->receive.receiver_authorization.protocol_version;
    ++f->receive.receiver_sequence;
    LXP_CASE_CHECK(binding_receive_payload(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_SEQUENCE_MISMATCH) == 0);
    --f->receive.receiver_sequence;
    f->receive.receiver_authorization.signed_context_hash[0] ^= 1U;
    LXP_CASE_CHECK(binding_receive_payload(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_ERR_CONTEXT_MISMATCH) == 0);
    f->receive.receiver_authorization.signed_context_hash[0] ^= 1U;
    f->receive.receiver_authorization.kind = 0U;
    LXP_CASE_CHECK(lxp_receive_execute(&f->receive, &environment, &receipt) ==
                    LXP_ERR_UNKNOWN_AUTHORITY_KIND);
    f->receive.receiver_authorization.kind = LXP_AUTH_OWNER;
    grants->grants[0].grant.signature[0] ^= 1U;
    LXP_CASE_CHECK(lxp_receive_execute(&f->receive, &environment, &receipt) ==
                    LXP_ERR_BAD_SIGNATURE);
    grants->grants[0].grant.signature[0] ^= 1U;
    LXP_CASE_CHECK(f->from->balance.lo == 100U && f->to->balance.lo == 0U &&
                    f->to->next_sequence == 0U && grants->grants[0].drawn_total.lo == 0U);
    LXP_CASE_CHECK(binding_receive_payload(f) == 0);
    LXP_CASE_CHECK(binding_invoke(f, true, LXP_OK) == 0);
    LXP_CASE_CHECK(binding_receive_applied(f) == 0);
    lxp_send_store_release(idempotency);
    free(idempotency);
    free(grants);
    LXP_CASE_CHECK(binding_close(f) == 0);
    (void)puts("receive_environment_copies_binding passed");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"binding_authority_substitutes_receiver_signature",
         binding_authority_substitutes_receiver_signature},
        {"native_receive_verifies_receiver_signature",
         native_receive_verifies_receiver_signature},
        {"receive_environment_copies_binding", receive_environment_copies_binding}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
