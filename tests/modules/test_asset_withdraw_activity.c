#include "test_asset_retirement.h"

#include "layerx/lx_asset.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_state_proof.h"
#include <openssl/evp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

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


#define CHECK(x) do { \
    if (!(x)) { \
        fprintf(stderr, "case %u line %d: expected %s true, observed false " \
                "(dispatch=%d, module=%d)\n", mode, __LINE__, #x, \
                (int)status, (int)result); \
        return 1; \
    } \
} while (0)

static int run(unsigned mode, int vectors)
{
    static uint8_t arena_bytes[1048576];
    const uint8_t seed[32] = {1U};
    static uint8_t canonical[LXP_MAX_ACTIVITY_BYTES];
    uint8_t public_key[32], signature[64], digest[32];
    uint8_t before[32], after[32], payload[109] = {0};
    const uint8_t actor[] = "did:key:a";
    const char *names[] = {"agent:did:key:a:main", "system:paxeer-withdrawals"};
    lx_asset_record asset = {0};
    lxp_transfer_asset_state asset_state;
    lx_account_registry accounts;
    lx_account *from, *to;
    lx_account *opened[2];
    lx_asset_runtime runtime;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_arena arena, wire_arena;
    lxp_byte_span wire;
    lxp_activity activity = {0};
    lxp_authority_resolved authority = {0};
    const lxp_module_registration *registration;
    uint64_t parameters = 1U;
    lxp_result status = LXP_OK, result = LXP_OK;
    CHECK(TEST_EQ(sign_raw(seed, payload, 0U, signature, public_key), 0));
    asset.asset_id[0] = 1U;
    asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    CHECK(TEST_EQ(lx_asset_transfer_state(&asset, &asset_state), LXP_OK));
    CHECK(TEST_EQ(lx_account_registry_init(&accounts), LXP_OK));
    for (size_t i = 0U; i < 2U; ++i) {
        uint8_t id[32];
        CHECK(TEST_EQ(lx_account_id_from_string((const uint8_t *)names[i], strlen(names[i]), id), LXP_OK));
        CHECK(TEST_EQ(lx_account_open(&accounts, (const uint8_t *)names[i], strlen(names[i]), id,
            1U, i == 0U ? LX_ACCOUNT_OPEN_CREDIT : LX_ACCOUNT_OPEN_GENESIS, NULL, &opened[i]), LXP_OK));
        CHECK(TEST_EQ(lxp_ledger_bootstrap_balance(opened[i], asset.asset_id,
            (lxp_u128){0U, i == 0U ? 100U : 0U}, 0U), LXP_OK));
    }
    from = opened[0]; to = opened[1];
    memcpy(from->authority_key, public_key, 32U); from->has_authority_key = true;
    CHECK(TEST_EQ(lxp_state_store_init(&state, 0U), LXP_OK));
    CHECK(TEST_EQ(lxp_state_store_bind_accounts(&state, &accounts), LXP_OK));
    CHECK(TEST_EQ(lxp_state_store_require_account_root(&state), LXP_OK));
    CHECK(TEST_EQ(lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U), LXP_OK));
    CHECK(TEST_EQ(lxp_kernel_register_module(&kernel, lx_asset_module_iface()), LXP_OK));
    CHECK(TEST_EQ(lxp_kernel_set_capabilities(&kernel, NULL, lxp_kernel_canonical_ledger_apply), LXP_OK));
    runtime = (lx_asset_runtime){&accounts, &asset, 1U, &asset_state, 1U, 7U, LXP_PROTOCOL_VERSION_OCCUPANCY, false};
    CHECK(TEST_EQ(lxp_kernel_bind_module_runtime(&kernel, LXP_MODULE_ASSET, &runtime), LXP_OK));
    CHECK(TEST_EQ(lxp_kernel_module_for_activity(&kernel, LX_ASSET_WITHDRAW, 0U, &registration), LXP_ERR_UNKNOWN_ACTIVITY));
    CHECK(TEST_EQ(lxp_kernel_module_for_activity(&kernel, LX_ASSET_SEND, 0U, &registration), LXP_OK));
    memcpy(payload, asset.asset_id, 32U); payload[47] = 25U;
    payload[48] = 9U; payload[68] = 3U; payload[107] = 100U;
    activity.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
    activity.network_id = 7U; activity.activity_type = LX_ASSET_WITHDRAW;
    activity.actor_did = (lxp_byte_span){actor, sizeof(actor) - 1U};
    activity.authority = (lxp_byte_span){public_key, 32U};
    activity.signature = (lxp_byte_span){signature, 64U};
    activity.payload = (lxp_byte_span){payload, 108U};
    activity.idempotency_key[0] = 1U;
    activity.fee_limit.lo = 100U;
    activity.timestamp_bound.not_after = 100U;
    authority.kind = LXP_AUTHORITY_OWNER;
    memcpy(authority.verified_key, public_key, 32U);
    memcpy(authority.principal, from->id, 32U);
    if (mode == 2U) payload[48] = 0U;
    if (mode == 3U) payload[68] = 0U;
    if (mode == 4U) activity.fee_limit.lo = 99U;
    if (mode == 5U || (mode >= 18U && mode <= 26U)) activity.account_sequence = 1U;
    if (mode == 5U || mode == 27U) memset(authority.principal, 0, 32U);
    if (mode == 6U) payload[47] = 101U;
    if (mode == 7U) payload[0] = 2U;
    if (mode == 8U) activity.payload.length = 107U;
    if (mode == 9U) activity.payload.length = 109U;
    if (mode == 10U) { from->has_authority_key = false; memset(from->authority_key, 0, 32U); }
    if (mode == 11U) authority.verified_key[0] ^= 1U;
    if (mode == 12U) payload[47] = 0U;
    if (mode == 13U) { asset.paused = true; asset_state.paused = true; }
    if (mode == 15U) from->authority_key[0] ^= 1U;
    if (mode == 16U) activity.fee_limit.hi = 1U;
    if (mode == 17U) from->frozen = true;
    CHECK(TEST_EQ(lxp_hash_payload(activity.payload.bytes, activity.payload.length, activity.payload_hash), LXP_OK));
    CHECK(TEST_EQ(lxp_activity_signing_preimage(&activity, digest), LXP_OK));
    CHECK(TEST_EQ(sign_raw(seed, digest, 32U, signature, public_key), 0));
    if (mode == 1U) signature[0] ^= 1U;
    if (mode == 14U) payload[48] ^= 1U;
    CHECK(TEST_EQ(lxp_arena_init(&wire_arena, canonical, sizeof(canonical)), LXP_OK));
    CHECK(TEST_EQ(lxp_activity_encode(&activity, &wire_arena, &wire), LXP_OK));
    CHECK(TEST_EQ(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)), LXP_OK));
    CHECK(TEST_EQ(lxp_effect_buffer_init(&effects), LXP_OK));
    CHECK(TEST_EQ(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_ASSET, 10U, 0U, 1U, 100000U, &arena, true), LXP_OK));
    ctx.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY; ctx.batch_number = 1U;
    CHECK(TEST_EQ(lxp_activity_id(wire.bytes, wire.length, ctx.activity_id), LXP_OK));
    if (mode == 28U) ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    if (mode >= 18U && mode <= 26U) {
        memcpy(authority.principal, from->id, 32U);
        authority.actor[0] = 7U;
        CHECK(TEST_EQ(lxp_kernel_bind_ledger_admission(&ctx, &authority, LX_ASSET_WITHDRAW), LXP_OK));
        CHECK(TEST_EQ(lxp_kernel_bind_ledger_admission(&ctx, &authority, LX_ASSET_WITHDRAW), LXP_ERR_CONTEXT_MISMATCH));
        if (mode == 19U) ctx.ledger_admission.activity_binding[0] ^= 1U;
        if (mode == 20U) ctx.ledger_admission.account_id[0] ^= 1U;
        if (mode == 21U) ctx.ledger_admission.actor[0] ^= 1U;
        if (mode == 22U) ctx.ledger_admission.verified_key[0] ^= 1U;
        if (mode == 23U) ctx.ledger_admission.next_sequence++;
        if (mode == 24U) ctx.ledger_admission.account_present = false;
        if (mode == 25U) ctx.ledger_admission.activity_type = LX_ASSET_SEND;
        if (mode == 26U) authority.principal[0] ^= 1U;
    }
    CHECK(TEST_EQ(lxp_module_ctx_bind_effects(&ctx, &effects), LXP_OK));
    CHECK(TEST_EQ(lxp_state_root(&kernel, before), LXP_OK));
    status = lxp_kernel_dispatch(registration, &ctx, &activity, &authority, &effects, &result);
    if (mode == 19U || mode == 25U) {
        CHECK(TEST_EQ(status, LXP_ERR_CONTEXT_MISMATCH));
    } else {
        CHECK(TEST_EQ(status, LXP_OK));
        CHECK(TEST_EQ(result, LXP_ERR_UNKNOWN_ACTIVITY));
    }
    CHECK(TEST_EQ(from->balance.hi, 0U) && TEST_EQ(to->balance.hi, 0U));
    CHECK(TEST_EQ(from->balance.lo, 100U) && TEST_EQ(to->balance.lo, 0U));
    CHECK(TEST_EQ(from->next_sequence, 0U));
    CHECK(TEST_EQ(effects.count, 0U) && TEST_EQ(ctx.ledger_receipt_present, 0));
    CHECK(TEST_EQ(kernel.module_kv_count, 0U));
    CHECK(TEST_EQ(lxp_state_root(&kernel, after), LXP_OK));
    CHECK(TEST_EQ(memcmp(before, after, 32U), 0));
    if (mode == 0U || mode == 18U) {
        uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES];
        uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES];
        lx_withdrawal_request request = {0};
        uint8_t destination_id[32];
        memcpy(destination_id, to->id, 32U);
        /* Archived records enter only through the explicit compatibility codec. */
        request.network_id = 7U;
        request.amount.lo = 25U;
        memcpy(request.withdrawal_id, ctx.activity_id, 32U);
        memcpy(request.account_id, from->id, 32U);
        memcpy(request.asset_id, asset.asset_id, 32U);
        request.payout_recipient[12] = 9U;
        request.checkpoint_id[0] = 3U;
        CHECK(TEST_EQ(lx_asset_historical_withdrawal_encode_v2(&request, key, value), LXP_OK));
        ctx.global_sequence = state.next_sequence;
        CHECK(TEST_EQ(lxp_state_journal_open(&state, ctx.global_sequence, &journal), LXP_OK));
        CHECK(TEST_EQ(lxp_ctx_kv_put(&ctx, key, sizeof(key), value, sizeof(value)), LXP_OK));
        CHECK(TEST_EQ(lxp_state_journal_commit(&journal), LXP_OK));
        CHECK(TEST_EQ(lxp_module_ctx_commit(&ctx), LXP_OK));
        CHECK(TEST_EQ(asset_history_proof(&kernel, &request, 0), 0));
        CHECK(TEST_EQ(asset_history_reopen(&kernel, &state, &journal, &accounts, &parameters, state.next_sequence - 1U), 0));
        CHECK(TEST_EQ(asset_history_proof(&kernel, &request, vectors), 0));
        size_t from_slot, to_slot;
        CHECK(TEST_EQ(lx_account_registry_index_lookup(&accounts, request.account_id, &from_slot), LXP_OK));
        CHECK(TEST_EQ(lx_account_registry_index_lookup(&accounts, destination_id, &to_slot), LXP_OK));
        CHECK(TEST_EQ(accounts.accounts[from_slot].balance.lo, 100U));
        CHECK(TEST_EQ(accounts.accounts[to_slot].balance.lo, 0U));
        CHECK(TEST_EQ(accounts.accounts[from_slot].next_sequence, 0U));
        CHECK(TEST_EQ(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)), LXP_OK));
        CHECK(TEST_EQ(lxp_effect_buffer_init(&effects), LXP_OK));
        CHECK(TEST_EQ(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_ASSET, 10U, 0U,
            2U, 100000U, &arena, true), LXP_OK));
        ctx.protocol_version = LXP_PROTOCOL_VERSION_OCCUPANCY;
        CHECK(TEST_EQ(lxp_module_ctx_bind_effects(&ctx, &effects), LXP_OK));
        CHECK(TEST_EQ(lxp_kernel_module_for_activity(&kernel, LX_ASSET_WITHDRAW,
            0U, &registration), LXP_ERR_UNKNOWN_ACTIVITY));
        CHECK(TEST_EQ(lxp_kernel_module_for_activity(&kernel, LX_ASSET_SEND,
            0U, &registration), LXP_OK));
        CHECK(TEST_EQ(lxp_state_root(&kernel, before), LXP_OK));
        CHECK(TEST_EQ(lxp_kernel_dispatch(registration, &ctx, &activity,
            &authority, &effects, &result), LXP_OK));
        CHECK(TEST_EQ(result, LXP_ERR_UNKNOWN_ACTIVITY));
        CHECK(TEST_EQ(effects.count, 0U) && TEST_EQ(ctx.ledger_receipt_present, 0));
        CHECK(TEST_EQ(lxp_state_root(&kernel, after), LXP_OK));
        CHECK(TEST_EQ(memcmp(before, after, 32U), 0));
        CHECK(TEST_EQ(asset_history_proof(&kernel, &request, 0), 0));
    }
    CHECK(TEST_EQ(lxp_state_store_destroy(&state), LXP_OK));
    return 0;
}

int main(int argc, char **argv)
{
    if (argc == 2 && strcmp(argv[1], "--vectors") == 0) return run(0U, 1);
    if (argc != 1) return 1;
    for (unsigned mode = 0U; mode < 29U; ++mode)
        if (run(mode, 0) != 0) return 1;
    return 0;
}
