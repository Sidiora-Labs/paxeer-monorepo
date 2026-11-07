#ifndef LAYERX_LXP_TEST_GENESIS_STORAGE_RETIREMENT_H
#define LAYERX_LXP_TEST_GENESIS_STORAGE_RETIREMENT_H

#include "layerx/lxp_genesis.h"
#include "layerx/programs.h"
#include "lxp_test_harness.h"

static int check_storage_retirement_activity_types(const lxp_module_iface *iface)
{
    static const uint32_t expected_types[] = {
        LX_PROGRAMS_DEPLOY, LX_PROGRAMS_UPGRADE, LX_PROGRAMS_CALL,
        LX_PROGRAMS_REGISTRY, LX_PROGRAMS_TRANSFER, LX_PROGRAMS_ACCOUNT,
        LX_PROGRAMS_WIND_DOWN, LX_PROGRAMS_SANDBOX,
        LX_PROGRAMS_SANDBOX_DESTROY, LX_PROGRAMS_STORAGE_RETIREMENT
    };
    if (LXP_ASSERT_U64(1U, iface != NULL) != 0) return 1;
    if (LXP_ASSERT_U64(LXP_MODULE_PROGRAMS, iface->module_id) != 0) return 1;
    if (LXP_ASSERT_U64(LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION,
                       iface->abi_version) != 0) return 1;
    if (LXP_ASSERT_U64(1U, iface->activity_types != NULL) != 0) return 1;
    if (LXP_ASSERT_U64(sizeof(expected_types) / sizeof(expected_types[0]),
                       iface->activity_type_count) != 0) return 1;
    return LXP_ASSERT_BYTES(expected_types, iface->activity_types,
                            sizeof(expected_types));
}

#endif
