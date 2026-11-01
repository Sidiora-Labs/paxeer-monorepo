#include "layerx/lx_escrow.h"
#include "../codec/lxp_kernel_case.h"

#include <stdint.h>
#include <string.h>

lxp_result lx_escrow_record_encode(const lx_escrow_record *record,
                                   uint8_t bytes[LX_ESCROW_RECORD_BYTES]);
lxp_result lx_escrow_record_decode(const uint8_t *bytes, size_t length,
                                   lx_escrow_record *record);

enum { ESCROW_CODEC_SAMPLES = 1000 };

static uint64_t next_value(uint64_t *state)
{
    *state ^= *state << 13U;
    *state ^= *state >> 7U;
    *state ^= *state << 17U;
    return *state;
}

static void fill_bytes(uint64_t *state, uint8_t bytes[32])
{
    size_t i;
    for (i = 0U; i < 32U; ++i) bytes[i] = (uint8_t)next_value(state);
}

static lxp_u128 amount_value(uint64_t *state, size_t sample)
{
    lxp_u128 value;
    value.lo = next_value(state);
    value.hi = next_value(state);
    switch (sample % 8U) {
    case 0U:
        value.lo = 0U;
        value.hi = 0U;
        break;
    case 1U:
        value.lo = UINT64_MAX;
        value.hi = UINT64_MAX;
        break;
    case 2U:
        value.lo = UINT64_MAX;
        value.hi = 0U;
        break;
    case 3U:
        value.lo = 0U;
        value.hi = 1U;
        break;
    default:
        break;
    }
    return value;
}

static void record_value(uint64_t *state, size_t sample,
                         lx_escrow_record *record)
{
    (void)memset(record, 0, sizeof(*record));
    fill_bytes(state, record->escrow_id);
    record->escrow_id[0] |= 1U;
    if (sample == 0U) {
        (void)memset(record->escrow_id, 0, sizeof(record->escrow_id));
        record->escrow_id[0] = 1U;
    }
    fill_bytes(state, record->owner);
    fill_bytes(state, record->escrow_account);
    fill_bytes(state, record->beneficiary);
    fill_bytes(state, record->arbiter);
    fill_bytes(state, record->asset_id);
    record->locked_amount = amount_value(state, sample);
    record->captured_amount = amount_value(state, sample + 1U);
    record->state = (lx_escrow_status)(1U + sample % 7U);
    record->expiry = next_value(state);
    record->dispute_window = next_value(state);
    if (sample % 8U == 0U) record->expiry = 0U;
    if (sample % 8U == 1U) record->expiry = UINT64_MAX;
    if (sample % 8U == 2U) record->dispute_window = 0U;
    if (sample % 8U == 3U) record->dispute_window = UINT64_MAX;
    fill_bytes(state, record->terms_hash);
    fill_bytes(state, record->agreement_reference);
}

static void result_value(uint64_t *state, size_t sample,
                         lx_escrow_economic_result *result)
{
    (void)memset(result, 0, sizeof(*result));
    fill_bytes(state, result->escrow_id);
    result->escrow_id[0] |= 1U;
    result->ordinal = (uint16_t)(1U + sample % 7U);
    result->state_after = (lx_escrow_status)(1U + (sample / 7U) % 7U);
    result->captured_after = amount_value(state, sample);
    result->locked_after = amount_value(state, sample + 1U);
    fill_bytes(state, result->asset_id);
    fill_bytes(state, result->from);
    fill_bytes(state, result->to);
    result->amount = amount_value(state, sample + 2U);
    result->secondary_amount = amount_value(state, sample + 3U);
    fill_bytes(state, result->transfer_set_root);
    result->global_sequence = next_value(state);
    result->timestamp = next_value(state);
    if (sample % 8U == 0U) result->global_sequence = 0U;
    if (sample % 8U == 1U) result->global_sequence = UINT64_MAX;
    if (sample % 8U == 2U) result->timestamp = 0U;
    if (sample % 8U == 3U) result->timestamp = UINT64_MAX;
}

static int records_equal(const lx_escrow_record *left,
                          const lx_escrow_record *right)
{
    return memcmp(left->escrow_id, right->escrow_id, 32U) == 0 &&
           memcmp(left->owner, right->owner, 32U) == 0 &&
           memcmp(left->escrow_account, right->escrow_account, 32U) == 0 &&
           memcmp(left->beneficiary, right->beneficiary, 32U) == 0 &&
           memcmp(left->arbiter, right->arbiter, 32U) == 0 &&
           memcmp(left->asset_id, right->asset_id, 32U) == 0 &&
           left->locked_amount.lo == right->locked_amount.lo &&
           left->locked_amount.hi == right->locked_amount.hi &&
           left->captured_amount.lo == right->captured_amount.lo &&
           left->captured_amount.hi == right->captured_amount.hi &&
           left->state == right->state && left->expiry == right->expiry &&
           left->dispute_window == right->dispute_window &&
           memcmp(left->terms_hash, right->terms_hash, 32U) == 0 &&
           memcmp(left->agreement_reference, right->agreement_reference,
                  32U) == 0;
}

static int results_equal(const lx_escrow_economic_result *left,
                          const lx_escrow_economic_result *right)
{
    return memcmp(left->escrow_id, right->escrow_id, 32U) == 0 &&
           left->ordinal == right->ordinal &&
           left->state_after == right->state_after &&
           left->captured_after.lo == right->captured_after.lo &&
           left->captured_after.hi == right->captured_after.hi &&
           left->locked_after.lo == right->locked_after.lo &&
           left->locked_after.hi == right->locked_after.hi &&
           memcmp(left->asset_id, right->asset_id, 32U) == 0 &&
           memcmp(left->from, right->from, 32U) == 0 &&
           memcmp(left->to, right->to, 32U) == 0 &&
           left->amount.lo == right->amount.lo &&
           left->amount.hi == right->amount.hi &&
           left->secondary_amount.lo == right->secondary_amount.lo &&
           left->secondary_amount.hi == right->secondary_amount.hi &&
           memcmp(left->transfer_set_root, right->transfer_set_root, 32U) == 0 &&
           left->global_sequence == right->global_sequence &&
           left->timestamp == right->timestamp &&
           left->context_bound == right->context_bound &&
           memcmp(left->context_digest, right->context_digest, 32U) == 0;
}

static int escrow_codecs_roundtrip(void)
{
    uint64_t state = UINT64_C(0x657363726f773137);
    size_t sample;

    for (sample = 0U; sample < (size_t)ESCROW_CODEC_SAMPLES; ++sample) {
        lx_escrow_record record;
        lx_escrow_record decoded_record;
        lx_escrow_economic_result result;
        uint8_t encoded_record[LX_ESCROW_RECORD_BYTES];
        uint8_t reencoded_record[LX_ESCROW_RECORD_BYTES];
        unsigned int bound;

        record_value(&state, sample, &record);
        (void)memset(&decoded_record, 0xa5, sizeof(decoded_record));
        LXP_CASE_CHECK(lx_escrow_record_encode(&record, encoded_record) ==
                       LXP_OK);
        LXP_CASE_CHECK(lx_escrow_record_decode(encoded_record,
                       sizeof(encoded_record), &decoded_record) == LXP_OK);
        LXP_CASE_CHECK(records_equal(&record, &decoded_record));
        LXP_CASE_CHECK(lx_escrow_record_encode(&decoded_record,
                       reencoded_record) == LXP_OK);
        LXP_CASE_CHECK(memcmp(encoded_record, reencoded_record,
                              sizeof(encoded_record)) == 0);

        result_value(&state, sample, &result);
        for (bound = 0U; bound < 2U; ++bound) {
            lx_escrow_economic_result decoded_result;
            lx_escrow_economic_result legacy_result;
            uint8_t encoded[LX_ESCROW_RESULT_BYTES];
            uint8_t reencoded[LX_ESCROW_RESULT_BYTES];

            result.context_bound = bound != 0U;
            if (result.context_bound) {
                fill_bytes(&state, result.context_digest);
                result.context_digest[0] |= 1U;
            }
            legacy_result = result;
            legacy_result.context_bound = false;
            (void)memset(legacy_result.context_digest, 0,
                         sizeof(legacy_result.context_digest));
            (void)memset(&decoded_result, 0xa5, sizeof(decoded_result));
            LXP_CASE_CHECK(sizeof(encoded) == 243U);
            LXP_CASE_CHECK(lx_escrow_result_encode(&result, encoded) == LXP_OK);
            LXP_CASE_CHECK(lx_escrow_result_decode(encoded, sizeof(encoded),
                           &decoded_result) == LXP_OK);
            LXP_CASE_CHECK(!decoded_result.context_bound);
            LXP_CASE_CHECK(results_equal(&legacy_result, &decoded_result));
            LXP_CASE_CHECK(lx_escrow_result_encode(&decoded_result,
                           reencoded) == LXP_OK);
            LXP_CASE_CHECK(memcmp(encoded, reencoded, sizeof(encoded)) == 0);

            if (result.context_bound) {
                uint8_t encoded_v2[LX_ESCROW_RESULT_V2_BYTES];
                uint8_t reencoded_v2[LX_ESCROW_RESULT_V2_BYTES];

                (void)memset(&decoded_result, 0xa5, sizeof(decoded_result));
                LXP_CASE_CHECK(lx_escrow_result_encode_v2(&result,
                               encoded_v2) == LXP_OK);
                LXP_CASE_CHECK(memcmp(encoded_v2, encoded, sizeof(encoded)) ==
                               0);
                LXP_CASE_CHECK(encoded_v2[LX_ESCROW_RESULT_BYTES] ==
                               (uint8_t)LX_ESCROW_RESULT_CONTEXT_VERSION);
                LXP_CASE_CHECK(lx_escrow_result_decode(encoded_v2,
                               sizeof(encoded_v2), &decoded_result) == LXP_OK);
                LXP_CASE_CHECK(decoded_result.context_bound);
                LXP_CASE_CHECK(results_equal(&result, &decoded_result));
                LXP_CASE_CHECK(lx_escrow_result_encode_v2(&decoded_result,
                               reencoded_v2) == LXP_OK);
                LXP_CASE_CHECK(memcmp(encoded_v2, reencoded_v2,
                                      sizeof(encoded_v2)) == 0);
            }
        }
    }
    return 0;
}

static int escrow_codecs_mutation_canonical(void)
{
    uint64_t state = UINT64_C(0x63616e6f6e313734);
    size_t accepted = 0U;
    size_t refused = 0U;
    size_t extensions = 0U;
    size_t sample;

    for (sample = 0U; sample < (size_t)ESCROW_CODEC_SAMPLES; ++sample) {
        lx_escrow_record record;
        lx_escrow_record decoded;
        uint8_t encoded[LX_ESCROW_RECORD_BYTES];
        uint8_t mutated[LX_ESCROW_RECORD_BYTES + 1U];
        uint8_t reencoded[LX_ESCROW_RECORD_BYTES];
        size_t offset;
        unsigned int replacement;

        record_value(&state, sample, &record);
        LXP_CASE_CHECK(lx_escrow_record_encode(&record, encoded) == LXP_OK);
        (void)memcpy(mutated, encoded, sizeof(encoded));
        for (offset = 0U; offset < sizeof(encoded); ++offset) {
            for (replacement = 0U; replacement <= UINT8_MAX; ++replacement) {
                lxp_result status;
                if ((uint8_t)replacement == encoded[offset]) continue;
                mutated[offset] = (uint8_t)replacement;
                status = lx_escrow_record_decode(mutated, sizeof(encoded),
                                                  &decoded);
                if (status == LXP_OK) {
                    LXP_CASE_CHECK(lx_escrow_record_encode(&decoded,
                                   reencoded) == LXP_OK);
                    LXP_CASE_CHECK(memcmp(mutated, reencoded,
                                          sizeof(reencoded)) == 0);
                    ++accepted;
                } else {
                    LXP_CASE_CHECK(status == LXP_ERR_NON_CANONICAL);
                    ++refused;
                }
            }
            mutated[offset] = encoded[offset];
        }
        for (replacement = 0U; replacement <= UINT8_MAX; ++replacement) {
            mutated[sizeof(encoded)] = (uint8_t)replacement;
            LXP_CASE_CHECK(lx_escrow_record_decode(mutated, sizeof(mutated),
                           &decoded) == LXP_ERR_NON_CANONICAL);
            ++extensions;
        }
    }
    LXP_CASE_CHECK(accepted > 0U);
    LXP_CASE_CHECK(refused > 0U);
    LXP_CASE_CHECK(accepted + refused ==
                   (size_t)ESCROW_CODEC_SAMPLES * LX_ESCROW_RECORD_BYTES *
                   UINT8_MAX);
    LXP_CASE_CHECK(extensions == (size_t)ESCROW_CODEC_SAMPLES * 256U);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "escrow_codecs_roundtrip", escrow_codecs_roundtrip },
        { "escrow_codecs_mutation_canonical", escrow_codecs_mutation_canonical }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
