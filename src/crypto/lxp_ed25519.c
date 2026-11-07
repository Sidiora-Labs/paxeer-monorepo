#include "layerx/lxp_crypto.h"

#include <openssl/evp.h>
#include <string.h>

static int little_endian_less(const uint8_t *left, const uint8_t *right,
                              size_t length)
{
    size_t i = length;
    while (i-- != 0U) {
        if (left[i] < right[i]) return 1;
        if (left[i] > right[i]) return 0;
    }
    return 0;
}

static int scalar_is_canonical(const uint8_t scalar[32])
{
    static const uint8_t order[32] = {
        0xedU,0xd3U,0xf5U,0x5cU,0x1aU,0x63U,0x12U,0x58U,
        0xd6U,0x9cU,0xf7U,0xa2U,0xdeU,0xf9U,0xdeU,0x14U,
        0U,0U,0U,0U,0U,0U,0U,0U,0U,0U,0U,0U,0U,0U,0U,0x10U
    };
    return little_endian_less(scalar, order, sizeof(order));
}

bool lxp_ed25519_pubkey_is_canonical(const uint8_t public_key[32])
{
    static const uint8_t field_prime[32] = {
        0xedU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,
        0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,
        0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,
        0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0x7fU
    };
    uint8_t y[32];
    uint8_t aggregate = 0U;
    size_t i;
    if (public_key == NULL) return false;
    (void)memcpy(y, public_key, sizeof(y));
    y[31] &= 0x7fU;
    for (i = 0U; i < sizeof(y); ++i) aggregate |= y[i];
    if (aggregate == 0U || (y[0] == 1U && lxp_ct_is_zero(y + 1U, 31U)))
        return false;
    return little_endian_less(y, field_prime, sizeof(y)) != 0;
}

static bool point_is_strict(const uint8_t encoded[32])
{
    /* The other small-order y coordinates from libsodium 1.0.20's
       ge25519_has_small_order; canonical checks already reject y = 0, 1, p, p+1.
       Masking x's sign covers both encodings of every torsion point. */
    static const uint8_t torsion_y[3][32] = {
        {0x26U,0xe8U,0x95U,0x8fU,0xc2U,0xb2U,0x27U,0xb0U,
         0x45U,0xc3U,0xf4U,0x89U,0xf2U,0xefU,0x98U,0xf0U,
         0xd5U,0xdfU,0xacU,0x05U,0xd3U,0xc6U,0x33U,0x39U,
         0xb1U,0x38U,0x02U,0x88U,0x6dU,0x53U,0xfcU,0x05U},
        {0xc7U,0x17U,0x6aU,0x70U,0x3dU,0x4dU,0xd8U,0x4fU,
         0xbaU,0x3cU,0x0bU,0x76U,0x0dU,0x10U,0x67U,0x0fU,
         0x2aU,0x20U,0x53U,0xfaU,0x2cU,0x39U,0xccU,0xc6U,
         0x4eU,0xc7U,0xfdU,0x77U,0x92U,0xacU,0x03U,0x7aU},
        {0xecU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,
         0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,
         0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,
         0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0xffU,0x7fU}
    };
    uint8_t y[32];
    size_t i;
    if (!lxp_ed25519_pubkey_is_canonical(encoded)) return false;
    (void)memcpy(y, encoded, sizeof(y));
    y[31] &= 0x7fU;
    for (i = 0U; i < sizeof(torsion_y) / sizeof(torsion_y[0]); ++i)
        if (lxp_ct_memcmp(y, torsion_y[i], sizeof(y)) == 0) return false;
    return true;
}

lxp_result lxp_ed25519_verify_raw(const uint8_t public_key[32],
                                  const uint8_t signature[64],
                                  const void *message, size_t message_length)
{
    EVP_PKEY *key;
    EVP_MD_CTX *context;
    int verified;
    if (public_key == NULL || signature == NULL ||
        (message == NULL && message_length != 0U) ||
        !point_is_strict(public_key) || !point_is_strict(signature) ||
        !scalar_is_canonical(signature + 32U)) return LXP_ERR_BAD_SIGNATURE;
    key = EVP_PKEY_new_raw_public_key(EVP_PKEY_ED25519, NULL, public_key, 32U);
    if (key == NULL) return LXP_ERR_BAD_SIGNATURE;
    context = EVP_MD_CTX_new();
    if (context == NULL) { EVP_PKEY_free(key); return LXP_ERR_BAD_SIGNATURE; }
    verified = EVP_DigestVerifyInit(context, NULL, NULL, NULL, key) == 1 &&
               EVP_DigestVerify(context, signature, 64U,
                                (const uint8_t *)message, message_length) == 1;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(key);
    return verified ? LXP_OK : LXP_ERR_BAD_SIGNATURE;
}

lxp_result lxp_ed25519_verify(const uint8_t public_key[32],
                              const uint8_t signature[64],
                              lxp_domain_tag_id domain, const void *message,
                              size_t message_length)
{
    uint8_t preimage[32];
    lxp_result result = lxp_hash_domain(domain, message, message_length, preimage);
    if (result == LXP_OK)
        result = lxp_ed25519_verify_raw(public_key, signature, preimage,
                                        sizeof(preimage));
    lxp_secure_zero(preimage, sizeof(preimage));
    return result;
}

lxp_result lxp_ed25519_verify_batch(const lxp_ed25519_verify_item *items,
                                    size_t count, bool *valid)
{
    size_t i;
    bool all_valid = true;
    if ((items == NULL || valid == NULL) && count != 0U)
        return LXP_ERR_NON_CANONICAL;
    for (i = 0U; i < count; ++i) {
        valid[i] = lxp_ed25519_verify(items[i].public_key, items[i].signature,
                                      items[i].domain, items[i].message,
                                      items[i].message_length) == LXP_OK;
        all_valid = all_valid && valid[i];
    }
    return all_valid ? LXP_OK : LXP_ERR_BAD_SIGNATURE;
}
