#include "layerx/lx_service.h"
#include "lxp_kernel_case.h"

#include <stdint.h>
#include <stdio.h>
#include <string.h>

lxp_result lx_service_offer_encode(
    const lx_service_offer *offer,
    uint8_t bytes[LX_SERVICE_OFFER_RECORD_BYTES]);
lxp_result lx_service_agreement_decode(const uint8_t *bytes, size_t length,
                                       lx_service_agreement *agreement);
lxp_result lx_service_dispute_decode(const uint8_t *bytes, size_t length,
                                     lx_service_dispute *dispute);

enum {
    SERVICE_CODEC_SAMPLES = 1000
};
#define SERVICE_CODEC_CAPACITY LX_SERVICE_AGREEMENT_RECORD_BYTES

typedef enum service_codec {
    SERVICE_OFFER,
    SERVICE_AGREEMENT,
    SERVICE_COMMITMENT,
    SERVICE_PROGRESS,
    SERVICE_DISPUTE,
    SERVICE_EXECUTION,
    SERVICE_CODEC_COUNT
} service_codec;

typedef union service_record {
    lx_service_offer offer;
    lx_service_agreement agreement;
    lx_service_commitment commitment;
    lx_service_progress progress;
    lx_service_dispute dispute;
    lx_service_execution execution;
} service_record;

static const char *const codec_names[SERVICE_CODEC_COUNT] = {
    "offer", "agreement", "commitment", "progress", "dispute", "execution"
};

_Static_assert(SERVICE_CODEC_CAPACITY >= LX_SERVICE_OFFER_RECORD_BYTES,
               "offer buffer capacity");
_Static_assert(SERVICE_CODEC_CAPACITY >= LX_SERVICE_COMMITMENT_RECORD_BYTES,
               "commitment buffer capacity");
_Static_assert(SERVICE_CODEC_CAPACITY >= LX_SERVICE_PROGRESS_RECORD_BYTES,
               "progress buffer capacity");
_Static_assert(SERVICE_CODEC_CAPACITY >= LX_SERVICE_DISPUTE_RECORD_BYTES,
               "dispute buffer capacity");
_Static_assert(SERVICE_CODEC_CAPACITY >= LX_SERVICE_EXECUTION_BYTES,
               "execution buffer capacity");

static uint64_t next_value(uint64_t *state)
{
    *state ^= *state << 13U;
    *state ^= *state >> 7U;
    *state ^= *state << 17U;
    return *state;
}

static uint64_t nonzero_value(uint64_t *state, size_t sample)
{
    uint64_t value = next_value(state);
    if (sample % 16U == 0U) return UINT64_C(1);
    if (sample % 16U == 1U) return UINT64_MAX;
    return value;
}

static uint64_t optional_value(uint64_t *state, size_t sample)
{
    uint64_t value = next_value(state);
    if (sample % 16U == 0U) return UINT64_C(0);
    if (sample % 16U == 1U) return UINT64_MAX;
    return value;
}

static void generate_bytes(uint64_t *state, uint8_t *bytes, size_t length)
{
    size_t i;
    for (i = 0U; i < length; ++i) bytes[i] = (uint8_t)next_value(state);
    bytes[0] |= 1U;
}

static void generate_hashes(uint64_t *state,
                           uint8_t hashes[LX_SERVICE_MAX_DELIVERABLES][32],
                           size_t count)
{
    size_t i;
    for (i = 0U; i < count; ++i) {
        generate_bytes(state, hashes[i], sizeof(hashes[i]));
        hashes[i][0] = (uint8_t)(i + 1U);
    }
}

static void generate_record(service_codec codec, uint64_t *state,
                            size_t sample, service_record *record)
{
    (void)memset(record, 0, sizeof(*record));
#define GENERATE(field) generate_bytes(state, record->field, \
                                       sizeof(record->field))
    switch (codec) {
    case SERVICE_OFFER:
        GENERATE(offer.offer_id);
        GENERATE(offer.activity_id);
        GENERATE(offer.offering_agent);
        GENERATE(offer.asset_id);
        record->offer.price.hi = optional_value(state, sample);
        record->offer.price.lo = nonzero_value(state, sample);
        GENERATE(offer.terms_hash);
        GENERATE(offer.deliverable_specification_hash);
        record->offer.delivery_deadline = nonzero_value(state, sample);
        record->offer.acceptance_window = nonzero_value(state, sample);
        record->offer.dispute_window = nonzero_value(state, sample);
        record->offer.default_outcome = (lx_service_default_outcome)
            (LX_SERVICE_DEFAULT_ACCEPT + sample % 2U);
        record->offer.offer_expiry = nonzero_value(state, sample);
        record->offer.global_sequence = optional_value(state, sample);
        record->offer.withdrawn = (sample / 2U) % 2U != 0U;
        record->offer.accepted = (sample / 4U) % 2U != 0U;
        break;
    case SERVICE_AGREEMENT:
        GENERATE(agreement.agreement_id);
        GENERATE(agreement.offer_id);
        GENERATE(agreement.provider);
        GENERATE(agreement.buyer);
        GENERATE(agreement.terms_hash);
        GENERATE(agreement.escrow_id);
        record->agreement.delivery_deadline = optional_value(state, sample);
        record->agreement.acceptance_window_end = optional_value(state, sample);
        record->agreement.dispute_window_end = optional_value(state, sample);
        record->agreement.default_outcome = (lx_service_default_outcome)
            (LX_SERVICE_DEFAULT_ACCEPT + (sample / 8U) % 2U);
        record->agreement.state = (lx_service_agreement_state)
            (LX_SERVICE_AGREEMENT_FORMED + sample % 8U);
        record->agreement.accepted_sequence = optional_value(state, sample);
        record->agreement.rejection_reason = (uint16_t)optional_value(state,
                                                                      sample);
        record->agreement.contested_hash_count =
            sample % (LX_SERVICE_MAX_DELIVERABLES + 1U);
        generate_hashes(state, record->agreement.contested_hashes,
                         record->agreement.contested_hash_count);
        record->agreement.default_applied = (sample / 16U) % 2U != 0U;
        record->agreement.outcome_sequence = optional_value(state, sample);
        record->agreement.outcome_timestamp = optional_value(state, sample);
        break;
    case SERVICE_COMMITMENT:
        GENERATE(commitment.commitment_id);
        GENERATE(commitment.activity_id);
        GENERATE(commitment.provider);
        GENERATE(commitment.agreement_id);
        GENERATE(commitment.task_hash);
        GENERATE(commitment.escrow_id);
        record->commitment.deadline = nonzero_value(state, sample);
        record->commitment.resource_bound = nonzero_value(state, sample);
        record->commitment.global_sequence = optional_value(state, sample);
        record->commitment.abandoned = sample % 2U != 0U;
        if (record->commitment.abandoned)
            record->commitment.abandon_reason =
                (uint16_t)(1U + next_value(state) % UINT16_MAX);
        break;
    case SERVICE_PROGRESS:
        GENERATE(progress.report_id);
        GENERATE(progress.activity_id);
        GENERATE(progress.commitment_id);
        GENERATE(progress.agreement_id);
        GENERATE(progress.provider);
        GENERATE(progress.note_hash);
        GENERATE(progress.availability_reference);
        record->progress.progress_bps = sample % 2U == 0U ?
            1U + (uint32_t)(next_value(state) % LX_SERVICE_PROGRESS_COMPLETE_BPS) :
            LX_SERVICE_PROGRESS_COMPLETE_BPS;
        record->progress.reported_at = optional_value(state, sample);
        record->progress.global_sequence = optional_value(state, sample);
        break;
    case SERVICE_DISPUTE:
        GENERATE(dispute.dispute_id);
        GENERATE(dispute.activity_id);
        GENERATE(dispute.agreement_id);
        GENERATE(dispute.raiser);
        record->dispute.evidence_hash_count =
            1U + sample % LX_SERVICE_MAX_DELIVERABLES;
        generate_hashes(state, record->dispute.evidence_hashes,
                         record->dispute.evidence_hash_count);
        record->dispute.global_sequence = optional_value(state, sample);
        record->dispute.resolved = (sample / LX_SERVICE_MAX_DELIVERABLES) % 2U
                                   != 0U;
        record->dispute.ruling = record->dispute.resolved ?
            (uint16_t)(1U + next_value(state) % UINT16_MAX) : 0U;
        record->dispute.provider_basis_points = sample % 3U == 0U ? 0U :
            sample % 3U == 1U ? LX_SERVICE_PROGRESS_COMPLETE_BPS :
            (uint32_t)(next_value(state) %
                       (LX_SERVICE_PROGRESS_COMPLETE_BPS + 1U));
        if (record->dispute.resolved) GENERATE(dispute.escrow_resolution_id);
        record->dispute.resolution_sequence = optional_value(state, sample);
        break;
    case SERVICE_EXECUTION:
        GENERATE(execution.attestation_id);
        GENERATE(execution.activity_id);
        GENERATE(execution.agreement_id);
        GENERATE(execution.commitment_id);
        GENERATE(execution.tool_id);
        GENERATE(execution.input_commitment_hash);
        GENERATE(execution.output_commitment_hash);
        record->execution.execution_start = optional_value(state, sample);
        record->execution.execution_end = optional_value(state, sample);
        record->execution.resource_units = optional_value(state, sample);
        GENERATE(execution.attestor_identity);
        GENERATE(execution.availability_reference);
        GENERATE(execution.public_key);
        GENERATE(execution.signature);
        record->execution.global_sequence = optional_value(state, sample);
        break;
    case SERVICE_CODEC_COUNT:
        break;
    }
#undef GENERATE
}

static lxp_result encode_record(service_codec codec,
                                const service_record *record,
                                uint8_t bytes[SERVICE_CODEC_CAPACITY],
                                size_t *length)
{
    switch (codec) {
    case SERVICE_OFFER:
        *length = LX_SERVICE_OFFER_RECORD_BYTES;
        return lx_service_offer_encode(&record->offer, bytes);
    case SERVICE_AGREEMENT:
        return lx_service_agreement_encode(&record->agreement, bytes, length);
    case SERVICE_COMMITMENT:
        *length = LX_SERVICE_COMMITMENT_RECORD_BYTES;
        return lx_service_commitment_encode(&record->commitment, bytes);
    case SERVICE_PROGRESS:
        *length = LX_SERVICE_PROGRESS_RECORD_BYTES;
        return lx_service_progress_encode(&record->progress, bytes);
    case SERVICE_DISPUTE:
        return lx_service_dispute_encode(&record->dispute, bytes, length);
    case SERVICE_EXECUTION:
        return lx_service_execution_encode(&record->execution, bytes,
                                            SERVICE_CODEC_CAPACITY, length);
    case SERVICE_CODEC_COUNT:
        return LXP_FATAL_INVARIANT;
    }
    return LXP_FATAL_INVARIANT;
}

static lxp_result decode_record(service_codec codec, const uint8_t *bytes,
                                size_t length, service_record *record)
{
    switch (codec) {
    case SERVICE_OFFER:
        return lx_service_offer_decode(bytes, length, &record->offer);
    case SERVICE_AGREEMENT:
        return lx_service_agreement_decode(bytes, length, &record->agreement);
    case SERVICE_COMMITMENT:
        return lx_service_commitment_decode(bytes, length, &record->commitment);
    case SERVICE_PROGRESS:
        return lx_service_progress_decode(bytes, length, &record->progress);
    case SERVICE_DISPUTE:
        return lx_service_dispute_decode(bytes, length, &record->dispute);
    case SERVICE_EXECUTION:
        return lx_service_execution_decode(bytes, length, &record->execution);
    case SERVICE_CODEC_COUNT:
        return LXP_FATAL_INVARIANT;
    }
    return LXP_FATAL_INVARIANT;
}

static int service_codecs_roundtrip(void)
{
    uint64_t state = UINT64_C(0x4c58536572766963);
    service_codec codec;
    size_t sample;

    for (codec = SERVICE_OFFER; codec < SERVICE_CODEC_COUNT; ++codec) {
        for (sample = 0U; sample < SERVICE_CODEC_SAMPLES; ++sample) {
            service_record original;
            service_record decoded;
            uint8_t bytes[SERVICE_CODEC_CAPACITY];
            uint8_t encoded[SERVICE_CODEC_CAPACITY];
            size_t length = 0U;
            size_t encoded_length = 0U;

            generate_record(codec, &state, sample, &original);
            (void)memset(bytes, 0xa5, sizeof(bytes));
            (void)memset(encoded, 0x5a, sizeof(encoded));
            LXP_CASE_CHECK(encode_record(codec, &original, bytes, &length)
                           == LXP_OK);
            LXP_CASE_CHECK(length != 0U && length <= sizeof(bytes));
            LXP_CASE_CHECK(decode_record(codec, bytes, length, &decoded)
                           == LXP_OK);
            LXP_CASE_CHECK(encode_record(codec, &decoded, encoded,
                                         &encoded_length) == LXP_OK);
            LXP_CASE_CHECK(encoded_length == length);
            LXP_CASE_CHECK(memcmp(bytes, encoded, length) == 0);
        }
        (void)printf("service_codecs_roundtrip: %s: %u values\n",
                     codec_names[codec], (unsigned int)SERVICE_CODEC_SAMPLES);
    }
    return 0;
}

static int mutation_is_canonical(service_codec codec, const uint8_t *bytes,
                                 size_t length)
{
    service_record decoded;
    uint8_t encoded[SERVICE_CODEC_CAPACITY];
    size_t encoded_length = 0U;

    if (decode_record(codec, bytes, length, &decoded) != LXP_OK) return 0;
    (void)memset(encoded, 0x5a, sizeof(encoded));
    LXP_CASE_CHECK(encode_record(codec, &decoded, encoded, &encoded_length)
                   == LXP_OK);
    LXP_CASE_CHECK(encoded_length == length);
    LXP_CASE_CHECK(memcmp(bytes, encoded, length) == 0);
    return 0;
}

static int service_codecs_mutation_canonical(void)
{
    uint64_t state = UINT64_C(0x4c58536572766963);
    service_codec codec;
    size_t sample;

    for (codec = SERVICE_OFFER; codec < SERVICE_CODEC_COUNT; ++codec) {
        for (sample = 0U; sample < SERVICE_CODEC_SAMPLES; ++sample) {
            service_record original;
            service_record decoded;
            uint8_t bytes[SERVICE_CODEC_CAPACITY + 1U];
            size_t length = 0U;
            size_t offset;
            unsigned int value;

            generate_record(codec, &state, sample, &original);
            (void)memset(bytes, 0xa5, sizeof(bytes));
            LXP_CASE_CHECK(encode_record(codec, &original, bytes, &length)
                           == LXP_OK);
            LXP_CASE_CHECK(length != 0U && length < sizeof(bytes));
            LXP_CASE_CHECK(decode_record(codec, bytes, length, &decoded)
                           == LXP_OK);
            for (offset = 0U; offset < length; ++offset) {
                uint8_t saved = bytes[offset];
                for (value = 0U; value <= UINT8_MAX; ++value) {
                    if (value == saved) continue;
                    bytes[offset] = (uint8_t)value;
                    if (mutation_is_canonical(codec, bytes, length) != 0) {
                        (void)fprintf(stderr,
                            "%s sample=%zu offset=%zu replacement=%u\n",
                            codec_names[codec], sample, offset, value);
                        return 1;
                    }
                }
                bytes[offset] = saved;
            }
            for (value = 0U; value <= UINT8_MAX; ++value) {
                bytes[length] = (uint8_t)value;
                LXP_CASE_CHECK(decode_record(codec, bytes, length + 1U,
                                              &decoded) != LXP_OK);
            }
        }
        (void)printf("service_codecs_mutation_canonical: %s: %u values, "
                     "all byte replacements and extensions\n",
                     codec_names[codec], (unsigned int)SERVICE_CODEC_SAMPLES);
        (void)fflush(stdout);
    }
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "service_codecs_roundtrip", service_codecs_roundtrip },
        { "service_codecs_mutation_canonical",
          service_codecs_mutation_canonical }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
