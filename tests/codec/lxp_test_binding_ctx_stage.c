#include "lxp_kernel_case.h"
#include "layerx/lx_asset.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_identity.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_protocol.h"

#include <openssl/evp.h>
#include <stdlib.h>
#include <string.h>

static const uint8_t stage_did[] = "did:key:binding";

typedef struct stage_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lx_asset_runtime runtime;
    lxp_module_ctx ctx;
    lxp_arena arena;
    lxp_activity activity;
    lxp_authority_resolved authority;
    uint64_t parameters;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[89];
    uint8_t arena_bytes[1048576];
    uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
} stage_fixture;

static int stage_open(stage_fixture *f, uint32_t type)
{
    const uint8_t seed[32] = {17U};
    uint8_t digest[32];
    lxp_arena wire_arena;
    lxp_byte_span wire;
    size_t key_length = sizeof(f->public_key);
    size_t signature_length = sizeof(f->signature);
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                 seed, sizeof(seed));
    EVP_MD_CTX *signer = EVP_MD_CTX_new();
    int signed_ok;

    LXP_CASE_CHECK(key != NULL && signer != NULL);
    LXP_CASE_CHECK(EVP_PKEY_get_raw_public_key(key, f->public_key,
                                              &key_length) == 1);
    LXP_CASE_CHECK(key_length == sizeof(f->public_key));
    f->payload[1] = 1U;
    f->payload[2] = 1U;
    if (type == LX_ASSET_REGISTER) {
        f->payload[66] = 1U;
        f->payload[67] = 'A';
        f->payload[68] = 1U;
        f->payload[69] = 'A';
        f->payload[86] = 100U;
        f->payload[87] = 1U;
    }
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    f->activity.network_id = 7U;
    f->activity.activity_type = type;
    f->activity.actor_did = (lxp_byte_span){stage_did, sizeof(stage_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.payload = (lxp_byte_span){f->payload,
        type == LX_ASSET_REGISTER ? sizeof(f->payload) : 34U};
    f->activity.timestamp_bound.not_after = 100U;
    f->activity.idempotency_key[0] = 1U;
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, f->activity.payload.length,
                                    f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    signed_ok = EVP_DigestSignInit(signer, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(signer, f->signature, &signature_length,
                       digest, sizeof(digest)) == 1;
    EVP_MD_CTX_free(signer);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(signed_ok && signature_length == sizeof(f->signature));
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    f->signature[0] ^= 1U;
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_ERR_BAD_SIGNATURE);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    LXP_CASE_CHECK(lxp_did_id_derive(stage_did, sizeof(stage_did) - 1U,
                                     f->authority.actor) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                     &f->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                              lx_asset_module_iface()) == LXP_OK);
    f->asset.asset_id[0] = 2U;
    f->asset.symbol[0] = 'A';
    f->asset.symbol_length = 1U;
    f->asset.name[0] = 'A';
    f->asset.name_length = 1U;
    f->asset.issuer_kind = 1U;
    f->asset.custody_kind = LX_ASSET_CUSTODY_NATIVE;
    (void)memcpy(f->asset.issuer_did32, f->authority.actor, 32U);
    LXP_CASE_CHECK(lx_asset_transfer_state(&f->asset, &f->transfer_asset) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    f->runtime.assets = &f->asset;
    f->runtime.asset_count = 1U;
    f->runtime.transfer_assets = &f->transfer_asset;
    f->runtime.transfer_asset_count = 1U;
    f->runtime.network_id = f->activity.network_id;
    f->runtime.protocol_version = f->activity.protocol_version;
    LXP_CASE_CHECK(!f->runtime.binding_authority);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET,
                                                  &f->runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_open(&f->state, 1U, &f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
                                  sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_ASSET,
        10U, 0U, 1U, 100000U, &f->arena, true) == LXP_OK);
    f->ctx.protocol_version = f->activity.protocol_version;
    f->ctx.activity = &f->activity;
    LXP_CASE_CHECK(lxp_arena_init(&wire_arena, f->wire_bytes,
                                  sizeof(f->wire_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_encode(&f->activity, &wire_arena, &wire) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_id(wire.bytes, wire.length,
                                   f->ctx.activity_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_ledger_admission(&f->ctx, &f->authority,
                                                   type) == LXP_OK);
    return 0;
}

static int stage_expect(stage_fixture *f, lxp_result expected)
{
    lx_account *account = NULL;
    lxp_result result = f->activity.activity_type == LX_ASSET_REGISTER ?
        lxp_ctx_asset_issuance_stage(&f->ctx, &f->activity, &f->authority, &account) :
        lxp_ctx_asset_account_stage(&f->ctx, &f->activity, &f->authority, &account);
    if (result != expected)
        (void)fprintf(stderr, "stage result %d, expected %d\n", (int)result, (int)expected);
    LXP_CASE_CHECK(result == expected);
    LXP_CASE_CHECK(f->accounts.count == 0U);
    if (expected == LXP_OK) {
        LXP_CASE_CHECK(f->ctx.staged_account_count == 1U);
        LXP_CASE_CHECK(account == &f->ctx.staged_accounts[0].account);
        LXP_CASE_CHECK(account->has_asset && memcmp(account->asset_id,
                                                   f->payload + 2U, 32U) == 0);
        LXP_CASE_CHECK(account->created_at_sequence == f->ctx.global_sequence);
        LXP_CASE_CHECK(lx_account_validate_canonical(account) == LXP_OK);
        if (f->activity.activity_type == LX_ASSET_REGISTER) {
            LXP_CASE_CHECK(account->kind == LX_ACCOUNT_MODULE_VALUE);
            LXP_CASE_CHECK(account->balance.hi == 0U && account->balance.lo == 100U);
        } else {
            LXP_CASE_CHECK(account->kind == LX_ACCOUNT_AGENT_ASSET);
            LXP_CASE_CHECK(account->has_authority_key && memcmp(account->authority_key,
                                                   f->public_key, 32U) == 0);
            LXP_CASE_CHECK(lxp_u128_is_zero(account->balance));
        }
    } else {
        LXP_CASE_CHECK(account == NULL && f->ctx.staged_account_count == 0U);
    }
    return 0;
}

static int stage_other_checks(stage_fixture *f)
{
    f->authority.kind = LXP_AUTHORITY_SESSION_KEY;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    --f->activity.authority.length;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    ++f->activity.authority.length;
    f->authority.verified_key[0] ^= 1U;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.verified_key[0] ^= 1U;
    f->activity.payload_hash[0] ^= 1U;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_PAYLOAD_HASH_MISMATCH) == 0);
    f->activity.payload_hash[0] ^= 1U;
    f->payload[1] = 2U;
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, f->activity.payload.length,
                                    f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_NON_CANONICAL) == 0);
    f->payload[1] = 1U;
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, f->activity.payload.length,
                                    f->activity.payload_hash) == LXP_OK);
    return 0;
}

static int stage_signature_required(stage_fixture *f)
{
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_BAD_SIGNATURE) == 0);
    f->kernel.module_runtime[LXP_MODULE_ASSET] = NULL;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_BAD_SIGNATURE) == 0);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET,
                                                  &f->runtime) == LXP_OK);
    f->runtime.binding_authority = true;
    return 0;
}

static int stage_close(stage_fixture *f)
{
    lxp_module_ctx_rollback(&f->ctx);
    f->runtime.binding_authority = false;
    f->signature[0] ^= 1U;
    LXP_CASE_CHECK(stage_expect(f, LXP_OK) == 0);
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(lxp_state_journal_rollback(&f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int issuance_stage_bound_skips_signature(void)
{
    stage_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL && stage_open(f, LX_ASSET_REGISTER) == 0);
    LXP_CASE_CHECK(stage_signature_required(f) == 0);
    LXP_CASE_CHECK(stage_other_checks(f) == 0);
    LXP_CASE_CHECK(stage_expect(f, LXP_OK) == 0);
    LXP_CASE_CHECK(stage_close(f) == 0);
    (void)puts("issuance_stage_bound_skips_signature passed");
    return 0;
}

static int account_stage_bound_keeps_other_checks(void)
{
    uint8_t long_did[LX_ACCOUNT_NAME_MAX];
    stage_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL && stage_open(f, LX_ASSET_ACCOUNT_OPEN) == 0);
    LXP_CASE_CHECK(stage_signature_required(f) == 0);
    LXP_CASE_CHECK(stage_other_checks(f) == 0);
    (void)memset(long_did, 'a', sizeof(long_did));
    f->activity.actor_did = (lxp_byte_span){long_did, LX_ACCOUNT_NAME_MAX - 76U};
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->activity.actor_did = (lxp_byte_span){stage_did, 0U};
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->activity.actor_did = (lxp_byte_span){NULL, sizeof(stage_did) - 1U};
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->activity.actor_did = (lxp_byte_span){stage_did, sizeof(stage_did) - 1U};
    LXP_CASE_CHECK(stage_expect(f, LXP_OK) == 0);
    LXP_CASE_CHECK(stage_close(f) == 0);
    (void)puts("account_stage_bound_keeps_other_checks passed");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"issuance_stage_bound_skips_signature", issuance_stage_bound_skips_signature},
        {"account_stage_bound_keeps_other_checks", account_stage_bound_keeps_other_checks}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
