#include "layerx/lxp_crypto.h"

#include <openssl/evp.h>
#include <stdio.h>
#include <string.h>

int lxp_fuzz_signature(const uint8_t public_key[32], const uint8_t signature[64],
                       const void *message, size_t message_length);

static int decode_hex(const char *hex, uint8_t *out, size_t length)
{
    size_t i;
    if (strlen(hex) != length * 2U) return 1;
    for (i = 0U; i < length; ++i) {
        unsigned value;
        if (sscanf(hex + i * 2U, "%2x", &value) != 1) return 1;
        out[i] = (uint8_t)value;
    }
    return 0;
}

static int test_ed25519_independent_strict_vectors(void)
{
    static const struct {
        const char *name;
        const char *public_key;
        const char *signature;
        const char *message;
        bool valid;
    } vectors[] = {
#include "ed25519_strict_vectors.inc"
    };
    size_t i;
    for (i = 0U; i < sizeof(vectors) / sizeof(vectors[0]); ++i) {
        uint8_t public_key[32], signature[64], message[64];
        size_t message_length = strlen(vectors[i].message) / 2U;
        lxp_result expected = vectors[i].valid ? LXP_OK : LXP_ERR_BAD_SIGNATURE;
        if (message_length > sizeof(message) ||
            decode_hex(vectors[i].public_key, public_key, sizeof(public_key)) != 0 ||
            decode_hex(vectors[i].signature, signature, sizeof(signature)) != 0 ||
            decode_hex(vectors[i].message, message, message_length) != 0 ||
            lxp_ed25519_verify_raw(public_key, signature, message,
                                   message_length) != expected) {
            (void)fprintf(stderr, "strict Ed25519 vector failed: %s\n", vectors[i].name);
            return 1;
        }
    }
    (void)puts("test_ed25519_independent_strict_vectors passed");
    return 0;
}

static int test_ed25519_point_refusals(const uint8_t public_key[32],
                                      const uint8_t signature[64],
                                      const void *message, size_t message_length)
{
    static const char *const points[] = {
        "0000000000000000000000000000000000000000000000000000000000000000",
        "0100000000000000000000000000000000000000000000000000000000000000",
        "26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05",
        "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a",
        "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
        "edffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
        "eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
        "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
        "0200000000000000000000000000000000000000000000000000000000000000"
    };
    uint8_t digest[32];
    size_t i;
    unsigned sign;
    if (lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message, message_length,
                         digest) != LXP_OK) return 1;
    for (i = 0U; i < sizeof(points) / sizeof(points[0]); ++i) {
        for (sign = 0U; sign < 2U; ++sign) {
            uint8_t point[32], changed_signature[64];
            bool valid[3] = {false, true, true};
            lxp_ed25519_verify_item items[3];
            if (decode_hex(points[i], point, sizeof(point)) != 0) return 1;
            point[31] |= (uint8_t)(sign << 7U);
            (void)memcpy(changed_signature, signature, sizeof(changed_signature));
            (void)memcpy(changed_signature, point, sizeof(point));
            items[0] = (lxp_ed25519_verify_item){public_key, signature,
                LXP_DOMAIN_SIGNATURE_PREIMAGE, message, message_length};
            items[1] = items[0];
            items[1].public_key = point;
            items[2] = items[0];
            items[2].signature = changed_signature;
            if (lxp_ed25519_verify_raw(point, signature, digest, sizeof(digest)) !=
                    LXP_ERR_BAD_SIGNATURE ||
                lxp_ed25519_verify_raw(public_key, changed_signature, digest,
                                       sizeof(digest)) != LXP_ERR_BAD_SIGNATURE ||
                lxp_ed25519_verify(point, signature, LXP_DOMAIN_SIGNATURE_PREIMAGE,
                                   message, message_length) != LXP_ERR_BAD_SIGNATURE ||
                lxp_ed25519_verify(public_key, changed_signature,
                    LXP_DOMAIN_SIGNATURE_PREIMAGE, message, message_length) !=
                    LXP_ERR_BAD_SIGNATURE ||
                lxp_ed25519_verify_batch(items, 3U, valid) != LXP_ERR_BAD_SIGNATURE ||
                !valid[0] || valid[1] || valid[2]) {
                (void)fprintf(stderr, "Ed25519 point refusal failed: %zu sign %u\n", i, sign);
                return 1;
            }
        }
    }
    (void)puts("test_ed25519_point_refusals passed (public key and R, both signs)");
    return 0;
}

int main(void)
{
    uint8_t seed[32], public_key[32], signature[64], digest[32];
    size_t public_length = sizeof(public_key), signature_length = sizeof(signature);
    EVP_PKEY *private_key;
    EVP_MD_CTX *context;
    bool valid[2];
    lxp_ed25519_verify_item items[2];
    static const uint8_t message[] = {0U,0U,0U,17U,'L','a','y','e','r','X'};

    if (test_ed25519_independent_strict_vectors() != 0) return 1;
    if (decode_hex("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60", seed, 32U) != 0)
        return 1;
    private_key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, 32U);
    if (private_key == NULL ||
        EVP_PKEY_get_raw_public_key(private_key, public_key, &public_length) != 1 ||
        public_length != 32U ||
        lxp_hash_domain(LXP_DOMAIN_SIGNATURE_PREIMAGE, message, sizeof(message), digest) != LXP_OK)
        return 1;
    context = EVP_MD_CTX_new();
    if (context == NULL || EVP_DigestSignInit(context, NULL, NULL, NULL, private_key) != 1 ||
        EVP_DigestSign(context, signature, &signature_length, digest, sizeof(digest)) != 1 ||
        signature_length != 64U) return 1;
    EVP_MD_CTX_free(context);
    EVP_PKEY_free(private_key);
    if (lxp_ed25519_verify(public_key, signature, LXP_DOMAIN_SIGNATURE_PREIMAGE,
                           message, sizeof(message)) != LXP_OK ||
        lxp_ed25519_verify(public_key, signature, LXP_DOMAIN_ACTIVITY_ID,
                           message, sizeof(message)) != LXP_ERR_BAD_SIGNATURE ||
        lxp_fuzz_signature(public_key, signature, digest, sizeof(digest)) != 0)
        return 1;
    items[0] = (lxp_ed25519_verify_item){public_key, signature,
        LXP_DOMAIN_SIGNATURE_PREIMAGE, message, sizeof(message)};
    items[1] = items[0];
    items[1].domain = LXP_DOMAIN_ACTIVITY_ID;
    if (lxp_ed25519_verify_batch(items, 2U, valid) != LXP_ERR_BAD_SIGNATURE ||
        !valid[0] || valid[1]) return 1;
    if (test_ed25519_point_refusals(public_key, signature, message,
                                   sizeof(message)) != 0) return 1;
    (void)memset(signature + 32U, 0xff, 32U);
    return lxp_ed25519_verify(public_key, signature,
        LXP_DOMAIN_SIGNATURE_PREIMAGE, message, sizeof(message)) ==
        LXP_ERR_BAD_SIGNATURE ? 0 : 1;
}
