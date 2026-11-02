#include "lxp_kernel_case.h"
#include "layerx/lx_asset.h"
#include "layerx/lx_budget.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_identity.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_protocol.h"

#include <openssl/evp.h>
#include <stdlib.h>
#include <string.h>

static const uint8_t signer_did[] = "did:key:binding";

typedef struct signer_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lx_account *owner;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lx_asset_runtime runtime;
    const lxp_module_registration *registration;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_arena arena;
    lxp_activity activity;
    lxp_authority_resolved authority;
    uint64_t parameters;
    uint8_t budget_id[32];
    uint8_t account_id[32];
    uint8_t account_name[LX_ACCOUNT_NAME_MAX];
    size_t account_name_length;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[LX_BUDGET_CREATE_PAYLOAD_BYTES];
    uint8_t arena_bytes[2U * LXP_MAX_ACTIVITY_BYTES];
    uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
} signer_fixture;

static int signer_open(signer_fixture *f)
{
    static const uint8_t owner_name[] = "agent:did:key:binding:main";
    static const uint8_t prefix[] = "agent:did:key:binding:budget:";
    static const uint8_t hex[] = "0123456789abcdef";
    const uint8_t seed[32] = {17U};
    uint8_t asset_bytes[512];
    size_t asset_length;
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
    f->authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    LXP_CASE_CHECK(lxp_did_id_derive(signer_did, sizeof(signer_did) - 1U,
                                     f->authority.actor) == LXP_OK);
    f->budget_id[0] = 1U;
    f->asset.asset_id[0] = 2U;
    f->asset.symbol[0] = 'A';
    f->asset.symbol_length = 1U;
    f->asset.name[0] = 'A';
    f->asset.name_length = 1U;
    f->asset.issuer_kind = 1U;
    f->asset.custody_kind = LX_ASSET_CUSTODY_NATIVE;
    f->asset.total_units.lo = 100U;
    (void)memcpy(f->asset.issuer_did32, f->authority.actor, 32U);
    LXP_CASE_CHECK(lx_asset_transfer_state(&f->asset, &f->transfer_asset) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(owner_name, sizeof(owner_name) - 1U,
                                             f->authority.principal) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, owner_name, sizeof(owner_name) - 1U,
        f->authority.principal, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &f->owner) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(f->owner, f->asset.asset_id,
                                                (lxp_u128){0U, 100U}, 0U) == LXP_OK);
    f->owner->has_authority_key = true;
    (void)memcpy(f->owner->authority_key, f->public_key, 32U);
    LXP_CASE_CHECK(lx_account_validate_canonical(f->owner) == LXP_OK);
    (void)memcpy(f->account_name, prefix, sizeof(prefix) - 1U);
    f->account_name_length = sizeof(prefix) - 1U;
    for (size_t i = 0U; i < 32U; ++i) {
        f->account_name[f->account_name_length++] = hex[f->budget_id[i] >> 4U];
        f->account_name[f->account_name_length++] = hex[f->budget_id[i] & 15U];
    }
    LXP_CASE_CHECK(lx_account_id_from_string(f->account_name,
        f->account_name_length, f->account_id) == LXP_OK);
    f->payload[1] = 1U;
    (void)memcpy(f->payload + 2U, f->budget_id, 32U);
    (void)memcpy(f->payload + 34U, f->account_id, 32U);
    (void)memcpy(f->payload + 66U, f->asset.asset_id, 32U);
    f->payload[98] = 3U;
    f->payload[145] = 100U;
    f->payload[177] = 25U;
    f->payload[185] = 100U;
    f->payload[201] = 200U;
    f->payload[209] = 1U;
    f->payload[210] = (uint8_t)LX_BUDGET_ROLLOVER_NONE;
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->activity.network_id = 7U;
    f->activity.activity_type = LX_BUDGET_CREATE;
    f->activity.actor_did = (lxp_byte_span){signer_did, sizeof(signer_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, sizeof(f->public_key)};
    f->activity.signature = (lxp_byte_span){f->signature, sizeof(f->signature)};
    f->activity.payload = (lxp_byte_span){f->payload, sizeof(f->payload)};
    f->activity.timestamp_bound.not_after = 100U;
    f->activity.idempotency_key[0] = 1U;
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, sizeof(f->payload),
                                    f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_ERR_BAD_SIGNATURE);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                                     &f->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                              lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
                                              lx_budget_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
                                               lxp_kernel_canonical_ledger_apply) == LXP_OK);
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
        0U, 0U, 1U, 100000U, &f->arena, true) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_record_encode(&f->asset, asset_bytes,
        sizeof(asset_bytes), &asset_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_kv_put(&f->ctx, f->asset.asset_id, 32U,
                                  asset_bytes, asset_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_open(&f->state, 2U, &f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
                                  sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_BUDGET,
        10U, 0U, 2U, 100000U, &f->arena, true) == LXP_OK);
    f->ctx.protocol_version = f->activity.protocol_version;
    LXP_CASE_CHECK(lxp_effect_buffer_init(&f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&f->ctx, &f->effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&wire_arena, f->wire_bytes,
                                  sizeof(f->wire_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_encode(&f->activity, &wire_arena, &wire) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_id(wire.bytes, wire.length,
                                   f->ctx.activity_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&f->kernel, LX_BUDGET_CREATE,
                                                  0U, &f->registration) == LXP_OK);
    return 0;
}

static int signer_expect(signer_fixture *f, lxp_result expected)
{
    lxp_result result;
    LXP_CASE_CHECK(lxp_kernel_dispatch(f->registration, &f->ctx, &f->activity,
        &f->authority, &f->effects, &result) == LXP_OK);
    if (result != expected)
        (void)fprintf(stderr, "budget result %d, expected %d\n", (int)result, (int)expected);
    LXP_CASE_CHECK(result == expected);
    if (expected != LXP_OK) {
        LXP_CASE_CHECK(f->ctx.staged_account_count == 0U && f->ctx.staged_count == 0U);
        LXP_CASE_CHECK(f->effects.count == 0U && f->accounts.count == 1U);
        LXP_CASE_CHECK(f->owner->balance.hi == 0U && f->owner->balance.lo == 100U);
        LXP_CASE_CHECK(f->owner->next_sequence == 0U);
    }
    return 0;
}

static int signer_close(signer_fixture *f)
{
    lxp_module_ctx_rollback(&f->ctx);
    if (f->journal.open)
        LXP_CASE_CHECK(lxp_state_journal_rollback(&f->journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int budget_signer_bound_skips_signature(void)
{
    lx_budget_record record;
    lx_account *account;
    uint8_t key[LX_BUDGET_STATE_KEY_BYTES];
    const uint8_t *bytes;
    size_t length;
    signer_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL && signer_open(f) == 0);
    f->runtime.binding_authority = true;
    f->authority.verified_key[0] ^= 1U;
    LXP_CASE_CHECK(signer_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.verified_key[0] ^= 1U;
    f->authority.actor[0] ^= 1U;
    LXP_CASE_CHECK(signer_expect(f, LXP_ERR_UNAUTHORIZED_DEBIT) == 0);
    f->authority.actor[0] ^= 1U;
    f->activity.payload_hash[0] ^= 1U;
    LXP_CASE_CHECK(signer_expect(f, LXP_ERR_PAYLOAD_HASH_MISMATCH) == 0);
    f->activity.payload_hash[0] ^= 1U;
    ++f->activity.account_sequence;
    LXP_CASE_CHECK(signer_expect(f, LXP_ERR_SEQUENCE_GAP) == 0);
    --f->activity.account_sequence;
    LXP_CASE_CHECK(signer_expect(f, LXP_OK) == 0);
    LXP_CASE_CHECK(f->ctx.staged_account_count == 1U);
    LXP_CASE_CHECK(lx_budget_state_key(f->budget_id, key) == LXP_OK);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&f->ctx, key, sizeof(key), &bytes, &length) == LXP_OK);
    LXP_CASE_CHECK(lx_budget_record_decode(bytes, length, &record) == LXP_OK);
    LXP_CASE_CHECK(memcmp(record.budget_id, f->budget_id, 32U) == 0);
    LXP_CASE_CHECK(memcmp(record.owner, f->owner->id, 32U) == 0);
    LXP_CASE_CHECK(memcmp(record.budget_account, f->account_id, 32U) == 0);
    LXP_CASE_CHECK(memcmp(record.asset_id, f->asset.asset_id, 32U) == 0);
    LXP_CASE_CHECK(record.per_period_limit.hi == 0U && record.per_period_limit.lo == 100U);
    LXP_CASE_CHECK(f->effects.count == 1U && f->effects.effects[0].kind == LXP_EFFECT_EVENT);
    LXP_CASE_CHECK(f->effects.effects[0].module_id == LXP_MODULE_BUDGET &&
                    f->effects.effects[0].event_type == 1U);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&f->journal) == LXP_OK);
    LXP_CASE_CHECK(lx_account_lookup(&f->accounts, f->account_name,
        f->account_name_length, f->account_id, &account) == LXP_OK);
    LXP_CASE_CHECK(account->kind == LX_ACCOUNT_AGENT_BUDGET);
    LXP_CASE_CHECK(account->name_length == f->account_name_length &&
        memcmp(account->name, f->account_name, f->account_name_length) == 0);
    LXP_CASE_CHECK(account->balance.hi == 0U && account->balance.lo == 25U);
    LXP_CASE_CHECK(f->owner->balance.hi == 0U && f->owner->balance.lo == 75U);
    LXP_CASE_CHECK(f->owner->next_sequence == 1U && f->accounts.count == 2U);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_ERR_BAD_SIGNATURE);
    LXP_CASE_CHECK(signer_close(f) == 0);
    (void)puts("budget_signer_bound_skips_signature passed");
    return 0;
}

static int budget_signer_unbound_verifies_signature(void)
{
    signer_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL && signer_open(f) == 0);
    LXP_CASE_CHECK(!f->runtime.binding_authority);
    LXP_CASE_CHECK(signer_expect(f, LXP_ERR_BAD_SIGNATURE) == 0);
    f->kernel.module_runtime[LXP_MODULE_ASSET] = NULL;
    LXP_CASE_CHECK(signer_expect(f, LXP_ERR_BAD_SIGNATURE) == 0);
    LXP_CASE_CHECK(signer_close(f) == 0);
    (void)puts("budget_signer_unbound_verifies_signature passed");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"budget_signer_bound_skips_signature", budget_signer_bound_skips_signature},
        {"budget_signer_unbound_verifies_signature", budget_signer_unbound_verifies_signature}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
