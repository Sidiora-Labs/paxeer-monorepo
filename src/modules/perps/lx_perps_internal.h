#ifndef LX_PERPS_INTERNAL_H
#define LX_PERPS_INTERNAL_H

#include "layerx/lx_perps.h"

lxp_result lx_perps_position_value(const lx_perps_position *position,
    lxp_u128 price, lxp_u128 scale, lxp_i128 *pnl);
lxp_result lx_perps_equity_check(const lx_perps_market *market,
    const lx_perps_position *position, lxp_u128 price, lxp_u128 scale,
    lxp_u128 cash, lxp_i128 funding_index, lxp_i128 *pnl,
    lxp_i128 *owed, lxp_i128 *equity, bool *liquidatable);
lxp_result lx_perps_liquidation_settle(lxp_module_ctx *ctx,
    const lx_perps_liquidation_request *request, lxp_i128 funding_index,
    lxp_u128 *covered, lxp_receipt *receipt);
lxp_result lx_perps_margin_encumbered(lxp_module_ctx *ctx,
    const uint8_t margin_id[32], const uint8_t *closing_market_id,
    bool *encumbered);

#endif
