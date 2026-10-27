#include "layerx/lx_stream.h"
#include "lxp_kernel_case.h"

#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

extern lxp_result lx_stream_record_encode(
    const lx_stream_record *record, uint8_t bytes[LX_STREAM_RECORD_BYTES]);
extern lxp_result lx_stream_record_decode(
    const uint8_t *bytes, size_t length, lx_stream_record *record);
extern lxp_result lx_stream_open_decode(
    const uint8_t *bytes, size_t length, lx_stream_open_payload *payload);

enum { STREAM_SAMPLES = 1000 };

typedef enum stream_codec {
    STREAM_RECORD,
    STREAM_OPEN,
    STREAM_AMOUNT,
    STREAM_METER,
    STREAM_KEYED,
    STREAM_ID,
    STREAM_CODEC_COUNT
} stream_codec;

typedef union stream_value {
    lx_stream_record record;
    lx_stream_open_payload open;
    lx_stream_amount_payload amount;
    lx_stream_meter_attestation meter;
    lx_stream_keyed_payload keyed;
    lx_stream_id_payload id;
} stream_value;

static const char *const codec_names[] = {
    "record", "open", "amount", "meter", "keyed", "id"
};

static uint64_t next_word(uint64_t *seed)
{
    *seed ^= *seed << 13U;
    *seed ^= *seed >> 7U;
    *seed ^= *seed << 17U;
    return *seed;
}

static void generate_bytes(uint64_t *seed, uint8_t *bytes, size_t length)
{
    size_t i;
    for (i = 0U; i < length; ++i) bytes[i] = (uint8_t)next_word(seed);
}

static uint64_t generate_word(uint64_t *seed, size_t sample)
{
    switch (sample) {
    case 0U: return 0U;
    case 1U: return 1U;
    case 2U: return UINT64_MAX;
    case 3U: return UINT64_C(1) << 63U;
    default: return next_word(seed);
    }
}

static lxp_u128 generate_amount(uint64_t *seed, size_t sample)
{
    lxp_u128 amount;
    amount.hi = generate_word(seed, sample);
    amount.lo = generate_word(seed, sample + 1U);
    return amount;
}

static void generate_record(uint64_t *seed, size_t sample,
                            lx_stream_record *record)
{
    lxp_u128 totals[3];
    size_t i;
    size_t j;

    (void)memset(record, 0, sizeof(*record));
    generate_bytes(seed, record->stream_id, 32U);
    generate_bytes(seed, record->payer, 32U);
    generate_bytes(seed, record->stream_account, 32U);
    generate_bytes(seed, record->recipient, 32U);
    generate_bytes(seed, record->asset_id, 32U);
    record->stream_id[0] = 1U;
    record->payer[0] = 2U;
    record->stream_account[0] = 3U;
    record->recipient[0] = 4U;
    record->asset_id[0] = 5U;
    record->meter_authority_count =
        sample % ((size_t)LX_STREAM_MAX_METER_AUTHORITIES + 1U);
    record->mode = record->meter_authority_count == 0U ?
        LX_STREAM_MODE_TIME : LX_STREAM_MODE_METERED;
    record->rate = generate_amount(seed, sample);
    record->rate_unit = generate_word(seed, sample);
    if (record->rate_unit == 0U) record->rate_unit = 1U;
    record->start_timestamp = (next_word(seed) >> 1U) | UINT64_C(1);
    record->last_accrual_timestamp = record->start_timestamp +
        next_word(seed) % (UINT64_MAX - record->start_timestamp);
    if ((sample & 1U) != 0U)
        record->end_timestamp = record->start_timestamp + 1U +
            next_word(seed) % (UINT64_MAX - record->start_timestamp);
    for (i = 0U; i < 3U; ++i)
        totals[i] = generate_amount(seed, sample + i);
    for (i = 0U; i < 3U; ++i) {
        for (j = i + 1U; j < 3U; ++j) {
            if (lxp_u128_cmp(totals[i], totals[j]) > 0) {
                lxp_u128 swap = totals[i];
                totals[i] = totals[j];
                totals[j] = swap;
            }
        }
    }
    record->settled_total = totals[0];
    record->accrued_total = totals[1];
    record->total_cap = totals[2];
    record->remainder_carry.lo = next_word(seed) % record->rate_unit;
    record->cumulative_meter = generate_word(seed, sample);
    for (i = 0U; i < record->meter_authority_count; ++i) {
        generate_bytes(seed, record->meter_authorities[i], 32U);
        record->meter_authorities[i][0] = (uint8_t)(i + 1U);
    }
    record->underfunded = (sample & 1U) != 0U;
    record->paused = (sample & 2U) != 0U;
    record->closed = (sample & 4U) != 0U;
}

static void generate_value(stream_codec codec, uint64_t *seed, size_t sample,
                           stream_value *value)
{
    (void)memset(value, 0, sizeof(*value));
    switch (codec) {
    case STREAM_RECORD:
        generate_record(seed, sample, &value->record);
        break;
    case STREAM_OPEN:
        generate_record(seed, sample, &value->open.record);
        value->open.initial_funding = generate_amount(seed, sample);
        break;
    case STREAM_AMOUNT:
        generate_bytes(seed, value->amount.stream_id, 32U);
        value->amount.stream_id[0] = 1U;
        value->amount.amount = generate_amount(seed, sample);
        break;
    case STREAM_METER:
        generate_bytes(seed, value->meter.stream_id, 32U);
        generate_bytes(seed, value->meter.authority_key, 32U);
        generate_bytes(seed, value->meter.signature, 64U);
        value->meter.stream_id[0] = 1U;
        value->meter.authority_key[0] = 2U;
        value->meter.signature[0] = 3U;
        value->meter.cumulative_reading = generate_word(seed, sample);
        break;
    case STREAM_KEYED:
        generate_bytes(seed, value->keyed.stream_id, 32U);
        generate_bytes(seed, value->keyed.idempotency_key, 32U);
        value->keyed.stream_id[0] = 1U;
        value->keyed.idempotency_key[0] = 2U;
        break;
    case STREAM_ID:
        generate_bytes(seed, value->id.stream_id, 32U);
        value->id.stream_id[0] = 1U;
        break;
    case STREAM_CODEC_COUNT:
        break;
    }
}

static size_t encoded_length(stream_codec codec, const stream_value *value)
{
    switch (codec) {
    case STREAM_RECORD: return (size_t)LX_STREAM_RECORD_BYTES;
    case STREAM_OPEN:
        return (size_t)LX_STREAM_OPEN_PAYLOAD_FIXED +
            value->open.record.meter_authority_count * 32U;
    case STREAM_AMOUNT: return (size_t)LX_STREAM_TOP_UP_PAYLOAD_BYTES;
    case STREAM_METER: return (size_t)LX_STREAM_METER_PAYLOAD_BYTES;
    case STREAM_KEYED: return (size_t)LX_STREAM_KEYED_PAYLOAD_BYTES;
    case STREAM_ID: return (size_t)LX_STREAM_ID_PAYLOAD_BYTES;
    case STREAM_CODEC_COUNT: return 0U;
    }
    return 0U;
}

static lxp_result encode_value(stream_codec codec, const stream_value *value,
                               uint8_t bytes[LX_STREAM_RECORD_BYTES],
                               size_t *length)
{
    switch (codec) {
    case STREAM_RECORD:
        *length = (size_t)LX_STREAM_RECORD_BYTES;
        return lx_stream_record_encode(&value->record, bytes);
    case STREAM_OPEN:
        return lx_stream_open_encode(&value->open, bytes,
                                     LX_STREAM_RECORD_BYTES, length);
    case STREAM_AMOUNT:
        return lx_stream_amount_encode(&value->amount, bytes,
                                       LX_STREAM_RECORD_BYTES, length);
    case STREAM_METER:
        return lx_stream_meter_encode(&value->meter, bytes,
                                      LX_STREAM_RECORD_BYTES, length);
    case STREAM_KEYED:
        return lx_stream_keyed_encode(&value->keyed, bytes,
                                      LX_STREAM_RECORD_BYTES, length);
    case STREAM_ID:
        return lx_stream_id_encode(&value->id, bytes,
                                   LX_STREAM_RECORD_BYTES, length);
    case STREAM_CODEC_COUNT:
        return LXP_FATAL_INVARIANT;
    }
    return LXP_FATAL_INVARIANT;
}

static lxp_result decode_value(stream_codec codec, const uint8_t *bytes,
                               size_t length, stream_value *value)
{
    switch (codec) {
    case STREAM_RECORD:
        return lx_stream_record_decode(bytes, length, &value->record);
    case STREAM_OPEN:
        return lx_stream_open_decode(bytes, length, &value->open);
    case STREAM_AMOUNT:
        return lx_stream_amount_decode(bytes, length, &value->amount);
    case STREAM_METER:
        return lx_stream_meter_decode(bytes, length, &value->meter);
    case STREAM_KEYED:
        return lx_stream_keyed_decode(bytes, length, &value->keyed);
    case STREAM_ID:
        return lx_stream_id_decode(bytes, length, &value->id);
    case STREAM_CODEC_COUNT:
        return LXP_FATAL_INVARIANT;
    }
    return LXP_FATAL_INVARIANT;
}

static int roundtrip_value(stream_codec codec, const stream_value *value)
{
    stream_value decoded;
    uint8_t bytes[LX_STREAM_RECORD_BYTES];
    uint8_t reencoded[LX_STREAM_RECORD_BYTES];
    size_t length = 0U;
    size_t reencoded_length = 0U;

    (void)memset(bytes, 0xa5, sizeof(bytes));
    (void)memset(reencoded, 0x5a, sizeof(reencoded));
    (void)memset(&decoded, 0xa5, sizeof(decoded));
    LXP_CASE_CHECK(encode_value(codec, value, bytes, &length) == LXP_OK);
    LXP_CASE_CHECK(length == encoded_length(codec, value));
    LXP_CASE_CHECK(length <= sizeof(bytes));
    LXP_CASE_CHECK(decode_value(codec, bytes, length, &decoded) == LXP_OK);
    LXP_CASE_CHECK(encode_value(codec, &decoded, reencoded,
                                &reencoded_length) == LXP_OK);
    LXP_CASE_CHECK(length == reencoded_length);
    LXP_CASE_CHECK(memcmp(bytes, reencoded, length) == 0);
    return 0;
}

static int stream_codecs_roundtrip(void)
{
    uint64_t seed = UINT64_C(0x6c7873747265616d);
    stream_codec codec;
    size_t sample;

    for (codec = STREAM_RECORD; codec < STREAM_CODEC_COUNT; ++codec) {
        for (sample = 0U; sample < (size_t)STREAM_SAMPLES; ++sample) {
            stream_value value;
            generate_value(codec, &seed, sample, &value);
            if (roundtrip_value(codec, &value) != 0) {
                (void)fprintf(stderr, "%s sample=%zu roundtrip failed\n",
                               codec_names[codec], sample);
                return 1;
            }
        }
        (void)printf("roundtrip %s: %u seeded values\n",
                      codec_names[codec], (unsigned int)STREAM_SAMPLES);
    }
    return 0;
}

static int refused_or_canonical(stream_codec codec, const uint8_t *bytes,
                                size_t length)
{
    stream_value decoded;
    uint8_t reencoded[LX_STREAM_RECORD_BYTES];
    size_t reencoded_length = 0U;
    lxp_result status;

    (void)memset(&decoded, 0xa5, sizeof(decoded));
    status = decode_value(codec, bytes, length, &decoded);
    if (status != LXP_OK) return 0;
    (void)memset(reencoded, 0x5a, sizeof(reencoded));
    LXP_CASE_CHECK(encode_value(codec, &decoded, reencoded,
                                &reencoded_length) == LXP_OK);
    LXP_CASE_CHECK(reencoded_length == length);
    LXP_CASE_CHECK(memcmp(bytes, reencoded, length) == 0);
    return 0;
}

static int stream_codecs_mutation_canonical(void)
{
    uint64_t seed = UINT64_C(0x6c7873747265616d);
    stream_codec codec;
    size_t sample;
    int failed = 0;

    for (codec = STREAM_RECORD; codec < STREAM_CODEC_COUNT; ++codec) {
        size_t extension_failures = 0U;
        for (sample = 0U; sample < (size_t)STREAM_SAMPLES; ++sample) {
            stream_value value;
            stream_value decoded;
            uint8_t bytes[LX_STREAM_RECORD_BYTES + 1];
            size_t length = 0U;
            size_t offset;
            unsigned int replacement;

            generate_value(codec, &seed, sample, &value);
            LXP_CASE_CHECK(encode_value(codec, &value, bytes, &length) ==
                           LXP_OK);
            LXP_CASE_CHECK(length == encoded_length(codec, &value));
            LXP_CASE_CHECK(length > 0U && length < sizeof(bytes));
            LXP_CASE_CHECK(decode_value(codec, bytes, length, &decoded) ==
                           LXP_OK);
            for (offset = 0U; offset < length; ++offset) {
                uint8_t original = bytes[offset];
                for (replacement = 0U; replacement <= UINT8_MAX;
                     ++replacement) {
                    if ((uint8_t)replacement == original) continue;
                    bytes[offset] = (uint8_t)replacement;
                    if (refused_or_canonical(codec, bytes, length) != 0) {
                        (void)fprintf(stderr,
                            "%s sample=%zu offset=%zu byte=%u\n",
                            codec_names[codec], sample, offset, replacement);
                        return 1;
                    }
                }
                bytes[offset] = original;
            }
            if (refused_or_canonical(codec, bytes, length - 1U) != 0) {
                (void)fprintf(stderr, "%s sample=%zu truncation failed\n",
                               codec_names[codec], sample);
                return 1;
            }
            for (replacement = 0U; replacement <= UINT8_MAX; ++replacement) {
                lxp_result status;
                bool expected;
                bytes[length] = (uint8_t)replacement;
                status = decode_value(codec, bytes, length + 1U, &decoded);
                expected = status == LXP_ERR_NON_CANONICAL;
                if (!expected) {
                    if (extension_failures == 0U)
                        (void)fprintf(stderr,
                            "%s sample=%zu extension byte=%u: expected "
                            "%s, got %" PRId32 "\n", codec_names[codec],
                            sample, replacement, "LXP_ERR_NON_CANONICAL",
                            status);
                    ++extension_failures;
                }
            }
        }
        (void)printf("mutation %s: %u seeded values, "
                     "%zu extension assertion failures\n",
                      codec_names[codec], (unsigned int)STREAM_SAMPLES,
                      extension_failures);
        if (extension_failures != 0U) failed = 1;
    }
    return failed;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "stream_codecs_roundtrip", stream_codecs_roundtrip },
        { "stream_codecs_mutation_canonical",
          stream_codecs_mutation_canonical }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
