#define OPENSSL_API_COMPAT 0x10100000L
#include "layerx/lxp_crypto.h"

#include <openssl/bn.h>
#include <openssl/ec.h>
#include <openssl/ecdsa.h>
#include <openssl/obj_mac.h>
#include <stdio.h>
#include <string.h>

static int compact_signature(const ECDSA_SIG *signature, uint8_t out[64])
{
    const BIGNUM *r,*s; ECDSA_SIG_get0(signature,&r,&s);
    return BN_bn2binpad(r,out,32)==32 && BN_bn2binpad(s,out+32U,32)==32 ? 0 : 1;
}

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

static int test_secp256k1_eip155_recovery_vector(void)
{
    /* EIP-155 example: chain ID 1, v = 37 gives recovery ID 0.
       The published signing key is 0x46 repeated 32 times. */
    static const char *const signature_hex =
        "28ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276"
        "67cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83";
    static const uint8_t signer[20] = {
        0x9dU,0x8aU,0x62U,0xf6U,0x56U,0xa8U,0xd1U,0x61U,0x5cU,0x12U,
        0x94U,0xfdU,0x71U,0xe9U,0xcfU,0xb3U,0xe4U,0x85U,0x5aU,0x4fU
    };
    static const char *const signing_data_hex =
        "ec098504a817c800825208943535353535353535353535353535353535353535"
        "880de0b6b3a764000080018080";
    uint8_t signature[64], digest[32], expected_digest[32], data[45];
    uint8_t address[20], public_key[65], changed[64];
    EC_GROUP *group = EC_GROUP_new_by_curve_name(NID_secp256k1);
    EC_POINT *point = group == NULL ? NULL : EC_POINT_new(group);
    BIGNUM *private_value = NULL;
    BIGNUM *order = BN_new(), *high_s = BN_new();
    int result = 1;
    if (group == NULL || point == NULL || order == NULL || high_s == NULL ||
        BN_hex2bn(&private_value,
            "4646464646464646464646464646464646464646464646464646464646464646") == 0 ||
        decode_hex(signature_hex, signature, sizeof(signature)) != 0 ||
        decode_hex(signing_data_hex, data, sizeof(data)) != 0 ||
        decode_hex("daf5a779ae972f972197303d7b574746c7ef83eadac0f2791ad23db92e4c8e53",
                   expected_digest, sizeof(expected_digest)) != 0 ||
        lxp_keccak256(data, sizeof(data), digest) != LXP_OK ||
        memcmp(digest, expected_digest, sizeof(digest)) != 0 ||
        !lxp_secp256k1_sig_is_low_s(signature) ||
        lxp_secp256k1_recover_address(signature, 0U, digest, address) != LXP_OK ||
        memcmp(address, signer, sizeof(signer)) != 0 ||
        EC_POINT_mul(group, point, private_value, NULL, NULL, NULL) != 1 ||
        EC_POINT_point2oct(group, point, POINT_CONVERSION_UNCOMPRESSED,
                           public_key, sizeof(public_key), NULL) != sizeof(public_key) ||
        lxp_secp256k1_address(public_key, sizeof(public_key), address) != LXP_OK ||
        memcmp(address, signer, sizeof(signer)) != 0 ||
        lxp_secp256k1_recover_address(signature, 1U, digest, address) != LXP_OK ||
        memcmp(address, signer, sizeof(signer)) == 0 ||
        lxp_secp256k1_recover_address(signature, 4U, digest, address) != LXP_ERR_BAD_SIGNATURE ||
        lxp_secp256k1_recover_address(signature, 255U, digest, address) != LXP_ERR_BAD_SIGNATURE)
        goto cleanup;
    digest[0] ^= 1U;
    if (lxp_secp256k1_recover_address(signature, 0U, digest, address) != LXP_OK ||
        memcmp(address, signer, sizeof(signer)) == 0) goto cleanup;
    (void)memcpy(changed, signature, sizeof(changed));
    if (EC_GROUP_get_order(group, order, NULL) != 1 ||
        BN_bin2bn(signature + 32U, 32, high_s) == NULL ||
        BN_sub(high_s, order, high_s) != 1 ||
        BN_bn2binpad(high_s, changed + 32U, 32) != 32 ||
        lxp_secp256k1_sig_is_low_s(changed) ||
        lxp_secp256k1_recover_address(changed, 1U, expected_digest, address) != LXP_ERR_BAD_SIGNATURE)
        goto cleanup;
    (void)memset(changed + 32U, 0, 32U);
    if (lxp_secp256k1_sig_is_low_s(changed) ||
        lxp_secp256k1_recover_address(changed, 0U, expected_digest, address) != LXP_ERR_BAD_SIGNATURE ||
        BN_bn2binpad(order, changed + 32U, 32) != 32 ||
        lxp_secp256k1_recover_address(changed, 0U, expected_digest, address) != LXP_ERR_BAD_SIGNATURE)
        goto cleanup;
    result = 0;
cleanup:
    EC_POINT_free(point);
    EC_GROUP_free(group);
    BN_free(private_value);
    BN_free(order);
    BN_free(high_s);
    if (result == 0) (void)puts("test_secp256k1_eip155_recovery_vector passed");
    else (void)fputs("EIP-155 recovery vector failed\n", stderr);
    return result;
}

int main(void)
{
    EC_KEY *key=EC_KEY_new_by_curve_name(NID_secp256k1);
    BIGNUM *private_value=BN_new();
    const EC_GROUP *group;
    EC_POINT *public_point;
    uint8_t public_key[33], signature[64], digest[32], address[20];
    static const uint8_t signer[20] = {
        0x7eU,0x5fU,0x45U,0x52U,0x09U,0x1aU,0x69U,0x12U,0x5dU,0x5dU,
        0xfcU,0xb7U,0xb8U,0xc2U,0x65U,0x90U,0x29U,0x39U,0x5bU,0xdfU
    };
    size_t public_length;
    ECDSA_SIG *signed_digest;
    static const uint8_t message[]={'d','i','d',':','l','x',0U,0U,0U,17U};
    uint8_t recovery;
    int found=0;
    if (test_secp256k1_eip155_recovery_vector() != 0) return 1;
    if (!key||!private_value||BN_set_word(private_value,1U)!=1 || EC_KEY_set_private_key(key,private_value)!=1) return 1;
    group=EC_KEY_get0_group(key); public_point=EC_POINT_new(group);
    if (!public_point || EC_POINT_mul(group,public_point,private_value,NULL,NULL,NULL)!=1 ||
        EC_KEY_set_public_key(key,public_point)!=1) return 1;
    public_length=EC_POINT_point2oct(group,public_point,POINT_CONVERSION_COMPRESSED,public_key,sizeof(public_key),NULL);
    if (public_length!=33U || lxp_hash_domain(LXP_DOMAIN_CHECKPOINT_CERTIFICATE,message,sizeof(message),digest)!=LXP_OK) return 1;
    signed_digest=ECDSA_do_sign(digest,32,key); if (!signed_digest || compact_signature(signed_digest,signature)!=0) return 1;
    if (!lxp_secp256k1_sig_is_low_s(signature)) {
        const BIGNUM *r,*s; BIGNUM *low; BIGNUM *order=BN_new(); ECDSA_SIG_get0(signed_digest,&r,&s);
        if (!order || EC_GROUP_get_order(group, order, NULL) != 1) return 1;
        low = BN_new();
        if (!low || BN_sub(low, order, s) != 1) return 1;
        if (BN_bn2binpad(low, signature + 32U, 32) != 32) return 1;
        BN_free(low);
        BN_free(order);
    }
    if (lxp_secp256k1_verify(public_key,public_length,signature,LXP_DOMAIN_CHECKPOINT_CERTIFICATE,message,sizeof(message))!=LXP_OK ||
        lxp_secp256k1_verify(public_key,public_length,signature,LXP_DOMAIN_ACTIVITY_ID,message,sizeof(message))!=LXP_ERR_BAD_SIGNATURE) return 1;
    for(recovery=0U;recovery<4U;++recovery) if(lxp_secp256k1_recover_address(signature,recovery,digest,address)==LXP_OK){
        if (memcmp(address, signer, sizeof(signer)) == 0) ++found;
    }
    if(found != 1 || lxp_secp256k1_address(public_key, public_length, address) != LXP_OK ||
       memcmp(address, signer, sizeof(signer)) != 0 ||
       lxp_secp256k1_recover_address(signature,4U,digest,address)!=LXP_ERR_BAD_SIGNATURE) return 1;
    {
        BIGNUM *order = BN_new();
        if (order == NULL || EC_GROUP_get_order(group, order, NULL) != 1 ||
            BN_bn2binpad(order, signature, 32) != 32) return 1;
        for (recovery = 0U; recovery < 4U; ++recovery)
            if (lxp_secp256k1_recover_address(signature, recovery, digest,
                                              address) != LXP_ERR_BAD_SIGNATURE)
                return 1;
        (void)memset(signature, 0, 32U);
        if (lxp_secp256k1_recover_address(signature, 0U, digest, address) !=
            LXP_ERR_BAD_SIGNATURE) return 1;
        if (BN_rshift1(order, order) != 1 ||
            BN_bn2binpad(order, signature + 32U, 32) != 32 ||
            !lxp_secp256k1_sig_is_low_s(signature) ||
            BN_add_word(order, 1U) != 1 ||
            BN_bn2binpad(order, signature + 32U, 32) != 32 ||
            lxp_secp256k1_sig_is_low_s(signature)) return 1;
        BN_free(order);
    }
    signature[32U]=0xffU;
    ECDSA_SIG_free(signed_digest);EC_POINT_free(public_point);BN_free(private_value);EC_KEY_free(key);
    return !lxp_secp256k1_sig_is_low_s(signature) ? 0 : 1;
}
