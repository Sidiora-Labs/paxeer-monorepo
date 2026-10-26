#include "layerx/lxp_fee.h"
#include "lxp_kernel_case.h"

static int fee_compute_component_overflow_refused(void)
{
    const lxp_fee_params parameters = {
        .version = 1U,
        .per_execution_unit = { 1ULL << 63, 0U },
        .multiplier_basis_points = LXP_BASIS_POINTS_ONE
    };
    const lxp_fee_meter meter = { .execution_units = 2U };
    const lxp_u128 sentinel = { 7U, 9U };
    lxp_u128 fee = sentinel;

    LXP_CASE_CHECK(lxp_fee_compute(&parameters, 0U, meter, &fee) ==
                   LXP_ERR_OVERFLOW);
    LXP_CASE_CHECK(fee.hi == sentinel.hi);
    LXP_CASE_CHECK(fee.lo == sentinel.lo);
    return 0;
}

static int fee_compute_byte_and_storage_overflow_refused(void)
{
    lxp_fee_params parameters = {
        .version = 1U,
        .per_encoded_byte = { 1ULL << 63, 0U },
        .multiplier_basis_points = LXP_BASIS_POINTS_ONE
    };
    lxp_fee_meter meter = { .canonical_encoded_bytes = 2U };
    const lxp_u128 sentinel = { 7U, 9U };
    lxp_u128 fee = sentinel;

    LXP_CASE_CHECK(lxp_fee_compute(&parameters, 0U, meter, &fee) ==
                   LXP_ERR_OVERFLOW);
    LXP_CASE_CHECK(fee.hi == sentinel.hi);
    LXP_CASE_CHECK(fee.lo == sentinel.lo);

    parameters.per_encoded_byte = (lxp_u128){ 0U, 0U };
    parameters.per_storage_unit = (lxp_u128){ 1ULL << 63, 0U };
    meter = (lxp_fee_meter){ .storage_units = 2U };
    fee = sentinel;
    LXP_CASE_CHECK(lxp_fee_compute(&parameters, 0U, meter, &fee) ==
                   LXP_ERR_OVERFLOW);
    LXP_CASE_CHECK(fee.hi == sentinel.hi);
    LXP_CASE_CHECK(fee.lo == sentinel.lo);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "fee_compute_component_overflow_refused",
          fee_compute_component_overflow_refused },
        { "fee_compute_byte_and_storage_overflow_refused",
          fee_compute_byte_and_storage_overflow_refused }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
