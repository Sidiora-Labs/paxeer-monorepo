#include "layerx/lxp_codec.h"
#include "layerx/lxp_admission.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lx_asset.h"

#include <openssl/evp.h>
#include <stdlib.h>
#include <string.h>

#include <stdint.h>
#include <stdio.h>

static int codec_primitive_strict(void)
{
    uint8_t storage[32];
    lxp_arena arena;
    lxp_codec_writer writer;
    lxp_codec_reader reader;

    if (lxp_arena_init(&arena, storage, sizeof(storage)) != LXP_OK ||
        lxp_codec_writer_init(&writer, &arena, sizeof(storage)) != LXP_OK ||
        lxp_codec_write_struct_header(&writer, 0x1001U) != LXP_OK ||
        lxp_codec_write_u8(&writer, 0xaaU) != LXP_OK) return 1;
    if (lxp_codec_reader_init(&reader, writer.bytes, writer.length) != LXP_OK ||
        lxp_codec_read_struct_header(&reader, 0x1001U) != LXP_OK ||
        lxp_codec_finish(&reader) != LXP_ERR_TRAILING_BYTES) return 1;
    {
        uint8_t value;
        if (lxp_codec_read_u8(&reader, &value) != LXP_OK || value != 0xaaU ||
            lxp_codec_finish(&reader) != LXP_OK) return 1;
    }
    if (lxp_codec_reader_init(&reader, writer.bytes, writer.length) != LXP_OK ||
        lxp_codec_read_struct_header(&reader, 0x1002U) !=
        LXP_ERR_VERSION_UNSUPPORTED) return 1;
    if (lxp_codec_reject_unknown_field(4U, 3U) != LXP_ERR_UNKNOWN_FIELD ||
        lxp_codec_reject_unknown_field(3U, 3U) != LXP_OK) return 1;
    return 0;
}

#define STRICT_CHECK(condition) do { \
    if (!(condition)) { \
        fprintf(stderr, "%s:%d: check failed: %s\n", __FILE__, __LINE__, #condition); \
        return 1; \
    } \
} while (0)

typedef struct strict_admission_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lxp_identity_store identities;
    lx_account *payer;
    lx_account *recipient;
    lx_account *treasury;
    lxp_activity activity;
    lxp_authority_resolved authority;
    lxp_kernel_execution execution;
    lxp_fee_params fees;
    lxp_receipt receipt;
    lxp_receipt untouched_receipt;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lx_asset_runtime runtime;
    uint64_t parameters;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[512];
    uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES + 1U];
    uint8_t arena_bytes[4U * LXP_MAX_ACTIVITY_BYTES];
    lxp_arena arena;
} strict_admission_fixture;

static const uint8_t strict_seed[32] = {0x31U};
static const uint8_t strict_did[] = "did:key:strict";

static int strict_sign(const uint8_t digest[32], uint8_t signature[64],
                        uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                 strict_seed, sizeof(strict_seed));
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    size_t public_length = 32U;
    size_t signature_length = 64U;
    int ok = key != NULL && ctx != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &public_length) == 1 &&
        public_length == 32U && EVP_DigestSignInit(ctx, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(ctx, signature, &signature_length, digest, 32U) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static int strict_account(strict_admission_fixture *f, const char *name,
                           uint64_t balance, lx_account **account)
{
    uint8_t id[32];
    bool system = strncmp(name, "system:", 7U) == 0;
    STRICT_CHECK(lx_account_id_from_string((const uint8_t *)name, strlen(name), id) == LXP_OK);
    STRICT_CHECK(lx_account_open(&f->accounts, (const uint8_t *)name, strlen(name),
        id, 1U, system ? LX_ACCOUNT_OPEN_GENESIS : LX_ACCOUNT_OPEN_CREDIT,
        NULL, account) == LXP_OK);
    STRICT_CHECK(lxp_ledger_bootstrap_balance(*account, f->asset.asset_id,
        (lxp_u128){0U, balance}, 0U) == LXP_OK);
    return 0;
}

static int strict_admission_open(strict_admission_fixture *f)
{
    uint8_t digest[32] = {0};
    uint8_t message[512];
    uint8_t material[144];
    size_t length;
    lxp_send send = {0};
    lxp_identity *identity;
    STRICT_CHECK(strict_sign(digest, f->signature, f->public_key) == 0);
    STRICT_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    f->asset.asset_id[0] = 3U;
    STRICT_CHECK(strict_account(f, "agent:did:key:strict:main", 100U, &f->payer) == 0);
    STRICT_CHECK(strict_account(f, "agent:did:key:recipient:main", 9U, &f->recipient) == 0);
    STRICT_CHECK(strict_account(f, "system:fees", 11U, &f->treasury) == 0);
    f->payer->has_authority_key = true;
    (void)memcpy(f->payer->authority_key, f->public_key, 32U);
    STRICT_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    STRICT_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    STRICT_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    f->parameters = 1U;
    STRICT_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
        &f->parameters, 1U) == LXP_OK);
    STRICT_CHECK(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()) == LXP_OK);
    STRICT_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    STRICT_CHECK(lx_asset_transfer_state(&f->asset, &f->transfer_asset) == LXP_OK);
    f->runtime = (lx_asset_runtime){&f->accounts, &f->asset, 1U,
        &f->transfer_asset, 1U, 7U, LXP_PROTOCOL_VERSION_STATE_COMMITMENT, false};
    STRICT_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET,
        &f->runtime) == LXP_OK);
    STRICT_CHECK(lxp_identity_register(&f->identities, strict_did,
        sizeof(strict_did) - 1U, f->public_key, &identity) == LXP_OK);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(f->authority.actor, identity->did_id, 32U);
    (void)memcpy(f->authority.principal, f->payer->id, 32U);
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    (void)memcpy(send.from, f->payer->id, 32U);
    (void)memcpy(send.to, f->recipient->id, 32U);
    (void)memcpy(send.asset, f->asset.asset_id, 32U);
    send.amount.lo = 1U;
    send.expires_at = 100U;
    send.idempotency_key[0] = 1U;
    send.authorization.kind = LXP_AUTH_OWNER;
    send.authorization.network_id = 7U;
    send.authorization.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    (void)memcpy(send.authorization.controller, send.from, 32U);
    (void)memcpy(send.authorization.public_key, f->public_key, 32U);
    (void)memcpy(material, send.from, 32U);
    (void)memcpy(material + 32U, send.to, 32U);
    (void)memcpy(material + 64U, send.asset, 32U);
    STRICT_CHECK(lxp_u128_to_be(send.amount, material + 96U) == LXP_OK);
    (void)memcpy(material + 112U, send.idempotency_key, 32U);
    STRICT_CHECK(lxp_hash_context_value(material, sizeof(material), send.context_hash) == LXP_OK);
    (void)memcpy(send.authorization.signed_context_hash, send.context_hash, 32U);
    STRICT_CHECK(lxp_send_authorization_message(&send, message, sizeof(message), &length) == LXP_OK);
    STRICT_CHECK(lxp_hash_signature_preimage(message, length, digest) == LXP_OK);
    STRICT_CHECK(strict_sign(digest, send.authorization.signature, f->public_key) == 0);
    STRICT_CHECK(lxp_send_encode(&send, f->payload, sizeof(f->payload), &length) == LXP_OK);
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->activity.network_id = 7U;
    f->activity.activity_type = LX_ASSET_SEND;
    f->activity.actor_did = (lxp_byte_span){strict_did, sizeof(strict_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.payload = (lxp_byte_span){f->payload, length};
    f->activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    f->activity.fee_limit.lo = 10U;
    (void)memcpy(f->activity.idempotency_key, send.idempotency_key, 32U);
    STRICT_CHECK(lxp_hash_payload(f->payload, length, f->activity.payload_hash) == LXP_OK);
    STRICT_CHECK(lxp_arena_init(&f->arena, f->arena_bytes, sizeof(f->arena_bytes)) == LXP_OK);
    f->fees.version = 1U;
    f->fees.base_fee.lo = 3U;
    f->fees.multiplier_basis_points = 10000U;
    f->execution.network_id = 7U;
    f->execution.epoch = 1U;
    f->execution.batch_number = 1U;
    f->execution.batch_timestamp_ms = 10U;
    f->execution.maximum_timestamp_window = 100U;
    f->execution.global_sequence = 1U;
    f->execution.recorded_module_version = 1U;
    f->execution.parameter_version = 1U;
    f->execution.identities = &f->identities;
    f->execution.authority = &f->authority;
    f->execution.fee_parameters = &f->fees;
    f->execution.fee_balance = f->payer->balance;
    f->execution.gas_limit = 100000U;
    f->execution.arena = &f->arena;
    f->execution.sequencer_private_key = strict_seed;
    (void)memset(&f->receipt, 0xa5, sizeof(f->receipt));
    (void)memcpy(&f->untouched_receipt, &f->receipt, sizeof(f->receipt));
    STRICT_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    return 0;
}

static int strict_state_unchanged(strict_admission_fixture *f,
                                  const lx_account before[3],
                                  const uint8_t root[32], size_t mark)
{
    uint8_t after[32];
    STRICT_CHECK(f->accounts.count == 3U);
    for (size_t i = 0U; i < f->accounts.count; ++i) {
        STRICT_CHECK(lxp_u128_cmp(f->accounts.accounts[i].balance, before[i].balance) == 0);
        STRICT_CHECK(f->accounts.accounts[i].next_sequence == before[i].next_sequence);
    }
    STRICT_CHECK(f->identities.count == 1U && f->identities.identities[0].next_sequence == 0U);
    STRICT_CHECK(f->state.next_sequence == 1U && !f->journal.open && f->journal.count == 0U);
    STRICT_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    STRICT_CHECK(memcmp(root, after, 32U) == 0);
    STRICT_CHECK(memcmp(root, f->kernel.current_state_root, 32U) == 0);
    STRICT_CHECK(memcmp(&f->receipt, &f->untouched_receipt, sizeof(f->receipt)) == 0);
    STRICT_CHECK(lxp_arena_mark(&f->arena) == mark);
    return 0;
}

static int codec_malformed_envelope_admission(void)
{
    static const lxp_result expected[] = {
        LXP_ERR_WRONG_NETWORK, LXP_ERR_PAYLOAD_HASH_MISMATCH,
        LXP_ERR_MALFORMED_ENVELOPE, LXP_ERR_MALFORMED_ENVELOPE,
        LXP_ERR_SEQUENCE_GAP
    };
    strict_admission_fixture *f = calloc(1U, sizeof(*f));
    lx_account before[3];
    uint8_t root[32];
    uint8_t digest[32];
    size_t mark;
    STRICT_CHECK(f != NULL && strict_admission_open(f) == 0);
    (void)memcpy(before, f->accounts.accounts, sizeof(before));
    (void)memcpy(root, f->kernel.current_state_root, 32U);
    mark = lxp_arena_mark(&f->arena);
    for (size_t i = 0U; i < sizeof(expected) / sizeof(expected[0]); ++i) {
        lxp_activity malformed = f->activity;
        lxp_activity received;
        lxp_arena wire_arena;
        lxp_byte_span wire;
        switch (i) {
        case 0U: ++malformed.network_id; break;
        case 1U: malformed.payload_hash[0] ^= 1U; break;
        case 2U: malformed.timestamp_bound = (lxp_timestamp_bound){100U, 1U}; break;
        case 3U: malformed.timestamp_bound.not_after = 102U; break;
        default: ++malformed.account_sequence; break;
        }
        STRICT_CHECK(lxp_activity_signing_preimage(&malformed, digest) == LXP_OK);
        STRICT_CHECK(strict_sign(digest, f->signature, f->public_key) == 0);
        STRICT_CHECK(lxp_arena_init(&wire_arena, f->wire_bytes, sizeof(f->wire_bytes)) == LXP_OK);
        STRICT_CHECK(lxp_activity_encode(&malformed, &wire_arena, &wire) == LXP_OK);
        STRICT_CHECK(lxp_activity_decode(wire.bytes, wire.length, &received) == LXP_OK);
        STRICT_CHECK(lxp_activity_verify_signature(&received) == LXP_OK);
        f->execution.signature_valid = true;
        STRICT_CHECK(lxp_kernel_execute_activity(&f->kernel, &received,
            &f->execution, &f->receipt) == expected[i]);
        STRICT_CHECK(strict_state_unchanged(f, before, root, mark) == 0);
    }
    {
        lxp_activity received;
        lxp_arena wire_arena;
        lxp_byte_span wire;
        lxp_admission_context context = {7U, 10U, 100U,
            f->payer->next_sequence, true, false, true};
        lxp_admission_result admitted;
        STRICT_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
        STRICT_CHECK(strict_sign(digest, f->signature, f->public_key) == 0);
        STRICT_CHECK(lxp_arena_init(&wire_arena, f->wire_bytes, sizeof(f->wire_bytes)) == LXP_OK);
        STRICT_CHECK(lxp_activity_encode(&f->activity, &wire_arena, &wire) == LXP_OK);
        STRICT_CHECK(lxp_activity_decode(wire.bytes, wire.length, &received) == LXP_OK);
        STRICT_CHECK(lxp_activity_verify_signature(&received) == LXP_OK);
        admitted = lxp_admit_activity(&received, &context);
        STRICT_CHECK(admitted.result_code == LXP_OK && admitted.assign_global_sequence &&
            admitted.consume_account_sequence && admitted.charge_fee);
        STRICT_CHECK(lxp_activity_decode(wire.bytes, wire.length - 1U, &received) == LXP_ERR_MALFORMED_ENVELOPE);
        STRICT_CHECK(strict_state_unchanged(f, before, root, mark) == 0);
        f->wire_bytes[wire.length] = 0U;
        STRICT_CHECK(lxp_activity_decode(f->wire_bytes, wire.length + 1U, &received) == LXP_ERR_MALFORMED_ENVELOPE);
        STRICT_CHECK(strict_state_unchanged(f, before, root, mark) == 0);
        f->wire_bytes[0] = 0U;
        f->wire_bytes[1] = 0U;
        STRICT_CHECK(lxp_activity_decode(f->wire_bytes, wire.length, &received) == LXP_ERR_MALFORMED_ENVELOPE);
        STRICT_CHECK(strict_state_unchanged(f, before, root, mark) == 0);
    }
    STRICT_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

int main(void)
{
    if (codec_primitive_strict() != 0) return 1;
    return codec_malformed_envelope_admission();
}
