#include "layerx/lxp_fee.h"
#include "../codec/lxp_kernel_case.h"

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

enum { FEE_PARAMS_SAMPLES = 2000 };

static const struct {
    uint16_t version;
    size_t length;
} fee_versions[] = {
    {1U, LXP_FEE_PARAMS_V1_BYTES},
    {2U, LXP_FEE_PARAMS_V2_BYTES},
    {3U, LXP_FEE_PARAMS_V3_BYTES},
    {4U, LXP_FEE_PARAMS_V4_BYTES}
};

static uint64_t next_value(uint64_t *state)
{
    *state ^= *state << 13U;
    *state ^= *state >> 7U;
    *state ^= *state << 17U;
    return *state;
}

static lxp_u128 next_price(uint64_t *state, size_t sample)
{
    lxp_u128 price;
    price.hi = next_value(state);
    price.lo = next_value(state);
    if (sample == 0U) return (lxp_u128){0U, 0U};
    if (sample == 1U) return (lxp_u128){UINT64_MAX, UINT64_MAX};
    if (sample == 2U) return (lxp_u128){1U, 0U};
    if (sample == 3U) return (lxp_u128){0U, UINT64_MAX};
    return price;
}

static lxp_fee_params next_parameters(uint64_t *state, uint16_t version,
                                     size_t sample)
{
    lxp_fee_params parameters = {0};
    parameters.version = version;
    parameters.base_fee = next_price(state, sample);
    parameters.per_activity_type_unit = next_price(state, sample);
    parameters.per_encoded_byte = next_price(state, sample);
    parameters.per_execution_unit = next_price(state, sample);
    parameters.per_storage_unit = next_price(state, sample);
    parameters.multiplier_basis_points = (uint32_t)next_value(state);
    if (sample == 0U) parameters.multiplier_basis_points = 0U;
    if (sample == 1U) parameters.multiplier_basis_points = UINT32_MAX;
    if (sample == 2U) parameters.multiplier_basis_points = 1U;
    if (sample == 3U) parameters.multiplier_basis_points = 10000U;
    if (version >= 2U) {
        parameters.asset_price_count = version == 2U ?
            LXP_ASSET_FEE_PRICE_COUNT : LXP_ASSET_FEE_PRICE_COUNT_V3;
        for (size_t i = 0U; i < parameters.asset_price_count; ++i)
            parameters.asset_prices[i] = next_price(state, sample);
        if (version >= 3U)
            parameters.asset_prices[LXP_ASSET_FEE_PRICE_COUNT].hi = 0U;
    }
    if (version == 4U) {
        parameters.module_price_count = LXP_MODULE_FEE_PRICE_COUNT;
        for (size_t i = 0U; i < parameters.module_price_count; ++i)
            parameters.module_prices[i] = next_price(state, sample);
    }
    return parameters;
}

static bool same_price(lxp_u128 left, lxp_u128 right)
{
    return left.hi == right.hi && left.lo == right.lo;
}

static bool same_parameters(const lxp_fee_params *left,
                            const lxp_fee_params *right)
{
    if (left->version != right->version ||
        !same_price(left->base_fee, right->base_fee) ||
        !same_price(left->per_activity_type_unit, right->per_activity_type_unit) ||
        !same_price(left->per_encoded_byte, right->per_encoded_byte) ||
        !same_price(left->per_execution_unit, right->per_execution_unit) ||
        !same_price(left->per_storage_unit, right->per_storage_unit) ||
        left->multiplier_basis_points != right->multiplier_basis_points ||
        left->asset_price_count != right->asset_price_count ||
        left->module_price_count != right->module_price_count)
        return false;
    for (size_t i = 0U; i < LXP_ASSET_FEE_PRICE_MAX; ++i)
        if (!same_price(left->asset_prices[i], right->asset_prices[i]))
            return false;
    for (size_t i = 0U; i < LXP_MODULE_FEE_PRICE_COUNT; ++i)
        if (!same_price(left->module_prices[i], right->module_prices[i]))
            return false;
    return true;
}

static int fee_params_roundtrip_canonical(void)
{
    uint64_t state = UINT64_C(0x4c58504665655061);
    for (size_t v = 0U; v < sizeof(fee_versions) / sizeof(fee_versions[0]); ++v) {
        for (size_t sample = 0U; sample < FEE_PARAMS_SAMPLES; ++sample) {
            lxp_fee_params original = next_parameters(&state,
                fee_versions[v].version, sample);
            lxp_fee_params decoded;
            uint8_t encoded[LXP_FEE_PARAMS_V4_BYTES];
            uint8_t restored[LXP_FEE_PARAMS_V4_BYTES];
            size_t length = 0U, restored_length = 0U;
            (void)memset(&decoded, 0xa5, sizeof(decoded));
            LXP_CASE_CHECK(lxp_fee_params_encode(&original, encoded,
                sizeof(encoded), &length) == LXP_OK);
            LXP_CASE_CHECK(length == fee_versions[v].length);
            LXP_CASE_CHECK(lxp_fee_params_decode(encoded, length, &decoded) == LXP_OK);
            LXP_CASE_CHECK(same_parameters(&original, &decoded));
            LXP_CASE_CHECK(lxp_fee_params_encode(&decoded, restored,
                sizeof(restored), &restored_length) == LXP_OK);
            LXP_CASE_CHECK(restored_length == length);
            LXP_CASE_CHECK(memcmp(encoded, restored, length) == 0);
        }
        (void)printf("fee_params_roundtrip_canonical: version %u, %u schedules\n",
            (unsigned)fee_versions[v].version, (unsigned)FEE_PARAMS_SAMPLES);
    }
    return 0;
}

static int fee_params_mutation_refused_or_identical(void)
{
    uint64_t state = UINT64_C(0x4c58504665655061);
    for (size_t v = 0U; v < sizeof(fee_versions) / sizeof(fee_versions[0]); ++v) {
        size_t accepted = 0U, refused = 0U;
        for (size_t sample = 0U; sample < FEE_PARAMS_SAMPLES; ++sample) {
            lxp_fee_params original = next_parameters(&state,
                fee_versions[v].version, sample);
            uint8_t mutated[LXP_FEE_PARAMS_V4_BYTES];
            uint8_t restored[LXP_FEE_PARAMS_V4_BYTES];
            size_t length = 0U;
            LXP_CASE_CHECK(lxp_fee_params_encode(&original, mutated,
                sizeof(mutated), &length) == LXP_OK);
            LXP_CASE_CHECK(length == fee_versions[v].length);
            for (size_t offset = 0U; offset < length; ++offset) {
                uint8_t saved = mutated[offset];
                for (unsigned value = 0U; value <= UINT8_MAX; ++value) {
                    lxp_fee_params decoded;
                    size_t restored_length = 0U;
                    if (value == saved) continue;
                    mutated[offset] = (uint8_t)value;
                    (void)memset(&decoded, 0xa5, sizeof(decoded));
                    if (lxp_fee_params_decode(mutated, length, &decoded) != LXP_OK) {
                        ++refused;
                        continue;
                    }
                    if (lxp_fee_params_encode(&decoded, restored,
                            sizeof(restored), &restored_length) != LXP_OK ||
                        restored_length != length ||
                        memcmp(mutated, restored, length) != 0 ||
                        same_parameters(&original, &decoded)) {
                        (void)fprintf(stderr,
                            "Noncanonical mutation: version %u, sample %zu, "
                            "offset %zu, byte %u -> %u\n",
                            (unsigned)fee_versions[v].version, sample, offset,
                            (unsigned)saved, value);
                        return 1;
                    }
                    ++accepted;
                }
                mutated[offset] = saved;
            }
        }
        LXP_CASE_CHECK(accepted > 0U && refused > 0U);
        LXP_CASE_CHECK(accepted + refused ==
            (size_t)FEE_PARAMS_SAMPLES * fee_versions[v].length * UINT8_MAX);
        (void)printf("fee_params_mutation_refused_or_identical: version %u, "
            "%zu accepted, %zu refused\n", (unsigned)fee_versions[v].version,
            accepted, refused);
    }
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"fee_params_roundtrip_canonical", fee_params_roundtrip_canonical},
        {"fee_params_mutation_refused_or_identical", fee_params_mutation_refused_or_identical}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
