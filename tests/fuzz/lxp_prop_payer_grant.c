#include "layerx/lxp_ledger.h"
#include "lxp_kernel_case.h"

#include <stdint.h>
#include <stdio.h>
#include <string.h>

enum { PAYER_GRANT_CASES = 2000, PAYER_GRANT_BYTES = 346 };

static uint64_t next_random(uint64_t *state)
{
    uint64_t value = *state;
    value ^= value << 13U;
    value ^= value >> 7U;
    value ^= value << 17U;
    *state = value;
    return value;
}

static void random_bytes(uint64_t *state, uint8_t *bytes, size_t length)
{
    size_t i;
    for (i = 0U; i < length; ++i)
        bytes[i] = (uint8_t)next_random(state);
}

static uint64_t random_word(uint64_t *state, size_t sample, size_t offset)
{
    static const uint64_t boundary[] = {
        0U, 1U, UINT32_MAX - 1U, UINT32_MAX,
        UINT64_C(0x100000000), UINT64_C(0x8000000000000000),
        UINT64_MAX - 1U, UINT64_MAX
    };
    const size_t count = sizeof(boundary) / sizeof(boundary[0]);
    if (sample < count) return boundary[(sample + offset) % count];
    return next_random(state);
}

static void generate_grant(uint64_t *state, size_t sample,
                           lxp_payer_grant *grant)
{
    (void)memset(grant, 0, sizeof(*grant));
    random_bytes(state, grant->grant_id, sizeof(grant->grant_id));
    random_bytes(state, grant->from, sizeof(grant->from));
    random_bytes(state, grant->recipient, sizeof(grant->recipient));
    random_bytes(state, grant->asset, sizeof(grant->asset));
    grant->per_draw_maximum.lo = random_word(state, sample, 0U);
    grant->per_draw_maximum.hi = random_word(state, sample, 1U);
    grant->allowance.lo = random_word(state, sample, 2U);
    grant->allowance.hi = random_word(state, sample, 3U);
    grant->recurring = (sample & 1U) != 0U;
    grant->window_length = random_word(state, sample, 4U);
    grant->expiration = random_word(state, sample, 5U);
    random_bytes(state, grant->purpose_hash, sizeof(grant->purpose_hash));
    grant->has_reference = (sample & 2U) != 0U;
    random_bytes(state, grant->reference_hash, sizeof(grant->reference_hash));
    grant->revocation_sequence = random_word(state, sample, 6U);
    random_bytes(state, grant->public_key, sizeof(grant->public_key));
    random_bytes(state, grant->signature, sizeof(grant->signature));
}

static int grants_equal(const lxp_payer_grant *left,
                        const lxp_payer_grant *right)
{
    return memcmp(left->grant_id, right->grant_id, sizeof(left->grant_id)) == 0 &&
           memcmp(left->from, right->from, sizeof(left->from)) == 0 &&
           memcmp(left->recipient, right->recipient,
                  sizeof(left->recipient)) == 0 &&
           memcmp(left->asset, right->asset, sizeof(left->asset)) == 0 &&
           left->per_draw_maximum.lo == right->per_draw_maximum.lo &&
           left->per_draw_maximum.hi == right->per_draw_maximum.hi &&
           left->allowance.lo == right->allowance.lo &&
           left->allowance.hi == right->allowance.hi &&
           left->recurring == right->recurring &&
           left->window_length == right->window_length &&
           left->expiration == right->expiration &&
           memcmp(left->purpose_hash, right->purpose_hash,
                  sizeof(left->purpose_hash)) == 0 &&
           left->has_reference == right->has_reference &&
           memcmp(left->reference_hash, right->reference_hash,
                  sizeof(left->reference_hash)) == 0 &&
           left->revocation_sequence == right->revocation_sequence &&
           memcmp(left->public_key, right->public_key,
                  sizeof(left->public_key)) == 0 &&
           memcmp(left->signature, right->signature,
                  sizeof(left->signature)) == 0;
}

static int payer_grant_roundtrip(void)
{
    uint64_t seed = UINT64_C(0x7061796572677261);
    size_t sample;

    for (sample = 0U; sample < PAYER_GRANT_CASES; ++sample) {
        lxp_payer_grant original;
        lxp_payer_grant decoded;
        uint8_t encoded[PAYER_GRANT_BYTES];
        uint8_t reencoded[PAYER_GRANT_BYTES];
        size_t length = 0U;
        size_t reencoded_length = 0U;

        generate_grant(&seed, sample, &original);
        LXP_CASE_CHECK(lxp_payer_grant_encode(&original, encoded,
                       sizeof(encoded), &length) == LXP_OK);
        LXP_CASE_CHECK(length == sizeof(encoded));
        (void)memset(&decoded, 0xa5, sizeof(decoded));
        LXP_CASE_CHECK(lxp_payer_grant_decode(encoded, length,
                                             &decoded) == LXP_OK);
        LXP_CASE_CHECK(grants_equal(&original, &decoded));
        LXP_CASE_CHECK(lxp_payer_grant_encode(&decoded, reencoded,
                       sizeof(reencoded), &reencoded_length) == LXP_OK);
        LXP_CASE_CHECK(reencoded_length == length);
        LXP_CASE_CHECK(memcmp(encoded, reencoded, length) == 0);
    }
    (void)printf("PASS payer_grant_roundtrip (2000 seeded grants)\n");
    return 0;
}

static int payer_grant_mutation_canonical(void)
{
    uint64_t seed = UINT64_C(0x7061796572677261);
    size_t sample;

    for (sample = 0U; sample < PAYER_GRANT_CASES; ++sample) {
        lxp_payer_grant original;
        lxp_payer_grant decoded;
        uint8_t encoded[PAYER_GRANT_BYTES];
        uint8_t mutated[PAYER_GRANT_BYTES + 1U];
        uint8_t reencoded[PAYER_GRANT_BYTES];
        size_t length = 0U;
        size_t position;
        unsigned int value;

        generate_grant(&seed, sample, &original);
        LXP_CASE_CHECK(lxp_payer_grant_encode(&original, encoded,
                       sizeof(encoded), &length) == LXP_OK);
        LXP_CASE_CHECK(length == sizeof(encoded));
        (void)memcpy(mutated, encoded, length);
        for (position = 0U; position < length; ++position) {
            for (value = 0U; value <= UINT8_MAX; ++value) {
                size_t reencoded_length = 0U;
                if ((uint8_t)value == encoded[position]) continue;
                mutated[position] = (uint8_t)value;
                if (lxp_payer_grant_decode(mutated, length, &decoded) != LXP_OK)
                    continue;
                if (lxp_payer_grant_encode(&decoded, reencoded,
                        sizeof(reencoded), &reencoded_length) != LXP_OK ||
                    reencoded_length != length ||
                    memcmp(mutated, reencoded, length) != 0) {
                    (void)fprintf(stderr,
                        "noncanonical mutation: sample=%zu byte=%zu value=%u\n",
                        sample, position, value);
                    return 1;
                }
            }
            mutated[position] = encoded[position];
        }
        for (value = 0U; value <= UINT8_MAX; ++value) {
            mutated[length] = (uint8_t)value;
            if (lxp_payer_grant_decode(mutated, length + 1U,
                                      &decoded) == LXP_OK) {
                (void)fprintf(stderr,
                    "accepted extension: sample=%zu value=%u\n", sample, value);
                return 1;
            }
        }
    }
    (void)printf("PASS payer_grant_mutation_canonical "
                 "(2000 grants, all byte mutations and extensions)\n");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "payer_grant_roundtrip", payer_grant_roundtrip },
        { "payer_grant_mutation_canonical", payer_grant_mutation_canonical }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
