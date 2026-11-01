#include "layerx/lxp_verify.h"

#ifdef LAYERX_LXP_GUARANTOR_H
enum { verify_header_includes_guarantor = 1 };
#else
enum { verify_header_includes_guarantor = 0 };
#endif

#include "lxp_kernel_case.h"

#include <openssl/evp.h>
#include <stdlib.h>

struct lxp_augmented_receipt;
struct lxp_guarantor_key_record;

extern lxp_result lxp_receipt_verify_checkpointed(
    const lxp_receipt *, const struct lxp_augmented_receipt *,
    const uint8_t[32], const struct lxp_guarantor_key_record *, size_t,
    const uint8_t[32], lxp_byte_span, lxp_arena *);
#pragma weak lxp_receipt_verify_checkpointed

static int verify_header_lacks_retired_api(void)
{
    FILE *header = fopen("include/layerx/lxp_verify.h", "rb");
    long length;
    size_t bytes;
    char *contents;
    int read_ok;
    int close_ok;
    int absent;

    LXP_CASE_CHECK(header != NULL);
    if (fseek(header, 0L, SEEK_END) != 0 || (length = ftell(header)) <= 0L ||
        fseek(header, 0L, SEEK_SET) != 0) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    bytes = (size_t)length;
    if (bytes == SIZE_MAX) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    contents = malloc(bytes + 1U);
    if (contents == NULL) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    read_ok = fread(contents, 1U, bytes, header) == bytes && !ferror(header);
    close_ok = fclose(header) == 0;
    contents[bytes] = '\0';
    absent = read_ok &&
        strstr(contents, "layerx/lxp_guarantor.h") == NULL &&
        strstr(contents, "lxp_receipt_verify_checkpointed") == NULL;
    free(contents);
    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_ok);
    LXP_CASE_CHECK(absent);
    return 0;
}

static int verify_header_without_guarantor(void)
{
    static const uint8_t private_key[32] = { 1U };
    static uint8_t arena_bytes[LXP_MAX_ACTIVITY_BYTES + 4096U];
    const uint8_t activity_id[32] = { 1U };
    const uint8_t previous[32] = { 2U };
    const uint8_t resulting[32] = { 3U };
    const uint8_t activity_root[32] = { 4U };
    const uint8_t batch_id[32] = { 5U };
    uint8_t public_key[32];
    size_t public_key_length = sizeof(public_key);
    lxp_arena arena;
    lxp_effect_buffer effects;
    lxp_receipt receipt = { .protocol_version = LXP_PROTOCOL_VERSION };
    lxp_receipt decoded;
    lxp_byte_span encoded;
    EVP_PKEY *key;
    int key_ok;

    LXP_CASE_CHECK(verify_header_includes_guarantor == 0);
    LXP_CASE_CHECK(verify_header_lacks_retired_api() == 0);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes,
                                  sizeof(arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_effect_buffer_init(&effects) == LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_build(
        &receipt, activity_id, 8U, previous, resulting, activity_root,
        LXP_OK, &effects, (lxp_u128){ 0U, 3U }, batch_id,
        LXP_MODULE_ASSET, 1U, 1U) == LXP_OK);
    receipt.operation = (uint8_t)LX_ASSET_SEND;
    receipt.asset[0] = 6U;
    receipt.amount = (lxp_u128){ 0U, 10U };
    receipt.from[0] = 7U;
    receipt.from_balance_before = (lxp_u128){ 0U, 20U };
    receipt.from_balance_after = (lxp_u128){ 0U, 10U };
    receipt.from_sequence = 1U;
    receipt.to[0] = 8U;
    receipt.to_balance_after = receipt.amount;
    receipt.supply_binding_version = 1U;
    receipt.total_units_before = (lxp_u128){ 0U, 20U };
    receipt.total_units_after = receipt.total_units_before;
    receipt.timestamp = 9U;
    LXP_CASE_CHECK(lxp_receipt_sign(&receipt, private_key, &arena) == LXP_OK);
    key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, private_key,
                                       sizeof(private_key));
    LXP_CASE_CHECK(key != NULL);
    key_ok = EVP_PKEY_get_raw_public_key(key, public_key,
                                        &public_key_length);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(key_ok == 1 && public_key_length == sizeof(public_key));
    LXP_CASE_CHECK(lxp_receipt_encode(&receipt, true, &arena, &encoded) ==
                   LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_decode(encoded.bytes, encoded.length, true,
                                      &decoded) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_reset(&arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_verify_offline(&decoded, public_key, &arena) ==
                   LXP_OK);
    decoded.timestamp += 1U;
    LXP_CASE_CHECK(lxp_receipt_verify_offline(&decoded, public_key, &arena) ==
                   LXP_ERR_BAD_SIGNATURE);
    return 0;
}

static int checkpointed_verify_absent(void)
{
    LXP_CASE_CHECK(lxp_receipt_verify_offline(NULL, NULL, NULL) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_receipt_verify_checkpointed == NULL);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "verify_header_without_guarantor", verify_header_without_guarantor },
        { "checkpointed_verify_absent", checkpointed_verify_absent }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
