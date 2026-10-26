#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "lxp_kernel_case.h"

static int custody_asset_id_matches_table(void)
{
    static const char *const ids[] = {
        "layerx-asset:125:PAX", "layerx-asset:125:SID",
        "layerx-asset:125:USDC", "layerx-asset:125:USDL"
    };
    size_t i;

    LXP_CASE_CHECK(LXP_GENESIS_CUSTODY_ASSET_COUNT == 4);
    LXP_CASE_CHECK(sizeof(ids) / sizeof(ids[0]) ==
                   LXP_GENESIS_CUSTODY_ASSET_COUNT);
    for (i = 0U; i < LXP_GENESIS_CUSTODY_ASSET_COUNT; ++i) {
        uint8_t expected[32];
        uint8_t actual[32];

        LXP_CASE_CHECK(lxp_hash_sha256(ids[i], strlen(ids[i]), expected) ==
                       LXP_OK);
        LXP_CASE_CHECK(lxp_genesis_custody_asset_id(i, actual) == LXP_OK);
        LXP_CASE_CHECK(memcmp(actual, expected, sizeof(actual)) == 0);
    }
    return 0;
}

static int custody_asset_id_bounds(void)
{
    uint8_t asset_id[32];
    uint8_t original[32];
    size_t i;

    memset(asset_id, 0xa5, sizeof(asset_id));
    memcpy(original, asset_id, sizeof(original));
    LXP_CASE_CHECK(lxp_genesis_custody_asset_id(4U, asset_id) ==
                   LXP_ERR_PARAMETER_BOUNDS);
    LXP_CASE_CHECK(memcmp(asset_id, original, sizeof(asset_id)) == 0);
    LXP_CASE_CHECK(lxp_genesis_custody_asset_id(SIZE_MAX, asset_id) ==
                   LXP_ERR_PARAMETER_BOUNDS);
    LXP_CASE_CHECK(memcmp(asset_id, original, sizeof(asset_id)) == 0);
    for (i = 0U; i < LXP_GENESIS_CUSTODY_ASSET_COUNT; ++i)
        LXP_CASE_CHECK(lxp_genesis_custody_asset_id(i, NULL) ==
                       LXP_ERR_NON_CANONICAL);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "custody_asset_id_matches_table", custody_asset_id_matches_table },
        { "custody_asset_id_bounds", custody_asset_id_bounds }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
