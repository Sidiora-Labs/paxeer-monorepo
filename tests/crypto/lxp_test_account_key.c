#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"
#include "lxp_kernel_case.h"

#include <openssl/evp.h>
#include <string.h>

lxp_result lxp_hash_account_id(const void *data, size_t length, uint8_t out[32]);
bool lxp_ed25519_pubkey_is_canonical(const uint8_t public_key[32]);

static int hash_account_id_domain_separated(void)
{
    static const uint8_t input[] = {
        0U, 'L', 'a', 'y', 'e', 'r', 'X', 0U, 0x80U, 0xffU
    };
    uint8_t account_id[LXP_HASH_SIZE] = {0U};
    uint8_t repeated[LXP_HASH_SIZE];
    uint8_t context_value[LXP_HASH_SIZE];
    uint8_t sha256[LXP_HASH_SIZE];

    (void)memset(repeated, 0xff, sizeof(repeated));
    LXP_CASE_CHECK(lxp_hash_account_id(input, sizeof(input), account_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_account_id(input, sizeof(input), repeated) == LXP_OK);
    LXP_CASE_CHECK(memcmp(account_id, repeated, sizeof(account_id)) == 0);
    LXP_CASE_CHECK(lxp_hash_context_value(input, sizeof(input), context_value) == LXP_OK);
    LXP_CASE_CHECK(lxp_hash_sha256(input, sizeof(input), sha256) == LXP_OK);
    LXP_CASE_CHECK(memcmp(account_id, context_value, sizeof(account_id)) != 0);
    LXP_CASE_CHECK(memcmp(account_id, sha256, sizeof(account_id)) != 0);
    return 0;
}

static int ed25519_pubkey_canonical(void)
{
    static const uint8_t seed[32] = {
        0x9dU, 0x61U, 0xb1U, 0x9dU, 0xefU, 0xfdU, 0x5aU, 0x60U,
        0xbaU, 0x84U, 0x4aU, 0xf4U, 0x92U, 0xecU, 0x2cU, 0xc4U,
        0x44U, 0x49U, 0xc5U, 0x69U, 0x7bU, 0x32U, 0x69U, 0x19U,
        0x70U, 0x3bU, 0xacU, 0x03U, 0x1cU, 0xaeU, 0x7fU, 0x60U
    };
    static const uint8_t expected_public_key[32] = {
        0xd7U, 0x5aU, 0x98U, 0x01U, 0x82U, 0xb1U, 0x0aU, 0xb7U,
        0xd5U, 0x4bU, 0xfeU, 0xd3U, 0xc9U, 0x64U, 0x07U, 0x3aU,
        0x0eU, 0xe1U, 0x72U, 0xf3U, 0xdaU, 0xa6U, 0x23U, 0x25U,
        0xafU, 0x02U, 0x1aU, 0x68U, 0xf7U, 0x07U, 0x51U, 0x1aU
    };
    uint8_t noncanonical[32];
    uint8_t identity[32] = {1U};
    uint8_t public_key[32];
    size_t public_length = sizeof(public_key);
    unsigned low_byte;
    unsigned sign;
    int extracted;
    EVP_PKEY *private_key;

    for (sign = 0U; sign < 2U; ++sign) {
        (void)memset(noncanonical, 0xff, sizeof(noncanonical));
        noncanonical[31] = (uint8_t)(0x7fU | (sign << 7U));
        for (low_byte = 0xedU; low_byte <= 0xffU; ++low_byte) {
            noncanonical[0] = (uint8_t)low_byte;
            LXP_CASE_CHECK(!lxp_ed25519_pubkey_is_canonical(noncanonical));
        }
        identity[31] = (uint8_t)(sign << 7U);
        LXP_CASE_CHECK(!lxp_ed25519_pubkey_is_canonical(identity));
    }

    private_key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                              seed, sizeof(seed));
    LXP_CASE_CHECK(private_key != NULL);
    extracted = EVP_PKEY_get_raw_public_key(private_key, public_key, &public_length);
    EVP_PKEY_free(private_key);
    LXP_CASE_CHECK(extracted == 1);
    LXP_CASE_CHECK(public_length == sizeof(public_key));
    LXP_CASE_CHECK(memcmp(public_key, expected_public_key, sizeof(public_key)) == 0);
    LXP_CASE_CHECK(lxp_ed25519_pubkey_is_canonical(public_key));
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"hash_account_id_domain_separated", hash_account_id_domain_separated},
        {"ed25519_pubkey_canonical", ed25519_pubkey_canonical}
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
