#include "layerx/lxp_bridge.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_receipt.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_state_proof.h"
#include "layerx/programs.h"

#include <openssl/evp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define NATIVE_CHECK(condition) do { if (!(condition)) { \
    fprintf(stderr, "%s:%d: %s\n", __FILE__, __LINE__, #condition); \
    return 1; } } while (0)

/* Shared real kernel fixture for funding and canonical fee-log replay. It
 * exercises native ledger SEND; Cosmos bank/bridgeOut wiring is owned by the
 * node and is recorded in test_beta_withdrawal_genesis.py, not simulated here. */
typedef struct native_ledger_fixture {
    lx_account_registry accounts;
    lx_account *sender;
    lx_account *recipient;
    lx_account *treasury;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lx_asset_runtime asset_runtime;
    lx_programs_transfer_runtime fee_runtime;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_identity_store identities;
    lxp_authority_resolved authority;
    lxp_fee_params fees;
    lxp_arena arena;
    uint8_t arena_bytes[2U * LXP_MAX_ACTIVITY_BYTES];
    uint8_t public_key[32];
    uint64_t parameters;
} native_ledger_fixture;

static const uint8_t native_ledger_seed[32] = {0x37U};
static const uint8_t native_ledger_actor[] = "did:key:funding";

static int native_ledger_sign(const uint8_t *message, size_t length,
                              uint8_t signature[64], uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(
        EVP_PKEY_ED25519, NULL, native_ledger_seed, 32U);
    EVP_MD_CTX *context = EVP_MD_CTX_new();
    size_t public_length = 32U;
    size_t signature_length = 64U;
    int ok = key != NULL && context != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &public_length) == 1 &&
        public_length == 32U &&
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(context, signature, &signature_length, message, length) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static int native_ledger_bind(native_ledger_fixture *f)
{
    f->asset_runtime = (lx_asset_runtime){&f->accounts, &f->asset, 1U,
        &f->transfer_asset, 1U, 125U, LXP_PROTOCOL_VERSION_STATE_COMMITMENT, false};
    (void)memset(&f->fee_runtime, 0, sizeof(f->fee_runtime));
    f->fee_runtime.accounts = &f->accounts;
    f->fee_runtime.assets = &f->transfer_asset;
    f->fee_runtime.asset_count = 1U;
    (void)memcpy(f->fee_runtime.occupancy_asset_id, f->asset.asset_id, 32U);
    NATIVE_CHECK(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET,
        &f->asset_runtime) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_PROGRAMS,
        &f->fee_runtime) == LXP_OK);
    return 0;
}

static int native_ledger_init(native_ledger_fixture *f)
{
    static const char *const names[] = {
        "agent:did:key:funding:main",
        "agent:did:key:funding:asset:0100000000000000000000000000000000000000000000000000000000000000",
        "system:fees"
    };
    lx_account *opened[3];
    lxp_identity *identity;
    uint8_t signature[64];
    (void)memset(f, 0, sizeof(*f));
    f->parameters = 1U;
    f->asset.asset_id[0] = 1U;
    f->asset.total_units.lo = 1000U;
    f->asset.symbol[0] = 'A';
    f->asset.symbol_length = 1U;
    f->asset.name[0] = 'A';
    f->asset.name_length = 1U;
    f->asset.issuer_kind = 1U;
    f->asset.issuer_did32[0] = 1U;
    f->asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    f->asset.custody_reference[0] = 1U;
    f->asset.custody_reference_length = 1U;
    NATIVE_CHECK(native_ledger_sign(native_ledger_seed, 32U, signature, f->public_key) == 0);
    NATIVE_CHECK(lx_asset_transfer_state(&f->asset, &f->transfer_asset) == LXP_OK);
    NATIVE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    for (size_t i = 0U; i < 3U; ++i) {
        uint8_t id[32];
        NATIVE_CHECK(lx_account_id_from_string((const uint8_t *)names[i], strlen(names[i]), id) == LXP_OK);
        NATIVE_CHECK(lx_account_open(&f->accounts, (const uint8_t *)names[i],
            strlen(names[i]), id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL, &opened[i]) == LXP_OK);
        NATIVE_CHECK(lxp_ledger_bootstrap_balance(opened[i], f->asset.asset_id,
            (lxp_u128){0U, i == 0U ? 1000U : 0U}, 0U) == LXP_OK);
    }
    f->sender = opened[0];
    f->recipient = opened[1];
    f->treasury = opened[2];
    NATIVE_CHECK(f->recipient->kind == LX_ACCOUNT_AGENT_ASSET);
    f->sender->has_authority_key = true;
    (void)memcpy(f->sender->authority_key, f->public_key, 32U);
    f->recipient->has_authority_key = true;
    (void)memcpy(f->recipient->authority_key, f->public_key, 32U);
    NATIVE_CHECK(lxp_identity_register(&f->identities, native_ledger_actor,
        sizeof(native_ledger_actor) - 1U, f->public_key, &identity) == LXP_OK);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(f->authority.principal, f->sender->id, 32U);
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    NATIVE_CHECK(lxp_did_id_derive(native_ledger_actor, sizeof(native_ledger_actor) - 1U,
        f->authority.actor) == LXP_OK);
    NATIVE_CHECK(lxp_authority_hash(f->authority.kind, f->authority.grant_id,
        f->public_key, f->authority.authority_hash) == LXP_OK);
    f->fees.version = 1U;
    f->fees.base_fee.lo = 7U;
    f->fees.multiplier_basis_points = 10000U;
    NATIVE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    NATIVE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    NATIVE_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal, &f->parameters, 0U) == LXP_OK);
    NATIVE_CHECK(native_ledger_bind(f) == 0);
    NATIVE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes, sizeof(f->arena_bytes)) == LXP_OK);
    return 0;
}

static int native_ledger_activity(native_ledger_fixture *f, uint64_t sequence,
                                  uint64_t amount, uint8_t *canonical,
                                  size_t capacity, size_t *length)
{
    lxp_send send = {0};
    lxp_activity activity = {0};
    lxp_byte_span encoded;
    uint8_t payload[1024], material[144], message[512], digest[32], signature[64];
    size_t payload_length, message_length;
    (void)memcpy(send.from, f->sender->id, 32U);
    (void)memcpy(send.to, f->recipient->id, 32U);
    (void)memcpy(send.asset, f->asset.asset_id, 32U);
    send.amount.lo = amount;
    send.sequence = f->sender->next_sequence;
    send.expires_at = 100U;
    send.idempotency_key[0] = (uint8_t)(sequence + 1U);
    send.authorization.kind = LXP_AUTH_OWNER;
    send.authorization.network_id = 125U;
    send.authorization.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    (void)memcpy(send.authorization.controller, send.from, 32U);
    (void)memcpy(material, send.from, 32U);
    (void)memcpy(material + 32U, send.to, 32U);
    (void)memcpy(material + 64U, send.asset, 32U);
    NATIVE_CHECK(lxp_u128_to_be(send.amount, material + 96U) == LXP_OK);
    (void)memcpy(material + 112U, send.idempotency_key, 32U);
    NATIVE_CHECK(lxp_hash_context_value(material, sizeof(material), send.context_hash) == LXP_OK);
    (void)memcpy(send.authorization.signed_context_hash, send.context_hash, 32U);
    NATIVE_CHECK(lxp_send_authorization_message(&send, message, sizeof(message), &message_length) == LXP_OK);
    NATIVE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message, message_length, digest) == LXP_OK);
    NATIVE_CHECK(native_ledger_sign(digest, sizeof(digest), send.authorization.signature,
        send.authorization.public_key) == 0);
    NATIVE_CHECK(lxp_send_encode(&send, payload, sizeof(payload), &payload_length) == LXP_OK);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.network_id = 125U;
    activity.activity_type = LX_ASSET_SEND;
    activity.actor_did = (lxp_byte_span){native_ledger_actor, sizeof(native_ledger_actor) - 1U};
    activity.account_sequence = sequence;
    activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    activity.fee_limit.lo = 7U;
    (void)memcpy(activity.idempotency_key, send.idempotency_key, 32U);
    activity.authority = (lxp_byte_span){f->public_key, 32U};
    activity.payload = (lxp_byte_span){payload, payload_length};
    NATIVE_CHECK(lxp_hash_payload(payload, payload_length, activity.payload_hash) == LXP_OK);
    NATIVE_CHECK(lxp_activity_signing_preimage(&activity, digest) == LXP_OK);
    NATIVE_CHECK(native_ledger_sign(digest, sizeof(digest), signature, f->public_key) == 0);
    activity.signature = (lxp_byte_span){signature, sizeof(signature)};
    NATIVE_CHECK(lxp_activity_verify_signature(&activity) == LXP_OK);
    NATIVE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    NATIVE_CHECK(lxp_activity_encode(&activity, &f->arena, &encoded) == LXP_OK);
    NATIVE_CHECK(encoded.length <= capacity);
    (void)memcpy(canonical, encoded.bytes, encoded.length);
    *length = encoded.length;
    return 0;
}

static lxp_result native_ledger_execute(native_ledger_fixture *f,
                                        const uint8_t *canonical, size_t length,
                                        lxp_receipt *receipt)
{
    lxp_activity activity;
    lxp_kernel_execution execution = {0};
    lxp_result status = lxp_activity_decode(canonical, length, &activity);
    if (status == LXP_OK) status = lxp_activity_verify_signature(&activity);
    if (status != LXP_OK) return status;
    status = lxp_arena_reset(&f->arena, 0U);
    if (status != LXP_OK) return status;
    execution.network_id = 125U;
    execution.batch_number = 1U;
    execution.batch_timestamp_ms = 10U;
    execution.maximum_timestamp_window = 100U;
    execution.global_sequence = f->state.next_sequence;
    execution.recorded_module_version = lx_asset_module_iface()->abi_version;
    execution.parameter_version = 1U;
    execution.signature_valid = true;
    execution.identities = &f->identities;
    execution.authority = &f->authority;
    execution.fee_parameters = &f->fees;
    execution.fee_balance = f->sender->balance;
    execution.gas_limit = 100000U;
    execution.arena = &f->arena;
    execution.sequencer_private_key = native_ledger_seed;
    return lxp_kernel_execute_activity(&f->kernel, &activity, &execution, receipt);
}

static int native_ledger_balance_proof(native_ledger_fixture *f,
                                        const lx_account *account,
                                        uint64_t expected)
{
    static lxp_state_witness proof;
    uint8_t key[33], root[32], value[LX_ACCOUNT_STATE_LEAF_VALUE_MAX_BYTES];
    size_t value_length;
    NATIVE_CHECK(account->balance.hi == 0U && account->balance.lo == expected);
    NATIVE_CHECK(lx_account_state_leaf_material(account, key, value, &value_length) == LXP_OK);
    NATIVE_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
    NATIVE_CHECK(lxp_state_proof_build(&f->kernel, 0U,
        (lxp_byte_span){key, sizeof(key)}, &proof) == LXP_OK);
    NATIVE_CHECK(proof.value_length == value_length && memcmp(proof.value, value, value_length) == 0);
    NATIVE_CHECK(lxp_state_proof_verify(&proof, root) == LXP_OK);
    proof.value[0] ^= 1U;
    NATIVE_CHECK(lxp_state_proof_verify(&proof, root) != LXP_OK);
    return 0;
}

static int native_ledger_destroy(native_ledger_fixture *f)
{
    NATIVE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    return 0;
}

#ifndef LXP_NATIVE_LEDGER_SUPPORT_ONLY
static int test_native_subaccount_funding_committed_proof(void)
{
    static native_ledger_fixture f;
    uint8_t canonical[2048], before[32], after[32];
    size_t length;
    lxp_receipt receipt;
    NATIVE_CHECK(native_ledger_init(&f) == 0);
    NATIVE_CHECK(lxp_state_root(&f.kernel, before) == LXP_OK);
    NATIVE_CHECK(native_ledger_activity(&f, 0U, 25U, canonical, sizeof(canonical), &length) == 0);
    NATIVE_CHECK(native_ledger_execute(&f, canonical, length, &receipt) == LXP_OK);
    NATIVE_CHECK(receipt.result_code == LXP_OK && receipt.fee_charged.hi == 0U && receipt.fee_charged.lo == 7U);
    NATIVE_CHECK(native_ledger_balance_proof(&f, f.sender, 968U) == 0);
    NATIVE_CHECK(native_ledger_balance_proof(&f, f.recipient, 25U) == 0);
    NATIVE_CHECK(native_ledger_balance_proof(&f, f.treasury, 7U) == 0);
    NATIVE_CHECK(lxp_state_root(&f.kernel, after) == LXP_OK && memcmp(before, after, 32U) != 0);
    NATIVE_CHECK(memcmp(f.kernel.current_state_root, receipt.resulting_state_root, 32U) == 0);
    NATIVE_CHECK(f.state.next_sequence == 2U && f.sender->next_sequence == 1U);
    NATIVE_CHECK(native_ledger_destroy(&f) == 0);
    return 0;
}

/* The retired registry cannot emit finalized proofs. These are historical
 * input-refusal checks, never evidence of funding or later inclusion guards. */
static int test_historical_deposit_refusals(void)
{
    static native_ledger_fixture f;
    lx_deposit_proof proof = {0}, altered;
    lx_checkpoint_registry *checkpoints = NULL;
    lxp_bridge_deposit_context bridge;
    lx_asset_transfer_request transfer = {0};
    lxp_receipt receipt, saved;
    lxp_module_ctx ctx;
    uint8_t before[32], after[32], first[32], second[32];
    NATIVE_CHECK(native_ledger_init(&f) == 0);
    NATIVE_CHECK(lxp_module_ctx_init(&ctx, &f.kernel, LXP_MODULE_ASSET, 10U,
        0U, 1U, 1000U, &f.arena, true) == LXP_OK);
    NATIVE_CHECK(lx_checkpoint_registry_create(f.public_key, 125U,
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT, &checkpoints) == LXP_ERR_UNKNOWN_ACTIVITY);
    NATIVE_CHECK(checkpoints == NULL);
    bridge = (lxp_bridge_deposit_context){&ctx, NULL, &f.accounts,
        checkpoints, 125U, LXP_PROTOCOL_VERSION_STATE_COMMITMENT};
    transfer.from = f.sender;
    transfer.to = f.recipient;
    transfer.asset = &f.asset;
    proof.deposit_id[0] = 4U;
    proof.asset_id[0] = 1U;
    proof.checkpoint_id[0] = 2U;
    proof.network_id = 125U;
    proof.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    proof.amount.lo = 25U;
    NATIVE_CHECK(lxp_deposit_nullifier(&proof, first) == LXP_OK);
    NATIVE_CHECK(lxp_deposit_nullifier(&proof, second) == LXP_OK && memcmp(first, second, 32U) == 0);
    NATIVE_CHECK(lxp_state_root(&f.kernel, before) == LXP_OK);
    for (unsigned variant = 0U; variant < 9U; ++variant) {
        altered = proof;
        if (variant == 1U) altered.network_id++;
        if (variant == 2U) altered.protocol_version++;
        if (variant == 3U) altered.inclusion_proof.leaf_count = 1U;
        if (variant == 4U) altered.inclusion_proof.siblings[0][0] ^= 0xffU;
        if (variant == 5U) altered.inclusion_proof.leaf_index = 1U;
        if (variant == 6U) altered.checkpoint_id[0] ^= 0xffU;
        if (variant == 7U) altered.custody_reference[0] ^= 0xffU;
        (void)memset(&receipt, 0x5a, sizeof(receipt));
        saved = receipt;
        const lx_deposit_proof *input = variant == 8U ? NULL : &altered;
        NATIVE_CHECK(lxp_deposit_proof_verify(input, checkpoints, 125U,
            LXP_PROTOCOL_VERSION_STATE_COMMITMENT) == LXP_ERR_DEPOSIT_PROOF_NOT_FINAL);
        NATIVE_CHECK(lxp_bridge_deposit_credit(&bridge, &transfer, input,
            &receipt) == LXP_ERR_DEPOSIT_PROOF_NOT_FINAL);
        NATIVE_CHECK(lx_asset_deposit_credit(&ctx, &transfer, input, checkpoints,
            125U, LXP_PROTOCOL_VERSION_STATE_COMMITMENT, &receipt) == LXP_ERR_UNKNOWN_ACTIVITY);
        NATIVE_CHECK(memcmp(&receipt, &saved, sizeof(receipt)) == 0);
        NATIVE_CHECK(ctx.staged_count == 0U && !ctx.transfer_applied);
        NATIVE_CHECK(lxp_state_root(&f.kernel, after) == LXP_OK && memcmp(before, after, 32U) == 0);
        NATIVE_CHECK(f.sender->balance.lo == 1000U && f.recipient->balance.lo == 0U);
    }
    NATIVE_CHECK(native_ledger_destroy(&f) == 0);
    return 0;
}

#include "lxp_test_harness.c"

int main(int argc, char **argv)
{
    NATIVE_CHECK(lxp_test_register("bridge.native-subaccount-committed-proof",
        test_native_subaccount_funding_committed_proof) == 0);
    NATIVE_CHECK(lxp_test_register("bridge.historical-deposit-refusals",
        test_historical_deposit_refusals) == 0);
    return lxp_test_main(argc, argv);
}
#endif
