#include "lxp_kernel_case.h"

#include "layerx/lx_asset.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"

#include <openssl/evp.h>
#include <stdlib.h>

typedef struct retired_fixture {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lx_account_registry accounts;
    lx_asset_runtime runtime;
    lx_asset_record asset;
    lxp_transfer_asset_state transfer_asset;
    lxp_arena arena;
    uint8_t arena_bytes[4U * 1024U * 1024U];
    uint64_t parameters;
    uint8_t public_key[32];
    uint8_t signature[64];
    uint8_t payload[512];
    lxp_activity activity;
    lxp_kernel_execution execution;
    lxp_authority_resolved authority;
    lxp_fee_params fees;
    lxp_receipt receipt;
    lxp_send send;
} retired_fixture;

static const uint8_t retired_seed[32] = {1U};
static const uint8_t retired_did[] = "did:key:alice";

static int retired_sign(const uint8_t digest[32], uint8_t signature[64],
                    uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, retired_seed, 32U);
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    size_t key_length = 32U;
    size_t signature_length = 64U;
    int ok = key != NULL && ctx != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &key_length) == 1 &&
        key_length == 32U && EVP_DigestSignInit(ctx, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(ctx, signature, &signature_length, digest, 32U) == 1 &&
        signature_length == 64U;
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    return ok ? 0 : 1;
}

static int retired_fixture_prepare(retired_fixture *f)
{
    const uint16_t version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    static const uint8_t from_name[] = "agent:did:key:alice:main";
    static const uint8_t to_name[] = "agent:did:key:bob:main";
    uint8_t digest[32] = {0};
    uint8_t material[144];
    uint8_t message[512];
    size_t message_length;
    size_t payload_length;
    lx_account *from;
    lx_account *to;
    lxp_identity *identity;
    lxp_send send;
    (void)memset(&send, 0, sizeof(send));
    LXP_CASE_CHECK(retired_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes, sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
                              &f->parameters, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
                                        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel, lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_register(&f->identities, retired_did, sizeof(retired_did) - 1U,
                                  f->public_key, &identity) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(from_name, sizeof(from_name) - 1U, send.from) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(to_name, sizeof(to_name) - 1U, send.to) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_consume_sequence(identity, 0U) == LXP_OK);
    send.asset[0] = 3U;
    send.amount.lo = 1U;
    send.sequence = 1U;
    send.expires_at = 100U;
    send.idempotency_key[0] = 7U;
    send.authorization.kind = LXP_AUTH_OWNER;
    send.authorization.network_id = 7U;
    send.authorization.protocol_version = version;
    (void)memcpy(send.authorization.controller, send.from, 32U);
    (void)memcpy(material, send.from, 32U);
    (void)memcpy(material + 32U, send.to, 32U);
    (void)memcpy(material + 64U, send.asset, 32U);
    LXP_CASE_CHECK(lxp_u128_to_be(send.amount, material + 96U) == LXP_OK);
    (void)memcpy(material + 112U, send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_context_value(material, sizeof(material), send.context_hash) == LXP_OK);
    (void)memcpy(send.authorization.signed_context_hash, send.context_hash, 32U);
    (void)memcpy(send.authorization.public_key, f->public_key, 32U);
    LXP_CASE_CHECK(lxp_send_authorization_message(&send, message, sizeof(message), &message_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message, message_length, digest) == LXP_OK);
    LXP_CASE_CHECK(retired_sign(digest, send.authorization.signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_send_encode(&send, f->payload, sizeof(f->payload), &payload_length) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, from_name, sizeof(from_name) - 1U,
                            send.from, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &from) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, to_name, sizeof(to_name) - 1U,
                            send.to, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &to) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(from, send.asset, (lxp_u128){0U, 1000U}, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(to, send.asset, (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    from->has_authority_key = true;
    (void)memcpy(from->authority_key, f->public_key, 32U);
    (void)memcpy(f->asset.asset_id, send.asset, 32U);
    (void)memcpy(f->transfer_asset.asset_id, send.asset, 32U);
    f->transfer_asset.registered = true;
    f->asset.total_units.lo = 1000U;
    f->runtime = (lx_asset_runtime){&f->accounts, &f->asset, 1U,
        &f->transfer_asset, 1U, 7U, version, false};
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel, LXP_MODULE_ASSET, &f->runtime) == LXP_OK);
    f->activity.protocol_version = version;
    f->activity.network_id = 7U;
    f->activity.account_sequence = 1U;
    f->activity.activity_type = LX_ASSET_SEND;
    f->activity.actor_did = (lxp_byte_span){retired_did, sizeof(retired_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    f->activity.payload = (lxp_byte_span){f->payload, payload_length};
    (void)memcpy(f->activity.idempotency_key, send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, payload_length, f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    LXP_CASE_CHECK(retired_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(f->authority.principal, send.from, 32U);
    (void)memcpy(f->authority.verified_key, f->public_key, 32U);
    (void)memcpy(f->authority.actor, identity->did_id, 32U);
    f->fees.version = 1U;
    f->fees.multiplier_basis_points = 10000U;
    f->execution.network_id = 7U;
    f->execution.epoch = f->kernel.epoch;
    f->execution.batch_number = 1U;
    f->execution.batch_timestamp_ms = 10U;
    f->execution.maximum_timestamp_window = 100U;
    f->execution.global_sequence = 1U;
    f->execution.recorded_module_version = 1U;
    f->execution.parameter_version = 1U;
    f->execution.signature_valid = true;
    f->execution.identities = &f->identities;
    f->execution.authority = &f->authority;
    f->execution.fee_parameters = &f->fees;
    f->execution.gas_limit = 10000U;
    f->execution.arena = &f->arena;
    f->execution.sequencer_private_key = retired_seed;
    f->execution.batch_id[0] = 5U;
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, f->kernel.current_state_root) == LXP_OK);
    f->send = send;
    return 0;
}

static int retired_activity_sequence(retired_fixture *f, uint64_t sequence)
{
    uint8_t digest[32];
    uint8_t message[512];
    size_t message_length;
    size_t payload_length;

    f->send.sequence = sequence;
    f->activity.account_sequence = sequence;
    f->execution.global_sequence = f->state.next_sequence;
    LXP_CASE_CHECK(lxp_send_authorization_message(&f->send, message,
        sizeof(message), &message_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message,
        message_length, digest) == LXP_OK);
    LXP_CASE_CHECK(retired_sign(digest, f->send.authorization.signature,
                                f->public_key) == 0);
    LXP_CASE_CHECK(lxp_send_encode(&f->send, f->payload, sizeof(f->payload),
                                  &payload_length) == LXP_OK);
    f->activity.payload = (lxp_byte_span){f->payload, payload_length};
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, payload_length,
                                   f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    LXP_CASE_CHECK(retired_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    return 0;
}

static int retired_balances(retired_fixture *f, uint64_t transfers)
{
    lx_account *from;
    lx_account *to;
    static const uint8_t from_name[] = "agent:did:key:alice:main";
    static const uint8_t to_name[] = "agent:did:key:bob:main";

    LXP_CASE_CHECK(lx_account_lookup(&f->accounts, from_name,
        sizeof(from_name) - 1U, f->send.from, &from) == LXP_OK);
    LXP_CASE_CHECK(lx_account_lookup(&f->accounts, to_name,
        sizeof(to_name) - 1U, f->send.to, &to) == LXP_OK);
    LXP_CASE_CHECK(from->balance.hi == 0U &&
                    from->balance.lo == 1000U - transfers);
    LXP_CASE_CHECK(to->balance.hi == 0U && to->balance.lo == transfers);
    LXP_CASE_CHECK(from->next_sequence == transfers + 1U);
    LXP_CASE_CHECK(f->identities.identities[0].next_sequence == transfers + 1U);
    LXP_CASE_CHECK(f->state.next_sequence == transfers + 1U);
    LXP_CASE_CHECK(!f->journal.open && !f->journal.has_idempotency);
    return 0;
}

static int retired_execute(retired_fixture *f, uint64_t sequence)
{
    uint8_t root[32];

    LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(retired_activity_sequence(f, sequence) == 0);
    LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt) == LXP_OK);
    LXP_CASE_CHECK(f->receipt.result_code == LXP_OK);
    LXP_CASE_CHECK(f->receipt.global_sequence == sequence);
    LXP_CASE_CHECK(f->receipt.effects.count != 0U);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root, f->receipt.resulting_state_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(root, f->kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(retired_balances(f, sequence) == 0);
    return 0;
}

static int retired_fixture_close(retired_fixture *f)
{
    while (f->kernel.blob_count != 0U)
        free(f->kernel.blobs[--f->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int reused_key_next_sequence_executes(void)
{
    retired_fixture *f = calloc(1U, sizeof(*f));
    uint8_t first_activity_id[32];
    uint8_t first_root[32];
    uint8_t key[32];

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(retired_fixture_prepare(f) == 0);
    (void)memcpy(key, f->activity.idempotency_key, 32U);
    LXP_CASE_CHECK(retired_execute(f, 1U) == 0);
    (void)memcpy(first_activity_id, f->receipt.activity_id, 32U);
    (void)memcpy(first_root, f->kernel.current_state_root, 32U);
    LXP_CASE_CHECK(retired_execute(f, 2U) == 0);
    LXP_CASE_CHECK(memcmp(key, f->activity.idempotency_key, 32U) == 0);
    LXP_CASE_CHECK(memcmp(first_activity_id, f->receipt.activity_id, 32U) != 0);
    LXP_CASE_CHECK(memcmp(first_root, f->kernel.current_state_root, 32U) != 0);
    LXP_CASE_CHECK(f->state.idempotency_count == 0U);
    return retired_fixture_close(f);
}

static int reused_sequence_refused(void)
{
    retired_fixture *f = calloc(1U, sizeof(*f));
    uint8_t before[32];
    uint8_t after[32];

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(retired_fixture_prepare(f) == 0);
    LXP_CASE_CHECK(retired_execute(f, 1U) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    LXP_CASE_CHECK(retired_activity_sequence(f, 1U) == 0);
    LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt) == LXP_ERR_SEQUENCE_REUSED);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(memcmp(before, f->kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(retired_balances(f, 1U) == 0);
    LXP_CASE_CHECK(f->state.idempotency_count == 0U);
    return retired_fixture_close(f);
}

static int idempotency_table_stays_empty(void)
{
    retired_fixture *f = calloc(1U, sizeof(*f));
    uint8_t key[32];

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(retired_fixture_prepare(f) == 0);
    (void)memcpy(key, f->activity.idempotency_key, 32U);
    for (uint64_t sequence = 1U; sequence <= 64U; ++sequence) {
        LXP_CASE_CHECK(retired_execute(f, sequence) == 0);
        LXP_CASE_CHECK(memcmp(key, f->activity.idempotency_key, 32U) == 0);
        LXP_CASE_CHECK(f->state.idempotency_count == 0U);
        LXP_CASE_CHECK(f->state.idempotency == NULL);
    }
    LXP_CASE_CHECK(retired_balances(f, 64U) == 0);
    LXP_CASE_CHECK(f->state.idempotency_count == 0U);
    return retired_fixture_close(f);
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "reused_key_next_sequence_executes", reused_key_next_sequence_executes },
        { "reused_sequence_refused", reused_sequence_refused },
        { "idempotency_table_stays_empty", idempotency_table_stays_empty }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
