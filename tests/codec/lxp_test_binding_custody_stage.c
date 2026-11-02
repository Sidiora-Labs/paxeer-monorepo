#include "lxp_kernel_case.h"
#include "layerx/lx_asset.h"
#include "layerx/lx_escrow.h"
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
    lx_escrow_runtime escrow_runtime;
    lxp_module_ctx ctx;
    lxp_arena arena;
    lxp_activity activity;
    uint64_t parameters;
    uint8_t object_id[32];
    uint8_t account_id[32];
    uint8_t account_name[LX_ACCOUNT_NAME_MAX];
    size_t account_name_length;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[LX_ESCROW_OPEN_PAYLOAD_BYTES];
    uint8_t arena_bytes[1048576];
    uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
} stage_fixture;

static int stage_open(stage_fixture *f)
{
    static const uint8_t prefix[] = "agent:did:key:binding:escrow:";
    static const uint8_t hex[] = "0123456789abcdef";
    const uint8_t seed[32] = {17U};
    lxp_arena wire_arena;
    lxp_byte_span wire;
    size_t key_length = sizeof(f->public_key);
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                 seed, sizeof(seed));

    LXP_CASE_CHECK(key != NULL);
    LXP_CASE_CHECK(EVP_PKEY_get_raw_public_key(key, f->public_key,
                                              &key_length) == 1);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(key_length == sizeof(f->public_key));
    f->object_id[0] = 1U;
    f->asset.asset_id[0] = 2U;
    (void)memcpy(f->account_name, prefix, sizeof(prefix) - 1U);
    f->account_name_length = sizeof(prefix) - 1U;
    for (size_t i = 0U; i < 32U; ++i) {
        f->account_name[f->account_name_length++] = hex[f->object_id[i] >> 4U];
        f->account_name[f->account_name_length++] = hex[f->object_id[i] & 15U];
    }
    LXP_CASE_CHECK(lx_account_id_from_string(f->account_name,
        f->account_name_length, f->account_id) == LXP_OK);
    (void)memcpy(f->payload, f->object_id, 32U);
    f->payload[32] = 3U;
    (void)memcpy(f->payload + 64U, f->account_id, 32U);
    f->payload[96] = 4U;
    f->payload[128] = 5U;
    (void)memcpy(f->payload + 160U, f->asset.asset_id, 32U);
    f->payload[207] = 100U;
    f->payload[215] = 100U;
    f->payload[223] = 10U;
    f->payload[224] = 1U;
    f->payload[256] = 1U;
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->activity.network_id = 7U;
    f->activity.activity_type = LX_ESCROW_OPEN;
    f->activity.actor_did = (lxp_byte_span){stage_did, sizeof(stage_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, sizeof(f->public_key)};
    f->activity.signature = (lxp_byte_span){f->signature, sizeof(f->signature)};
    f->activity.payload = (lxp_byte_span){f->payload, sizeof(f->payload)};
    f->activity.timestamp_bound.not_after = 100U;
    f->activity.idempotency_key[0] = 1U;
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, sizeof(f->payload),
                                    f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_ERR_BAD_SIGNATURE);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                     &f->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                              lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                              lx_escrow_module_iface()) == LXP_OK);
    f->asset.symbol[0] = 'A';
    f->asset.symbol_length = 1U;
    f->asset.name[0] = 'A';
    f->asset.name_length = 1U;
    f->asset.issuer_kind = 1U;
    f->asset.custody_kind = LX_ASSET_CUSTODY_NATIVE;
    LXP_CASE_CHECK(lxp_did_id_derive(stage_did, sizeof(stage_did) - 1U,
                                     f->asset.issuer_did32) == LXP_OK);
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
    f->escrow_runtime.accounts = &f->accounts;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ESCROW,
                                                  &f->escrow_runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_open(&f->state, 1U, &f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
                                  sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_ESCROW,
        10U, 0U, 1U, 100000U, &f->arena, true) == LXP_OK);
    f->ctx.protocol_version = f->activity.protocol_version;
    f->ctx.activity = &f->activity;
    LXP_CASE_CHECK(lxp_arena_init(&wire_arena, f->wire_bytes,
                                  sizeof(f->wire_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_encode(&f->activity, &wire_arena, &wire) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_id(wire.bytes, wire.length,
                                   f->ctx.activity_id) == LXP_OK);
    return 0;
}

static int stage_expect(stage_fixture *f, lxp_result expected)
{
    lx_account *account = NULL;
    size_t mark = lxp_arena_mark(&f->arena);
    lxp_result result = lxp_ctx_account_stage_module_custody(&f->ctx,
        &f->activity, f->object_id, f->asset.asset_id, f->account_id, &account);
    if (result != expected)
        (void)fprintf(stderr, "stage result %d, expected %d\n", (int)result, (int)expected);
    LXP_CASE_CHECK(result == expected);
    LXP_CASE_CHECK(lxp_arena_mark(&f->arena) == mark);
    LXP_CASE_CHECK(f->accounts.count == 0U);
    if (expected == LXP_OK) {
        LXP_CASE_CHECK(f->ctx.staged_account_count == 1U);
        LXP_CASE_CHECK(account == &f->ctx.staged_accounts[0].account);
        LXP_CASE_CHECK(account->kind == LX_ACCOUNT_AGENT_ESCROW);
        LXP_CASE_CHECK(account->name_length == f->account_name_length);
        LXP_CASE_CHECK(memcmp(account->name, f->account_name,
                              f->account_name_length) == 0);
        LXP_CASE_CHECK(memcmp(account->id, f->account_id, 32U) == 0);
        LXP_CASE_CHECK(account->has_asset &&
            memcmp(account->asset_id, f->asset.asset_id, 32U) == 0);
        LXP_CASE_CHECK(!account->has_authority_key && lxp_u128_is_zero(account->balance));
        LXP_CASE_CHECK(account->created_at_sequence == f->ctx.global_sequence);
        LXP_CASE_CHECK(lx_account_validate_canonical(account) == LXP_OK);
    } else {
        LXP_CASE_CHECK(account == NULL && f->ctx.staged_account_count == 0U);
    }
    return 0;
}

static int stage_close(stage_fixture *f)
{
    lxp_module_ctx_rollback(&f->ctx);
    LXP_CASE_CHECK(lxp_state_journal_rollback(&f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int custody_stage_bound_skips_signature(void)
{
    uint8_t long_did[LX_ACCOUNT_NAME_MAX];
    stage_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL && stage_open(f) == 0);
    f->runtime.binding_authority = true;
    f->activity.payload_hash[0] ^= 1U;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_PAYLOAD_HASH_MISMATCH) == 0);
    f->activity.payload_hash[0] ^= 1U;
    f->ctx.activity_id[0] ^= 1U;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_CONTEXT_MISMATCH) == 0);
    f->ctx.activity_id[0] ^= 1U;
    ++f->activity.account_sequence;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_CONTEXT_MISMATCH) == 0);
    --f->activity.account_sequence;
    f->ctx.module_id = LXP_MODULE_ASSET;
    f->activity.activity_type = LX_ASSET_REGISTER;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->ctx.module_id = LXP_MODULE_ESCROW;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->activity.activity_type = LX_ESCROW_OPEN;
    (void)memset(long_did, 'a', sizeof(long_did));
    f->activity.actor_did = (lxp_byte_span){long_did, LX_ACCOUNT_NAME_MAX - 77U};
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->activity.actor_did = (lxp_byte_span){stage_did, 0U};
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->activity.actor_did = (lxp_byte_span){NULL, sizeof(stage_did) - 1U};
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->activity.actor_did = (lxp_byte_span){stage_did, sizeof(stage_did) - 1U};
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->ctx.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->ctx.mutable = false;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->ctx.mutable = true;
    LXP_CASE_CHECK(stage_expect(f, LXP_OK) == 0);
    LXP_CASE_CHECK(stage_close(f) == 0);
    (void)puts("custody_stage_bound_skips_signature passed");
    return 0;
}

static int custody_stage_unbound_verifies_signature(void)
{
    stage_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL && stage_open(f) == 0);
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_BAD_SIGNATURE) == 0);
    f->kernel.module_runtime[LXP_MODULE_ASSET] = NULL;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_BAD_SIGNATURE) == 0);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET,
                                                  &f->runtime) == LXP_OK);
    f->runtime.binding_authority = true;
    LXP_CASE_CHECK(stage_expect(f, LXP_OK) == 0);
    lxp_module_ctx_rollback(&f->ctx);
    f->runtime.binding_authority = false;
    LXP_CASE_CHECK(stage_expect(f, LXP_ERR_BAD_SIGNATURE) == 0);
    LXP_CASE_CHECK(stage_close(f) == 0);
    (void)puts("custody_stage_unbound_verifies_signature passed");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"custody_stage_bound_skips_signature", custody_stage_bound_skips_signature},
        {"custody_stage_unbound_verifies_signature", custody_stage_unbound_verifies_signature}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
