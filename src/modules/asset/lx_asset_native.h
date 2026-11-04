#ifndef LAYERX_LX_ASSET_NATIVE_H
#define LAYERX_LX_ASSET_NATIVE_H

#include "layerx/lx_asset.h"

typedef struct lx_asset_native_supply_report {
    uint8_t asset_id[32];
    lxp_u128 bank_units;
    lxp_u128 registered_units;
    lxp_u128 issuance_remaining;
    bool issuance_present;
} lx_asset_native_supply_report;

/* Reconcile committed native account balances with registered supply. */
lxp_result lx_asset_native_reconcile(const lx_asset_record *asset,
    const lx_account_registry *accounts, lx_asset_native_supply_report *report);
lxp_result lx_asset_native_supply_check(const lx_asset_registry *assets,
    const lx_account_registry *accounts);

#endif
