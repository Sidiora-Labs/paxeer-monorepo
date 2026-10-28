#include "lxp_kernel_case.h"
#include "layerx/lx_asset.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"

#include <openssl/evp.h>
#include <stdlib.h>

typedef struct sequence_fixture {
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
    bool state_initialized;
} sequence_fixture;

static const uint8_t sequence_seed[32] = {1U};
static const uint8_t sequence_did[] = "did:key:alice";
static const uint8_t sequence_from[] =
    "agent:did:key:alice:asset:0300000000000000000000000000000000000000000000000000000000000000";
static const uint8_t sequence_to[] =
    "agent:did:key:bob:asset:0300000000000000000000000000000000000000000000000000000000000000";

static int sequence_sign(const uint8_t digest[32], uint8_t signature[64],
                         uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(
        EVP_PKEY_ED25519, NULL, sequence_seed, sizeof(sequence_seed));
    EVP_MD_CTX *context = EVP_MD_CTX_new();
    size_t key_length = 32U;
    size_t signature_length = 64U;
    int ok = key != NULL && context != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &key_length) == 1 &&
        key_length == 32U &&
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(context, signature, &signature_length, digest, 32U) == 1 &&
        signature_length == 64U;

    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(ok);
    return 0;
}

static int sequence_prepare_send(sequence_fixture *f, uint8_t idempotency_key)
{
    lxp_send send = {0};
    lxp_identity *identity;
    uint8_t material[144];
    uint8_t message[512];
    uint8_t digest[32];
    size_t message_length;
    size_t payload_length;

    LXP_CASE_CHECK(lxp_identity_resolve(&f->identities, sequence_did,
        sizeof(sequence_did) - 1U, &identity) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(sequence_from,
        sizeof(sequence_from) - 1U, send.from) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(sequence_to,
        sizeof(sequence_to) - 1U, send.to) == LXP_OK);
    memcpy(send.asset, f->asset.asset_id, 32U);
    send.amount.lo = 1U;
    send.sequence = f->accounts.accounts[0].next_sequence;
    send.expires_at = 100U;
    send.idempotency_key[0] = idempotency_key;
    send.authorization.kind = LXP_AUTH_OWNER;
    send.authorization.network_id = 7U;
    send.authorization.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    memcpy(send.authorization.controller, send.from, 32U);
    memcpy(material, send.from, 32U);
    memcpy(material + 32U, send.to, 32U);
    memcpy(material + 64U, send.asset, 32U);
    LXP_CASE_CHECK(lxp_u128_to_be(send.amount, material + 96U) == LXP_OK);
    memcpy(material + 112U, send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_context_value(material, sizeof(material),
        send.context_hash) == LXP_OK);
    memcpy(send.authorization.signed_context_hash, send.context_hash, 32U);
    memcpy(send.authorization.public_key, f->public_key, 32U);
    LXP_CASE_CHECK(lxp_send_authorization_message(&send, message,
        sizeof(message), &message_length) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message,
        message_length, digest) == LXP_OK);
    LXP_CASE_CHECK(sequence_sign(digest, send.authorization.signature,
        f->public_key) == 0);
    LXP_CASE_CHECK(lxp_send_encode(&send, f->payload, sizeof(f->payload),
        &payload_length) == LXP_OK);

    memset(&f->activity, 0, sizeof(f->activity));
    f->activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->activity.network_id = 7U;
    f->activity.activity_type = LX_ASSET_SEND;
    f->activity.actor_did = (lxp_byte_span){sequence_did,
        sizeof(sequence_did) - 1U};
    f->activity.authority = (lxp_byte_span){f->public_key, 32U};
    f->activity.signature = (lxp_byte_span){f->signature, 64U};
    f->activity.account_sequence = identity->next_sequence;
    f->activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    f->activity.payload = (lxp_byte_span){f->payload, payload_length};
    memcpy(f->activity.idempotency_key, send.idempotency_key, 32U);
    LXP_CASE_CHECK(lxp_hash_payload(f->payload, payload_length,
        f->activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&f->activity, digest) == LXP_OK);
    LXP_CASE_CHECK(sequence_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&f->activity) == LXP_OK);
    return 0;
}

static int sequence_fixture_init(sequence_fixture *f)
{
    uint8_t digest[32] = {0};
    uint8_t from_id[32];
    uint8_t to_id[32];
    lx_account *from;
    lx_account *to;
    lxp_identity *identity;

    LXP_CASE_CHECK(sequence_sign(digest, f->signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
        sizeof(f->arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 7U) == LXP_OK);
    f->state_initialized = true;
    f->parameters = 1U;
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
        &f->parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->kernel,
        lx_asset_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_register(&f->identities, sequence_did,
        sizeof(sequence_did) - 1U, f->public_key, &identity) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(sequence_from,
        sizeof(sequence_from) - 1U, from_id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_id_from_string(sequence_to,
        sizeof(sequence_to) - 1U, to_id) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, sequence_from,
        sizeof(sequence_from) - 1U, from_id, 1U, LX_ACCOUNT_OPEN_CREDIT,
        NULL, &from) == LXP_OK);
    LXP_CASE_CHECK(lx_account_open(&f->accounts, sequence_to,
        sizeof(sequence_to) - 1U, to_id, 1U, LX_ACCOUNT_OPEN_CREDIT,
        NULL, &to) == LXP_OK);
    f->asset.asset_id[0] = 3U;
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(from, f->asset.asset_id,
        (lxp_u128){0U, 10U}, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(to, f->asset.asset_id,
        (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    from->has_authority_key = true;
    memcpy(from->authority_key, f->public_key, 32U);
    memcpy(f->transfer_asset.asset_id, f->asset.asset_id, 32U);
    f->transfer_asset.registered = true;
    f->runtime = (lx_asset_runtime){&f->accounts, &f->asset, 1U,
        &f->transfer_asset, 1U, 7U, LXP_PROTOCOL_VERSION_STATE_COMMITMENT, false};
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_ASSET, &f->runtime) == LXP_OK);
    f->authority.kind = LXP_AUTHORITY_OWNER;
    memcpy(f->authority.principal, from_id, 32U);
    memcpy(f->authority.verified_key, f->public_key, 32U);
    memcpy(f->authority.actor, identity->did_id, 32U);
    f->fees.version = 1U;
    f->fees.multiplier_basis_points = 10000U;
    f->execution.network_id = 7U;
    f->execution.batch_number = 1U;
    f->execution.batch_timestamp_ms = 10U;
    f->execution.maximum_timestamp_window = 100U;
    f->execution.global_sequence = 7U;
    f->execution.recorded_module_version = 1U;
    f->execution.parameter_version = 1U;
    f->execution.signature_valid = true;
    f->execution.identities = &f->identities;
    f->execution.authority = &f->authority;
    f->execution.fee_parameters = &f->fees;
    f->execution.gas_limit = 10000U;
    f->execution.arena = &f->arena;
    f->execution.sequencer_private_key = sequence_seed;
    f->execution.batch_id[0] = 5U;
    LXP_CASE_CHECK(sequence_prepare_send(f, 1U) == 0);
    LXP_CASE_CHECK(lxp_state_root(&f->kernel,
        f->kernel.current_state_root) == LXP_OK);
    return 0;
}

static int sequence_refusal_case(sequence_fixture *f, bool reuse)
{
    lxp_state_proof proof;
    uint8_t initial_root[32];
    uint8_t before_root[32];
    uint8_t after_root[32];
    uint64_t next_sequence;
    uint64_t identity_sequence;
    uint64_t account_sequence;
    size_t idempotency_count;
    lxp_result result;

    LXP_CASE_CHECK(lxp_state_root_proof(&f->kernel, 0U,
        initial_root, &proof) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt) == LXP_OK);
    LXP_CASE_CHECK(f->receipt.result_code == LXP_OK);
    LXP_CASE_CHECK(f->receipt.global_sequence == 7U);
    LXP_CASE_CHECK(f->state.next_sequence == 8U);
    LXP_CASE_CHECK(lxp_u128_cmp(f->accounts.accounts[0].balance,
        (lxp_u128){0U, 9U}) == 0);
    LXP_CASE_CHECK(lxp_u128_cmp(f->accounts.accounts[1].balance,
        (lxp_u128){0U, 1U}) == 0);
    LXP_CASE_CHECK(sequence_prepare_send(f, 2U) == 0);
    next_sequence = f->state.next_sequence;
    identity_sequence = f->identities.identities[0].next_sequence;
    account_sequence = f->accounts.accounts[0].next_sequence;
    idempotency_count = f->state.idempotency_count;
    f->execution.global_sequence = reuse ? 7U : next_sequence + 2U;
    LXP_CASE_CHECK(lxp_state_root_proof(&f->kernel, 0U,
        before_root, &proof) == LXP_OK);
    LXP_CASE_CHECK(memcmp(initial_root, before_root, 32U) != 0);
    LXP_CASE_CHECK(memcmp(before_root, f->kernel.current_state_root, 32U) == 0);

    LXP_CASE_CHECK(lxp_state_journal_open(&f->state,
        f->execution.global_sequence, &f->journal) == LXP_ERR_SEQUENCE_GAP);
    LXP_CASE_CHECK(!f->journal.open);
    LXP_CASE_CHECK(f->state.next_sequence == next_sequence);
    LXP_CASE_CHECK(lxp_state_root_proof(&f->kernel, 0U,
        after_root, &proof) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before_root, after_root, 32U) == 0);

    result = lxp_kernel_execute_activity(&f->kernel, &f->activity,
        &f->execution, &f->receipt);

    LXP_CASE_CHECK(lxp_state_root_proof(&f->kernel, 0U,
        after_root, &proof) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before_root, after_root, 32U) == 0);
    LXP_CASE_CHECK(memcmp(before_root, f->kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(f->state.next_sequence == next_sequence);
    LXP_CASE_CHECK(f->identities.identities[0].next_sequence == identity_sequence);
    LXP_CASE_CHECK(f->accounts.accounts[0].next_sequence == account_sequence);
    LXP_CASE_CHECK(f->state.idempotency_count == idempotency_count);
    LXP_CASE_CHECK(!f->journal.open);
    LXP_CASE_CHECK(lxp_u128_cmp(f->accounts.accounts[0].balance,
        (lxp_u128){0U, 9U}) == 0);
    LXP_CASE_CHECK(lxp_u128_cmp(f->accounts.accounts[1].balance,
        (lxp_u128){0U, 1U}) == 0);
    if (result != LXP_ERR_SEQUENCE_GAP)
        fprintf(stderr, "%s: global_sequence=%llu next_sequence=%llu: "
                "expected LXP_ERR_SEQUENCE_GAP (%d), got %d\n",
                reuse ? "execute_refuses_sequence_reuse" :
                        "execute_refuses_sequence_gap",
                (unsigned long long)f->execution.global_sequence,
                (unsigned long long)next_sequence,
                (int)LXP_ERR_SEQUENCE_GAP, (int)result);
    LXP_CASE_CHECK(result == LXP_ERR_SEQUENCE_GAP);
    return 0;
}

static int sequence_run(bool reuse)
{
    sequence_fixture *f = calloc(1U, sizeof(*f));
    int result;

    LXP_CASE_CHECK(f != NULL);
    result = sequence_fixture_init(f);
    if (result == 0) result = sequence_refusal_case(f, reuse);
    if (f->state_initialized && lxp_state_store_destroy(&f->state) != LXP_OK)
        result = 1;
    free(f);
    return result;
}

static int execute_refuses_sequence_gap(void)
{
    return sequence_run(false);
}

static int execute_refuses_sequence_reuse(void)
{
    return sequence_run(true);
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "execute_refuses_sequence_gap", execute_refuses_sequence_gap },
        { "execute_refuses_sequence_reuse", execute_refuses_sequence_reuse }
    };
    const size_t count = sizeof(cases) / sizeof(cases[0]);
    int result = 0;
    int arg;

    if (argc <= 1) return lxp_kernel_case_main(cases, count, argc, argv);
    for (arg = 1; arg < argc; ++arg) {
        char *named[] = {argv[0], argv[arg]};
        int status = lxp_kernel_case_main(cases, count, 2, named);
        if (status > result) result = status;
    }
    return result;
}
