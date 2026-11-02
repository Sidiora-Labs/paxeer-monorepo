#include "layerx/lxp_fee.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_ledger.h"
#include "layerx/lxp_transfer.h"

#include <openssl/evp.h>
#include <stdio.h>
#include <string.h>

#define CHECK(condition) do { if (!(condition)) { \
    (void)fprintf(stderr, "payment check failed at line %d\n", __LINE__); \
    return 1; } } while (0)

static int public_from_seed(const uint8_t seed[32], uint8_t public_key[32])
{
    size_t length = 32U;
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                  seed, 32U);
    int result = key == NULL ||
        EVP_PKEY_get_raw_public_key(key, public_key, &length) != 1 || length != 32U;
    EVP_PKEY_free(key);
    return result;
}

static int sign_domain(const uint8_t seed[32], lxp_domain_tag_id domain,
                       const uint8_t *message, size_t message_length,
                       uint8_t signature[64])
{
    uint8_t digest[32];
    size_t signature_length = 64U;
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                  seed, 32U);
    EVP_MD_CTX *context;
    if (key == NULL || lxp_hash_domain(domain, message, message_length,
                                       digest) != LXP_OK) return 1;
    context = EVP_MD_CTX_new();
    if (context == NULL ||
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) != 1 ||
        EVP_DigestSign(context, signature, &signature_length, digest,
                       sizeof(digest)) != 1 || signature_length != 64U) return 1;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    return 0;
}

static int sign_send(lxp_send *send, const uint8_t seed[32])
{
    uint8_t message[512];
    size_t length;
    if (lxp_send_authorization_message(send, message, sizeof(message),
                                        &length) != LXP_OK) return 1;
    return sign_domain(seed, LXP_DOMAIN_SIGNATURE_PREIMAGE, message, length,
                       send->authorization.signature);
}

int main(void)
{
    static const uint8_t payer_seed[32] = { 1U };
    static const uint8_t receiver_seed[32] = { 2U };
    lx_account_registry accounts;
    lx_account *from;
    lx_account *to;
    const char *from_name = "agent:did:key:payer:main";
    const char *to_name = "agent:did:key:merchant:main";
    uint8_t asset_id[32] = { 4U };
    lxp_send send = {0};
    lxp_send decoded;
    uint8_t encoded[1024];
    size_t encoded_length;
    lxp_send_store idempotency = {0};
    lxp_transfer_asset_state asset = {0};
    lxp_send_environment environment;
    lxp_send_receipt_projection receipt;
    lxp_authority_scope recurring = {0};

    CHECK(lx_account_registry_init(&accounts) == LXP_OK);
    CHECK(lx_account_id_from_string((const uint8_t *)from_name, strlen(from_name),
                                     send.from) == LXP_OK);
    CHECK(lx_account_id_from_string((const uint8_t *)to_name, strlen(to_name),
                                     send.to) == LXP_OK);
    CHECK(lx_account_open(&accounts, (const uint8_t *)from_name, strlen(from_name),
                           send.from, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &from) == LXP_OK);
    CHECK(lx_account_open(&accounts, (const uint8_t *)to_name, strlen(to_name),
                           send.to, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &to) == LXP_OK);
    CHECK(lxp_ledger_bootstrap_balance(from, asset_id, (lxp_u128){0U, 100U}, 0U) == LXP_OK);
    CHECK(lxp_ledger_bootstrap_balance(to, asset_id, (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    CHECK(public_from_seed(payer_seed, from->authority_key) == 0);
    CHECK(public_from_seed(receiver_seed, to->authority_key) == 0);
    from->has_authority_key = true;
    to->has_authority_key = true;
    (void)memcpy(send.asset, asset_id, 32U);
    send.amount = (lxp_u128){0U, 30U};
    send.expires_at = 100U;
    send.idempotency_key[0] = 1U;
    send.context_hash[0] = 8U;
    send.authorization.kind = LXP_AUTH_OWNER;
    (void)memcpy(send.authorization.controller, from->id, 32U);
    (void)memcpy(send.authorization.public_key, from->authority_key, 32U);
    (void)memcpy(send.authorization.signed_context_hash, send.context_hash, 32U);
    send.authorization.network_id = 7U;
    send.authorization.protocol_version = LXP_PROTOCOL_VERSION;
    CHECK(sign_send(&send, payer_seed) == 0);
    CHECK(lxp_send_encode(&send, encoded, sizeof(encoded), &encoded_length) == LXP_OK);
    CHECK(lxp_send_decode(encoded, encoded_length, &decoded) == LXP_OK);
    CHECK(memcmp(&send, &decoded, sizeof(send)) == 0);
    CHECK(lxp_send_decode(encoded, encoded_length - 1U, &decoded) == LXP_ERR_MALFORMED_SEND);
    (void)memcpy(asset.asset_id, asset_id, 32U);
    asset.registered = true;
    environment = (lxp_send_environment){ &accounts, &asset, 1U,
        &idempotency, 10U, 7U, LXP_PROTOCOL_VERSION, NULL, false };
    CHECK(lxp_send_execute(&send, &environment, &receipt) == LXP_OK);
    CHECK(from->balance.lo == 70U && to->balance.lo == 30U &&
          from->next_sequence == 1U && to->next_sequence == 0U);
    CHECK(lxp_send_execute(&send, &environment, &receipt) == LXP_ERR_SEQUENCE_REUSED);
    CHECK(from->balance.lo == 70U && to->balance.lo == 30U);
    send.sequence = UINT64_MAX;
    send.idempotency_key[0] = 3U;
    CHECK(lxp_ledger_bootstrap_balance(from, asset_id, (lxp_u128){0U, 70U},
                                       UINT64_MAX) == LXP_OK);
    CHECK(sign_send(&send, payer_seed) == 0);
    CHECK(lxp_send_execute(&send, &environment, &receipt) == LXP_ERR_SEQUENCE_EXHAUSTED);
    CHECK(from->balance.lo == 70U && to->balance.lo == 30U &&
          from->next_sequence == UINT64_MAX && to->next_sequence == 0U);
    CHECK(lxp_ledger_bootstrap_balance(from, asset_id, (lxp_u128){0U, 70U}, 1U) == LXP_OK);
    send.sequence = 1U;
    send.amount = (lxp_u128){0U, 20U};
    send.idempotency_key[0] = 4U;
    CHECK(sign_send(&send, payer_seed) == 0);
    CHECK(lxp_send_execute(&send, &environment, &receipt) == LXP_OK);
    CHECK(from->balance.lo == 50U && to->balance.lo == 50U &&
          from->next_sequence == 2U && to->next_sequence == 0U);
    recurring.period_length = 10U;
    recurring.maximum_per_activity = (lxp_u128){0U, 30U};
    recurring.maximum_per_period = (lxp_u128){0U, 50U};
    CHECK(lxp_authority_charge_allowance(&recurring, (lxp_u128){0U, 30U},
                                          12U) == LXP_OK);
    CHECK(lxp_authority_charge_allowance(&recurring, (lxp_u128){0U, 21U},
                                          13U) == LXP_ERR_GRANT_EXHAUSTED);
    CHECK(lxp_authority_charge_allowance(&recurring, (lxp_u128){0U, 30U},
                                          20U) == LXP_OK);
    CHECK(recurring.spent_total.lo == 60U &&
          recurring.spent_this_period.lo == 30U && recurring.period_start == 20U);
    {
        const size_t window = (size_t)LXP_SEND_STORE_CAPACITY;
        const size_t beyond = window + 2U;
        const uint64_t window_bytes =
            (uint64_t)window * (uint64_t)LXP_SEND_HISTORY_RECORD_BYTES;
        lxp_send bulk;
        lxp_send oldest_send;
        lxp_send_store spilling;
        lxp_send_environment spilling_environment;
        lxp_meter_ctx meter;
        lxp_send_receipt_projection oldest;
        lxp_send_receipt_projection replay;
        size_t total = 0U;
        size_t index;

        if (lxp_send_store_init(&spilling, &meter) != LXP_OK ||
            lxp_meter_init(&meter, UINT64_MAX, UINT64_MAX,
                           (lxp_u128){ 0U, 1U }, (lxp_u128){ 0U, UINT64_MAX },
                           1U, true) != LXP_OK ||
            lxp_ledger_bootstrap_balance(from, asset_id,
                                         (lxp_u128){ 0U, 200U }, 0U) !=
                LXP_OK ||
            lxp_ledger_bootstrap_balance(to, asset_id, (lxp_u128){ 0U, 0U },
                                         0U) != LXP_OK) return 1;
        bulk = send;
        bulk.amount = (lxp_u128){ 0U, 1U };
        spilling_environment = (lxp_send_environment){ &accounts, &asset,
            1U, &spilling, 10U, 7U, LXP_PROTOCOL_VERSION, NULL, false };
        (void)memset(&oldest, 0, sizeof(oldest));
        (void)memset(&oldest_send, 0, sizeof(oldest_send));
        for (index = 0U; index < beyond; ++index) {
            bulk.sequence = (uint64_t)index;
            bulk.idempotency_key[0] = 0U;
            bulk.idempotency_key[1] = (uint8_t)(index + 1U);
            if (sign_send(&bulk, payer_seed) != 0 ||
                lxp_send_execute(&bulk, &spilling_environment, &receipt) !=
                    LXP_OK) return 1;
            if (index == 0U) {
                oldest = receipt;
                oldest_send = bulk;
            }
        }
        if (lxp_send_store_total(&spilling, &total) != LXP_OK ||
            total != beyond || spilling.count != beyond - window ||
            spilling.history == NULL || spilling.history->count != window ||
            spilling.history->stored_bytes != window_bytes ||
            meter.net_storage_bytes != window_bytes ||
            from->balance.lo != 200U - beyond || to->balance.lo != beyond ||
            from->next_sequence != beyond || to->next_sequence != 0U) return 1;
        if (lxp_send_execute(&oldest_send, &spilling_environment,
                                &receipt) != LXP_ERR_SEQUENCE_REUSED ||
            from->balance.lo != 200U - beyond) return 1;
        bulk.sequence = (uint64_t)beyond;
        bulk.idempotency_key[0] = 0U;
        bulk.idempotency_key[1] = 1U;
        oldest.replayed = true;
        if (sign_send(&bulk, payer_seed) != 0 ||
            lxp_send_execute(&bulk, &spilling_environment, &replay) !=
                LXP_ERR_IDEMPOTENT_REPLAY ||
            memcmp(&replay, &oldest, sizeof(replay)) != 0 ||
            from->balance.lo != 200U - beyond || to->balance.lo != beyond ||
            from->next_sequence != beyond || to->next_sequence != 0U) return 1;
        lxp_send_store_release(&spilling);
        if (spilling.count != 0U || spilling.history != NULL) return 1;
    }
    lxp_send_store_release(&idempotency);
    lx_account_registry_release(&accounts);
    return 0;
}
