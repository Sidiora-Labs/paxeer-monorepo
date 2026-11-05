#include "layerx/lx_oracle.h"

#include "layerx/lxp_crypto.h"

#include <openssl/evp.h>
#include <string.h>

static void put_u64(uint8_t bytes[8], uint64_t value)
{
    size_t i;
    for (i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> ((7U - i) * 8U));
}

lxp_result lx_oracle_observation_encode(
    const lx_oracle_observation *observation, uint8_t *bytes,
    size_t capacity, size_t *length)
{
    if (observation == NULL || bytes == NULL || length == NULL ||
        capacity < LX_ORACLE_OBSERVATION_BYTES ||
        lxp_ct_is_zero(observation->market_id, 32U) ||
        observation->observation_sequence == 0U ||
        lxp_u128_is_zero(observation->price) ||
        observation->observed_at == 0U ||
        observation->source_identifier == 0U)
        return LXP_ERR_NON_CANONICAL;
    (void)memcpy(bytes, observation->market_id, 32U);
    put_u64(bytes + 32U, observation->observation_sequence);
    if (lxp_u128_to_be(observation->price, bytes + 40U) != LXP_OK)
        return LXP_ERR_INVALID_AMOUNT;
    put_u64(bytes + 56U, observation->observed_at);
    put_u64(bytes + 64U, observation->source_identifier);
    *length = LX_ORACLE_OBSERVATION_BYTES;
    return LXP_OK;
}

lxp_result lx_oracle_observation_sign(lx_oracle_observation *observation,
                                      const uint8_t private_key[32])
{
    static const uint8_t tag[] = "LXP:ORACLE:OBSERVATION:v1";
    uint8_t payload[LX_ORACLE_OBSERVATION_BYTES];
    uint8_t message[sizeof(tag) - 1U + LX_ORACLE_OBSERVATION_BYTES];
    uint8_t digest[32];
    size_t payload_length;
    size_t public_length = 32U;
    size_t signature_length = 64U;
    EVP_PKEY *key;
    EVP_MD_CTX *context;
    lxp_result status;
    if (observation == NULL || private_key == NULL)
        return LXP_ERR_NON_CANONICAL;
    status = lx_oracle_observation_encode(observation, payload,
                                          sizeof(payload), &payload_length);
    if (status != LXP_OK) return status;
    (void)memcpy(message, tag, sizeof(tag) - 1U);
    (void)memcpy(message + sizeof(tag) - 1U, payload, payload_length);
    status = lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message,
                             sizeof(message), digest);
    if (status != LXP_OK) return status;
    key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                       private_key, 32U);
    context = EVP_MD_CTX_new();
    if (key == NULL || context == NULL ||
        EVP_PKEY_get_raw_public_key(key, observation->oracle_public_key,
                                    &public_length) != 1 ||
        EVP_DigestSignInit(context, NULL, NULL, NULL, key) != 1 ||
        EVP_DigestSign(context, observation->signature, &signature_length,
                       digest, sizeof(digest)) != 1 ||
        signature_length != 64U) status = LXP_ERR_BAD_SIGNATURE;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    lxp_secure_zero(digest, sizeof(digest));
    return status;
}

