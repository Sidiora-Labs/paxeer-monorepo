#ifndef TEST_NATIVE_REPAIR_HARNESS
#define TEST_NATIVE_REPAIR_HARNESS

#include "test_asset_retirement.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_identity.h"
#include <openssl/evp.h>
#include <stdio.h>

#define NATIVE_CHECK(expression) do { \
    if (!(expression)) { \
        (void)fprintf(stderr, "%s:%d: %s\n", __FILE__, __LINE__, #expression); \
        return 1; \
    } \
} while (0)

/* Shared in-scope harness: all supply and transfers enter registered production
 * dispatch. No balance bootstrap or fabricated custody authority funds it. */
typedef struct native_repair_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lx_asset_registry assets;
    lx_asset_record genesis_asset;
    lxp_transfer_asset_state genesis_transfer;
    lxp_transfer_asset_state transfer_states[LX_ASSET_REGISTRY_CAPACITY];
    lx_asset_runtime runtime;
    lxp_ledger_receipt_input receipt;
    uint64_t parameters;
    uint8_t asset_ids[2][32];
    uint8_t holder_ids[2][2][32];
    uint8_t arena_bytes[2U * LXP_MAX_ACTIVITY_BYTES + 262144U];
    uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
} native_repair_fixture;

static inline int native_sign(const uint8_t seed[32], const uint8_t *message,
    size_t length, uint8_t signature[64], uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, 32U);
    EVP_MD_CTX *signer = EVP_MD_CTX_new();
    size_t key_length = 32U, signature_length = 64U;
    int ok = key != NULL && signer != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &key_length) == 1 &&
        key_length == 32U && EVP_DigestSignInit(signer, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(signer, signature, &signature_length, message, length) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(signer);
    EVP_PKEY_free(key);
    NATIVE_CHECK(ok);
    return 0;
}

static inline lx_account *native_holder(native_repair_fixture *f,
    size_t asset_index, size_t party)
{
    size_t slot;
    if (lx_account_registry_index_lookup(&f->accounts,
            f->holder_ids[asset_index][party], &slot) != LXP_OK) return NULL;
    return &f->accounts.accounts[slot];
}

static inline int native_submit(native_repair_fixture *f, const char *did,
    const uint8_t seed[32], uint32_t type, const uint8_t *payload, size_t length,
    const uint8_t idempotency[32], uint64_t account_sequence,
    const uint8_t *principal, bool full_effects, lxp_result *result)
{
    lxp_activity activity = {0}, transported;
    lxp_authority_resolved authority = {0};
    const lxp_module_registration *registration;
    lxp_module_ctx context;
    lxp_effect_buffer emitted;
    lxp_arena memory, wire_memory;
    lxp_byte_span wire;
    uint8_t signature[64], public_key[32], digest[32];
    uint64_t sequence = f->state.next_sequence;
    NATIVE_CHECK(native_sign(seed, payload, length, signature, public_key) == 0);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.network_id = 7U;
    activity.activity_type = type;
    activity.actor_did = (lxp_byte_span){(const uint8_t *)did, strlen(did)};
    activity.authority = (lxp_byte_span){public_key, 32U};
    activity.signature = (lxp_byte_span){signature, 64U};
    activity.payload = (lxp_byte_span){payload, length};
    activity.account_sequence = account_sequence;
    memcpy(activity.idempotency_key, idempotency, 32U);
    activity.fee_limit.lo = 1000000U;
    activity.timestamp_bound.not_after = 100U;
    NATIVE_CHECK(lxp_hash_payload(payload, length, activity.payload_hash) == LXP_OK);
    NATIVE_CHECK(lxp_activity_signing_preimage(&activity, digest) == LXP_OK);
    NATIVE_CHECK(native_sign(seed, digest, sizeof(digest), signature, public_key) == 0);
    authority.kind = LXP_AUTHORITY_OWNER;
    memcpy(authority.verified_key, public_key, 32U);
    NATIVE_CHECK(lxp_did_id_derive((const uint8_t *)did, strlen(did), authority.actor) == LXP_OK);
    memcpy(authority.principal, principal == NULL ? authority.actor : principal, 32U);
    NATIVE_CHECK(lxp_kernel_module_for_activity(&f->kernel, type, 0U, &registration) == LXP_OK);
    NATIVE_CHECK(lxp_arena_init(&wire_memory, f->wire_bytes, sizeof(f->wire_bytes)) == LXP_OK);
    NATIVE_CHECK(lxp_activity_encode(&activity, &wire_memory, &wire) == LXP_OK);
    NATIVE_CHECK(lxp_activity_decode(wire.bytes, wire.length, &transported) == LXP_OK);
    NATIVE_CHECK(lxp_arena_init(&memory, f->arena_bytes, sizeof(f->arena_bytes)) == LXP_OK);
    NATIVE_CHECK(lxp_effect_buffer_init(&emitted) == LXP_OK);
    NATIVE_CHECK(lxp_state_journal_open(&f->state, sequence, &f->journal) == LXP_OK);
    NATIVE_CHECK(lxp_module_ctx_init(&context, &f->kernel, (uint16_t)(type >> 16U),
        10U, 0U, sequence, 1000000U, &memory, true) == LXP_OK);
    context.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    context.batch_number = sequence;
    NATIVE_CHECK(lxp_activity_id(wire.bytes, wire.length, context.activity_id) == LXP_OK);
    NATIVE_CHECK(lxp_module_ctx_bind_effects(&context, &emitted) == LXP_OK);
    if (full_effects)
        for (size_t i = 0U; i < LXP_MAX_EFFECTS; ++i)
            NATIVE_CHECK(lxp_ctx_emit_event(&context, 1U, NULL, 0U) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_dispatch(registration, &context, &transported,
        &authority, &emitted, result) == LXP_OK);
    f->receipt = context.ledger_receipt;
    if (*result == LXP_OK) {
        NATIVE_CHECK(lxp_module_ctx_commit(&context) == LXP_OK);
        NATIVE_CHECK(lxp_state_journal_commit(&f->journal) == LXP_OK);
        NATIVE_CHECK(lx_asset_runtime_restore(&f->kernel, &f->assets,
            f->transfer_states, &f->runtime) == LXP_OK);
    } else {
        lxp_module_ctx_rollback(&context);
        NATIVE_CHECK(!context.transfer_applied);
        NATIVE_CHECK(lxp_state_journal_rollback(&f->journal) == LXP_OK);
    }
    return 0;
}

static inline int native_fixture_init(native_repair_fixture *f)
{
    memset(f, 0, sizeof(*f));
    f->parameters = 1U;
    NATIVE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    NATIVE_CHECK(lx_asset_registry_init(&f->assets, 0U) == LXP_OK);
    NATIVE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    NATIVE_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    NATIVE_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal, &f->parameters, 0U) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL, lxp_kernel_canonical_ledger_apply) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    /* Binding requires a nonempty registered transfer table even before the
     * first activity creates an asset. This zero-supply genesis entry is
     * replaced by committed records after the registration succeeds. */
    f->genesis_asset.asset_id[0] = 0x5bU;
    NATIVE_CHECK(lx_asset_transfer_state(&f->genesis_asset, &f->genesis_transfer) == LXP_OK);
    f->runtime.assets = &f->genesis_asset;
    f->runtime.asset_count = 1U;
    f->runtime.transfer_assets = &f->genesis_transfer;
    f->runtime.transfer_asset_count = 1U;
    f->runtime.network_id = 7U;
    f->runtime.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    NATIVE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET, &f->runtime) == LXP_OK);
    return 0;
}

static inline int native_issue(native_repair_fixture *f, size_t index)
{
    const char *dids[] = {"did:key:native-a", "did:key:native-b"};
    const uint8_t seeds[2][32] = {{31U}, {32U}};
    static const char digits[] = "0123456789abcdef";
    uint8_t registration[89] = {0}, account_payload[34] = {0}, mint[82] = {0};
    uint8_t actor[32], idempotency[32] = {0};
    lxp_hash_context hash;
    lxp_result outcome;
    registration[1] = 1U;
    registration[34] = (uint8_t)(index + 1U);
    NATIVE_CHECK(lxp_did_id_derive((const uint8_t *)dids[0], strlen(dids[0]), actor) == LXP_OK);
    lxp_hash_init(&hash);
    NATIVE_CHECK(lxp_hash_update(&hash, "LX:ASSET:v1", 11U) == LXP_OK);
    NATIVE_CHECK(lxp_hash_update(&hash, actor, 32U) == LXP_OK);
    NATIVE_CHECK(lxp_hash_update(&hash, registration + 34U, 32U) == LXP_OK);
    NATIVE_CHECK(lxp_hash_final(&hash, f->asset_ids[index]) == LXP_OK);
    memcpy(registration + 2U, f->asset_ids[index], 32U);
    registration[66] = 1U; registration[67] = (uint8_t)('A' + index);
    registration[68] = 1U; registration[69] = (uint8_t)('A' + index);
    registration[86] = 100U; registration[87] = 1U;
    idempotency[0] = (uint8_t)f->state.next_sequence;
    NATIVE_CHECK(native_submit(f, dids[0], seeds[0], LX_ASSET_REGISTER,
        registration, sizeof(registration), idempotency, 0U, NULL, false, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_OK);
    account_payload[1] = 1U;
    memcpy(account_payload + 2U, f->asset_ids[index], 32U);
    for (size_t party = 0U; party < 2U; ++party) {
        char name[LX_ACCOUNT_NAME_MAX];
        char asset_hex[65];
        for (size_t i = 0U; i < 32U; ++i) {
            asset_hex[2U * i] = digits[f->asset_ids[index][i] >> 4U];
            asset_hex[2U * i + 1U] = digits[f->asset_ids[index][i] & 15U];
        }
        asset_hex[64] = '\0';
        int count = snprintf(name, sizeof(name), "agent:%s:asset:%s", dids[party], asset_hex);
        NATIVE_CHECK(count > 0 && (size_t)count < sizeof(name));
        NATIVE_CHECK(lx_account_id_from_string((const uint8_t *)name, (size_t)count,
            f->holder_ids[index][party]) == LXP_OK);
        idempotency[0] = (uint8_t)f->state.next_sequence;
        NATIVE_CHECK(native_submit(f, dids[party], seeds[party], LX_ASSET_ACCOUNT_OPEN,
            account_payload, sizeof(account_payload), idempotency, 0U, NULL, false, &outcome) == 0);
        NATIVE_CHECK(outcome == LXP_OK);
    }
    mint[1] = 1U;
    memcpy(mint + 2U, f->asset_ids[index], 32U);
    memcpy(mint + 34U, f->holder_ids[index][0], 32U);
    mint[81] = 100U;
    idempotency[0] = (uint8_t)f->state.next_sequence;
    NATIVE_CHECK(native_submit(f, dids[0], seeds[0], LX_ASSET_MINT,
        mint, sizeof(mint), idempotency, 0U, NULL, false, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_OK && f->receipt.supply_binding_version == 1U &&
        f->receipt.total_units_after.lo == 100U);
    NATIVE_CHECK(lx_asset_native_supply_check(&f->assets, &f->accounts) == LXP_OK);
    return 0;
}

static inline int native_make_send(native_repair_fixture *f, size_t index,
    size_t party, const uint8_t to[32], uint64_t amount, uint8_t marker,
    lxp_send *send)
{
    const uint8_t seeds[2][32] = {{31U}, {32U}};
    uint8_t material[144], message[512], digest[32];
    size_t length;
    lx_account *from = native_holder(f, index, party);
    NATIVE_CHECK(from != NULL);
    memset(send, 0, sizeof(*send));
    memcpy(send->from, from->id, 32U);
    memcpy(send->to, to, 32U);
    memcpy(send->asset, f->asset_ids[index], 32U);
    send->amount.lo = amount;
    send->sequence = from->next_sequence;
    send->idempotency_key[0] = marker;
    send->expires_at = 100U;
    memcpy(material, send->from, 32U);
    memcpy(material + 32U, send->to, 32U);
    memcpy(material + 64U, send->asset, 32U);
    NATIVE_CHECK(lxp_u128_to_be(send->amount, material + 96U) == LXP_OK);
    memcpy(material + 112U, send->idempotency_key, 32U);
    NATIVE_CHECK(lxp_hash_context_value(material, sizeof(material), send->context_hash) == LXP_OK);
    send->authorization.kind = LXP_AUTH_OWNER;
    send->authorization.network_id = 7U;
    send->authorization.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    memcpy(send->authorization.controller, send->from, 32U);
    memcpy(send->authorization.signed_context_hash, send->context_hash, 32U);
    NATIVE_CHECK(native_sign(seeds[party], material, sizeof(material),
        send->authorization.signature, send->authorization.public_key) == 0);
    NATIVE_CHECK(lxp_send_authorization_message(send, message, sizeof(message), &length) == LXP_OK);
    NATIVE_CHECK(lxp_hash_signature_preimage(message, length, digest) == LXP_OK);
    NATIVE_CHECK(native_sign(seeds[party], digest, sizeof(digest),
        send->authorization.signature, send->authorization.public_key) == 0);
    return 0;
}

static inline int native_send_submit(native_repair_fixture *f, const lxp_send *send,
    size_t signer, bool full_effects, lxp_result *result)
{
    const char *dids[] = {"did:key:native-a", "did:key:native-b"};
    const uint8_t seeds[2][32] = {{31U}, {32U}};
    uint8_t payload[512];
    size_t length;
    NATIVE_CHECK(lxp_send_encode(send, payload, sizeof(payload), &length) == LXP_OK);
    return native_submit(f, dids[signer], seeds[signer], LX_ASSET_SEND, payload,
        length, send->idempotency_key, send->sequence, send->from, full_effects, result);
}

static inline int native_transfer_cases(void)
{
    native_repair_fixture *f = calloc(1U, sizeof(*f));
    lxp_send send;
    lxp_result outcome;
    uint8_t before[32], after[32];
    NATIVE_CHECK(f != NULL);
    NATIVE_CHECK(native_fixture_init(f) == 0 && native_issue(f, 0U) == 0 && native_issue(f, 1U) == 0);
    NATIVE_CHECK(native_holder(f, 0U, 0U)->balance.lo == 100U &&
        native_holder(f, 0U, 1U)->balance.lo == 0U);
    NATIVE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    NATIVE_CHECK(native_make_send(f, 0U, 0U, f->holder_ids[0][1], 25U, 70U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 1U, false, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_ERR_UNAUTHORIZED_DEBIT);
    NATIVE_CHECK(native_send_submit(f, &send, 0U, true, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_ERR_LENGTH_LIMIT);
    NATIVE_CHECK(native_make_send(f, 0U, 0U, f->holder_ids[1][1], 25U, 71U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 0U, false, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_ERR_ASSET_MISMATCH);
    NATIVE_CHECK(native_make_send(f, 0U, 0U, f->holder_ids[0][1], 101U, 72U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 0U, false, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_ERR_INSUFFICIENT_BALANCE);
    NATIVE_CHECK(native_make_send(f, 0U, 0U, f->holder_ids[0][1], 0U, 75U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 0U, false, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_ERR_INVALID_AMOUNT);
    NATIVE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK && memcmp(before, after, 32U) == 0);
    NATIVE_CHECK(native_holder(f, 0U, 0U)->balance.lo == 100U &&
        native_holder(f, 0U, 1U)->balance.lo == 0U &&
        native_holder(f, 0U, 0U)->next_sequence == 0U);
    NATIVE_CHECK(native_make_send(f, 0U, 0U, f->holder_ids[0][1], 25U, 73U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 0U, false, &outcome) == 0 && outcome == LXP_OK);
    NATIVE_CHECK(native_holder(f, 0U, 0U)->balance.lo == 75U &&
        native_holder(f, 0U, 1U)->balance.lo == 25U &&
        native_holder(f, 0U, 0U)->next_sequence == 1U &&
        f->receipt.amount.lo == 25U && f->receipt.from_balance_before.lo == 100U &&
        f->receipt.from_balance_after.lo == 75U && f->receipt.to_balance_after.lo == 25U);
    NATIVE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    NATIVE_CHECK(native_send_submit(f, &send, 0U, false, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_ERR_SEQUENCE_MISMATCH);
    NATIVE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK && memcmp(before, after, 32U) == 0);
    NATIVE_CHECK(native_make_send(f, 0U, 1U, f->holder_ids[0][0], 10U, 74U, &send) == 0);
    NATIVE_CHECK(native_send_submit(f, &send, 1U, false, &outcome) == 0 && outcome == LXP_OK);
    NATIVE_CHECK(native_holder(f, 0U, 0U)->balance.lo == 85U &&
        native_holder(f, 0U, 1U)->balance.lo == 15U &&
        native_holder(f, 1U, 0U)->balance.lo == 100U &&
        native_holder(f, 1U, 1U)->balance.lo == 0U);
    NATIVE_CHECK(lx_asset_native_supply_check(&f->assets, &f->accounts) == LXP_OK);
    NATIVE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    NATIVE_CHECK(asset_history_reopen(&f->kernel, &f->state, &f->journal,
        &f->accounts, &f->parameters, f->state.next_sequence - 1U) == 0);
    NATIVE_CHECK(lx_asset_runtime_restore(&f->kernel, &f->assets,
        f->transfer_states, &f->runtime) == LXP_OK);
    NATIVE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET, &f->runtime) == LXP_OK);
    NATIVE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK && memcmp(before, after, 32U) == 0);
    NATIVE_CHECK(native_holder(f, 0U, 0U)->balance.lo == 85U &&
        native_holder(f, 0U, 1U)->balance.lo == 15U &&
        native_holder(f, 0U, 0U)->next_sequence == 1U &&
        native_holder(f, 0U, 1U)->next_sequence == 1U);
    NATIVE_CHECK(native_send_submit(f, &send, 1U, false, &outcome) == 0);
    NATIVE_CHECK(outcome == LXP_ERR_SEQUENCE_MISMATCH);
    NATIVE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK && memcmp(before, after, 32U) == 0);
    NATIVE_CHECK(lx_asset_native_supply_check(&f->assets, &f->accounts) == LXP_OK);
    NATIVE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static inline int native_fixture_destroy(native_repair_fixture *f)
{
    NATIVE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    return 0;
}
#endif

#ifndef ASSET_NATIVE_HARNESS_ONLY
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
        CHECK(TEST_EQ(asset_history_proof(&kernel, &request, 0), 0));
        if (vectors) {
            (void)printf("{\"protocol_version\":2,\"retired\":true,"
                         "\"evidence_kind\":\"historical-codec-only\",\"vector\":");
            CHECK(TEST_EQ(asset_history_proof(&kernel, &request, 1), 0));
            (void)puts("}");
        }
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

static int test_native_committed_activity(void)
{
    return native_transfer_cases();
}

int main(int argc, char **argv)
{
    if (argc == 2 && strcmp(argv[1], "--vectors") == 0) return run(0U, 1);
    if (argc != 1) return 1;
    if (test_native_committed_activity() != 0) return 1;
    for (unsigned mode = 0U; mode < 29U; ++mode)
        if (run(mode, 0) != 0) return 1;
    return 0;
}

#endif
