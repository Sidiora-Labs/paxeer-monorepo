#include "layerx/lxp_snapshot.h"
#include "lxp_kernel_case.h"

#include <stdint.h>
#include <string.h>

lxp_result lx_account_record_encode(const lx_account *account, uint8_t *bytes,
                                    size_t capacity, size_t *length);
lxp_result lx_account_record_decode(const uint8_t *bytes, size_t length,
                                    lx_account *account);

enum { ACCOUNT_RECORD_CAPACITY = LX_ACCOUNT_NAME_MAX + 256 };

static uint64_t account_record_random(uint64_t *state)
{
    *state ^= *state << 13U;
    *state ^= *state >> 7U;
    *state ^= *state << 17U;
    return *state;
}

static void account_record_generate(lx_account *account, uint64_t *state,
                                    size_t sample, size_t name_length)
{
    size_t i;

    (void)memset(account, 0, sizeof(*account));
    account->name_length = (uint16_t)name_length;
    for (i = 0U; i < name_length; ++i)
        account->name[i] = (uint8_t)account_record_random(state);
    for (i = 0U; i < sizeof(account->id); ++i) {
        account->id[i] = (uint8_t)account_record_random(state);
        account->asset_id[i] = (uint8_t)account_record_random(state);
        account->authority_key[i] = (uint8_t)account_record_random(state);
    }
    account->kind = (lx_account_kind)(1U + sample % 14U);
    account->balance.hi = account_record_random(state);
    account->balance.lo = account_record_random(state);
    account->next_sequence = account_record_random(state);
    account->created_at_sequence = account_record_random(state);
    account->has_asset = (sample & 1U) != 0U;
    account->frozen = (sample & 2U) != 0U;
    account->has_open_reference = (sample & 4U) != 0U;
    account->has_authority_key = (sample & 8U) != 0U;
    if (sample < 2U) {
        uint64_t boundary = sample == 0U ? UINT64_C(0) : UINT64_MAX;
        account->balance.hi = boundary;
        account->balance.lo = boundary;
        account->next_sequence = boundary;
        account->created_at_sequence = boundary;
    }
}

static int account_record_equal(const lx_account *left, const lx_account *right)
{
    LXP_CASE_CHECK(left->name_length == right->name_length);
    LXP_CASE_CHECK(memcmp(left->name, right->name, left->name_length) == 0);
    LXP_CASE_CHECK(memcmp(left->id, right->id, sizeof(left->id)) == 0);
    LXP_CASE_CHECK(left->kind == right->kind);
    LXP_CASE_CHECK(left->balance.hi == right->balance.hi);
    LXP_CASE_CHECK(left->balance.lo == right->balance.lo);
    LXP_CASE_CHECK(memcmp(left->asset_id, right->asset_id,
                          sizeof(left->asset_id)) == 0);
    LXP_CASE_CHECK(left->has_asset == right->has_asset);
    LXP_CASE_CHECK(left->next_sequence == right->next_sequence);
    LXP_CASE_CHECK(left->created_at_sequence == right->created_at_sequence);
    LXP_CASE_CHECK(left->frozen == right->frozen);
    LXP_CASE_CHECK(left->has_open_reference == right->has_open_reference);
    LXP_CASE_CHECK(memcmp(left->authority_key, right->authority_key,
                          sizeof(left->authority_key)) == 0);
    LXP_CASE_CHECK(left->has_authority_key == right->has_authority_key);
    return 0;
}

static int account_record_roundtrip(void)
{
    uint64_t state = UINT64_C(0x176acced12345678);
    size_t sample;

    for (sample = 0U; sample < 1000U; ++sample) {
        lx_account account;
        lx_account decoded;
        uint8_t encoded[ACCOUNT_RECORD_CAPACITY];
        uint8_t reencoded[ACCOUNT_RECORD_CAPACITY];
        size_t length = 0U;
        size_t reencoded_length = 0U;
        size_t name_length = 1U + sample % LX_ACCOUNT_NAME_MAX;

        account_record_generate(&account, &state, sample, name_length);
        LXP_CASE_CHECK(lx_account_record_encode(
            &account, encoded, sizeof(encoded), &length) == LXP_OK);
        LXP_CASE_CHECK(length > name_length && length <= sizeof(encoded));
        (void)memset(&decoded, 0xa5, sizeof(decoded));
        LXP_CASE_CHECK(lx_account_record_decode(
            encoded, length, &decoded) == LXP_OK);
        LXP_CASE_CHECK(account_record_equal(&account, &decoded) == 0);
        LXP_CASE_CHECK(lx_account_record_encode(
            &decoded, reencoded, sizeof(reencoded), &reencoded_length) == LXP_OK);
        LXP_CASE_CHECK(reencoded_length == length);
        LXP_CASE_CHECK(memcmp(encoded, reencoded, length) == 0);
    }
    return 0;
}

static int account_record_refusals(void)
{
    static const size_t name_lengths[] = { 1U, 255U, 256U, LX_ACCOUNT_NAME_MAX };
    uint64_t state = UINT64_C(0x176dec0de1234567);
    lx_account account;
    lx_account decoded;
    uint8_t encoded[ACCOUNT_RECORD_CAPACITY];
    uint8_t mutated[ACCOUNT_RECORD_CAPACITY];
    uint8_t reencoded[ACCOUNT_RECORD_CAPACITY];
    size_t length = 0U;
    size_t sample;

    account_record_generate(&account, &state, 0U, 1U);
    account.name_length = 0U;
    LXP_CASE_CHECK(lx_account_record_encode(
        &account, encoded, sizeof(encoded), &length) == LXP_ERR_NON_CANONICAL);
    account.name_length = (uint16_t)(LX_ACCOUNT_NAME_MAX + 1U);
    LXP_CASE_CHECK(lx_account_record_encode(
        &account, encoded, sizeof(encoded), &length) == LXP_ERR_NON_CANONICAL);

    for (sample = 0U; sample < sizeof(name_lengths) / sizeof(name_lengths[0]);
         ++sample) {
        size_t offset;
        size_t accepted = 0U;
        size_t refused = 0U;
        unsigned int replacement;

        account_record_generate(&account, &state, sample, name_lengths[sample]);
        LXP_CASE_CHECK(lx_account_record_encode(
            &account, encoded, sizeof(encoded), &length) == LXP_OK);
        LXP_CASE_CHECK(length > 0U && length < sizeof(encoded));
        LXP_CASE_CHECK(lx_account_record_decode(
            encoded, length, &decoded) == LXP_OK);
        (void)memcpy(mutated, encoded, length);
        for (replacement = 0U; replacement <= UINT8_MAX; ++replacement) {
            mutated[length] = (uint8_t)replacement;
            LXP_CASE_CHECK(lx_account_record_decode(
                mutated, length + 1U, &decoded) == LXP_ERR_TRAILING_BYTES);
        }
        for (offset = 0U; offset < length; ++offset) {
            for (replacement = 0U; replacement <= UINT8_MAX; ++replacement) {
                size_t reencoded_length = 0U;
                lxp_result status;

                if ((uint8_t)replacement == encoded[offset]) continue;
                mutated[offset] = (uint8_t)replacement;
                status = lx_account_record_decode(mutated, length, &decoded);
                if (status == LXP_OK) {
                    ++accepted;
                    LXP_CASE_CHECK(lx_account_record_encode(
                        &decoded, reencoded, sizeof(reencoded),
                        &reencoded_length) == LXP_OK);
                    LXP_CASE_CHECK(reencoded_length == length);
                    LXP_CASE_CHECK(memcmp(mutated, reencoded, length) == 0);
                } else {
                    ++refused;
                }
            }
            mutated[offset] = encoded[offset];
        }
        LXP_CASE_CHECK(accepted > 0U);
        LXP_CASE_CHECK(refused > 0U);
        LXP_CASE_CHECK(accepted + refused == length * UINT8_MAX);
    }
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "account_record_roundtrip", account_record_roundtrip },
        { "account_record_refusals", account_record_refusals }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
